package awml

import (
	"fmt"
	"strings"
)

// publishedLines gives a workflow's lines, with each data line of a 0.6 file that publishes made a
// publish line, its label followed by what the publish adds or loosens. Older files keep the lines
// they wrote. The file may still have mistakes, so a line that isn't sound is left as it is, for the
// checks to report.
func publishedLines(doc *Map) any {
	lines := doc.get("lines")
	list, ok := lines.([]any)
	if doc.get("awml") != "0.6" || !ok {
		return lines
	}
	scopes, parts := soundMaps(doc.get("scopes")), soundMaps(doc.get("parts"))
	byID, parent, children := map[string]*Map{}, map[string]string{}, map[string][]string{}
	for _, s := range scopes {
		byID[str(s, "id")] = s
		parent[str(s, "id")] = str(s, "in")
		children[str(s, "in")] = append(children[str(s, "in")], str(s, "id"))
	}
	known := map[string]bool{}
	for _, p := range parts {
		known[str(p, "id")] = true
	}
	where := placed(parts, soundMaps(lines))
	out := make([]any, len(list))
	for i, item := range list {
		out[i] = item
		l, ok := item.(*Map)
		if !ok || l.get("kind") != "data" || l.get("breaks") == true || !known[str(l, "from")] || !known[str(l, "to")] {
			continue
		}
		if _, text := l.get("label").(string); l.has("label") && !text {
			continue
		}
		source, target := where[str(l, "from")], where[str(l, "to")]
		if source != "" && byID[source] == nil || target != "" && byID[target] == nil || insideBox(parent, target, source) {
			continue
		}
		said := []string{}
		if label := str(l, "label"); label != "" {
			said = append(said, label)
		}
		said = append(said, publishes(byID, children, source, target)...)
		line := copyMap(l)
		line.set("kind", "publish")
		line.set("label", strings.Join(said, " · "))
		out[i] = line
	}
	return out
}

// publishes says what a move from one scope into another, which isn't inside it, adds or loosens:
// who the target holds that the source doesn't, and the source's terms and the target's when the
// target's allow more. Where the scopes say neither, it names the target.
func publishes(byID map[string]*Map, children map[string][]string, source, target string) []string {
	if target == "" {
		return []string{"publishes outside every scope"}
	}
	var said []string
	from, fromKnown := members(byID, children, source, map[string]bool{})
	to, toKnown := members(byID, children, target, map[string]bool{})
	if fromKnown && toKnown {
		had := setOf(from)
		var added []string
		for _, name := range to {
			if !had[name] {
				added = append(added, name)
			}
		}
		if added != nil {
			said = append(said, "adds "+naturalList(added))
		}
	}
	if byID[source].has("processing") && byID[target].has("processing") {
		was, _ := readProcessing(byID[source].get("processing"), "")
		now, _ := readProcessing(byID[target].get("processing"), "")
		if was.valid && now.valid && loosens(was, now) {
			said = append(said, fmt.Sprintf("loosens %s to %s", terms(was), terms(now)))
		}
	}
	if said == nil {
		said = []string{"publishes into " + scopeName(str(byID[target], "label"))}
	}
	return said
}

// members gives the people in a scope, from its label: the names after its colon, and, when they
// start with +, everyone in the scopes inside it too. It's false when the label doesn't say.
func members(byID map[string]*Map, children map[string][]string, scope string, seen map[string]bool) ([]string, bool) {
	if byID[scope] == nil || seen[scope] {
		return nil, false
	}
	seen[scope] = true
	_, list, found := strings.Cut(str(byID[scope], "label"), ":")
	list = strings.TrimSpace(list)
	adds := strings.HasPrefix(list, "+")
	var names []string
	for _, name := range strings.Split(strings.TrimPrefix(list, "+"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if !found || names == nil && !adds {
		return nil, false
	}
	if !adds {
		return names, true
	}
	var all []string
	for _, inner := range children[scope] {
		more, ok := members(byID, children, inner, seen)
		if !ok {
			return nil, false
		}
		all = append(all, more...)
	}
	in := setOf(all)
	for _, name := range names {
		if !in[name] {
			all = append(all, name)
			in[name] = true
		}
	}
	return all, true
}

// loosens is whether the terms a move goes to allow a company or an operation that those it comes
// from don't.
func loosens(was, now processingTerms) bool {
	if was.all {
		return false
	}
	if now.all {
		return true
	}
	for company, allowed := range now.companies {
		had, ok := was.companies[company]
		switch {
		case !ok:
			return true
		case had.all:
			continue
		case allowed.all:
			return true
		}
		for op := range allowed.ops {
			if !had.ops[op] {
				return true
			}
		}
	}
	return false
}

// terms says in words which companies a scope's processing allows.
func terms(t processingTerms) string {
	if t.all {
		return "any company"
	}
	if len(t.companies) == 0 {
		return "no company"
	}
	var allowed []string
	for _, company := range sortedCompanyKeys(t.companies) {
		if ops := t.companies[company]; ops.all {
			allowed = append(allowed, company)
		} else {
			allowed = append(allowed, company+" for "+naturalList(sortedKeys(ops.ops)))
		}
	}
	return naturalList(allowed) + " only"
}

// insideBox is whether a box is the outer one or sits inside it. "" is outside every box, so every
// box is inside it, and seen stops a box nested in itself.
func insideBox(parent map[string]string, box, outer string) bool {
	for seen := map[string]bool{}; outer != "" && !seen[box]; box = parent[box] {
		if box == outer {
			return true
		}
		seen[box] = true
	}
	return outer == ""
}

// soundMaps gives the mappings in a list, skipping anything else, for a file not yet checked.
func soundMaps(v any) []*Map {
	list, _ := v.([]any)
	var maps []*Map
	for _, item := range list {
		if m, ok := item.(*Map); ok {
			maps = append(maps, m)
		}
	}
	return maps
}
