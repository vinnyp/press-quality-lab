//go:build form_signals_process

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("signals-process") }
