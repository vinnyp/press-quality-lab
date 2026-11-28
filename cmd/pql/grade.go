package main

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// commandOptions is the parsed request handed to the pipeline.
type commandOptions struct {
	JSON        bool
	Masker      *record.Masker
	Output      io.Writer
	Diagnostic  io.Writer
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
	if o.Diagnostic != nil {
		if _, err := io.WriteString(o.Diagnostic, grade.ComputationWarning+"\n"); err != nil {
			return err
		}
	}
	d := collectionDependencies(o)
	if o.Masker != nil {
		d.Masker = o.Masker
	}
	if d.Stderr == nil {
		d.Stderr = o.Diagnostic
	}
	code, err := grade.Run(grade.Options{SubjectFile: o.SubjectFile, Scope: o.Scope, Baseline: o.Baseline, Retired: o.Retired, Accept: o.Accept, Anchor: anchor, JSON: o.JSON, Scripted: o.Verifier == "scripted"}, d, o.Output)
	return &collectionExit{code, err}
}

var runGrade = func(o commandOptions) error { return executeCollection(o, false) }
var runAnchor = func(o commandOptions) error { return executeCollection(o, true) }

func newCollectionCommand(name string, maskers ...*record.Masker) *cobra.Command {
	options := commandOptions{}
	if len(maskers) > 0 {
		options.Masker = maskers[0]
	}
	var scope string
	command := &cobra.Command{
		Use:   name + " <subject-file>",
		Short: "Collect evidence for " + name,
		Long:  "Collect evidence for " + name + ". The text output is not a stable interface; use --json for scripts and agents.\n\n" + grade.ComputationWarning,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{errors.New("exactly one subject-file is required")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			options.SubjectFile = args[0]
			options.Output = cmd.OutOrStdout()
			options.Diagnostic = cmd.ErrOrStderr()
			if err := validateTestEntry(cmd, options); err != nil {
				return err
			}
			if name == "grade" {
				options.Scope = strings.Split(scope, ",")
				return runGrade(options)
			}
			options.Scope = []string{"conformance"}
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
	command.Flags().BoolVar(&options.JSON, "json", false, "Print the judge output as one JSON line instead of the scorecard")
	addTestEntryFlags(command, &options)
	return command
}
