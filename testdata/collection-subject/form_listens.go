//go:build form_listens

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("listens") }
