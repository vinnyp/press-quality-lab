//go:build form_ignores_home

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("ignores-home") }
