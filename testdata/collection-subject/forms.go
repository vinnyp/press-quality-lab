//go:build !form_known_good

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// Keep the known-good compiled source set unchanged. Forms intercept only their
// named operation and delegate all other operations to the original runner.
func runSelectedForm(form string) {
	os.Exit(runForm(form, os.Args[1:], os.Getenv("HOME"), os.Stdout, os.Stderr))
}

func runForm(form string, args []string, home string, out, errout io.Writer) int {
	words := make([]string, 0, len(args))
	for _, word := range args {
		if word != "--offline" {
			words = append(words, word)
		}
	}
	if len(words) > 0 && words[0] == "widgets" {
		words = words[1:]
	}
	root := len(words) == 0 || words[0] == "--help" || words[0] == "-h"
	if root {
		_, _ = io.WriteString(out, formHelp(form))
		if form == "help-fails" && len(words) > 0 && words[0] == "--help" {
			return 1
		}
		return 0
	}
	command := words[0]
	help := slices.Contains(words[1:], "--help") || slices.Contains(words[1:], "-h")
	if (form == "help-deep" || form == "help-cycle") && help && (command == "a" || command == "x" || command == "y") {
		child := "a"
		if form == "help-cycle" {
			child = "x"
			if words[len(words)-2] == "x" {
				child = "y"
			}
		}
		_, _ = fmt.Fprintf(out, "Usage:\n  widgets [command]\n\nAvailable Commands:\n  %s   Descend\n\nFlags:\n  --help   Show help\n", child)
		return 0
	}
	if form == "renames-command" {
		if command == "count" {
			_, _ = io.WriteString(errout, "widgets: unknown command: count\n")
			return 2
		}
		if command == "tally" {
			words[0] = "count"
			args = words
			command = "count"
		}
	}
	if form == "adds-command" && command == "export" {
		words[0] = "list"
		if !help {
			words = append(words, "--json")
		}
		args = words
		command = "list"
	}
	if form == "ignores-help" && command == "add" && len(words) == 2 && words[1] == "--help" {
		path := filepath.Join(home, "data.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if os.WriteFile(path, []byte("{\"widgets\":[]}\n"), 0600) != nil {
				return 5
			}
		}
		return run([]string{"add", "help"}, home, out, errout, selectedForm)
	}
	if help {
		return run(args, home, out, errout, selectedForm)
	}
	if command == "account" && len(words) != 1 {
		return run(args, home, out, errout, selectedForm)
	}
	if command == "account" && (form == "never-reads-secret" || form == "opens-secret" || strings.HasPrefix(form, "leak-")) {
		path := filepath.Join(home, ".config/widgets/token")
		if form == "opens-secret" {
			f, err := os.Open(path)
			if err != nil {
				_, _ = io.WriteString(errout, "widgets: no token\n")
				return 4
			}
			_ = f.Close()
		}
		if form == "never-reads-secret" || form == "opens-secret" {
			_, _ = io.WriteString(out, "{\"account\":\"ok\",\"token_bytes\":19}\n")
			return 0
		}
		token, err := os.ReadFile(path)
		if err != nil {
			_, _ = io.WriteString(errout, "widgets: no token\n")
			return 4
		}
		text := strings.TrimSuffix(string(token), "\n")
		if form == "leak-file" || form == "leak-past-bound" {
			cache := filepath.Join(home, ".cache/widgets")
			if os.MkdirAll(cache, 0700) != nil {
				return 5
			}
			name, content := "debug.log", token
			if form == "leak-past-bound" {
				name = "big.log"
				content = append(bytes.Repeat([]byte("x"), 16<<20), []byte(text+"\n")...)
			}
			if os.WriteFile(filepath.Join(cache, name), content, 0600) != nil {
				return 5
			}
			_, _ = fmt.Fprintf(out, "{\"account\":\"ok\",\"token_bytes\":%d}\n", len(token))
			return 0
		}
		value := struct {
			Account string `json:"account"`
			Debug   string `json:"debug"`
			Bytes   int    `json:"token_bytes"`
		}{"ok", encodeLeak(strings.TrimPrefix(form, "leak-"), []byte(text)), len(token)}
		if json.NewEncoder(out).Encode(value) != nil {
			return 1
		}
		return 0
	}
	if command == "count" && (form == "big-number" || form == "crashes" || form == "kills-self" || form == "overruns") {
		var checked bytes.Buffer
		if code := run(args, home, &checked, errout, selectedForm); code != 0 {
			_, _ = out.Write(checked.Bytes())
			return code
		}
		switch form {
		case "big-number":
			_, _ = io.WriteString(out, "{\"count\":123456789012345678901234567890123}\n")
			return 0
		case "crashes":
			// Crash mode still ends by SIGABRT. Closing the diagnostic descriptor
			// keeps randomized runtime stacks and source paths out of recordings.
			_ = os.Stderr.Close()
			debug.SetTraceback("crash")
			panic("synthetic count crash")
		case "kills-self":
			_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
			return 1
		case "overruns":
			for {
			}
		}
	}
	if command != "list" && !(command == "count" && form == "ignores-store") {
		return run(args, home, out, errout, selectedForm)
	}
	var captured bytes.Buffer
	code := run(args, home, &captured, errout, selectedForm)
	if code != 0 {
		_, _ = out.Write(captured.Bytes())
		return code
	}
	if form == "ignores-store" && command == "count" {
		_, _ = io.WriteString(out, "{\"count\":0}\n")
		return 0
	}
	jsonOutput := slices.Contains(words, "--json")
	switch form {
	case "drops-last":
		var state store
		data, err := os.ReadFile(filepath.Join(home, "data.json"))
		if err != nil || json.Unmarshal(data, &state) != nil {
			return 5
		}
		tag, tagSet, limit := "", false, 0
		for i := 1; i < len(words); i++ {
			switch words[i] {
			case "--tag":
				i++
				tag = words[i]
				tagSet = true
			case "--limit":
				i++
				limit, _ = strconv.Atoi(words[i])
			}
		}
		items := make([]widget, 0, len(state.Widgets))
		matched := 0
		for i, w := range state.Widgets {
			if tagSet && w.Tag != tag {
				continue
			}
			if limit > 0 && matched >= limit {
				break
			}
			matched++
			if i != len(state.Widgets)-1 {
				items = append(items, w)
			}
		}
		if jsonOutput {
			var result struct {
				Source    string   `json:"source"`
				Truncated bool     `json:"truncated"`
				Widgets   []widget `json:"widgets"`
			}
			if json.Unmarshal(captured.Bytes(), &result) != nil {
				return 1
			}
			result.Widgets = items
			if json.NewEncoder(out).Encode(result) != nil {
				return 1
			}
		} else {
			for _, w := range items {
				_, _ = fmt.Fprintln(out, w.Name)
			}
		}
		return 0
	case "ignores-store":
		if jsonOutput {
			_, _ = io.WriteString(out, "{\"source\":\"local\",\"truncated\":false,\"widgets\":[]}\n")
		}
		return 0
	case "writes-store":
		path := filepath.Join(home, "data.json")
		b, err := os.ReadFile(path)
		var s store
		if err != nil || json.Unmarshal(b, &s) != nil {
			return 5
		}
		slices.Reverse(s.Widgets)
		var bld strings.Builder
		bld.WriteString("{\"widgets\":[\n")
		for i, w := range s.Widgets {
			if i > 0 {
				bld.WriteString(",\n")
			}
			encoded, _ := json.Marshal(w)
			bld.Write(encoded)
		}
		if len(s.Widgets) > 0 {
			bld.WriteByte('\n')
		}
		bld.WriteString("]}\n")
		if os.WriteFile(path, []byte(bld.String()), 0600) != nil {
			return 5
		}
	case "creates-file", "writes-wal", "writes-empty-wal", "writes-shm":
		name, content := "notes.txt", []byte("seen\n")
		switch form {
		case "writes-wal":
			name = "data.json-wal"
			content = []byte("12345678")
		case "writes-empty-wal":
			name = "data.json-wal"
			content = []byte{}
		case "writes-shm":
			name = "data.json-shm"
			content = make([]byte, 32768)
		}
		if os.WriteFile(filepath.Join(home, name), content, 0600) != nil {
			return 5
		}
	case "creates-link":
		if os.Symlink("data.json", filepath.Join(home, "latest")) != nil {
			if _, err := os.Readlink(filepath.Join(home, "latest")); err != nil {
				return 5
			}
		}
	case "huge-output":
		if !jsonOutput {
			line := strings.Repeat("w", 63) + "\n"
			for remaining := 17 << 20; remaining > 0; {
				n := min(remaining, len(line))
				if _, err := io.WriteString(out, line[:n]); err != nil {
					return 1
				}
				remaining -= n
			}
			return 0
		}
	case "invalid-utf8":
		if !jsonOutput {
			_, _ = out.Write(bytes.ReplaceAll(captured.Bytes(), []byte("\n"), []byte{0xff, '\n'}))
			return 0
		}
	}
	_, _ = out.Write(captured.Bytes())
	return 0
}

func encodeLeak(name string, b []byte) string {
	switch name {
	case "plain":
		return string(b)
	case "base64":
		return base64.StdEncoding.EncodeToString(b)
	case "base64-nopad":
		return base64.RawStdEncoding.EncodeToString(b)
	case "base64url":
		return base64.URLEncoding.EncodeToString(b)
	case "base64url-nopad":
		return base64.RawURLEncoding.EncodeToString(b)
	case "hex-lower":
		return hex.EncodeToString(b)
	case "hex-upper":
		return strings.ToUpper(hex.EncodeToString(b))
	case "percent-upper", "percent-lower":
		var out strings.Builder
		for _, c := range b {
			if name == "percent-upper" {
				_, _ = fmt.Fprintf(&out, "%%%02X", c)
			} else {
				_, _ = fmt.Fprintf(&out, "%%%02x", c)
			}
		}
		return out.String()
	}
	return ""
}

func formHelp(form string) string {
	switch form {
	case "hides-command":
		return strings.Replace(rootHelp, "  count       Count widgets\n", "", 1)
	case "renames-command":
		return strings.Replace(rootHelp, "count       Count", "tally       Count", 1)
	case "adds-command":
		return strings.Replace(rootHelp, "  list        List", "  export      Export widgets\n  list        List", 1)
	case "help-urfave":
		return "NAME:\n   widgets - keeps widgets\n\nUSAGE:\n   widgets command\n\nCOMMANDS:\n   account, a   Show the account the token belongs to\n   add   Add a widget\n   count   Count widgets\n   list   List widgets\n   remove   Remove a widget (destructive)\n\nGLOBAL OPTIONS:\n   --help   Show help\n"
	case "help-clap":
		return "Usage: widgets <COMMAND>\n\nCommands:\n  account   Show the account the token belongs to\n  add   Add a widget\n  count   Count widgets\n  list   List widgets\n  remove   Remove a widget (destructive)\n  help   Print help\n\nOptions:\n  --help   Print help\n"
	case "help-cobra-group":
		return "Usage:\n  widgets [command]\n\nWidget Commands:\n  add   Add a widget\n  count   Count widgets\n  list   List widgets\n  remove   Remove a widget (destructive)\n\nAccount Commands:\n  account   Show the account the token belongs to\n\nFlags:\n  --help   Show help\n"
	case "help-manpage":
		return "NAME\n    widgets - keeps widgets\n\nSYNOPSIS\n    widgets command\n\nCOMMANDS\n    account   Show the account the token belongs to\n    add   Add a widget\n    count   Count widgets\n    list   List widgets\n    remove   Remove a widget (destructive)\n\nOPTIONS\n    --help   Show help\n"
	case "help-no-section":
		return "Usage:\n  widgets [command]\n\nFlags:\n  -h, --help   help for widgets\n"
	case "help-deep", "help-cycle":
		child := "a"
		if form == "help-cycle" {
			child = "x"
		}
		return "Usage:\n  widgets [command]\n\nAvailable Commands:\n  " + child + "   Descend\n\nFlags:\n  --help   Show help\n"
	}
	return rootHelp
}
