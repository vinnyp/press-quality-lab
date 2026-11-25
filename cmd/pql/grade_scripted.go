//go:build scripted

package main

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/vinnyp/press-quality-lab/internal/scripted"
)

func addTestEntryFlags(command *cobra.Command, options *commandOptions) {
	command.Flags().StringVar(&options.Verifier, "verifier", "real", "Network verifier")
	command.Flags().StringVar(&options.Host, "host", "", "Scripted host file")
	command.Flags().StringVar(&options.Behaviour, "behaviour", "", "Scripted recording file")
}

func validateTestEntry(command *cobra.Command, options commandOptions) error {
	return validateScriptedEntry(command, options)
}

func validateScriptedEntry(command *cobra.Command, options commandOptions) error {
	if options.Verifier != "real" && options.Verifier != "scripted" {
		return &usageError{errors.New("unknown verifier")}
	}
	host := command.Flags().Changed("host")
	behaviour := command.Flags().Changed("behaviour")
	if (options.Verifier == "scripted" || host || behaviour) &&
		!(options.Verifier == "scripted" && host && behaviour) {
		return &rejectionError{2, "QT1", "incomplete test entry"}
	}
	return nil
}

// buildScriptedSource owns input and trace file handles. U25 must call closeTrace
// on every pipeline exit, and propagate its error.
func buildScriptedSource(options commandOptions) (*scripted.Source, func() error, error) {
	host, err := openScriptedInput(options.Host)
	if err != nil {
		return nil, nil, err
	}
	recording, err := openScriptedInput(options.Behaviour)
	if err != nil {
		return nil, nil, err
	}
	var writer io.Writer
	closeTrace := func() error { return nil }
	if path := os.Getenv(scripted.TraceEnv); path != "" {
		trace, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return nil, nil, err
		}
		writer = trace
		closeTrace = trace.Close
	}
	source, err := scripted.Read(host, recording, writer)
	if err != nil {
		return nil, nil, errors.Join(err, closeTrace())
	}

	return source, closeTrace, nil
}

func openScriptedInput(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, scriptedInputError(err)
	}
	return readScriptedInput(file)
}

func readScriptedInput(file io.ReadCloser) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(file, (16<<20)+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, scriptedInputError(err)
	}
	return data, nil
}

func scriptedInputError(err error) error {
	return errors.Join(&scripted.Error{Code: 2, Rule: "QT1", Detail: "scripted input unreadable"}, err)
}

func buildGradeSource(o commandOptions) (observe.Source, func() error, error) {
	if o.Verifier != "scripted" {
		return grade.ReleasedSource{}, func() error { return nil }, nil
	}
	source, close, err := buildScriptedSource(o)
	if err != nil {
		var e *scripted.Error
		if errors.As(err, &e) {
			err = &grade.Error{Code: e.Code, Rule: e.Rule, Cause: e.Detail}
		}
	}
	return source, close, err
}
