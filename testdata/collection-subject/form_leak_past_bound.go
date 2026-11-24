//go:build form_leak_past_bound

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("leak-past-bound") }
