//go:build form_creates_link

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("creates-link") }
