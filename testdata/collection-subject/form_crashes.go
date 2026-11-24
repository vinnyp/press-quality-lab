//go:build form_crashes

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("crashes") }
