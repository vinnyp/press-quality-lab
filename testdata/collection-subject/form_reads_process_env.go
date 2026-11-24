//go:build form_reads_process_env

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("reads-process-env") }
