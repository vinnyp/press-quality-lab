//go:build form_starts_child

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("starts-child") }
