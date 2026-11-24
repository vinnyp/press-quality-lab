// widgets is the synthetic collection subject. A build tag selects its form.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type widget struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}
type store struct {
	Widgets []widget `json:"widgets"`
}

const rootHelp = `widgets keeps a local list of widgets.

Usage:
  widgets [command]

Available Commands:
  account     Show the account the token belongs to
  add         Add a widget
  count       Count widgets
  list        List widgets
  remove      Remove a widget (destructive)

Flags:
  -h, --help   help for widgets
`

func main() { os.Exit(run(os.Args[1:], os.Getenv("HOME"), os.Stdout, os.Stderr, selectedForm)) }

type behaviour struct{ source string }

func run(args []string, home string, out, errout io.Writer, form behaviour) int {
	fail := func(code int, message string, values ...any) int {
		_, _ = fmt.Fprintf(errout, "widgets: "+message+"\n", values...)
		return code
	}
	words := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--offline" {
			words = append(words, arg)
		}
	}
	if len(words) > 0 && words[0] == "widgets" {
		words = words[1:]
	}
	if len(words) == 0 {
		_, _ = io.WriteString(out, rootHelp)
		return 0
	}
	command := words[0]
	if command == "--help" || command == "-h" {
		_, _ = io.WriteString(out, rootHelp)
		return 0
	}
	if !strings.Contains("|list|count|add|remove|account|", "|"+command+"|") {
		if strings.HasPrefix(command, "-") {
			return fail(2, "unknown flag: %s", command)
		}
		return fail(2, "unknown command: %s", command)
	}
	for _, word := range words[1:] {
		if word == "--help" || word == "-h" {
			_, _ = fmt.Fprintf(out, "Usage:\n  widgets %s\n\nFlags:\n  -h, --help   help for %s\n", command, command)
			return 0
		}
	}
	if command == "list" {
		state := filepath.Join(home, ".local/state/widgets")
		if err := os.MkdirAll(state, 0700); err != nil {
			return fail(5, "cannot write state")
		}
		if err := os.WriteFile(filepath.Join(state, "last-command"), []byte("list\n"), 0600); err != nil {
			return fail(5, "cannot write state")
		}
	}
	jsonOutput, confirm, tagSet := false, false, false
	tag, name, limit := "none", "", 0
	for i := 1; i < len(words); i++ {
		word := words[i]
		switch {
		case word == "--json" && command == "list":
			jsonOutput = true
		case word == "--confirm" && command == "remove":
			confirm = true
		case word == "--tag" && (command == "list" || command == "count" || command == "add"):
			if i+1 >= len(words) {
				return fail(2, "missing value for --tag")
			}
			i++
			tag = words[i]
			tagSet = true
		case word == "--limit" && command == "list":
			value := ""
			if i+1 < len(words) {
				i++
				value = words[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return fail(2, "invalid value for --limit: %q", value)
			}
			limit = n
		case strings.HasPrefix(word, "-"):
			return fail(2, "unknown flag: %s", word)
		case (command == "add" || command == "remove") && name == "":
			name = word
		default:
			return fail(2, "unexpected argument: %s", word)
		}
	}
	if command == "add" && name == "" {
		return fail(2, "add needs a name")
	}
	if command == "remove" && !confirm {
		return fail(3, "refusing to remove %q without --confirm", name)
	}
	emit := func(value any) int {
		if json.NewEncoder(out).Encode(value) != nil {
			return 1
		}
		return 0
	}
	if command == "account" {
		token, err := os.ReadFile(filepath.Join(home, ".config/widgets/token"))
		if err != nil {
			return fail(4, "no token")
		}
		return emit(struct {
			Account string `json:"account"`
			Bytes   int    `json:"token_bytes"`
		}{"ok", len(token)})
	}
	path := filepath.Join(home, "data.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(5, "no store")
	}
	var s store
	if json.Unmarshal(data, &s) != nil {
		return fail(5, "invalid store")
	}
	switch command {
	case "list", "count":
		items := make([]widget, 0, len(s.Widgets))
		for _, w := range s.Widgets {
			if !tagSet || w.Tag == tag {
				items = append(items, w)
			}
		}
		if command == "count" {
			return emit(struct {
				Count int `json:"count"`
			}{len(items)})
		}
		truncated := limit > 0 && limit < len(items)
		if truncated {
			items = items[:limit]
		}
		if jsonOutput {
			return emit(struct {
				Source    string   `json:"source"`
				Truncated bool     `json:"truncated"`
				Widgets   []widget `json:"widgets"`
			}{form.source, truncated, items})
		}
		for _, w := range items {
			_, _ = fmt.Fprintln(out, w.Name)
		}
		return 0
	case "add":
		for _, w := range s.Widgets {
			if w.Name == name {
				return emit(struct {
					Added bool   `json:"added"`
					Name  string `json:"name"`
				}{false, name})
			}
		}
		s.Widgets = append(s.Widgets, widget{name, tag})
	case "remove":
		items := make([]widget, 0, len(s.Widgets))
		for _, w := range s.Widgets {
			if w.Name != name {
				items = append(items, w)
			}
		}
		s.Widgets = items
	}
	var b strings.Builder
	b.WriteString("{\"widgets\":[\n")
	for i, w := range s.Widgets {
		if i > 0 {
			b.WriteString(",\n")
		}
		encoded, _ := json.Marshal(w)
		b.Write(encoded)
	}
	if len(s.Widgets) > 0 {
		b.WriteByte('\n')
	}
	b.WriteString("]}\n")
	if os.WriteFile(path, []byte(b.String()), 0600) != nil {
		return fail(5, "cannot write store")
	}
	if command == "add" {
		return emit(struct {
			Added bool   `json:"added"`
			Name  string `json:"name"`
		}{true, name})
	}
	return emit(struct {
		Removed bool `json:"removed"`
	}{true})
}
