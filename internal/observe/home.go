package observe

import (
	"os"
	"time"
)

// FileFacts records identity and timestamps before any content inspection.
type FileFacts struct {
	Access, Modified time.Time
	Device, Inode    uint64
}

// HomeEntry retains content identity and the metadata used to guard inspection.
type HomeEntry struct {
	Path, Kind, Digest string
	Size               int64
	Unreadable         bool
	Info               os.FileInfo
}

// HomeSnapshot is a bounded home inspection, including incomplete observations.
type HomeSnapshot struct {
	Entries                []HomeEntry
	Unreadable, Incomplete bool
}
