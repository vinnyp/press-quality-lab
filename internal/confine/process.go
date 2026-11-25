package confine

import "github.com/vinnyp/press-quality-lab/internal/observe"

// Invocation supplies a resolved profile and the exact program words. The loop
// owns home creation, environment construction and invocation sequencing.
type Invocation struct {
	Paths                      Paths
	Words, Env                 []string
	Directory                  string
	Computation, Control, Fast bool
}

// Process remains unreaped until Wait has ended its process group. Kill is also
// available to the exit funnel and the later budget controller.
type Process interface {
	Kill() error
	Wait() (observe.Run, error)
}

type groupMember struct {
	pid    int
	zombie bool
}

// groupEnded accepts an empty group or one containing only zombies. A zombie
// cannot execute; any live member still needs to be ended under QH23.
func groupEnded(members []groupMember) bool {
	for _, member := range members {
		if !member.zombie {
			return false
		}
	}
	return true
}
