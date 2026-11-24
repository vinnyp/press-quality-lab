//go:build form_cache_outside_home

package main

var selectedForm = behaviour{source: "local"}

func init() { runDecoyForm("cache-outside-home") }
