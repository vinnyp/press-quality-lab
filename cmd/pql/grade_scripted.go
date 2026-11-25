//go:build scripted

package main

import (
	"errors"

	"github.com/spf13/cobra"
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
