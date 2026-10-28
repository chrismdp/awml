package awml

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Check finds the structural mistakes in an AWML document: what would draw wrongly, or not at all.
func Check(doc any) []string {
	found, _ := checkDoc(doc)
	return found.messages()
}

// Unsound finds the mistakes that stop a document being drawn: every one Check finds, except a line
// that breaks a rule, which Draw draws red.
func Unsound(doc any) []string {
	found, _ := checkDoc(doc)
	var unsound mistakeList
	for _, m := range found {
		if m.lines == nil {
			unsound = append(unsound, m)
		}
	}
	return unsound.messages()
}

// mistake is one thing Check reports, with the id of the check that found it. A mistake that is a
// line breaking a rule also has the lines, their places in the file's list of lines, and breach, the
// check's words for what's wrong, which the line's label adds.
type mistake struct {
	check, message string
	lines          []*Map
	places         []int
	breach         string
}

type mistakeList []mistake

func (m *mistakeList) add(check, message string) {
	*m = append(*m, mistake{check: check, message: message})
}

// addBreach adds a mistake that is a line breaking a rule. Its message is the breach, then advice.
func (m *mistakeList) addBreach(check, breach, advice string, lines ...*Map) {
	*m = append(*m, mistake{check: check, message: breach + ": " + advice, lines: lines, breach: breach})
}

func (m mistakeList) messages() []string {
	var messages []string
	for _, found := range m {
		messages = append(messages, found.message)
	}
	return messages
}

// The version of the file format this release reads, and the older ones awml convert upgrades.
const currentFormat = "0.8"

var olderFormats = []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7"}

// checkDoc gives the mistakes in a document, and whether a mistake stopped the checks that follow
// those of the file's own fields: a version this release doesn't read, or a broken list.
func checkDoc(doc any) (mistakeList, bool) {
	file, ok := doc.(*Map)
	if !ok {
		return mistakeList{{check: "file.yaml", message: "the file must be a mapping of fields, starting with awml"}}, true
	}
	if wrong := checkVersion(file); wrong != nil {
		return wrong, true
	}
	if file.has("platform") {
		return checkPlatformDescription(file), false
	}
	return checkCanonical(file)
}

// checkVersion reports an awml that isn't the current version. An older file is upgraded with awml
// convert. A file with no awml is left to the checks of its fields.
func checkVersion(file *Map) mistakeList {
	if !file.has("awml") {
		return nil
	}
	version, ok := file.get("awml").(string)
	switch {
	case !ok:
		return mistakeList{{check: "file.version", message: fmt.Sprintf("awml: YAML read it as %s, not text: put it in quotes", show(file.get("awml")))}}
	case isOneOf(version, olderFormats...):
		return mistakeList{{check: "file.version", message: fmt.Sprintf(`awml is "%s", which this release no longer reads: upgrade the file with awml convert`, version)}}
	case version != currentFormat:
		return mistakeList{{check: "file.version", message: fmt.Sprintf(`awml must be "%s", not "%s"`, currentFormat, version)}}
	}
	return nil
}

// checkView checks a view projected from a workflow. projected is whether it's a platform view,
// whose fields the file's own checks have already held under the names the file uses, so they
// aren't reported again as a group's or a part's.
func checkView(file *Map, projected bool) (mistakeList, bool) {
	var errors mistakeList
	checkFields := func(what, name string, item *Map) {
		if !projected {
			fields(what, name, item, &errors)
		}
	}
	checkFields("file", "the file", file)
	var lists [3][]*Map
	var broken mistakeList
	for n, kw := range [3][2]string{{"groups", "group"}, {"parts", "part"}, {"lines", "line"}} {
		items := file.get(kw[0])
		if empty(items) {
			continue
		}
		list, ok := items.([]any)
		if !ok {
			broken.add("fields.known", kw[0]+" must be a list")
			continue
		}
		for _, item := range list {
			m, ok := item.(*Map)
			if !ok {
				broken.add("fields.known", fmt.Sprintf("each %s must be a mapping of fields", kw[1]))
				break
			}
			lists[n] = append(lists[n], m)
		}
	}
	// Past here every list is sound, or each line would report a part it can't find.
	if broken != nil {
		return append(errors, broken...), true
	}
	groups, parts, lines := lists[0], lists[1], lists[2]

	seen := map[any]bool{}
	for _, item := range append(append([]*Map{}, groups...), parts...) {
		i := item.get("id")
		if i == nil {
			continue
		}
		if seen[keyOf(i)] {
			errors.add("ids.unique", fmt.Sprintf(`id "%s" is used twice`, show(i)))
		}
		seen[keyOf(i)] = true
		if s, ok := i.(string); ok && !id.MatchString(s) {
			errors.add("ids.written", fmt.Sprintf(`id "%s" must be letters, digits and underscores, starting with a letter`, s))
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
		checkFields("group", name, g)
		if in := g.get("in"); in != nil && !groupIDs[keyOf(in)] {
			errors.add("references.known", fmt.Sprintf(`%s is in unknown group "%s"`, name, show(in)))
		}
		// A block's processing is already an unknown field of the block.
		if g.has("processing") && !projected {
			_, processingErrors := readProcessing(g.get("processing"), name)
			errors = append(errors, processingErrors...)
		}
		if g.has("platform") {
			if file.get("view") != "platform" {
				errors.add("platform.blocks-only", name+" has a platform, but only a group in the platform view takes one")
			} else if s, ok := g.get("platform").(string); !ok {
				errors.add("values.text", fmt.Sprintf("%s: YAML read its platform as %s, not text: put it in quotes", name, show(g.get("platform"))))
			} else if !companyID.MatchString(s) && !registeredPlatform.MatchString(s) {
				errors.add("ids.written", fmt.Sprintf(`%s: platform "%s" must be a company id, or owner/platforms/slug for a registered platform`, name, s))
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
				errors.add("nesting.acyclic", fmt.Sprintf(`group "%s" is nested in itself`, show(g.get("id"))))
				break
			}
			path[keyOf(at)] = true
			at = parent[keyOf(at)]
		}
	}

	for _, p := range parts {
		name := fmt.Sprintf(`part "%s"`, show(p.get("id")))
		checkFields("part", name, p)
		if p.has("kind") {
			if k, ok := p.get("kind").(string); !ok || shapes[k] == "" {
				errors.add("kinds.known", fmt.Sprintf(`%s has unknown kind "%s"`, name, show(p.get("kind"))))
			}
		}
		if !projected {
			errors = append(errors, marks(name, p)...)
			errors = append(errors, refMistakes(name, p)...)
		}
		if in := p.get("in"); in != nil && !groupIDs[keyOf(in)] {
			errors.add("references.known", fmt.Sprintf(`%s is in unknown group "%s"`, name, show(in)))
		}
		if op := p.get("op"); op != nil {
			if k, _ := p.get("kind").(string); !steps[k] {
				errors.add("ops.known", name+" has an op, but only an ai, code or person step takes one")
			} else if s, _ := op.(string); ops[s] == "" {
				errors.add("ops.known", fmt.Sprintf(`%s has unknown op "%s"`, name, show(op)))
			}
		}
		if p.has("wait") {
			if p.get("kind") != "trigger" {
				errors.add("wait.duration", name+" has a wait, but only a trigger takes one")
			} else if wait, ok := p.get("wait").(string); !ok || !validDuration(wait) {
				errors.add("wait.duration", fmt.Sprintf(`%s: wait must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "%s"`, name, show(p.get("wait"))))
			}
		}
		if p.has("at") {
			if p.get("kind") != "context" {
				errors.add("at.path", name+" has an at, but only a context takes one")
			} else if at, ok := p.get("at").(string); !ok || !validPath(at) {
				errors.add("at.path", fmt.Sprintf(`%s: at must be a path, not "%s"`, name, show(p.get("at"))))
			}
		}
	}

	for n, l := range lines {
		name := lineName(n, l)
		checkFields("line", name, l)
		for _, end := range [2][2]string{{"from", "starts"}, {"to", "ends"}} {
			if _, ok := kinds[keyOf(l.get(end[0]))]; l.has(end[0]) && !ok {
				errors.add("references.known", fmt.Sprintf(`%s %s at unknown part "%s"`, name, end[1], show(l.get(end[0]))))
			}
		}
		kind := l.get("kind")
		if kind == nil {
			continue
		}
		k, ok := kind.(string)
		if !ok || arrows[k] == "" {
			errors.add("kinds.known", fmt.Sprintf(`%s has unknown kind "%s"`, name, show(kind)))
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
				errors.add("comments.tie-only", fmt.Sprintf(`comment "%s" is linked by a %s line: a comment takes only tie`, show(c), k))
			}
		}
		if k == "tie" && len(comments) == 0 {
			errors.add("ties.comment", fmt.Sprintf(`the tie from "%s" to "%s" has no comment at either end`, show(l.get("from")), show(l.get("to"))))
		}
		// A trigger only starts its own workflow, so only a tie or a hidden line, which lead nowhere,
		// goes into it.
		if k != "tie" && k != "hidden" && kinds[keyOf(l.get("to"))] == "trigger" {
			errors.add("triggers.no-line-in", fmt.Sprintf(`%s goes into trigger "%s": a trigger only starts its own workflow, so start another workflow with an outside action whose ref names its trigger`, name, show(l.get("to"))))
		}
	}
	errors = append(errors, operations(parts, lines)...)
	if file.get("view") == "security" {
		errors = append(errors, tiers(parts, lines)...)
		errors = append(errors, untrustedOutside(parts, lines)...)
		errors = append(errors, nestedProcessing(groups)...)
	}
	return errors, false
}

// lineName names the nth line of a file, by its ends where it has both.
func lineName(n int, l *Map) string {
	if l.has("from") && l.has("to") {
		return fmt.Sprintf(`the line from "%s" to "%s"`, show(l.get("from")), show(l.get("to")))
	}
	return fmt.Sprintf("line %d", n+1)
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

func readProcessing(value any, name string) (processingTerms, mistakeList) {
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
		return terms, mistakeList{{check: "processing.written", message: name + ": processing must be any, none or a map from company ids to any or operations"}}
	}
	var errors mistakeList
	for _, company := range mapping.Keys() {
		if mapping.nonTextKeys[company] {
			errors.add("processing.written", fmt.Sprintf(`%s: processing company "%s" must be text: put it in quotes`, name, company))
			terms.valid = false
		} else if !companyID.MatchString(company) {
			errors.add("processing.written", fmt.Sprintf(`%s: processing company "%s" must be lower-case letters, digits and hyphens, starting with a letter`, name, company))
			terms.valid = false
		}
		value := mapping.get(company)
		if value == "any" {
			terms.companies[company] = operationTerms{all: true}
			continue
		}
		list, ok := value.([]any)
		if !ok {
			errors.add("processing.written", fmt.Sprintf(`%s: processing for "%s" must be any or a list of operations`, name, company))
			terms.valid = false
			continue
		}
		allowed := operationTerms{ops: map[string]bool{}}
		for _, item := range list {
			op, ok := item.(string)
			if !ok || ops[op] == "" {
				errors.add("processing.written", fmt.Sprintf(`%s: processing for "%s" has unknown operation "%s"`, name, company, show(item)))
				terms.valid = false
				continue
			}
			allowed.ops[op] = true
		}
		terms.companies[company] = allowed
	}
	return terms, errors
}

func nestedProcessing(groups []*Map) mistakeList {
	byID := map[string]*Map{}
	for _, group := range groups {
		byID[str(group, "id")] = group
	}
	var errors mistakeList
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
			errors.add("processing.nested", fmt.Sprintf(`group "%s" allows any company to process any operation, but outer group "%s" doesn't`, innerName, outerName))
			continue
		}
		for _, company := range sortedCompanyKeys(inside.companies) {
			in, out := inside.companies[company], outside.companies[company]
			if out.all {
				continue
			}
			if in.all {
				if len(out.ops) == 0 {
					errors.add("processing.nested", fmt.Sprintf(`group "%s" allows %s to process any operation, but outer group "%s" doesn't allow %s`, innerName, company, outerName, company))
				} else {
					errors.add("processing.nested", fmt.Sprintf(`group "%s" allows %s to process any operation, but outer group "%s" allows it only for %s`, innerName, company, outerName, naturalList(sortedKeys(out.ops))))
				}
				continue
			}
			for _, op := range sortedKeys(in.ops) {
				if !out.ops[op] {
					errors.add("processing.nested", fmt.Sprintf(`group "%s" allows %s to %s, but outer group "%s" doesn't`, innerName, company, op, outerName))
				}
			}
		}
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

// lineLeads is whether a line starts something, moves data or publishes. A red line is one the
// check has reported, so it leads like any other.
func lineLeads(line *Map) bool {
	return isOneOf(line.get("kind"), "start", "data", "publish")
}

// operations holds each step to what its op must not do, wherever the drawing shows it. Only a do
// step leads to an outside action, directly or through a review, a standing approval or a fork,
// and an answer or check step never writes to a context.
func operations(parts, lines []*Map) mistakeList {
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
	var errors mistakeList
	for _, out := range parts {
		if out.get("kind") != "outside" {
			continue
		}
		// reported gives where a step's mistake is, so each line it leads along is added to it.
		seen, reported := map[*Map]bool{}, map[*Map]int{}
		var back func(to *Map)
		back = func(to *Map) {
			seen[to] = true
			for _, l := range lines {
				from := byID[keyOf(l.get("from"))]
				if !lineLeads(l) || from == nil || byID[keyOf(l.get("to"))] != to {
					continue
				}
				if k := from.get("kind"); k == "review" || k == "approval" || k == "fork" {
					if !seen[from] {
						back(from)
					}
				} else if op := opOf(from); op != "" && op != "do" {
					if n, ok := reported[from]; ok {
						errors[n].lines = append(errors[n].lines, l)
						continue
					}
					reported[from] = len(errors)
					errors.addBreach("operations.only-do-acts", fmt.Sprintf(`part "%s" leads to outside action "%s", but its op is %s`,
						show(from.get("id")), show(out.get("id")), op), "only a do step leads to an outside action", l)
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
			errors.addBreach("operations.answer-check-read-only", fmt.Sprintf(`part "%s" writes to context "%s", but its op is %s`,
				show(from.get("id")), show(to.get("id")), op), fmt.Sprintf("%s %s step never writes to a context", a, op), l)
		}
	}
	return errors
}

// tiers holds each outside action's tier to the shape in front of it. A way into an action is a
// line that leads to it, followed back through forks. A review, a standing approval or a person's
// own step puts a person on that way. An Ask action has a person on every way in, an Allow action
// has no yes in front of it, and nothing leads to a Block action but a red line. A standing approval
// also lists what it covers on a surface.
func tiers(parts, lines []*Map) mistakeList {
	byID := map[any]*Map{}
	for _, p := range parts {
		byID[keyOf(p.get("id"))] = p
	}
	var errors mistakeList
	for _, out := range parts {
		if out.get("kind") != "outside" {
			continue
		}
		name := show(out.get("id"))
		tier := outsideTier(str(out, "label"))
		if tier == "" {
			errors.add("tiers.written", fmt.Sprintf(`outside action "%s" has no tier: end its label with Allow, Ask or Block after a colon, as in "Send the mail: Ask"`, name))
			continue
		}
		// Each way in is a part, with the lines it leads along.
		type way struct {
			from  *Map
			lines []*Map
		}
		var ways []*way
		found := map[*Map]*way{}
		seen := map[*Map]bool{out: true}
		var back func(to *Map)
		back = func(to *Map) {
			for _, l := range lines {
				from := byID[keyOf(l.get("from"))]
				if !lineLeads(l) || from == nil || byID[keyOf(l.get("to"))] != to || seen[from] {
					continue
				}
				if from.get("kind") == "fork" {
					seen[from] = true
					back(from)
				} else if w := found[from]; w != nil {
					w.lines = append(w.lines, l)
				} else {
					found[from] = &way{from, []*Map{l}}
					ways = append(ways, found[from])
				}
			}
		}
		back(out)
		for _, w := range ways {
			kind, id := w.from.get("kind"), show(w.from.get("id"))
			var breach, advice string
			switch {
			case tier == "Ask" && !isOneOf(kind, "review", "approval", "person"):
				breach = fmt.Sprintf(`outside action "%s" is Ask, but part "%s" leads to it with no person in between`, name, id)
				advice = "put a review or a standing approval in front of it, or have a person's own do step take it"
			case tier == "Allow" && isOneOf(kind, "review", "approval"):
				what := map[any]string{"review": "review", "approval": "standing approval"}[kind]
				breach = fmt.Sprintf(`outside action "%s" is Allow, but %s "%s" stands in front of it`, name, what, id)
				advice = "an Allow action just happens, so make it Ask or take the yes away"
			case tier == "Block":
				breach = fmt.Sprintf(`outside action "%s" is Block, but part "%s" leads to it`, name, id)
				advice = "a blocked action is never taken, so a line into it is red"
			default:
				continue
			}
			errors.addBreach("tiers.shape", breach, advice, w.lines...)
		}
	}
	for _, approval := range parts {
		if approval.get("kind") != "approval" {
			continue
		}
		listed := false
		for _, l := range lines {
			to := byID[keyOf(l.get("to"))]
			listed = listed || lineLeads(l) && byID[keyOf(l.get("from"))] == approval && to != nil && to.get("kind") == "surface"
		}
		if !listed {
			errors.add("approvals.listed", fmt.Sprintf(`standing approval "%s" leads to no surface: draw a line from it to the surface where each action it covers is listed`, show(approval.get("id"))))
		}
	}
	return errors
}

var fieldSets = map[string][2][]string{
	"file":  {{"awml", "title", "view", "parts"}, {"groups", "lines"}},
	"group": {{"id", "label"}, {"in", "processing", "platform"}},
	"part":  {{"id", "kind", "label"}, {"in", "op", "wait", "at", "untrusted", "cleans", "ref"}},
	"line":  {{"from", "to", "kind"}, {"label"}},
}

// The fields that must be text, when they're given at all.
var textFields = map[string]bool{"title": true, "view": true, "id": true, "label": true, "in": true, "kind": true, "from": true, "to": true, "op": true, "ref": true}

// Mermaid reads anything else in an id as part of the syntax around it.
var id = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// fields reports what an item lacks, what it has that it shouldn't, and what YAML didn't read as text.
func fields(what, name string, item *Map, errors *mistakeList) {
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
		errors.add("fields.known", fmt.Sprintf("%s has no %s", name, f))
	}
	for _, f := range item.Keys() {
		// The check works out which lines are red, so none is marked by hand.
		if what == "line" && f == "breaks" {
			errors.add("fields.known", name+" has breaks, but awml check works out which lines break a rule, and awml draw draws them red")
		} else if !need[f] && !may[f] {
			errors.add("fields.known", fmt.Sprintf(`%s has unknown field "%s"`, name, f))
		}
	}
	// YAML reads true and false, numbers and dates as something other than text.
	for _, f := range item.Keys() {
		v := item.get(f)
		if _, ok := v.(string); textFields[f] && v != nil && !ok {
			errors.add("values.text", fmt.Sprintf("%s: YAML read its %s as %s, not text: put it in quotes", name, f, show(v)))
		}
	}
}

var companyID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// A platform published to the registry is named owner/platforms/slug. The owner and slug follow the registry's rule for names: lower-case letters, digits and
// hyphens, up to 40 characters, starting and ending with a letter or a digit.
var registeredPlatform = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?/platforms/[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$`)
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
