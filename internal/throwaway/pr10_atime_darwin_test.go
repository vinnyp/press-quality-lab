package throwaway

import (
	"syscall"
	"time"
)

func statAccessTime(s *syscall.Stat_t) time.Time { return time.Unix(s.Atimespec.Sec, s.Atimespec.Nsec) }
