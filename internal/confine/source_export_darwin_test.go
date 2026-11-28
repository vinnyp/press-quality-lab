package confine

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

// NewSourceForTest is deliberately absent from a released binary.
func NewSourceForTest(w *throwaway.Workspace, copies, controls *Copies, subject, computation Invocation, resolve func(observe.Key, observe.Spec) (Invocation, error), afterWait func(observe.Spec) error) *Source {
	return &Source{workspace: w, copies: copies, controlCopies: controls, subject: subject, computation: computation, resolve: resolve, afterWait: afterWait}
}

func SourceControlsForTest(s *Source) (observe.Run, ControlFacts, string, string) {
	return s.controlProbe, s.controlFacts, s.product, s.build
}
