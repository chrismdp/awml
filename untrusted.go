package awml

import (
	"errors"
	"fmt"
	"strings"
)

// marks holds a part to where untrusted and cleans may go. Only a trigger, connector
// or context takes in text from strangers, and only a code step can promise to pass on just what's
// safe, since strangers' text can steer an AI step however it's written.
func marks(name string, p *Map) mistakeList {
	var errors mistakeList
	for _, field := range []string{"untrusted", "cleans"} {
		if v := p.get(field); p.has(field) {
			if _, ok := v.(bool); !ok {
				check := map[string]string{"untrusted": "untrusted.parts", "cleans": "cleans.code-only"}[field]
				errors.add(check, fmt.Sprintf(`%s: %s must be true or false, not "%s"`, name, field, show(v)))
			}
		}
	}
	kind := p.get("kind")
	if p.get("untrusted") == true && !isOneOf(kind, "trigger", "connector", "context") {
		errors.add("untrusted.parts", name+" is marked untrusted, but only a trigger, connector or context takes it")
	}
	if p.get("cleans") == true && kind != "code" {
		message := name + " cleans, but only a code step takes cleans"
		if kind == "ai" {
			message += ": strangers' text can steer an AI step however it's written"
		}
		errors.add("cleans.code-only", message)
	}
	return errors
}

// stops is whether a part stops untrusted text: a person's review, a person's own step, or a code
// step that promises to pass on only what's safe. A standing approval never sees the text, so it
// doesn't.
func stops(p *Map) bool {
	kind := p.get("kind")
	return kind == "review" || kind == "person" || kind == "code" && p.get("cleans") == true
}

// reach follows the text of one untrusted part along every line that leads on. It gives the parts
// the text reaches, the source among them, and the parts that stop it, which it reaches but goes no
// further than.
func reach(source *Map, parts, lines []*Map) (reached, stopped map[*Map]bool) {
	byID := map[any]*Map{}
	for _, p := range parts {
		byID[keyOf(p.get("id"))] = p
	}
	reached, stopped = map[*Map]bool{source: true}, map[*Map]bool{}
	for queue := []*Map{source}; len(queue) > 0; queue = queue[1:] {
		for _, l := range lines {
			to := byID[keyOf(l.get("to"))]
			if !lineLeads(l) || to == nil || byID[keyOf(l.get("from"))] != queue[0] || reached[to] || stopped[to] {
				continue
			}
			if stops(to) {
				stopped[to] = true
				continue
			}
			reached[to] = true
			queue = append(queue, to)
		}
	}
	return reached, stopped
}

// untrustedParts gives the parts a file marks untrusted, in the file's order.
func untrustedParts(parts []*Map) []*Map {
	var found []*Map
	for _, p := range parts {
		if p.get("untrusted") == true {
			found = append(found, p)
		}
	}
	return found
}

// untrustedWarnings warns of each outside action that untrusted text reaches, naming the parts the
// text comes from.
func untrustedWarnings(parts, lines []*Map) mistakeList {
	from := map[*Map][]string{}
	for _, source := range untrustedParts(parts) {
		reached, _ := reach(source, parts, lines)
		for p := range reached {
			if p.get("kind") == "outside" {
				from[p] = append(from[p], fmt.Sprintf(`"%s"`, show(source.get("id"))))
			}
		}
	}
	var warnings mistakeList
	for _, p := range parts {
		if sources := from[p]; sources != nil {
			what := "part"
			if len(sources) > 1 {
				what = "parts"
			}
			warnings.add("warn.untrusted-outside", fmt.Sprintf(`untrusted text from %s %s reaches outside action "%s": a review, a person's own step or a code step marked cleans in between would stop it`,
				what, naturalList(sources), show(p.get("id"))))
		}
	}
	return warnings
}

// UntrustedSummary names one untrusted part, the steps and outside actions its text reaches, and
// the reviews and steps that stop it. A step that stops the text takes it in, so it is among the
// steps too.
type UntrustedSummary struct {
	Part           string   `json:"part"`
	Steps          []string `json:"steps"`
	OutsideActions []string `json:"outsideActions"`
	StoppedBy      []string `json:"stoppedBy"`
}

func summariseUntrusted(parts, lines []*Map) []UntrustedSummary {
	var out []UntrustedSummary
	for _, source := range untrustedParts(parts) {
		reached, stopped := reach(source, parts, lines)
		entry := UntrustedSummary{Part: str(source, "label")}
		for _, p := range parts {
			label := str(p, "label")
			if (reached[p] || stopped[p]) && steps[str(p, "kind")] {
				entry.Steps = append(entry.Steps, label)
			}
			if reached[p] && p.get("kind") == "outside" {
				entry.OutsideActions = append(entry.OutsideActions, label)
			}
			if stopped[p] {
				entry.StoppedBy = append(entry.StoppedBy, label)
			}
		}
		out = append(out, entry)
	}
	return out
}

// HasUntrusted is whether a workflow marks any part untrusted, and so has an untrusted view.
func HasUntrusted(doc *Map) bool {
	return IsCanonical(doc) && len(untrustedParts(items(doc, "parts"))) > 0
}

// UntrustedView projects a sound workflow into its untrusted view: the security view, drawn to show
// where untrusted text reaches. The source is never changed.
func UntrustedView(doc *Map) (*Map, error) {
	if !HasUntrusted(doc) {
		return nil, errors.New("the file marks no part untrusted, so it has no untrusted view")
	}
	view, err := View(doc, "")
	if err != nil {
		return nil, err
	}
	view.set("view", "untrusted")
	return view, nil
}

// untrustedLooks gives the untrusted view's class for each part the text reaches, and marks each
// part that stops it.
func untrustedLooks(parts, lines []*Map) (class map[string]string, marked map[string]bool) {
	class, marked = map[string]string{}, map[string]bool{}
	for _, source := range untrustedParts(parts) {
		reached, stopped := reach(source, parts, lines)
		for p := range reached {
			class[str(p, "id")] = "taint"
			if p.get("kind") == "outside" {
				class[str(p, "id")] = "taintout"
			}
		}
		for p := range stopped {
			marked[str(p, "id")] = true
		}
	}
	return class, marked
}

// untrustedLabel adds a part's marks to its label: ⚠️ for a part marked untrusted, and in the
// untrusted view 🛡️ for one that stops the text.
func untrustedLabel(p *Map, words string, stopper bool) string {
	var marks []string
	if p.get("untrusted") == true {
		marks = append(marks, "⚠️")
	}
	if stopper {
		marks = append(marks, "🛡️")
	}
	return strings.Join(append([]string{words}, marks...), " ")
}
