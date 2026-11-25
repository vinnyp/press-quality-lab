//go:build !darwin

package confine

import "fmt"

type Copies struct{}

func NewCopies(_, _, _ string) (*Copies, error) {
	return nil, fmt.Errorf("QH1: confinement requires macOS 27.2")
}
func (*Copies) Start(Invocation) (Process, error) {
	return nil, fmt.Errorf("QH1: confinement requires macOS 27.2")
}
