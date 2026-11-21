// Command pql measures agent-facing command-line tools.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const (
	exitUsage    = 64
	exitSoftware = 70
)

type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

var errNotImplemented = errors.New("judge: not implemented")

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root := &cobra.Command{
		Use: "pql", Short: "Measure agent-facing command-line tools",
		SilenceUsage: true, SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &usageError{fmt.Errorf("unknown command %q", args[0])}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error { return &usageError{errors.New("a subcommand is required")} },
	}
	root.SetIn(stdin)
	root.SetOut(stderr)
	root.SetErr(stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err} })
	root.AddCommand(&cobra.Command{
		Use: "judge", Short: "Judge an evidence record",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &usageError{errors.New("judge accepts no positional arguments")}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error { return errNotImplemented },
	})
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	code := exitSoftware
	var usage *usageError
	if errors.As(err, &usage) {
		code = exitUsage
	} else if errors.Is(err, errNotImplemented) {
		code = exitSoftware
	}
	if _, writeErr := fmt.Fprintf(stderr, "pql: %s\n", err); writeErr != nil {
		return exitSoftware
	}
	if code == exitUsage {
		if _, writeErr := fmt.Fprintln(stderr, "Run 'pql --help' for usage."); writeErr != nil {
			return exitSoftware
		}
	}
	return code
}
