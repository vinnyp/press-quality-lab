//go:build form_live_first

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("live-first") }
