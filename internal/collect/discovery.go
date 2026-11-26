package collect

import (
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func walk(ch plan.Check, c observe.Controls, source observe.Source) (*decide.Discovery, []plan.Object, error) {
	d := &decide.Discovery{}
	keys := []plan.Object{plan.DiscoveryKey(plan.Array{})}
	binary := discoveryObject(ch.Subject["binary"])
	if !c.ConfinementApplied || !c.SelfTestPassed || !c.SystemNamed || !c.PrivateCopyUnchanged || binary["placeholder"] != nil {
		return d, keys, nil
	}
	d.Ran = true
	keys = nil
	pending := []plan.Array{{}}
	for len(pending) > 0 {
		words := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if len(words) > 64 || len(keys) == 4096 {
			d.Bound = true
			break
		}
		key := plan.DiscoveryKey(words)
		run, err := source.Observe(map[string]any(key), observe.Spec{BudgetSeconds: 10, CheckID: ch.ID, Preimage: map[string]any(ch.Preimage)})
		if err != nil {
			return nil, nil, err
		}
		keys = append(keys, key)
		d.Invocations = append(d.Invocations, observe.Invocation{Key: map[string]any(key), Run: run})
		a, err := observe.Attribute(run)
		if err != nil {
			return nil, nil, err
		}
		if a == observe.Unconfined || a == observe.CouldNotStart {
			continue
		}
		children, heading := observe.HelpCommands(run)
		if a != observe.Exited || run.End.Code != 0 || len(run.Stdout)+len(run.Stderr) == 0 {
			d.Unreadable = true
		}
		if len(words) == 0 && !heading {
			commands := discoveryArray(ch.Subject["commands"])
			for _, v := range commands {
				cmd := discoveryObject(v)
				w := discoveryArray(cmd["words"])
				if len(w) > 0 {
					d.Unreadable = true
				}
			}
		}
		for _, name := range children {
			child := append(append(plan.Array{}, words...), name)
			d.Found = append(d.Found, child)
		}
		for i := len(children) - 1; i >= 0; i-- {
			pending = append(pending, append(append(plan.Array{}, words...), children[i]))
		}
	}
	return d, keys, nil
}

func discoveryObject(v any) plan.Object {
	if o, ok := v.(plan.Object); ok {
		return o
	}
	return nil
}
func discoveryArray(v any) plan.Array {
	if a, ok := v.(plan.Array); ok {
		return a
	}
	return nil
}
