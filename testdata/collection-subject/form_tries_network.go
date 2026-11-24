//go:build form_tries_network

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("tries-network") }
