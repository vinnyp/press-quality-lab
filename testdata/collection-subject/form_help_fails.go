//go:build form_help_fails

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-fails") }
