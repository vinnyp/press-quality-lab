package grade

import (
	"golang.org/x/sys/unix"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func hostFields() (strictjson.Object, error) {
	version, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return nil, failure(70, "QE9", err.Error())
	}
	build, err := unix.Sysctl("kern.osversion")
	if err != nil {
		return nil, failure(70, "QE9", err.Error())
	}
	arch, err := unix.Sysctl("hw.machine")
	if err != nil {
		return nil, failure(70, "QE9", err.Error())
	}
	return parseHost(version, build, arch)
}
