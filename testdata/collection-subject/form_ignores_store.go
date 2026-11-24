//go:build form_ignores_store

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("ignores-store") }
