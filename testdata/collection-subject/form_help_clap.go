//go:build form_help_clap

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-clap") }
