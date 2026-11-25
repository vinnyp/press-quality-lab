package grade

import (
	"syscall"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func hostFields() (strictjson.Object, error) {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return nil, failure(70, "QE9", err.Error())
	}
	return parseUname(unameString(u.Release[:]), unameString(u.Version[:]), unameString(u.Machine[:]))
}

func unameString(v []int8) string {
	b := make([]byte, 0, len(v))
	for _, c := range v {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}
