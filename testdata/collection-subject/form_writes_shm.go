//go:build form_writes_shm

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("writes-shm") }
