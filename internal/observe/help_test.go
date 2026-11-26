package observe

import (
	"reflect"
	"testing"
)

func TestHelpReader(t *testing.T) {
	// QD2: credit-and-routes §5, E-26 and E-27 define headings, sections and descriptions.
	for _, heading := range []string{"Available Commands:", "Additional Commands:", "Group Commands:", "COMMANDS:", "Commands:", "SUBCOMMANDS:"} {
		t.Run(headingName(heading), func(t *testing.T) {
			got, section := HelpCommands(Run{Stdout: []byte(heading + "\n  one, o  description\n  help  help\n  completion  complete\n")})
			if !section || !reflect.DeepEqual(got, []string{"one"}) {
				t.Fatal(got, section)
			}
		})
	}
	for _, row := range []struct {
		name, text string
		want       []string
		heading    bool
	}{
		{"SeveralSections", "Commands:\n  a  desc\n\n  ignored  desc\nCOMMANDS:\n  b  desc\nend\n  ignored  desc\nSUBCOMMANDS:\n  c  desc\n\tstop\n  ignored  desc", []string{"a", "b", "c"}, true},
		{"IndentedHeading", " Commands:\n  a  desc", nil, false},
		{"Leaf", "Usage: leaf\nplain text", nil, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, h := HelpCommands(Run{Stdout: []byte(row.text)})
			if !reflect.DeepEqual(got, row.want) || h != row.heading {
				t.Fatal(got, h)
			}
		})
	}
	for _, marker := range []string{"(hidden)", "[hidden]", "HIDDEN"} {
		for _, row := range []struct {
			name, line string
			hidden     bool
		}{
			{"Direct", "  cmd  " + marker, true}, {"Alias", "  cmd, c  " + marker, true}, {"Tab", "  cmd\t" + marker, true}, {"AliasTab", "  cmd, c\t" + marker, true}, {"SingleSpace", "  cmd " + marker, false}, {"Later", "  cmd  shown " + marker, false},
		} {
			t.Run(headingName(marker)+row.name, func(t *testing.T) {
				got, _ := HelpCommands(Run{Stdout: []byte("Commands:\n" + row.line)})
				want := []string{"cmd"}
				if row.hidden {
					want = nil
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal(got, want)
				}
			})
		}
	}
	t.Run("Streams", func(t *testing.T) {
		for _, stdout := range []string{"", "Commands:\n  out  desc"} {
			got, _ := HelpCommands(Run{Stdout: []byte(stdout), Stderr: []byte("Commands:\n  err  desc")})
			want := "out"
			if stdout == "" {
				want = "err"
			}
			if !reflect.DeepEqual(got, []string{want}) {
				t.Fatal(got)
			}
		}
	})
}
func headingName(s string) string {
	out := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			out += string(c)
		}
	}
	return out
}
