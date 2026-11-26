//go:build scripted

package scripted

import (
	"math/big"
	"regexp"
	"strings"

	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type object = strictjson.Object
type array = strictjson.Array
type number = strictjson.Number
type predicate func(any) bool

func obj(v any) object {
	o, ok := v.(object)
	if !ok {
		return nil
	}
	return o
}
func arr(v any) array {
	a, ok := v.(array)
	if !ok {
		return nil
	}
	return a
}
func textValue(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
func boolean(v any) bool             { _, ok := v.(bool); return ok }
func stringValue(v any) bool         { _, ok := v.(string); return ok }
func nullable(p predicate) predicate { return func(v any) bool { return v == nil || p(v) } }
func pattern(p string) predicate {
	r := regexp.MustCompile(p)
	return func(v any) bool { s, ok := v.(string); return ok && r.MatchString(s) }
}
func enum(values ...string) predicate {
	return func(v any) bool {
		for _, s := range values {
			if v == s {
				return true
			}
		}
		return false
	}
}
func fields(v any, fs map[string]predicate) bool {
	o, ok := v.(object)
	if !ok || len(o) != len(fs) {
		return false
	}
	for k, p := range fs {
		x, present := o[k]
		if !present || !p(x) {
			return false
		}
	}
	return true
}
func list(p predicate) predicate {
	return func(v any) bool {
		a, ok := v.(array)
		if !ok {
			return false
		}
		for _, x := range a {
			if !p(x) {
				return false
			}
		}
		return true
	}
}
func integer(v any) bool {
	n, ok := v.(number)
	if !ok {
		return false
	}
	r, ok := new(big.Rat).SetString(string(n))
	return ok && r.IsInt()
}
func boundedInteger(v any, lo, hi int64) bool {
	if !integer(v) {
		return false
	}
	r, _ := new(big.Rat).SetString(string(numberValue(v)))
	return r.Cmp(new(big.Rat).SetInt64(lo)) >= 0 && (hi < lo || r.Cmp(new(big.Rat).SetInt64(hi)) <= 0)
}
func relative(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func keyShape(v any) bool {
	if !fields(v, map[string]predicate{"words": nullable(list(stringValue)), "offline_args": list(stringValue), "env": list(pattern(`^[A-Za-z_][A-Za-z0-9_]*$`)), "runbook": nullable(stringValue), "step": nullable(stringValue), "computation": nullable(list(stringValue)), "home": stringValue}) {
		return false
	}
	o := obj(v)
	if o["computation"] == nil {
		if o["words"] == nil {
			return false
		}
		if o["runbook"] == nil {
			if o["step"] != nil || !enum("seed", "empty", "planted")(o["home"]) {
				return false
			}
		} else if o["step"] == nil || o["home"] != "runbook" {
			return false
		}
		last := ""
		for _, name := range arr(o["env"]) {
			s := textValue(name)
			if s <= last {
				return false
			}
			last = s
		}
		return true
	}
	a := arr(o["computation"])
	return len(a) > 0 && (relative(a[0]) || pattern(`^/(usr/)?bin/[^/]+$`)(a[0])) && o["words"] == nil && o["runbook"] != nil && o["step"] != nil && o["home"] == "scratch" && len(arr(o["env"])) == 0 && len(arr(o["offline_args"])) == 0
}
func streamShape(v any) bool {
	if !fields(v, map[string]predicate{"bytes": func(v any) bool { return boundedInteger(v, 0, -1) }, "sha256": pattern(`^[0-9a-f]{64}$`), "cut": boolean, "head": stringValue, "kept": nullable(pattern(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`))}) {
		return false
	}
	o := obj(v)
	return (o["cut"] == true) == (o["kept"] == nil)
}
func changeShape(v any) bool {
	if !fields(v, map[string]predicate{"path": relative, "kind": enum("file", "directory", "link", "special"), "change": enum("created", "modified", "removed"), "sha256": nullable(pattern(`^[0-9a-f]{64}$`))}) {
		return false
	}
	o := obj(v)
	needs := o["kind"] == "file" && o["change"] != "removed"
	return needs == (o["sha256"] != nil)
}
func leakPlace(v any) bool {
	if v == "stdout" || v == "stderr" {
		return true
	}
	s, ok := v.(string)
	return ok && strings.HasPrefix(s, "home/") && relative(strings.TrimPrefix(s, "home/"))
}
func runShape(v any) bool {
	return fields(v, map[string]predicate{
		"markers": func(v any) bool {
			return fields(v, map[string]predicate{"confinement": boolean, "could_not_start": boolean})
		},
		"end":    endShape,
		"stdout": streamShape,
		"stderr": streamShape,
		"home_changes": func(v any) bool {
			return fields(v, map[string]predicate{"changes": list(changeShape), "control_detected": nullable(boolean)})
		},
		"planted": nullable(func(v any) bool {
			return fields(v, map[string]predicate{"atime_moved": boolean, "same_file": boolean, "control_end": nullable(endShape)})
		}),
		"leak_scan": func(v any) bool {
			return fields(v, map[string]predicate{"bound_reached": boolean, "leaks": list(func(v any) bool {
				return fields(v, map[string]predicate{"place": leakPlace, "encoding": enum("plain", "base64", "base64-nopad", "base64url", "base64url-nopad", "hex-lower", "hex-upper", "percent-upper", "percent-lower")})
			})})
		},
		"decoys": list(func(v any) bool {
			return fields(v, map[string]predicate{"name": pattern(`^[a-z][a-z0-9_]*$`), "touched": boolean})
		}),
		"printed_value": nullable(func(v any) bool { return fields(v, map[string]predicate{"value": func(any) bool { return true }}) }),
	})
}
func recordingShape(v any) bool {
	return fields(v, map[string]predicate{
		"behaviour_version":          enum("1.0"),
		"form":                       pattern(`^[a-z0-9][a-z0-9-]*$`),
		"violating":                  boolean,
		"form_sha256":                pattern(`^[0-9a-f]{64}$`),
		"subject_profile_sha256":     pattern(`^[0-9a-f]{64}$`),
		"computation_profile_sha256": pattern(`^[0-9a-f]{64}$`),
		"launcher_sha256":            pattern(`^[0-9a-f]{64}$`),
		"commit":                     pattern(`^[0-9a-f]{40}$`),
		"system": func(v any) bool {
			return fields(v, map[string]predicate{"product_version": pattern(`^[0-9]+(\.[0-9]+)*$`), "build": pattern(`^[0-9A-Za-z]+$`)})
		},
		"invocations": func(v any) bool {
			return len(arr(v)) > 0 && list(func(v any) bool {
				return fields(v, map[string]predicate{"key": keyShape, "unconfined": nullable(runShape), "confined": func(v any) bool { return len(arr(v)) == 5 && list(runShape)(v) }})
			})(v)
		},
	})
}
func hostShape(v any) bool {
	return fields(v, map[string]predicate{
		"confinement": func(v any) bool {
			if !fields(v, map[string]predicate{"applied": boolean, "sandbox_exec_exit": nullable(integer)}) {
				return false
			}
			o := obj(v)
			return (o["applied"] == true) == (o["sandbox_exec_exit"] == nil)
		},
		"self_test":        enum("passed", "failed"),
		"known_fast":       enum("within", "over"),
		"read_calibration": enum("moved", "not_moved"),
		"store_control":    enum("detected", "not_detected"),
		"read_control":     enum("killed", "not_killed"),
		"private_copy":     enum("unchanged", "changed"),
		"kill_attribution": list(func(v any) bool {
			return fields(v, map[string]predicate{"invocation": keyShape, "by": enum("memory", "code_signing", "engine")})
		}),
	})
}

func numberValue(v any) number {
	n, ok := v.(number)
	if !ok {
		return ""
	}
	return n
}
