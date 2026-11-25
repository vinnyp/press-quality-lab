//go:build !scripted

package main

import (
	"fmt"
	"testing"
)

func TestReleasedTestFlags(t *testing.T) {
	stubCollection(t)
	// QT1: released commands parse test flags but reject their use.
	for _, command := range []string{"grade", "anchor"} {
		for i, tail := range [][]string{
			{"--verifier", "scripted"},
			{"--host", "host"},
			{"--host="},
			{"--behaviour", "recording"},
			{"--behaviour="},
		} {
			t.Run(fmt.Sprintf("%s_Row%d", command, i), func(t *testing.T) {
				checkCLI(t, append([]string{command, "subject"}, tail...), 2, "pql: rejected: QT1: test entry unavailable\n")
			})
		}
		checkCLI(t, []string{command, "subject", "--verifier", "unknown"}, 64, "")
		checkCLI(t, []string{command, "subject", "--verifier", "real"}, 70, "pql: rejected: QE16: grade pipeline not built (U25)\n")
	}
}
