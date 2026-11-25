//go:build darwin

package main

import (
	"github.com/vinnyp/press-quality-lab/internal/spike/hostilehome"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(64)
	}
	hostilehome.Hostile(os.Stdout, os.Args[1])
}
