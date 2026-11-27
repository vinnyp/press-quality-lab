package observe

import "strings"

// HelpCommands reads the retained stream using credit-and-routes §5 and E-27.
// The boolean records a recognized section, including an empty one.
func HelpCommands(run Run) ([]string, bool) {
	output := run.Stdout
	if run.StdoutSeen == 0 {
		output = run.Stderr
	}
	if HelpCut(run) {
		return nil, false
	}
	var commands []string
	seen := map[string]bool{}
	section, heading := false, false
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" || line[0] != ' ' {
			section = line == "Available Commands:" || line == "Additional Commands:" || strings.HasSuffix(line, " Commands:") || line == "COMMANDS:" || line == "Commands:" || line == "SUBCOMMANDS:"
			heading = heading || section
			continue
		}
		if !section {
			continue
		}
		text := strings.TrimLeft(line, " ")
		end := strings.IndexAny(text, " \t")
		if end < 0 {
			end = len(text)
		}
		name := strings.TrimSuffix(text[:end], ",")
		description := ""
		for i := end; i < len(text); {
			if text[i] != ' ' && text[i] != '\t' {
				i++
				continue
			}
			start := i
			tab := false
			for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
				tab = tab || text[i] == '\t'
				i++
			}
			if tab || i-start >= 2 {
				description = text[i:]
				break
			}
		}
		hidden := strings.HasPrefix(description, "(hidden)") || strings.HasPrefix(description, "[hidden]") || strings.HasPrefix(description, "HIDDEN")
		if name != "" && name != "help" && name != "completion" && !hidden && !seen[name] {
			seen[name] = true
			commands = append(commands, strings.Clone(name))
		}
	}
	return commands, heading
}

// HelpCut reports E-32's unreadable selected stream, chosen by written bytes.
func HelpCut(run Run) bool {
	if run.StdoutSeen == 0 {
		return run.StderrCut
	}
	return run.StdoutCut
}
