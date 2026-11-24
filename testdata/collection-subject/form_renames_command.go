//go:build form_renames_command

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("renames-command") }
