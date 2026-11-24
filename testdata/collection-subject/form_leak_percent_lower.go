//go:build form_leak_percent_lower

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-percent-lower") }
