//go:build form_writes_empty_wal

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("writes-empty-wal") }
