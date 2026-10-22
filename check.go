package awml

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Check finds the structural mistakes in an AWML document: what would draw wrongly, or not at all.
func Check(doc any) []string {
	file, ok := doc.(*Map)
	if !ok {
		return []string{"the file must be a mapping of fields, starting with awml"}
	}
	if IsCanonical(file) {
		return checkCanonical(file)
	}
	if file.has("platform") {
		return checkPlatformDescription(file)
	}
	var errors []string
	fileVersion := show(file.get("awml"))
	fields("file", "the file", file, fileVersion, &errors)
	if file.has("awml") && fileVersion != "0.2" && fileVersion != "0.3" {
		errors = append(errors, fmt.Sprintf(`awml must be "0.2" or "0.3", not "%s"`, fileVersion))
	}
	if file.has("view") && !isOneOf(file.get("view"), "security", "platform") {
		errors = append(errors, fmt.Sprintf(`view must be "security" or "platform", not "%s"`, show(file.get("view"))))
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
		fields("group", name, g, fileVersion, &errors)
		if in := g.get("in"); in != nil && !groupIDs[keyOf(in)] {
			errors = append(errors, fmt.Sprintf(`%s is in unknown group "%s"`, name, show(in)))
		}
		if fileVersion == "0.3" {
			if g.has("processing") {
				if file.get("view") != "security" {
					errors = append(errors, name+" has processing, but only a group in the security view takes it")
				} else {
					_, processingErrors := readProcessing(g.get("processing"), name)
					errors = append(errors, processingErrors...)
				}
			}
			if g.has("platform") {
				if file.get("view") != "platform" {
					errors = append(errors, name+" has a platform, but only a group in the platform view takes one")
				} else if s, ok := g.get("platform").(string); !ok {
					errors = append(errors, fmt.Sprintf("%s: YAML read its platform as %s, not text: put it in quotes", name, show(g.get("platform"))))
				} else if !companyID.MatchString(s) {
					errors = append(errors, fmt.Sprintf(`%s: platform "%s" must be a company id`, name, s))
				}
			}
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
		fields("part", name, p, fileVersion, &errors)
		if p.has("kind") {
			if k, ok := p.get("kind").(string); !ok || shapes[k] == "" {
				errors = append(errors, fmt.Sprintf(`%s has unknown kind "%s"`, name, show(p.get("kind"))))
			}
		}
		if in := p.get("in"); in != nil && !groupIDs[keyOf(in)] {
			errors = append(errors, fmt.Sprintf(`%s is in unknown group "%s"`, name, show(in)))
		}
		if op := p.get("op"); op != nil {
			if k, _ := p.get("kind").(string); !steps[k] {
				errors = append(errors, name+" has an op, but only an ai, code or person step takes one")
			} else if s, _ := op.(string); ops[s] == "" {
				errors = append(errors, fmt.Sprintf(`%s has unknown op "%s"`, name, show(op)))
			}
		}
		if fileVersion == "0.3" && p.has("wait") {
			if p.get("kind") != "trigger" {
				errors = append(errors, name+" has a wait, but only a trigger takes one")
			} else if wait, ok := p.get("wait").(string); !ok || !validDuration(wait) {
				errors = append(errors, fmt.Sprintf(`%s: wait must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "%s"`, name, show(p.get("wait"))))
			}
		}
		if fileVersion == "0.3" && p.has("at") {
			if p.get("kind") != "context" {
				errors = append(errors, name+" has an at, but only a context takes one")
			} else if at, ok := p.get("at").(string); !ok || !validPath(at) {
				errors = append(errors, fmt.Sprintf(`%s: at must be a path, not "%s"`, name, show(p.get("at"))))
			}
		}
	}

	for n, l := range lines {
		name := fmt.Sprintf("line %d", n+1)
		if l.has("from") && l.has("to") {
			name = fmt.Sprintf(`the line from "%s" to "%s"`, show(l.get("from")), show(l.get("to")))
		}
		fields("line", name, l, fileVersion, &errors)
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
	errors = append(errors, operations(parts, lines)...)
	if file.get("view") == "security" {
		errors = append(errors, scopes(groups, parts, lines)...)
		if fileVersion == "0.3" {
			errors = append(errors, nestedProcessing(groups)...)
		}
	}
	return errors
}

type processingTerms struct {
	all       bool
	companies map[string]operationTerms
	valid     bool
}

type operationTerms struct {
	all bool
	ops map[string]bool
}

func readProcessing(value any, name string) (processingTerms, []string) {
	terms := processingTerms{companies: map[string]operationTerms{}, valid: true}
	if value == "any" {
		terms.all = true
		return terms, nil
	}
	if value == "none" {
		return terms, nil
	}
	mapping, ok := value.(*Map)
	if !ok {
		terms.valid = false
		return terms, []string{name + ": processing must be any, none or a map from company ids to any or operations"}
	}
	var errors []string
	for _, company := range mapping.Keys() {
		if mapping.nonTextKeys[company] {
			errors = append(errors, fmt.Sprintf(`%s: processing company "%s" must be text: put it in quotes`, name, company))
			terms.valid = false
		} else if !companyID.MatchString(company) {
			errors = append(errors, fmt.Sprintf(`%s: processing company "%s" must be lower-case letters, digits and hyphens, starting with a letter`, name, company))
			terms.valid = false
		}
		value := mapping.get(company)
		if value == "any" {
			terms.companies[company] = operationTerms{all: true}
			continue
		}
		list, ok := value.([]any)
		if !ok {
			errors = append(errors, fmt.Sprintf(`%s: processing for "%s" must be any or a list of operations`, name, company))
			terms.valid = false
			continue
		}
		allowed := operationTerms{ops: map[string]bool{}}
		for _, item := range list {
			op, ok := item.(string)
			if !ok || ops[op] == "" {
				errors = append(errors, fmt.Sprintf(`%s: processing for "%s" has unknown operation "%s"`, name, company, show(item)))
				terms.valid = false
				continue
			}
			allowed.ops[op] = true
		}
		terms.companies[company] = allowed
	}
	return terms, errors
}

func nestedProcessing(groups []*Map) []string {
	byID := map[string]*Map{}
	for _, group := range groups {
		byID[str(group, "id")] = group
	}
	var errors []string
	for _, inner := range groups {
		outer := byID[str(inner, "in")]
		if outer == nil || !inner.has("processing") {
			continue
		}
		inside, _ := readProcessing(inner.get("processing"), "")
		outside := processingTerms{companies: map[string]operationTerms{}, valid: true}
		if !outer.has("processing") {
			continue
		}
		outside, _ = readProcessing(outer.get("processing"), "")
		if !inside.valid || !outside.valid || outside.all {
			continue
		}
		innerName, outerName := str(inner, "id"), str(outer, "id")
		if inside.all {
			errors = append(errors, fmt.Sprintf(`group "%s" allows any company to process any operation, but outer group "%s" doesn't`, innerName, outerName))
			continue
		}
		for _, company := range sortedCompanyKeys(inside.companies) {
			in, out := inside.companies[company], outside.companies[company]
			if out.all {
				continue
			}
			if in.all {
				if len(out.ops) == 0 {
					errors = append(errors, fmt.Sprintf(`group "%s" allows %s to process any operation, but outer group "%s" doesn't allow %s`, innerName, company, outerName, company))
				} else {
					errors = append(errors, fmt.Sprintf(`group "%s" allows %s to process any operation, but outer group "%s" allows it only for %s`, innerName, company, outerName, naturalList(sortedKeys(out.ops))))
				}
				continue
			}
			for _, op := range sortedKeys(in.ops) {
				if !out.ops[op] {
					errors = append(errors, fmt.Sprintf(`group "%s" allows %s to %s, but outer group "%s" doesn't`, innerName, company, op, outerName))
				}
			}
		}
	}
	return errors
}

// scopes holds each data line in the security view to the boxes it moves between. A move narrows
// only into the box it starts in or one inside it, and narrowing is routing, drawn as an ordinary
// line. Any other move adds viewers or loosens the terms, so it's publishing, or a read the design
// refuses. A part in no box sits outside every box, so it's wider than any of them, and a trigger
// with none is in the box of what it starts. A red line draws what a live system did, not the
// design, so this rule doesn't hold it.
func scopes(groups, parts, lines []*Map) []string {
	parent := map[string]string{}
	for _, g := range groups {
		parent[str(g, "id")] = str(g, "in")
	}
	known := map[string]bool{}
	for _, p := range parts {
		known[str(p, "id")] = true
	}
	// inside is whether a box is the outer one or sits inside it. "" is outside every box, and seen
	// stops a group nested in itself.
	inside := func(box, outer string) bool {
		for seen := map[string]bool{}; outer != "" && !seen[box]; box = parent[box] {
			if box == outer {
				return true
			}
			seen[box] = true
		}
		return outer == ""
	}
	where := placed(parts, lines)
	var errors []string
	for _, l := range lines {
		from, to := str(l, "from"), str(l, "to")
		if l.get("kind") != "data" || l.get("breaks") == true || !known[from] || !known[to] {
			continue
		}
		source, target := where[from], where[to]
		if inside(target, source) {
			continue
		}
		into := fmt.Sprintf(`into group "%s", which isn't inside it`, target)
		if target == "" {
			into = "to the top level, outside every group"
		}
		errors = append(errors, fmt.Sprintf(`the line from "%s" to "%s" moves content from group "%s" %s: draw it as publish, or as refused if the design refuses the read`,
			from, to, source, into))
	}
	return errors
}

// The operations a step may name, each with the emoji it is drawn with, and the kinds of part that
// are steps. other is for work that fits none of the nine, and is held to no operation's rules.
var ops = map[string]string{
	"bring-in": "📥", "sort": "🗂️", "learn": "🧠", "prepare": "📋", "check": "🔍",
	"reflect": "🪞", "draft": "✏️", "do": "🛠️", "answer": "💬", "other": "❔",
}
var steps = setOf([]string{"ai", "code", "person"})

// lineLeads is whether a line starts something, moves data or publishes. A red line is a finding
// about a live system, not part of the design.
func lineLeads(line *Map) bool {
	kind := line.get("kind")
	return (kind == "start" || kind == "data" || kind == "publish") && line.get("breaks") != true
}

// operations holds each step to what its op must not do, wherever the drawing shows it. Only a do
// step leads to an outside action, directly or through a review or a fork, and an answer or check
// step never writes to a context. A red line draws what a live system did, not the design, so
// these rules don't hold it.
func operations(parts, lines []*Map) []string {
	byID := map[any]*Map{}
	for _, p := range parts {
		byID[keyOf(p.get("id"))] = p
	}
	// opOf gives the op a step is held to: nothing if it names none, other, or one that isn't known.
	opOf := func(p *Map) string {
		if k, _ := p.get("kind").(string); !steps[k] {
			return ""
		}
		op, _ := p.get("op").(string)
		if ops[op] == "" || op == "other" {
			return ""
		}
		return op
	}
	var errors []string
	for _, out := range parts {
		if out.get("kind") != "outside" {
			continue
		}
		seen, reported := map[*Map]bool{}, map[*Map]bool{}
		var back func(to *Map)
		back = func(to *Map) {
			seen[to] = true
			for _, l := range lines {
				from := byID[keyOf(l.get("from"))]
				if !lineLeads(l) || from == nil || byID[keyOf(l.get("to"))] != to {
					continue
				}
				if k := from.get("kind"); k == "review" || k == "fork" {
					if !seen[from] {
						back(from)
					}
				} else if op := opOf(from); op != "" && op != "do" && !reported[from] {
					reported[from] = true
					errors = append(errors, fmt.Sprintf(`part "%s" leads to outside action "%s", but its op is %s: only a do step leads to an outside action`,
						show(from.get("id")), show(out.get("id")), op))
				}
			}
		}
		back(out)
	}
	for _, l := range lines {
		from, to := byID[keyOf(l.get("from"))], byID[keyOf(l.get("to"))]
		if !lineLeads(l) || from == nil || to == nil || to.get("kind") != "context" {
			continue
		}
		if op := opOf(from); op == "answer" || op == "check" {
			a := map[string]string{"answer": "an", "check": "a"}[op]
			errors = append(errors, fmt.Sprintf(`part "%s" writes to context "%s", but its op is %s: %s %s step never writes to a context`,
				show(from.get("id")), show(to.get("id")), op, a, op))
		}
	}
	return errors
}

var fieldSets = map[string][2][]string{
	"file":  {{"awml", "title", "view", "parts"}, {"groups", "lines"}},
	"group": {{"id", "label"}, {"in"}},
	"part":  {{"id", "kind", "label"}, {"in", "op"}},
	"line":  {{"from", "to", "kind"}, {"label", "breaks"}},
}

var fields03 = map[string][]string{
	"group": {"processing", "platform"},
	"part":  {"wait", "at"},
}

// The fields that must be text, when they're given at all.
var textFields = map[string]bool{"title": true, "view": true, "id": true, "label": true, "in": true, "kind": true, "from": true, "to": true, "op": true}

// Mermaid reads anything else in an id as part of the syntax around it.
var id = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// fields reports what an item lacks, what it has that it shouldn't, and what YAML didn't read as text.
func fields(what, name string, item *Map, version string, errors *[]string) {
	need, may := setOf(fieldSets[what][0]), setOf(fieldSets[what][1])
	if version == "0.3" {
		for _, field := range fields03[what] {
			may[field] = true
		}
	}
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

var companyID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var pathName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func validPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return false
	}
	for _, name := range strings.Split(path, "/") {
		if name == "." || name == ".." || !pathName.MatchString(name) {
			return false
		}
	}
	return true
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCompanyKeys(values map[string]operationTerms) []string { return sortedKeys(values) }

func naturalList(items []string) string {
	switch len(items) {
	case 0:
		return "nothing"
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
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
