//go:build form_kills_self

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("kills-self") }
