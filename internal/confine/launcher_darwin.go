package confine

import (
	"golang.org/x/sys/unix"
	"os"
)

// Hidden writes only the protocol lines; replacement preserves the process id.
func Hidden(mode, marker string, program func() (string, []string)) int {
	if err := writeLine(MarkerPrefix + marker + "\n"); err != nil {
		return 70
	}
	if mode == FastMode {
		return 0
	}
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &limit); err != nil {
		return 70
	}
	for fd := uint64(3); fd < limit.Cur; fd++ {
		if err := unix.Close(int(fd)); err != nil && err != unix.EBADF {
			return 70
		}
	}
	executable, words := program()
	if err := unix.Exec(executable, append([]string{executable}, words...), os.Environ()); err != nil {
		if err := writeLine(FailurePrefix + marker + "\n"); err != nil {
			return 70
		}
	}
	return CouldNotStart
}

func writeLine(line string) error {
	for len(line) > 0 {
		n, err := unix.Write(2, []byte(line))
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return unix.EIO
		}
		line = line[n:]
	}
	return nil
}
