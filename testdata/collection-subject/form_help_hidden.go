//go:build form_help_hidden

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-hidden") }
