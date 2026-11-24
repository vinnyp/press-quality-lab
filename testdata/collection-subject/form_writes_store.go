//go:build form_writes_store

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("writes-store") }
