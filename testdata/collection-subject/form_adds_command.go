//go:build form_adds_command

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("adds-command") }
