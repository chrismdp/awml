package awml

import (
	"fmt"
	"strings"
)

// Warnings finds choices that are valid AWML but are usually better split into smaller steps, and
// each outside action that untrusted text reaches.
func Warnings(doc any) []string {
	return warningList(doc).messages()
}

func warningList(doc any) mistakeList {
	file, ok := doc.(*Map)
	if !ok || !IsCanonical(file) {
		return nil
	}
	partValues, ok := file.get("parts").([]any)
	if !ok {
		return nil
	}
	parts := make([]*Map, 0, len(partValues))
	for _, value := range partValues {
		part, ok := value.(*Map)
		if !ok {
			return nil
		}
		parts = append(parts, part)
	}
	var lines []*Map
	if lineValues, ok := file.get("lines").([]any); ok {
		for _, value := range lineValues {
			if line, ok := value.(*Map); ok {
				lines = append(lines, line)
			}
		}
	}

	byID := map[any]*Map{}
	for _, part := range parts {
		byID[keyOf(part.get("id"))] = part
	}
	var warnings mistakeList
	for _, step := range parts {
		if step.get("op") != "do" || !steps[str(step, "kind")] {
			continue
		}
		id := show(step.get("id"))
		if step.get("kind") == "ai" {
			warnings.add("warn.do-by-code", fmt.Sprintf(`step "%s" is a do step decided by AI: a do step should be code acting on what an earlier step decided`, id))
		}

		reached := map[*Map]bool{}
		seen := map[*Map]bool{step: true}
		var follow func(*Map)
		follow = func(from *Map) {
			for _, line := range lines {
				if !lineLeads(line) || byID[keyOf(line.get("from"))] != from {
					continue
				}
				to := byID[keyOf(line.get("to"))]
				if to == nil {
					continue
				}
				switch to.get("kind") {
				case "review", "approval", "fork":
					if !seen[to] {
						seen[to] = true
						follow(to)
					}
				case "surface", "context":
					// A standing approval's surface lists what it covers, which is no change of the do step's.
					if from.get("kind") != "approval" {
						reached[to] = true
					}
				case "outside":
					reached[to] = true
				}
			}
		}
		follow(step)

		changes, hasContext := 0, false
		var names []string
		for _, part := range parts {
			if !reached[part] {
				continue
			}
			names = append(names, fmt.Sprintf(`"%s"`, show(part.get("id"))))
			switch part.get("kind") {
			case "outside", "surface":
				changes++
			case "context":
				hasContext = true
			}
		}
		if hasContext {
			changes++
		}
		if changes >= 2 {
			warnings.add("warn.one-change", fmt.Sprintf(`do step "%s" leads to %d changes (%s): each do step should make one`, id, changes, strings.Join(names, ", ")))
		}
	}
	return append(warnings, untrustedWarnings(parts, lines)...)
}
