package subject

import "github.com/vinnyp/press-quality-lab/internal/strictjson"

// Create describes the supplied binary without inspecting or executing it.
func Create(binary string) File {
	placeholder := func(sentence string) any { return strictjson.Object{"placeholder": sentence} }
	return File{
		"subject_file_version": "1.0", "binary": binary,
		"facts": strictjson.Object{
			"read_only":           placeholder("State whether the subject is read only so applicability can be judged."),
			"has_destructive_ops": placeholder("State whether destructive operations exist so applicability can be judged."),
			"store_backed":        placeholder("State whether a store exists so applicability can be judged."),
			"has_cli":             placeholder("State whether a CLI exists so applicability can be judged."),
			"bounding":            placeholder("Describe output bounding so applicability can be judged."),
		},
		"declared_count": placeholder("Supply the command count so discovery completeness can be measured."),
		"seed":           placeholder("Supply the pristine seed so runs can start from known state."),
		"store":          placeholder("Name the store so changes can be measured."),
		"secret":         placeholder("Describe a planted secret so leakage can be measured."),
		"commands":       strictjson.Array{}, "runbooks": strictjson.Array{}, "markers": strictjson.Array{},
		"offline": strictjson.Array{}, "bookkeeping": strictjson.Array{},
		"prompts": strictjson.Object{"version": ""},
	}
}
