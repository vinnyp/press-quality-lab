//go:build form_leak_hex_lower

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-hex-lower") }
