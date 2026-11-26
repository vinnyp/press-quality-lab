package subject

import (
	"path"
	"strconv"
	"strings"

	"github.com/vinnyp/press-quality-lab/internal/jcs"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func obj(v any) strictjson.Object {
	if o, ok := v.(strictjson.Object); ok {
		return o
	}
	return nil
}
func arr(v any) strictjson.Array {
	if a, ok := v.(strictjson.Array); ok {
		return a
	}
	return nil
}
func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func placeholder(v any) bool { _, ok := obj(v)["placeholder"]; return ok }
func key(v any) string       { return string(strictjson.Write(v)) }

// HomePath implements the specification's definition of a path relative to home.
func HomePath(s string) bool {
	if s == "" || strings.HasPrefix(s, "/") || strings.ContainsRune(s, 0) {
		return false
	}
	for _, c := range strings.Split(s, "/") {
		if c == "" || c == "." || c == ".." {
			return false
		}
	}
	return true
}

func semantic(o File, bad map[string]bool) {
	commands := map[string]strictjson.Object{}
	for _, v := range arr(o["commands"]) {
		c := obj(v)
		k := key(c["words"])
		if _, exists := commands[k]; exists {
			bad["QS6"] = true
		}
		commands[k] = c
		if len(arr(c["words"])) == 0 && c["class"] == "destructive" {
			bad["QS6"] = true
		}
	}
	declared := func(words any) bool {
		if len(arr(words)) == 0 {
			return true
		}
		_, ok := commands[key(words)]
		return ok
	}
	invocation := func(words any) bool {
		a := arr(words)
		if len(a) == 0 {
			return true
		}
		for _, c := range commands {
			prefix := arr(c["words"])
			if len(prefix) == 0 || len(prefix) > len(a) {
				continue
			}
			match := true
			for i := range prefix {
				if !strictjson.EqualJSON(prefix[i], a[i]) {
					match = false
				}
			}
			if match {
				return true
			}
		}
		return false
	}
	envs := map[string]map[string]bool{}
	for _, v := range arr(o["offline"]) {
		setting := obj(v)
		for _, a := range arr(setting["args"]) {
			switch a {
			case "--help",
				"-h",
				"help",
				"--version",
				"-v",
				"version":
				bad["QS7"] = true
			}
		}
		seen := map[string]bool{}
		for _, w := range arr(setting["applies_to"]) {
			if !declared(w) {
				bad["QS7"] = true
			}
			k := key(w)
			if seen[k] {
				continue
			}
			seen[k] = true
			if envs[k] == nil {
				envs[k] = map[string]bool{}
			}
			for name := range obj(setting["env"]) {
				switch name {
				case "HOME",
					"TMPDIR",
					"PATH",
					"LANG",
					"LC_ALL",
					"TZ",
					"__CF_USER_TEXT_ENCODING":
					bad["QS7"] = true
				}
				if envs[k][name] {
					bad["QS7"] = true
				}
				envs[k][name] = true
			}
		}
	}
	if s, ok := o["store"].(string); ok && !HomePath(s) {
		bad["QS9"] = true
	}
	for _, v := range arr(o["bookkeeping"]) {
		if !HomePath(text(v)) {
			bad["QS9"] = true
		}
	}
	secret := obj(o["secret"])
	if !placeholder(secret) && secret != nil {
		if !HomePath(text(secret["path"])) {
			bad["QS9"] = true
		}
		for _, w := range arr(secret["commands"]) {
			if !declared(w) {
				bad["QS6"] = true
			}
			if commands[key(w)]["class"] == "destructive" {
				bad["QP10"] = true
			}
		}
	}
	books := map[string]bool{}
	count := 0
	for _, v := range arr(o["runbooks"]) {
		book := obj(v)
		name := text(book["name"])
		if books[name] {
			bad["QS10"] = true
		}
		books[name] = true
		steps := map[string]bool{}
		for _, v := range arr(book["steps"]) {
			step := obj(v)
			name := text(step["name"])
			if steps[name] {
				bad["QS10"] = true
			}
			if !invocation(step["words"]) {
				bad["QS6"] = true
			}
			for _, v := range arr(step["checks"]) {
				count++
				check := obj(v)
				expected := obj(check["expected"])
				if literal, exists := expected["literal"]; exists {
					b, err := jcs.Canonical(literal)
					if err != nil || len(b) > 64<<10 || literalDepth(literal) > 48 {
						bad["QS11"] = true
					}
				}
				for _, holder := range []strictjson.Object{expected, obj(check["precondition"])} {
					if c := obj(holder["computation"]); c != nil {
						computationRules(c, steps, bad)
					}
				}
				predicateRules(check, bad)
			}
			steps[name] = true
		}
	}
	if count > 2048 {
		bad["QS11"] = true
	}
}

func literalDepth(v any) int {
	depth := 0
	switch x := v.(type) {
	case strictjson.Array:
		for _, c := range x {
			if n := literalDepth(c); n > depth {
				depth = n
			}
		}
		return depth + 1
	case strictjson.Object:
		for _, c := range x {
			if n := literalDepth(c); n > depth {
				depth = n
			}
		}
		return depth + 1
	}
	return 0
}

func computationRules(c strictjson.Object, earlier map[string]bool, bad map[string]bool) {
	argv := arr(c["argv"])
	if len(argv) > 0 {
		executable := text(argv[0])
		if strings.HasPrefix(executable, "/") && !(strings.HasPrefix(path.Clean(executable), "/bin/") || strings.HasPrefix(path.Clean(executable), "/usr/bin/")) {
			bad["QA4"] = true
		}
	}
	captures := map[string]bool{}
	for _, v := range arr(c["captures"]) {
		name := text(v)
		captures[name] = true
		if !earlier[name] {
			bad["QA4"] = true
		}
	}
	for _, v := range argv {
		s := text(v)
		for {
			start := strings.Index(s, "{CAPTURE:")
			if start < 0 {
				break
			}
			s = s[start+9:]
			end := strings.IndexByte(s, '}')
			if end < 0 {
				bad["QA4"] = true
				break
			}
			name := s[:end]
			if !captures[name] || !earlier[name] {
				bad["QA4"] = true
			}
			s = s[end+1:]
		}
	}
}
func pointer(s string) bool {
	if s == "" {
		return true
	}
	if s[0] != '/' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			i++
			if i == len(s) || s[i] != '0' && s[i] != '1' {
				return false
			}
		}
	}
	return true
}
func whole(v any) bool {
	n, ok := v.(strictjson.Number)
	if !ok {
		return false
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	return err == nil && i >= 0 && i <= 9007199254740991
}
func predicateRules(c strictjson.Object, bad map[string]bool) {
	p := text(c["predicate"])
	expectedValue, hasLiteral := obj(c["expected"])["literal"]
	number, numeric := expectedValue.(strictjson.Number)
	asserts := p == "stdout_empty" || p == "row_count" && hasLiteral && numeric && whole(number) && strictjson.Normalize(number) == "0"
	if asserts != (c["precondition"] != nil) {
		bad["QA9"] = true
	}
	if len(arr(obj(obj(c["precondition"])["computation"])["captures"])) != 0 {
		bad["QA9"] = true
	}
	if p == "json_path_equals" || p == "row_count" {
		s, ok := c["path"].(string)
		if !ok || !pointer(s) {
			bad["QA9"] = true
		}
	} else if c["path"] != nil {
		bad["QA9"] = true
	}
	if p == "exit_code" && c["exit_codes"] != nil {
		bad["QA9"] = true
	}
	expected := obj(c["expected"])
	value, literal := expected["literal"]
	if p == "stdout_empty" {
		if !literal || value != nil {
			bad["QA9"] = true
		}
		return
	}
	if !literal {
		return
	}
	switch p {
	case "exit_code",
		"stdout_bytes_at_most",
		"row_count":
		if !whole(value) {
			bad["QA9"] = true
		}
	case "stdout_equals",
		"stdout_contains",
		"stdout_not_contains",
		"stderr_contains":
		if _, ok := value.(string); !ok {
			bad["QA9"] = true
		}
	}
}

// Computations returns only computations declared in authored expected values
// and preconditions, never lookalike objects within a literal.
func Computations(s File) []strictjson.Object {
	var result []strictjson.Object
	for _, b := range arr(s["runbooks"]) {
		for _, step := range arr(obj(b)["steps"]) {
			for _, check := range arr(obj(step)["checks"]) {
				c := obj(check)
				for _, holder := range []strictjson.Object{obj(c["expected"]), obj(c["precondition"])} {
					if computation := obj(holder["computation"]); computation != nil {
						result = append(result, computation)
					}
				}
			}
		}
	}
	return result
}
