//go:build form_leak_hex_upper

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-hex-upper") }
