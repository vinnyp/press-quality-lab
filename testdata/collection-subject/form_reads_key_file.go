//go:build form_reads_key_file

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("reads-key-file") }
