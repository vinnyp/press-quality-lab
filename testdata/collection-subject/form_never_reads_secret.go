//go:build form_never_reads_secret

package main

var selectedForm = behaviour{source: "local"}

func init() { runSelectedForm("never-reads-secret") }
