//go:build form_forges_atime

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("forges-atime") }
