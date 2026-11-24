//go:build form_leak_file

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-file") }
