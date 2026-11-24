//go:build form_links_decoy

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("links-decoy") }
