//go:build form_help_cycle

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-cycle") }
