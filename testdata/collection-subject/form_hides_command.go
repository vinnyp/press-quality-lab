//go:build form_hides_command

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("hides-command") }
