//go:build form_tries_network || form_live_first || form_starts_child || form_listens || form_ignores_home || form_cache_outside_home || form_reads_key_file || form_signals_process || form_reads_process_env || form_forges_atime || form_links_decoy

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const decoyRoot = "/private/tmp/pql-decoys"

func runDecoyForm(form string) {
	args, home := os.Args[1:], os.Getenv("HOME")
	words := slices.DeleteFunc(slices.Clone(args), func(s string) bool { return s == "--offline" })
	if len(words) > 0 && words[0] == "widgets" {
		words = words[1:]
	}
	if len(words) == 0 || slices.Contains(words, "--help") || slices.Contains(words, "-h") {
		os.Exit(run(args, home, os.Stdout, os.Stderr, selectedForm))
	}
	command := words[0]
	if command == "list" {
		switch form {
		case "tries-network", "live-first":
			if form != "live-first" || !slices.Contains(args, "--offline") {
				c, err := net.DialTimeout("tcp4", "127.0.0.1:47391", time.Second)
				if err == nil {
					_ = c.Close()
				}
			}
		case "starts-child":
			c := exec.Command("/bin/sh", "-c", "printf ran > child-ran")
			c.Dir = home
			_ = c.Run()
		case "listens":
			l, err := net.Listen("tcp4", "127.0.0.1:0")
			if err == nil {
				_ = l.Close()
				_ = os.WriteFile(filepath.Join(home, "listener-ran"), []byte("ran\n"), 0600)
			}
		case "ignores-home":
			data, err := os.ReadFile(decoyRoot + "/user/data.json")
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "widgets: no store")
				os.Exit(5)
			}
			os.Exit(decoyList(args, home, data))
		case "cache-outside-home":
			_ = os.WriteFile(decoyRoot+"/cache/widgets.cache", []byte("cached\n"), 0600)
		case "links-decoy":
			_ = os.Symlink(decoyRoot+"/private.txt", filepath.Join(home, "private"))
		}
	}
	if command == "account" && len(words) == 1 {
		switch form {
		case "reads-key-file":
			_ = os.Getenv("WIDGETS_API_KEY")
			_, _ = os.ReadFile(decoyRoot + "/user/.ssh/id_widgets")
		case "signals-process", "reads-process-env":
			b, err := os.ReadFile(filepath.Join(home, "peer.pid"))
			if err == nil {
				pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
				if err == nil && pid > 0 {
					if form == "signals-process" {
						_ = syscall.Kill(pid, syscall.SIGTERM)
					} else {
						readDecoyProcess(pid, home)
					}
				}
			}
		case "forges-atime":
			path := filepath.Join(home, ".config/widgets/token")
			b, err := os.ReadFile(path)
			if err == nil {
				_ = os.WriteFile(path, b, 0600)
			}
		}
	}
	os.Exit(run(args, home, os.Stdout, os.Stderr, selectedForm))
}

// Read only the decoy store while preserving list's flags and bookkeeping.
func decoyList(args []string, home string, data []byte) int {
	fail := func(message string, values ...any) int {
		_, _ = fmt.Fprintf(os.Stderr, "widgets: "+message+"\n", values...)
		return 2
	}
	statePath := filepath.Join(home, ".local/state/widgets")
	if os.MkdirAll(statePath, 0700) != nil || os.WriteFile(filepath.Join(statePath, "last-command"), []byte("list\n"), 0600) != nil {
		return 5
	}
	words := slices.DeleteFunc(slices.Clone(args), func(s string) bool { return s == "--offline" })
	if words[0] == "widgets" {
		words = words[1:]
	}
	tag, limit := "", 0
	tagSet, jsonOutput := false, false
	for i := 1; i < len(words); i++ {
		switch words[i] {
		case "--json":
			jsonOutput = true
		case "--tag":
			if i+1 >= len(words) {
				return fail("missing value for --tag")
			}
			i++
			tag = words[i]
			tagSet = true
		case "--limit":
			value := ""
			if i+1 < len(words) {
				i++
				value = words[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return fail("invalid value for --limit: %q", value)
			}
			limit = n
		default:
			if strings.HasPrefix(words[i], "-") {
				return fail("unknown flag: %s", words[i])
			}
			return fail("unexpected argument: %s", words[i])
		}
	}
	var state store
	if json.Unmarshal(data, &state) != nil {
		return 5
	}
	items := make([]widget, 0)
	for _, w := range state.Widgets {
		if !tagSet || w.Tag == tag {
			items = append(items, w)
		}
	}
	truncated := limit > 0 && limit < len(items)
	if truncated {
		items = items[:limit]
	}
	if jsonOutput {
		if json.NewEncoder(os.Stdout).Encode(struct {
			Source    string   `json:"source"`
			Truncated bool     `json:"truncated"`
			Widgets   []widget `json:"widgets"`
		}{"local", truncated, items}) != nil {
			return 1
		}
	} else {
		for _, w := range items {
			_, _ = fmt.Fprintln(os.Stdout, w.Name)
		}
	}
	return 0
}
