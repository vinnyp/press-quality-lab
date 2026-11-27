package collect

import (
	"strings"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
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

	// E-34: use kern.argmax on the running Darwin host (256 KiB on older
	// releases, 1 MiB on newer ones). Include argv pointers and terminating NULs.
	argumentLimit, err := observe.HelpArgumentLimit()
	if err != nil {
		return nil, nil, err
	}
	type node struct {
		parent int
		refs   int
		name   string
	}
	nodes := []node{{parent: -1, refs: 1}}
	free := []int{}
	// Pending entries and children own references. Recycle abandoned branches,
	// including their names, instead of accumulating every truncated frontier.
	release := func(index int) {
		for index >= 0 {
			nodes[index].refs--
			if nodes[index].refs != 0 {
				return
			}
			parent := nodes[index].parent
			nodes[index] = node{parent: -1}
			free = append(free, index)
			index = parent
		}
	}
	addNode := func(parent int, name string) int {
		n := node{parent: parent, refs: 1, name: name}
		nodes[parent].refs++
		if len(free) > 0 {
			index := free[len(free)-1]
			free = free[:len(free)-1]
			nodes[index] = n
			return index
		}
		nodes = append(nodes, n)
		return len(nodes) - 1
	}
	pending := []int{0}
	declared := discoveryArray(ch.Subject["commands"])
	declaredByKey := make(map[string][]int, len(declared))
	d.DeclaredFound = make([]bool, len(declared))
	for i, v := range declared {
		words := discoveryArray(discoveryObject(v)["words"])
		if len(words) == 0 {
			d.DeclaredFound[i] = true
			continue
		}
		k := string(strictjson.Write(words))
		declaredByKey[k] = append(declaredByKey[k], i)
	}
	namesBytes := 0

	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		words := plan.Array{}
		for i := current; nodes[i].parent >= 0; i = nodes[i].parent {
			words = append(words, nodes[i].name)
		}
		for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
			words[i], words[j] = words[j], words[i]
		}

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
		retained := run
		retained.Stdout = nil
		retained.Stderr = nil
		retained.Details.Stdout.KeptBase64 = nil
		retained.Details.Stderr.KeptBase64 = nil
		retained.Details.Stdout.Head = ""
		retained.Details.Stderr.Head = ""
		d.Invocations = append(d.Invocations, observe.Invocation{Key: map[string]any(key), Run: retained})
		a, err := observe.Attribute(run)
		if err != nil {
			return nil, nil, err
		}
		if a == observe.Unconfined || a == observe.CouldNotStart {
			release(current)
			continue
		}
		children, heading := observe.HelpCommands(run)
		if a != observe.Exited || run.End.Code != 0 || run.StdoutSeen+run.StderrSeen == 0 || observe.HelpCut(run) {
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

		next := make([]int, 0, min(len(children), 4096-len(keys)))
		for _, name := range children {
			size := 8 + len("--help") + 1
			for _, word := range words {
				text, ok := word.(string)
				if !ok {
					panic("QD2: discovery word is not a string")
				}
				size += 8 + len(text) + 1
			}
			size += 8 + len(name) + 1
			var child plan.Array
			if len(declaredByKey) > 0 || len(d.Names) < 512 && namesBytes+size <= 64<<10 {
				child = append(append(plan.Array{}, words...), name)
			}
			d.FoundCount++
			var indexes []int
			ok := false
			if len(declaredByKey) > 0 && utf8.ValidString(name) {
				indexes, ok = declaredByKey[string(strictjson.Write(child))]
			}
			if !ok {
				d.Undeclared = true
			}
			for _, i := range indexes {
				d.DeclaredFound[i] = true
			}
			if strings.ContainsRune(name, 0) || !utf8.ValidString(name) || size > argumentLimit {
				d.Unreadable = true
				continue
			}
			// The scorecard preview has both an entry and a byte bound.
			if len(d.Names) < 512 && namesBytes+size <= 64<<10 {
				d.Names = append(d.Names, child)
				namesBytes += size
			}
			if len(next) < 4096-len(keys) {
				if len(next)+len(pending) == 4096-len(keys) {
					d.Bound = true
					release(pending[len(pending)-1])
					pending = pending[:len(pending)-1]
				}
				next = append(next, addNode(current, name))
			} else {
				d.Bound = true
			}
		}
		remaining := 4096 - len(keys) - len(next)
		if len(pending) > remaining {
			d.Bound = true
			for _, index := range pending[remaining:] {
				release(index)
			}
			pending = pending[:remaining]
		}
		pending = append(next, pending...)
		release(current)

	}
	for _, found := range d.DeclaredFound {
		d.Missing = d.Missing || !found
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
