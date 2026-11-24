//go:build form_leak_base64

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-base64") }
