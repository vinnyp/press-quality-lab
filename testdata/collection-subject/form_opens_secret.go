//go:build form_opens_secret

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("opens-secret") }
