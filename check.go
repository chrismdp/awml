package awml

import (
	"fmt"
	"regexp"
	"sort"
)

// Check finds the structural mistakes in an AWML document: what would draw wrongly, or not at all.
func Check(doc any) []string {
	file, ok := doc.(*Map)
	if !ok {
		return []string{"the file must be a mapping of fields, starting with awml"}
	}
	var errors []string
	fields("file", "the file", file, &errors)
	if file.has("awml") && show(file.get("awml")) != version {
		errors = append(errors, fmt.Sprintf(`awml must be "%s", not "%s"`, version, show(file.get("awml"))))
	}
	if file.has("view") && !isOneOf(file.get("view"), "scopes", "where") {
		errors = append(errors, fmt.Sprintf(`view must be "scopes" or "where", not "%s"`, show(file.get("view"))))
	}
	var lists [3][]*Map
	var broken []string
	for n, kw := range [3][2]string{{"groups", "group"}, {"parts", "part"}, {"lines", "line"}} {
		items := file.get(kw[0])
		if empty(items) {
			continue
		}
		list, ok := items.([]any)
		if !ok {
			broken = append(broken, kw[0]+" must be a list")
			continue
		}
		for _, item := range list {
			m, ok := item.(*Map)
			if !ok {
				broken = append(broken, fmt.Sprintf("each %s must be a mapping of fields", kw[1]))
				break
			}
			lists[n] = append(lists[n], m)
		}
	}
	// Past here every list is sound, or each line would report a part it can't find.
	if broken != nil {
		return append(errors, broken...)
	}
	groups, parts, lines := lists[0], lists[1], lists[2]

	seen := map[any]bool{}
	for _, item := range append(append([]*Map{}, groups...), parts...) {
		i := item.get("id")
		if i == nil {
			continue
		}
		if seen[keyOf(i)] {
			errors = append(errors, fmt.Sprintf(`id "%s" is used twice`, show(i)))
		}
		seen[keyOf(i)] = true
		if s, ok := i.(string); ok && !id.MatchString(s) {
			errors = append(errors, fmt.Sprintf(`id "%s" must be letters, digits and underscores, starting with a letter`, s))
		}
	}
	groupIDs := map[any]bool{}
	for _, g := range groups {
		groupIDs[keyOf(g.get("id"))] = true
	}
	kinds := map[any]any{}
	for _, p := range parts {
		kinds[keyOf(p.get("id"))] = p.get("kind")
	}

	for _, g := range groups {
		name := fmt.Sprintf(`group "%s"`, show(g.get("id")))
		fields("group", name, g, &errors)
		if in := g.get("in"); in != nil && !groupIDs[keyOf(in)] {
			errors = append(errors, fmt.Sprintf(`%s is in unknown group "%s"`, name, show(in)))
		}
	}
	parent := map[any]any{}
	for _, g := range groups {
		parent[keyOf(g.get("id"))] = g.get("in")
	}
	for _, g := range groups {
		self := keyOf(g.get("id"))
		at, path := parent[self], map[any]bool{}
		for at != nil && !path[keyOf(at)] {
			if _, ok := parent[keyOf(at)]; !ok {
				break
			}
			if keyOf(at) == self {
				errors = append(errors, fmt.Sprintf(`group "%s" is nested in itself`, show(g.get("id"))))
				break
			}
			path[keyOf(at)] = true
			at = parent[keyOf(at)]
		}
	}

	for _, p := range parts {
		name := fmt.Sprintf(`part "%s"`, show(p.get("id")))
		fields("part", name, p, &errors)
		if p.has("kind") {
			if k, ok := p.get("kind").(string); !ok || shapes[k] == "" {
				errors = append(errors, fmt.Sprintf(`%s has unknown kind "%s"`, name, show(p.get("kind"))))
			}
		}
		if in := p.get("in"); in != nil && !groupIDs[keyOf(in)] {
			errors = append(errors, fmt.Sprintf(`%s is in unknown group "%s"`, name, show(in)))
		}
	}

	for n, l := range lines {
		name := fmt.Sprintf("line %d", n+1)
		if l.has("from") && l.has("to") {
			name = fmt.Sprintf(`the line from "%s" to "%s"`, show(l.get("from")), show(l.get("to")))
		}
		fields("line", name, l, &errors)
		if b := l.get("breaks"); b != nil {
			if _, ok := b.(bool); !ok {
				errors = append(errors, fmt.Sprintf(`%s: breaks must be true or false, not "%s"`, name, show(b)))
			}
		}
		for _, end := range [2][2]string{{"from", "starts"}, {"to", "ends"}} {
			if _, ok := kinds[keyOf(l.get(end[0]))]; l.has(end[0]) && !ok {
				errors = append(errors, fmt.Sprintf(`%s %s at unknown part "%s"`, name, end[1], show(l.get(end[0]))))
			}
		}
		kind := l.get("kind")
		if kind == nil {
			continue
		}
		k, ok := kind.(string)
		if !ok || arrows[k] == "" {
			errors = append(errors, fmt.Sprintf(`%s has unknown kind "%s"`, name, show(kind)))
			continue
		}
		var comments []any
		for _, end := range []string{"from", "to"} {
			if kinds[keyOf(l.get(end))] == "comment" {
				comments = append(comments, l.get(end))
			}
		}
		// A hidden line only steers the layout, so it links nothing.
		if k != "tie" && k != "hidden" {
			for _, c := range comments {
				errors = append(errors, fmt.Sprintf(`comment "%s" is linked by a %s line: a comment takes only tie`, show(c), k))
			}
		}
		if k == "tie" && len(comments) == 0 {
			errors = append(errors, fmt.Sprintf(`the tie from "%s" to "%s" has no comment at either end`, show(l.get("from")), show(l.get("to"))))
		}
	}
	return errors
}

const version = "0.1"

var fieldSets = map[string][2][]string{
	"file":  {{"awml", "title", "view", "parts"}, {"groups", "lines"}},
	"group": {{"id", "label"}, {"in"}},
	"part":  {{"id", "kind", "label"}, {"in"}},
	"line":  {{"from", "to", "kind"}, {"label", "breaks"}},
}

// The fields that must be text, when they're given at all.
var textFields = map[string]bool{"title": true, "view": true, "id": true, "label": true, "in": true, "kind": true, "from": true, "to": true}

// Mermaid reads anything else in an id as part of the syntax around it.
var id = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// fields reports what an item lacks, what it has that it shouldn't, and what YAML didn't read as text.
func fields(what, name string, item *Map, errors *[]string) {
	need, may := setOf(fieldSets[what][0]), setOf(fieldSets[what][1])
	// A fork is a bare bar, so it needs no label.
	if what == "part" && item.get("kind") == "fork" {
		delete(need, "label")
		may["label"] = true
	}
	var missing []string
	for f := range need {
		if !item.has(f) {
			missing = append(missing, f)
		}
	}
	sort.Strings(missing)
	for _, f := range missing {
		*errors = append(*errors, fmt.Sprintf("%s has no %s", name, f))
	}
	for _, f := range item.Keys() {
		if !need[f] && !may[f] {
			*errors = append(*errors, fmt.Sprintf(`%s has unknown field "%s"`, name, f))
		}
	}
	// YAML reads true and false, numbers and dates as something other than text.
	for _, f := range item.Keys() {
		v := item.get(f)
		if _, ok := v.(string); textFields[f] && v != nil && !ok {
			*errors = append(*errors, fmt.Sprintf("%s: YAML read its %s as %s, not text: put it in quotes", name, f, show(v)))
		}
	}
}

func setOf(items []string) map[string]bool {
	s := map[string]bool{}
	for _, i := range items {
		s[i] = true
	}
	return s
}

func isOneOf(v any, choices ...string) bool {
	for _, c := range choices {
		if v == c {
			return true
		}
	}
	return false
}

// empty is whether YAML gave nothing for a list: null, an empty list, or a value that is false or zero.
func empty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case []any:
		return len(v) == 0
	case *Map:
		return len(v.keys) == 0
	case string:
		return v == ""
	case bool:
		return !v
	case int:
		return v == 0
	case float64:
		return v == 0
	}
	return false
}

// keyOf turns a value into something a Go map can hold, as a list or a mapping can't be.
func keyOf(v any) any {
	switch v.(type) {
	case []any, *Map:
		return fmt.Sprintf("%T %s", v, show(v))
	}
	return v
}
