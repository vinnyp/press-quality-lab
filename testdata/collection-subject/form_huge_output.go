//go:build form_huge_output

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("huge-output") }
