//go:build form_leak_percent_upper

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-percent-upper") }
