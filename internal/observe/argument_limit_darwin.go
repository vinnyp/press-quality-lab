package observe

import "syscall"

// HelpArgumentLimit reads the running QH1 host's argument limit. Darwin releases
// differ, so discovery must not assume the historical 256 KiB value.
func HelpArgumentLimit() (int, error) {
	limit, err := syscall.SysctlUint32("kern.argmax")
	return int(limit), err
}
