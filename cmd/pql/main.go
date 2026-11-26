// Command pql measures agent-facing command-line tools.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/subject"
	"io"
	"os"

	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/judge"

	"github.com/spf13/cobra"
)

const (
	exitUsage    = 64
	exitSoftware = 70
)

type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

var judging = judge.Judge

func main() { os.Exit(entry()) }

// stdout carries help, except under pql judge, whose stdout is reserved for the judge's document.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	resultCode := 0
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
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err} })
	judgeCmd := &cobra.Command{
		Use: "judge", Short: "Judge an evidence record",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &usageError{errors.New("judge accepts no positional arguments")}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			input, err := io.ReadAll(io.LimitReader(stdin, judge.MaxInput+1))
			if err != nil {
				return err
			}
			output, code, defect := judge.Safe(judging, input)
			if defect != nil {
				return defect
			}
			n, err := stdout.Write(output)
			if err != nil {
				return err
			}
			if n != len(output) {
				return io.ErrShortWrite
			}
			resultCode = code
			return nil
		},
	}
	judgeCmd.SetOut(stderr)
	createCmd := &cobra.Command{
		Use: "create <binary>", Short: "Write a subject file with placeholders",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{errors.New("create requires one binary path")}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			encoder := json.NewEncoder(stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(subject.Create(args[0]))
		},
	}
	root.AddCommand(createCmd, judgeCmd, newCollectionCommand("grade"), newCollectionCommand("anchor"))
	root.SetHelpCommand(&cobra.Command{
		Use: "help [command]", Short: "Help about any command",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{errors.New("help accepts at most one argument")}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			for _, command := range root.Commands() {
				if command.Name() == args[0] {
					command.SetOut(stdout)
					return command.Help()
				}
			}
			return &usageError{fmt.Errorf("unknown help topic %q", args[0])}
		},
	})
	// Completion machinery is a public surface the owner has not decided to add.
	if len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd) {
		if _, err := fmt.Fprintf(stderr, "pql: unknown command %q\nRun 'pql --help' for usage.\n", args[0]); err != nil {
			return exitSoftware
		}
		return exitUsage
	}
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return resultCode
	}
	var collection *collectionExit
	if errors.As(err, &collection) {
		if collection.err == nil {
			return collection.code
		}
		if _, writeErr := fmt.Fprintf(stderr, "pql: %s\n", collection.err); writeErr != nil {
			return exitSoftware
		}
		return collection.code
	}
	code := exitSoftware
	var rejection *rejectionError
	if errors.As(err, &rejection) {
		code = rejection.code
	}
	var usage *usageError
	if errors.As(err, &usage) {
		code = exitUsage
	}
	if _, writeErr := fmt.Fprintf(stderr, "pql: %s\n", grade.Escape(err.Error())); writeErr != nil {
		return exitSoftware
	}
	if code == exitUsage {
		if _, writeErr := fmt.Fprintln(stderr, "Run 'pql --help' for usage."); writeErr != nil {
			return exitSoftware
		}
	}
	return code
}
