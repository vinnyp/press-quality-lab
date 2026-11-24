//go:build form_leak_plain

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-plain") }
