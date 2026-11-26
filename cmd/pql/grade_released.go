//go:build !scripted

package main

import (
	"errors"
	"github.com/vinnyp/press-quality-lab/internal/grade"
	"github.com/vinnyp/press-quality-lab/internal/observe"

	"github.com/spf13/cobra"
)

func addTestEntryFlags(command *cobra.Command, options *commandOptions) {
	command.Flags().StringVar(&options.Verifier, "verifier", "real", "Network verifier")
	command.Flags().StringVar(&options.Host, "host", "", "Test host file")
	command.Flags().StringVar(&options.Behaviour, "behaviour", "", "Test recording file")
}

func validateTestEntry(command *cobra.Command, options commandOptions) error {
	if options.Verifier != "real" && options.Verifier != "scripted" {
		return &usageError{errors.New("unknown verifier")}
	}
	if options.Verifier == "scripted" || command.Flags().Changed("host") || command.Flags().Changed("behaviour") {
		return &rejectionError{2, "QT1", "test entry unavailable"}
	}
	return nil
}

func buildGradeSource(commandOptions) (observe.Source, func() error, error) {
	return grade.ReleasedSource{}, func() error { return nil }, nil
}
