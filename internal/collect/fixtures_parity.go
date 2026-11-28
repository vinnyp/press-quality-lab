//go:build necessity_test

package collect

import (
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

// Options binds home preparation to the run workspace and reports fresh canaries.
type Options struct {
	RunDirectory string
	CanaryDrawn  func(string)
	AfterObserve func(observe.Key, observe.Spec, observe.Run) error
	HomeFiles    []throwaway.NamedFile
}

func applyHomeFixtures(home string, o Options) error {
	return throwaway.WriteHomeFiles(home, o.HomeFiles)
}
func CollectWithOptions(p plan.Plan, source observe.Source, options Options) ([]Check, error) {
	return collectWithOptions(p, source, options)
}
