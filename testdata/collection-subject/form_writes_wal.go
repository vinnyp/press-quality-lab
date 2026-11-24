//go:build form_writes_wal

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("writes-wal") }
