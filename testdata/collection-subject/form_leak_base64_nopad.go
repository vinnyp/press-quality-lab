//go:build form_leak_base64_nopad

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-base64-nopad") }
