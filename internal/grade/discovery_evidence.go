package grade

import (
	"sort"
	"strconv"
	"strings"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func discoveryEvidence(ch collect.Check) strictjson.Object {
	d := ch.Discovery
	out := strictjson.Object{"ran": false, "found_count": nil, "declared_count": nil, "found_names": nil, "missing_names": nil, "undeclared_names": nil, "undeclared_count": nil, "request_bound_reached": false, "depth_bound_reached": false, "found_cut": false, "undeclared_cut": false}
	if d == nil || !d.Ran {
		return out
	}
	out["ran"] = true
	out["found_count"] = d.FoundCount
	if _, ok := ch.Plan.Subject["declared_count"].(strictjson.Number); ok {
		out["declared_count"] = ch.Plan.Subject["declared_count"]
	}
	found, missing, undeclared := strictjson.Array{}, strictjson.Array{}, strictjson.Array{}
	for _, name := range d.Names {
		found = append(found, name)
	}
	for i, cmd := range subjectArray(ch.Plan.Subject["commands"]) {
		if i < len(d.DeclaredFound) && !d.DeclaredFound[i] {
			missing = append(missing, subjectObject(cmd)["words"])
		}
	}
	for _, name := range d.UndeclaredNames {
		undeclared = append(undeclared, name)
	}
	out["found_names"] = found
	out["missing_names"] = missing
	out["undeclared_names"] = undeclared
	out["undeclared_count"] = d.UndeclaredCount
	out["request_bound_reached"] = d.RequestBound
	out["depth_bound_reached"] = d.DepthBound
	out["found_cut"] = d.FoundCut
	out["undeclared_cut"] = d.UndeclaredCut
	return out
}

func placeholderLocations(s subject.File) strictjson.Array {
	locations := strictjson.Array{}
	add := func(path string, v any) {
		o := subjectObject(v)
		if len(o) == 1 && o["placeholder"] != nil {
			locations = append(locations, path)
		}
	}
	for _, key := range []string{"binary", "declared_count", "seed", "store", "secret"} {
		add("/"+key, s[key])
	}
	facts := subjectObject(s["facts"])
	names := make([]string, 0, len(facts))
	for name := range facts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		escaped := strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
		add("/facts/"+escaped, facts[name])
	}
	for i, v := range subjectArray(s["commands"]) {
		cmd := subjectObject(v)
		for _, key := range []string{"class", "writes"} {
			add("/commands/"+strconv.Itoa(i)+"/"+key, cmd[key])
		}
	}
	return locations
}

func shortEnvNames(s subject.File) strictjson.Array {
	names := map[string]bool{}
	for _, setting := range subjectArray(s["offline"]) {
		for name, value := range subjectObject(subjectObject(setting)["env"]) {
			if v, ok := value.(string); ok && len(v) < 8 {
				names[name] = true
			}
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	out := strictjson.Array{}
	for _, name := range sorted {
		out = append(out, name)
	}
	return out
}
