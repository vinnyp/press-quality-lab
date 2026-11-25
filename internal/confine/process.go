package confine

import "syscall"

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

// exitDetail decodes Darwin kevent fields without depending on host headers.
// NOTE_EXIT_DETAIL_MASK is inside NOTE_PDATAMASK, in data, not fflags.
func exitDetail(flags uint16, fflags uint32, data int64) (uint32, error) {
	const evError = 0x4000
	const detailMask = 0x00070000
	if flags&evError != 0 {
		return 0, syscall.Errno(data)
	}
	return uint32(data) & detailMask, nil
}
