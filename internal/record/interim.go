package record

import "sort"

// Interim builds the envelope's subject display fields and the judge's actions.
// It never decides a reason; every action is attached to the judge's own order.
func Interim(subject, output Object, masker *Masker) (actions, offline, bookkeeping Array) {
	actions, offline, bookkeeping = Array{}, Array{}, Array{}
	table := NextActions()
	for _, v := range array(output["checks"]) {
		ch := object(v)
		reason, ok := ch["reason"].(string)
		if !ok {
			continue
		}
		if action, ok := table[reason]; ok {
			actions = append(actions, Object{"check_id": ch["id"], "dimension": nil, "reason": reason, "action": action})
		}
	}
	for _, v := range array(output["dimensions"]) {
		dim := object(v)
		reason, ok := dim["reason"].(string)
		if !ok {
			continue
		}
		if reason == "placeholder_unfilled" || reason == "scope_not_requested" {
			actions = append(actions, Object{"check_id": nil, "dimension": dim["dimension"], "reason": reason, "action": table[reason]})
		}
	}
	for _, v := range array(subject["offline"]) {
		setting := object(v)
		names := make([]string, 0, len(object(setting["env"])))
		for name := range object(setting["env"]) {
			names = append(names, name)
		}
		sort.Strings(names)
		env := Array{}
		for _, name := range names {
			env = append(env, name)
		}
		offline = append(offline, Object{"applies_to": setting["applies_to"], "args": setting["args"], "env_names": env})
	}
	bookkeeping = append(bookkeeping, array(subject["bookkeeping"])...)
	for i, v := range offline {
		offline[i] = maskValue(v, masker)
	}
	for i, v := range bookkeeping {
		bookkeeping[i] = maskValue(v, masker)
	}
	return actions, offline, bookkeeping
}
