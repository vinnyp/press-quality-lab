//go:build form_drops_last

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("drops-last") }
