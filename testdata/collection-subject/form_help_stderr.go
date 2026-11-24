//go:build form_help_stderr

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-stderr") }
