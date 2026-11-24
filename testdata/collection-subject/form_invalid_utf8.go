//go:build form_invalid_utf8

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("invalid-utf8") }
