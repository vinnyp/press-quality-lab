package main

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

// commandOptions is the parsed request handed to the pipeline.
type commandOptions struct {
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

func pipelineNotBuilt(commandOptions) error {
	return &rejectionError{exitSoftware, "QE16", "grade pipeline not built (U25)"}
}

var runGrade = pipelineNotBuilt
var runAnchor = pipelineNotBuilt

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
