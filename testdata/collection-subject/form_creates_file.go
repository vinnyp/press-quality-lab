//go:build form_creates_file

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("creates-file") }
