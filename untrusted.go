package awml

import (
	"fmt"
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

// trail is where the text of one untrusted part goes: the parts it reaches, the source among them,
// and the parts that stop it, which it reaches but goes no further than. carries holds each line
// it goes along, and cleared each line out of a part that stopped it. unstopped holds each line on
// its way to an outside action that nothing stops and nobody accepted, and breaches each line that
// brings it into such an action. shown holds each line that brings it onto a surface, but a standing
// approval's, which shows what went out. accepted holds each outside action it reaches under a
// standing approval, which accepts the risk, with that approval.
type trail struct {
	reached, stopped            map[*Map]bool
	carries, cleared, unstopped map[*Map]bool
	breaches, shown             []*Map
	accepted                    map[[2]*Map]bool
}

// follow follows the text of one untrusted part along every line that leads on. A standing approval
// in front of an outside action accepts the risk of the text reaching it.
func follow(source *Map, parts, lines []*Map) trail {
	byID := partsOf(parts)
	out := map[*Map][]*Map{}
	for _, l := range lines {
		if from := byID[keyOf(l.get("from"))]; from != nil && lineLeads(l) && byID[keyOf(l.get("to"))] != nil {
			out[from] = append(out[from], l)
		}
	}
	tr := trail{reached: map[*Map]bool{source: true}, stopped: map[*Map]bool{}, carries: map[*Map]bool{},
		cleared: map[*Map]bool{}, unstopped: map[*Map]bool{}, accepted: map[[2]*Map]bool{}}
	type step struct {
		from *Map
		line *Map
		to   *Map
	}
	var steps []step
	breached := map[*Map]bool{}
	seen := map[*Map]bool{source: true}
	for queue := []*Map{source}; len(queue) > 0; queue = queue[1:] {
		at := queue[0]
		for _, l := range out[at] {
			to := byID[keyOf(l.get("to"))]
			tr.carries[l] = true
			steps = append(steps, step{at, l, to})
			if to.get("kind") == "outside" {
				if at.get("kind") == "approval" {
					tr.accepted[[2]*Map{to, at}] = true
				} else if !breached[l] {
					breached[l] = true
					tr.breaches = append(tr.breaches, l)
				}
			}
			if to.get("kind") == "surface" && at.get("kind") != "approval" {
				tr.shown = append(tr.shown, l)
			}
			if stops(to) {
				tr.stopped[to] = true
				continue
			}
			tr.reached[to] = true
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	for p := range tr.stopped {
		for _, l := range out[p] {
			tr.cleared[l] = true
		}
	}
	// Each step that leads on to a breach is on the text's way to it, back to the source.
	breach := func(s step) bool { return s.from.get("kind") != "approval" && s.to.get("kind") == "outside" }
	bad := map[*Map]bool{}
	for changed := true; changed; {
		changed = false
		for _, s := range steps {
			if (breach(s) || bad[s.to]) && !bad[s.from] {
				bad[s.from], changed = true, true
			}
		}
	}
	for _, s := range steps {
		if breach(s) || bad[s.to] {
			tr.unstopped[s.line] = true
		}
	}
	return tr
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

// The words awml check adds after a line that brings untrusted text into an outside action with
// nothing to stop it or accept the risk.
const untrustedAdvice = "a review, a person's own step or a code step marked cleans on the way would stop it, or a standing approval in front of the action would accept the risk"

// untrustedBreach gives the words for untrusted text from the parts named that reaches an outside
// action with nothing to stop it or accept the risk.
func untrustedBreach(sources []string, action string) string {
	what := "part"
	if len(sources) > 1 {
		what = "parts"
	}
	return fmt.Sprintf(`untrusted text from %s %s reaches outside action "%s" with nothing to stop it or accept the risk`, what, naturalList(sources), action)
}

// untrustedOutside reports each outside action that untrusted text reaches with nothing on the way
// to stop it or accept the risk, with each line that brings it there, naming the parts the text
// comes from. Such a line breaks a rule, so it's drawn red.
func untrustedOutside(parts, lines []*Map) mistakeList {
	from, into := untrustedInto(parts, lines, func(tr trail) []*Map { return tr.breaches })
	var errors mistakeList
	for _, p := range parts {
		if from[p] == nil {
			continue
		}
		var breaching []*Map
		for _, l := range lines {
			if into[p][l] {
				breaching = append(breaching, l)
			}
		}
		errors.addBreach("untrusted.outside", untrustedBreach(from[p], show(p.get("id"))), untrustedAdvice, breaching...)
	}
	return errors
}

// untrustedInto gives each part that untrusted text comes into along the lines along picks from its
// trail, with the parts it comes from, named in quotes in the file's order, and those lines.
func untrustedInto(parts, lines []*Map, along func(trail) []*Map) (from map[*Map][]string, into map[*Map]map[*Map]bool) {
	byID := partsOf(parts)
	from, into = map[*Map][]string{}, map[*Map]map[*Map]bool{}
	for _, source := range untrustedParts(parts) {
		for _, l := range along(follow(source, parts, lines)) {
			to := byID[keyOf(l.get("to"))]
			if into[to] == nil {
				into[to] = map[*Map]bool{}
			}
			if name := fmt.Sprintf(`"%s"`, show(source.get("id"))); !isOneOf(name, from[to]...) {
				from[to] = append(from[to], name)
			}
			into[to][l] = true
		}
	}
	return from, into
}

// The words awml check adds after a warning that untrusted text reaches a surface.
const surfaceAdvice = "people reading it see text a stranger could have written, and a review, a person's own step or a code step marked cleans on the way would stop it"

// untrustedShown gives the words for untrusted text from the parts named that reaches a surface.
func untrustedShown(sources []string, surface string) string {
	what := "part"
	if len(sources) > 1 {
		what = "parts"
	}
	return fmt.Sprintf(`untrusted text from %s %s reaches surface "%s": %s`, what, naturalList(sources), surface, surfaceAdvice)
}

// untrustedSurfaces warns of each surface that untrusted text reaches with nothing on the way to stop
// it, naming the parts the text comes from. A standing approval's line to the surface that lists
// what went out never warns, since showing that is the line's job.
func untrustedSurfaces(parts, lines []*Map) mistakeList {
	from, _ := untrustedInto(parts, lines, func(tr trail) []*Map { return tr.shown })
	var warnings mistakeList
	for _, p := range parts {
		if from[p] != nil {
			warnings.add("warn.untrusted-surface", untrustedShown(from[p], show(p.get("id"))))
		}
	}
	return warnings
}

// partsOf indexes parts by id.
func partsOf(parts []*Map) map[any]*Map {
	byID := map[any]*Map{}
	for _, p := range parts {
		byID[keyOf(p.get("id"))] = p
	}
	return byID
}

// untrustedLooks gives how each view draws untrusted text: for each line it touches, carries where
// the text goes along it, stopped where it leaves what stopped the text, and unstopped where it's
// on the text's way to an outside action that nothing stops and nobody accepted, which wins; and
// each part marked untrusted whose text goes that way.
func untrustedLooks(parts, lines []*Map) (looks map[*Map]string, unstopped map[*Map]bool) {
	looks, unstopped = map[*Map]string{}, map[*Map]bool{}
	rank := map[string]int{"stopped": 1, "carries": 2, "unstopped": 3}
	mark := func(l *Map, look string) {
		if rank[look] > rank[looks[l]] {
			looks[l] = look
		}
	}
	for _, source := range untrustedParts(parts) {
		tr := follow(source, parts, lines)
		for l := range tr.cleared {
			mark(l, "stopped")
		}
		for l := range tr.carries {
			mark(l, "carries")
		}
		for l := range tr.unstopped {
			mark(l, "unstopped")
		}
		if tr.breaches != nil {
			unstopped[source] = true
		}
	}
	return looks, unstopped
}

// UntrustedSummary names one untrusted part, the steps and outside actions its text reaches, the
// reviews and steps that stop it, and each outside action it reaches under a standing approval
// that accepts the risk. A step that stops the text takes it in, so it is among the steps too.
type UntrustedSummary struct {
	Part           string         `json:"part"`
	Steps          []string       `json:"steps"`
	OutsideActions []string       `json:"outsideActions"`
	StoppedBy      []string       `json:"stoppedBy"`
	RisksAccepted  []AcceptedRisk `json:"risksAccepted"`
}

// AcceptedRisk is an outside action that untrusted text reaches, and the standing approval in front
// of it that accepts the risk, whose label says who accepted it and on what evidence.
type AcceptedRisk struct {
	OutsideAction string `json:"outsideAction"`
	Approval      string `json:"approval"`
}

func summariseUntrusted(parts, lines []*Map) []UntrustedSummary {
	var out []UntrustedSummary
	for _, source := range untrustedParts(parts) {
		tr := follow(source, parts, lines)
		entry := UntrustedSummary{Part: str(source, "label")}
		for _, p := range parts {
			label := str(p, "label")
			if (tr.reached[p] || tr.stopped[p]) && steps[str(p, "kind")] {
				entry.Steps = append(entry.Steps, label)
			}
			if tr.reached[p] && p.get("kind") == "outside" {
				entry.OutsideActions = append(entry.OutsideActions, label)
			}
			if tr.stopped[p] {
				entry.StoppedBy = append(entry.StoppedBy, label)
			}
		}
		for _, action := range parts {
			for _, approval := range parts {
				if tr.accepted[[2]*Map{action, approval}] {
					entry.RisksAccepted = append(entry.RisksAccepted, AcceptedRisk{str(action, "label"), str(approval, "label")})
				}
			}
		}
		out = append(out, entry)
	}
	return out
}
