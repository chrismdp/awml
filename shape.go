package awml

import (
	"fmt"
	"strings"
)

// shapeWarnings finds what makes a drawing hard to follow. A trigger's run is every part reached
// from it along start lines, whatever kind of part each is. Triggers whose runs share a part are
// one workflow, so triggers that fall into more than one group mean the file holds several. It
// also warns of each part no line links to a trigger, following lines either way and leaving out
// comments and their ties, of each comment tied to nothing, and of each step, review or standing
// approval that is in no trigger's run.
func shapeWarnings(parts, lines []*Map) mistakeList {
	byID := map[any]*Map{}
	for _, part := range parts {
		byID[keyOf(part.get("id"))] = part
	}
	ends := func(line *Map) (*Map, *Map) {
		return byID[keyOf(line.get("from"))], byID[keyOf(line.get("to"))]
	}
	var triggers []*Map
	for _, part := range parts {
		if part.get("kind") == "trigger" {
			triggers = append(triggers, part)
		}
	}

	// Each trigger's run, and which group of triggers it falls in.
	inRun := map[*Map]bool{}
	group := make([]int, len(triggers))
	firstIn := map[*Map]int{}
	for n, trigger := range triggers {
		group[n] = n
		run := map[*Map]bool{trigger: true}
		queue := []*Map{trigger}
		for len(queue) > 0 {
			from := queue[0]
			queue = queue[1:]
			for _, line := range lines {
				if line.get("kind") != "start" {
					continue
				}
				if f, to := ends(line); f == from && to != nil && !run[to] {
					run[to] = true
					queue = append(queue, to)
				}
			}
		}
		for _, part := range parts {
			if !run[part] || part == trigger {
				continue
			}
			inRun[part] = true
			if m, ok := firstIn[part]; ok {
				// Join the two groups, keeping the lower number for both.
				a, b := group[m], group[n]
				if a > b {
					a, b = b, a
				}
				for i := range group {
					if group[i] == b {
						group[i] = a
					}
				}
			} else {
				firstIn[part] = n
			}
		}
	}

	var warnings mistakeList
	var groups []string
	for n := range triggers {
		if group[n] != n {
			continue
		}
		var labels []string
		for m, trigger := range triggers {
			if group[m] == n {
				labels = append(labels, fmt.Sprintf(`"%s"`, show(trigger.get("label"))))
			}
		}
		groups = append(groups, "the one started by "+strings.Join(labels, " or "))
	}
	if len(groups) > 1 {
		warnings.add("warn.several-workflows", fmt.Sprintf(`this file holds %d workflows: %s: split it at its triggers into workflows that refer to each other`,
			len(groups), naturalList(groups)))
	}

	// Every part a line links to a trigger, either way and through any parts, leaving out comments.
	linked := map[*Map]bool{}
	queue := append([]*Map{}, triggers...)
	for _, trigger := range triggers {
		linked[trigger] = true
	}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, line := range lines {
			from, to := ends(line)
			if from == nil || to == nil || from.get("kind") == "comment" || to.get("kind") == "comment" {
				continue
			}
			for _, pair := range [2][2]*Map{{from, to}, {to, from}} {
				if pair[0] == at && !linked[pair[1]] {
					linked[pair[1]] = true
					queue = append(queue, pair[1])
				}
			}
		}
	}

	runs := map[any]string{"ai": "step", "code": "step", "person": "step", "review": "review", "approval": "standing approval"}
	for _, part := range parts {
		id := show(part.get("id"))
		kind := part.get("kind")
		if kind == "comment" {
			tied := false
			for _, line := range lines {
				from, to := ends(line)
				tied = tied || line.get("kind") == "tie" && from != nil && to != nil && (from == part || to == part)
			}
			if !tied {
				warnings.add("warn.untied-comment", fmt.Sprintf(`comment "%s" is tied to no part: tie it to what it explains, or remove it`, id))
			}
			continue
		}
		if !linked[part] {
			warnings.add("warn.unlinked-part", fmt.Sprintf(`part "%s" has no line linking it to a trigger: connect it or remove it`, id))
		}
		if what := runs[kind]; what != "" && !inRun[part] {
			warnings.add("warn.unstarted-step", fmt.Sprintf(`%s "%s" is started by no trigger: add the start line it's missing, or split its work into a workflow of its own that an outside action starts`, what, id))
		}
	}
	return warnings
}
