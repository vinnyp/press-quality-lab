//go:build form_ignores_help

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("ignores-help") }
