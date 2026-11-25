//go:build scripted

package main

import (
	"fmt"
	"testing"
)

func TestScriptedIncompleteFlags(t *testing.T) {
	// QT1: all three test-entry flags are required together.
	flags := [][]string{{"--verifier", "scripted"}, {"--host", "host"}, {"--behaviour", "recording"}}
	for _, command := range []string{"grade", "anchor"} {
		checkCLI(t, []string{command, "subject", "--host", "host", "--verifier", "unknown"}, 64, "")
		for mask := 1; mask < 7; mask++ {
			t.Run(fmt.Sprintf("%s_Row%d", command, mask), func(t *testing.T) {
				args := []string{command, "subject"}
				for i, flag := range flags {
					if mask&(1<<i) != 0 {
						args = append(args, flag...)
					}
				}
				checkCLI(t, args, 2, "pql: rejected: QT1: incomplete test entry\n")
			})
		}
		checkCLI(t, []string{command, "subject", "--verifier", "scripted", "--host", "host", "--behaviour", "recording"}, 70, "pql: rejected: QE16: grade pipeline not built (U25)\n")
	}
}
