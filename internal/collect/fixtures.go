//go:build !necessity_test

package collect

import "github.com/vinnyp/press-quality-lab/internal/observe"

// Options binds home preparation to the run workspace and reports fresh canaries.
type Options struct {
	RunDirectory string
	CanaryDrawn  func(string)
	AfterObserve func(observe.Key, observe.Spec, observe.Run) error
}

func applyHomeFixtures(string, Options) error { return nil }
