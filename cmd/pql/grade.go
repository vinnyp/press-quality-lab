package main

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// commandOptions is the parsed request handed to the pipeline.
type commandOptions struct {
	Output      io.Writer
	SubjectFile string
	Scope       []string
	Baseline    string
	Retired     string
	Accept      string
	Verifier    string
	Host        string
	Behaviour   string
}

type rejectionError struct {
	code  int
	rule  string
	cause string
}

func (e *rejectionError) Error() string { return "rejected: " + e.rule + ": " + e.cause }

type collectionExit struct {
	code int
	err  error
}

func (e *collectionExit) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return ""
}

var collectionDependencies = func(o commandOptions) grade.Dependencies {
	return grade.Dependencies{Source: func() (source observe.Source, close func() error, err error) {
		return buildGradeSource(o)
	}}
}

func executeCollection(o commandOptions, anchor bool) error {
	d := collectionDependencies(o)
	code, err := grade.Run(grade.Options{SubjectFile: o.SubjectFile, Scope: o.Scope, Baseline: o.Baseline, Retired: o.Retired, Accept: o.Accept, Anchor: anchor, Scripted: o.Verifier == "scripted"}, d, o.Output)
	return &collectionExit{code, err}
}

var runGrade = func(o commandOptions) error { return executeCollection(o, false) }
var runAnchor = func(o commandOptions) error { return executeCollection(o, true) }

func newCollectionCommand(name string) *cobra.Command {
	options := commandOptions{}
	var scope string
	command := &cobra.Command{
		Use:   name + " <subject-file>",
		Short: "Collect evidence for " + name,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{errors.New("exactly one subject-file is required")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			options.SubjectFile = args[0]
			options.Output = cmd.OutOrStdout()
			if err := validateTestEntry(cmd, options); err != nil {
				return err
			}
			if name == "grade" {
				options.Scope = strings.Split(scope, ",")
				return runGrade(options)
			}
			return runAnchor(options)
		},
	}
	if name == "grade" {
		command.Flags().StringVar(&scope, "scope", "conformance", "Comma-separated scope tokens")
		command.Flags().StringVar(&options.Baseline, "baseline", "", "Baseline file")
	} else {
		command.Flags().StringVar(&options.Retired, "retired", "", "Retired file")
		command.Flags().StringVar(&options.Accept, "accept", "", "Acceptance file")
	}
	addTestEntryFlags(command, &options)
	return command
}
