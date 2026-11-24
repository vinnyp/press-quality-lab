//go:build form_help_cobra_group

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("help-cobra-group") }
