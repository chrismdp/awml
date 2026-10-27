package awml

import (
	"fmt"
	"strings"
)

// carriesRun is whether a run goes on through a part: a step, a fork, a review or a standing
// approval, which pass the work along. A context, connector, surface or outside action ends it.
var carriesRun = setOf([]string{"ai", "code", "person", "fork", "review", "approval"})

// runs gives each trigger's run: every part that carries a run on reached from it by start lines,
// out of the trigger and out of each part reached. A data line says what a step reads, not what
// runs next, so it doesn't carry a run on.
func runs(triggers, parts, lines []*Map) map[*Map]map[*Map]bool {
	byID := map[any]*Map{}
	for _, p := range parts {
		byID[keyOf(p.get("id"))] = p
	}
	found := map[*Map]map[*Map]bool{}
	for _, trigger := range triggers {
		run := map[*Map]bool{}
		for queue := []*Map{trigger}; len(queue) > 0; queue = queue[1:] {
			for _, l := range lines {
				to := byID[keyOf(l.get("to"))]
				if l.get("kind") != "start" || to == nil || byID[keyOf(l.get("from"))] != queue[0] || run[to] || !carriesRun[str(to, "kind")] {
					continue
				}
				run[to] = true
				queue = append(queue, to)
			}
		}
		found[trigger] = run
	}
	return found
}

// shapeWarnings warns of what makes a drawing hard to follow: a file that holds several workflows,
// a part linked to no trigger, a comment tied to nothing and a step no trigger starts.
func shapeWarnings(file *Map, parts, lines []*Map) mistakeList {
	var warnings mistakeList
	var triggers []*Map
	for _, p := range parts {
		if p.get("kind") == "trigger" {
			triggers = append(triggers, p)
		}
	}
	started := runs(triggers, parts, lines)

	// Two triggers belong together when their runs share a part, so each group is the triggers
	// joined through the parts their runs reach.
	leader := map[*Map]*Map{}
	var lead func(*Map) *Map
	lead = func(t *Map) *Map {
		if leader[t] == nil || leader[t] == t {
			return t
		}
		return lead(leader[t])
	}
	for i, trigger := range triggers {
		for _, other := range triggers[:i] {
			for p := range started[trigger] {
				if started[other][p] {
					// The earlier trigger leads the group, so the groups keep the file's order.
					a, b := lead(other), lead(trigger)
					if indexOf(triggers, b) < indexOf(triggers, a) {
						a, b = b, a
					}
					leader[b] = a
					break
				}
			}
		}
	}
	labels := map[*Map][]string{}
	var leaders []*Map
	for _, t := range triggers {
		l := lead(t)
		if labels[l] == nil {
			leaders = append(leaders, l)
		}
		labels[l] = append(labels[l], fmt.Sprintf(`"%s"`, show(t.get("label"))))
	}
	var named []string
	for _, l := range leaders {
		named = append(named, "the one started by "+strings.Join(labels[l], " or "))
	}
	if len(named) > 1 {
		warnings.add("warn.one-workflow", fmt.Sprintf("this file holds %d workflows: %s", len(named), naturalList(named)))
	}

	// A part is linked to a trigger by lines followed either way, leaving out comments, their ties
	// and hidden lines, which only steer the layout.
	byID := map[any]*Map{}
	for _, p := range parts {
		byID[keyOf(p.get("id"))] = p
	}
	linked := map[*Map]bool{}
	queue := append([]*Map(nil), triggers...)
	for _, t := range triggers {
		linked[t] = true
	}
	for ; len(queue) > 0; queue = queue[1:] {
		for _, l := range lines {
			if isOneOf(l.get("kind"), "tie", "hidden") {
				continue
			}
			from, to := byID[keyOf(l.get("from"))], byID[keyOf(l.get("to"))]
			if from == nil || to == nil || from.get("kind") == "comment" || to.get("kind") == "comment" {
				continue
			}
			for _, pair := range [][2]*Map{{from, to}, {to, from}} {
				if pair[0] == queue[0] && !linked[pair[1]] {
					linked[pair[1]] = true
					queue = append(queue, pair[1])
				}
			}
		}
	}
	inRun := map[*Map]bool{}
	for _, run := range started {
		for p := range run {
			inRun[p] = true
		}
	}
	tied := map[*Map]bool{}
	for _, l := range lines {
		if l.get("kind") != "tie" {
			continue
		}
		from, to := byID[keyOf(l.get("from"))], byID[keyOf(l.get("to"))]
		if from != nil && to != nil {
			tied[from], tied[to] = true, true
		}
	}

	for _, p := range parts {
		id := show(p.get("id"))
		switch kind := str(p, "kind"); {
		case kind == "comment":
			if !tied[p] {
				warnings.add("warn.untied-comment", untied(id))
			}
		case !linked[p]:
			warnings.add("warn.unreached", fmt.Sprintf(`part "%s" is linked to no trigger: connect it to the work, or remove it`, id))
		case steps[kind] && !inRun[p]:
			warnings.add("warn.unstarted", fmt.Sprintf(`step "%s" is started by no trigger: add the start line that leads into it, or, if it begins work of its own, move it into another workflow that an outside action starts`, id))
		}
	}

	// An implementation's comment is tied by its ties.
	if implementations, ok := file.get("implementations").(*Map); ok {
		for _, key := range implementations.keys {
			implementation, _ := implementations.get(key).(*Map)
			if implementation == nil {
				continue
			}
			comments, _ := implementation.get("comments").([]any)
			for _, value := range comments {
				if comment, ok := value.(*Map); ok {
					if ties, _ := comment.get("ties").([]any); len(ties) == 0 {
						warnings.add("warn.untied-comment", untied(show(comment.get("id"))))
					}
				}
			}
		}
	}
	return warnings
}

func untied(id string) string {
	return fmt.Sprintf(`comment "%s" is tied to nothing: tie it to the part it explains, or remove it`, id)
}

func indexOf(parts []*Map, p *Map) int {
	for i, q := range parts {
		if q == p {
			return i
		}
	}
	return -1
}
