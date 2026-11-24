//go:build form_help_manpage

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-manpage") }
