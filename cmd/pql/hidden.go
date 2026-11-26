package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/confine"
)

func hidden(args []string) (int, bool) {
	if len(args) == 0 || !strings.HasPrefix(args[0], confine.HiddenPrefix) {
		return 0, false
	}
	if args[0] == confine.SelfTestMode {
		if len(args) != 4 {
			return exitUsage, true
		}
		return confine.SelfTestHidden(args[1], args[2], args[3]), true
	}
	if args[0] == confine.DecoyMode {
		if len(args) != 1 {
			return exitUsage, true
		}
		return confine.DecoyHidden(), true
	}
	if args[0] != confine.LauncherMode && args[0] != confine.FastMode {
		return exitUsage, true
	}
	n := 5
	if args[0] == confine.FastMode {
		n = 2
	}
	if len(args) != n {
		return exitUsage, true
	}
	marker, err := hex.DecodeString(args[1])
	if err != nil || len(marker) != 32 || strings.ToLower(args[1]) != args[1] {
		return exitUsage, true
	}
	var words, env []string
	if n == 5 {
		if !utf8.ValidString(args[3]) || !utf8.ValidString(args[4]) {
			return exitUsage, true
		}
		if err := json.Unmarshal([]byte(args[3]), &words); err != nil {
			return exitUsage, true
		}
		if err := json.Unmarshal([]byte(args[4]), &env); err != nil {
			return exitUsage, true
		}
		if _, err := confine.LauncherArgs(args[1], args[2], words, env); err != nil {
			return exitUsage, true
		}
	}
	return confine.Hidden(args[0], args[1], func() (string, []string, []string) {
		if n == 2 {
			return "", nil, nil
		}
		return args[2], words, env
	}), true
}

// Keep hidden dispatch ahead of Cobra and its error/diagnostic machinery.
func entry() int {
	if code, ok := hidden(os.Args[1:]); ok {
		return code
	}
	return run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
