// Package judge implements the sealed judge contract as a pure transformation.
package judge

import (
	"fmt"
	"runtime/debug"
)

// Rule identifies a deciding clause of the rulebook.
type Rule string

const (
	X1  Rule = "X1"
	X2  Rule = "X2"
	X3  Rule = "X3"
	X4  Rule = "X4"
	X5  Rule = "X5"
	X6  Rule = "X6"
	X7  Rule = "X7"
	X8  Rule = "X8"
	X9  Rule = "X9"
	X10 Rule = "X10"
	X11 Rule = "X11"
	X12 Rule = "X12"
	C1  Rule = "C1"
	C2  Rule = "C2"
	C3  Rule = "C3"
	C4  Rule = "C4"
	C5  Rule = "C5"
	C6  Rule = "C6"
	C7  Rule = "C7"
	C8  Rule = "C8"
	D0  Rule = "D0"
	D1  Rule = "D1"
	D2  Rule = "D2"
	D3  Rule = "D3"
	D4  Rule = "D4"
	D5  Rule = "D5"
	D6  Rule = "D6"
	D7  Rule = "D7"
	D8  Rule = "D8"
	G1  Rule = "G1"
	G2  Rule = "G2"
	G3  Rule = "G3"
	G4  Rule = "G4"
	G5  Rule = "G5"
	G6  Rule = "G6"
	P0  Rule = "P0"
	P1  Rule = "P1"
	P3  Rule = "P3"
	P4  Rule = "P4"
	P5  Rule = "P5"
	K1  Rule = "K1"
	K2  Rule = "K2"
	K3  Rule = "K3"
	K4  Rule = "K4"
	B1  Rule = "B1"
	B2  Rule = "B2"
	E1  Rule = "E1"
	E2  Rule = "E2"
	E3  Rule = "E3"
	E4  Rule = "E4"
	E5  Rule = "E5"
	E6  Rule = "E6"
	E7  Rule = "E7"
	E8  Rule = "E8"
	E9  Rule = "E9"
	E9a Rule = "E9a"
	E9b Rule = "E9b"
	E10 Rule = "E10"
	E11 Rule = "E11"
	R1  Rule = "R1"
	R2  Rule = "R2"
	R3  Rule = "R3"
	N1  Rule = "N1"
	N2  Rule = "N2"
	N3  Rule = "N3"
	N4  Rule = "N4"
	N5  Rule = "N5"
	N6  Rule = "N6"
	N7  Rule = "N7"
)

// Safe recovers an implementation defect in the caller's goroutine.
func Safe(judge func([]byte) ([]byte, int), input []byte) (output []byte, exitCode int, defect error) {
	defer func() {
		if p := recover(); p != nil {
			output = nil
			exitCode = 70
			defect = fmt.Errorf("judge panic: %v\n%s", p, debug.Stack())
		}
	}()
	output, exitCode = judge(input)
	return
}
func emptyOutput() object {
	return object{"rulebook_version": "1.0", "outcome": nil, "rejected_by": nil, "exit_code": nil, "scope": nil, "verdict": nil, "own_verdict": nil, "counts": nil, "stale_markers": array{}, "checks": array{}, "dimensions": array{}, "groups": array{}, "comparison": nil, "set_aside": array{}, "earlier_pools": array{}, "anchor": nil}
}
func rejection(rule Rule) ([]byte, int) {
	o := emptyOutput()
	o["outcome"] = Rejected
	o["rejected_by"] = rule
	o["exit_code"] = 2
	return write(o), 2
}

// Judge decides a request without consulting external state.
func Judge(input []byte) (output []byte, exitCode int) {
	v, ok := readDocument(input, inputShape)
	if !ok {
		return rejection(X12)
	}
	decodeEnums(obj(v))
	r := newRequest(obj(v))
	if rule := r.earlyRejection(); rule != "" {
		return rejection(rule)
	}
	r.collect()
	if rule := r.lateRejection(); rule != "" {
		return rejection(rule)
	}
	o, code := r.evaluate()
	return write(o), code
}
