package grade

import (
	"strings"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func parseHost(version, build, arch string) (strictjson.Object, error) {
	for _, value := range []string{version, build, arch} {
		if utf8.RuneCountInString(value) > 32 {
			return nil, failure(70, "QE9", "host field exceeds 32 characters")
		}
	}
	return strictjson.Object{"os_version": version, "os_build": build, "arch": arch}, nil
}

func parseUname(release, version, machine string) (strictjson.Object, error) {
	fields := strings.Fields(version)
	build := ""
	if len(fields) > 0 {
		build = fields[0]
	}
	return parseHost(release, build, machine)
}
