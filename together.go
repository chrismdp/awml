package awml

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// A Workflow is one workflow file given with others: the name it was given by, which messages use;
// Path, where it is as an absolute path, which its refs are followed from and matched against; and
// its document. A workflow given without a Path, as the WebAssembly build gives each, is taken to be
// at its name from one root.
type Workflow struct {
	File string
	Path string
	Doc  *Map
}

// at gives where a workflow's file is, as a cleaned path from the root, with forward slashes.
func (w Workflow) at() string {
	if w.Path != "" {
		return path.Clean(filepath.ToSlash(w.Path))
	}
	return path.Join("/", filepath.ToSlash(w.File))
}

// partAt is one workflow's copy of a part.
type partAt struct {
	workflow int
	part     *Map
}

// linked is what following each ref between workflows given together finds: the parts they share,
// and what's wrong with the refs.
type linked struct {
	workflows []Workflow
	// shared holds each part more than one workflow has, as every copy, with the part every ref
	// leads back to first, in the order the workflows and their parts are given.
	shared [][]partAt
	// root gives each copy of a shared part the copy every ref leads back to.
	root map[*Map]partAt
	// found holds the mistakes and warnings found in each workflow.
	found []mistakeList
}

// link follows each ref between the workflows given. A workflow that can't be read as one, or whose
// lists are broken, takes no part.
func link(workflows []Workflow) linked {
	l := linked{workflows: workflows, root: map[*Map]partAt{}, found: make([]mistakeList, len(workflows))}
	sound := make([]bool, len(workflows))
	for i, w := range workflows {
		if IsCanonical(w.Doc) {
			_, stopped := checkDoc(w.Doc)
			sound[i] = !stopped
		}
	}
	to := map[*Map]partAt{}
	var referring []partAt
	for i, w := range workflows {
		if !sound[i] {
			continue
		}
		for _, p := range items(w.Doc, "parts") {
			workflow, id, ok := readRef(p.get("ref"))
			if !ok || !refKinds[str(p, "kind")] {
				continue
			}
			t := refersTo(workflows, w, workflow)
			if t == i {
				l.found[i].add("refs.owner", fmt.Sprintf(`part "%s" refers to %s#%s, which is in its own workflow: a ref names a part of another workflow`, str(p, "id"), workflow, id))
				continue
			}
			if t < 0 || !sound[t] {
				continue
			}
			name := fmt.Sprintf(`part "%s" refers to %s#%s, but `, str(p, "id"), workflow, id)
			target := partsByID(workflows[t].Doc)[id]
			if target == nil {
				l.found[i].add("refs.known", fmt.Sprintf(`%s%s has no part "%s"`, name, workflow, id))
				continue
			}
			for _, clause := range disagreements(w.Doc, p, workflows[t].Doc, target) {
				l.found[i].add("refs.agree", name+clause)
			}
			to[p] = partAt{t, target}
			referring = append(referring, partAt{i, p})
		}
	}
	roots, circles := owners(to, referring)
	for _, circle := range circles {
		for n, c := range circle {
			var through []string
			for k := 1; k < len(circle); k++ {
				through = append(through, workflows[circle[(n+k)%len(circle)].workflow].File)
			}
			l.found[c.workflow].add("refs.owner", fmt.Sprintf(`part "%s" refers to %s, but its refs go round in a circle through %s, so no workflow owns it: the owner's copy has no ref`,
				str(c.part, "id"), str(c.part, "ref"), naturalList(through)))
		}
	}
	rootOf := func(p partAt) partAt { return roots[p.part] }
	byRoot := map[*Map]int{}
	for i, w := range workflows {
		if !sound[i] {
			continue
		}
		for _, p := range items(w.Doc, "parts") {
			if _, refers := to[p]; !refers {
				continue
			}
			root := rootOf(partAt{i, p})
			n, ok := byRoot[root.part]
			if !ok {
				n = len(l.shared)
				byRoot[root.part] = n
				l.shared = append(l.shared, []partAt{root})
				l.root[root.part] = root
			}
			if p == root.part {
				continue
			}
			l.shared[n] = append(l.shared[n], partAt{i, p})
			l.root[p] = root
		}
	}
	// Each shared part's copies, root first, then in the order given, and the shared parts in the
	// order the first copy of each is given.
	position := map[*Map][2]int{}
	for i, w := range workflows {
		if !sound[i] {
			continue
		}
		for n, p := range soundMaps(w.Doc.get("parts")) {
			position[p] = [2]int{i, n}
		}
	}
	before := func(a, b *Map) bool {
		x, y := position[a], position[b]
		return x[0] < y[0] || x[0] == y[0] && x[1] < y[1]
	}
	first := func(copies []partAt) *Map {
		earliest := copies[0].part
		for _, c := range copies {
			if before(c.part, earliest) {
				earliest = c.part
			}
		}
		return earliest
	}
	for _, copies := range l.shared {
		rest := copies[1:]
		sort.SliceStable(rest, func(a, b int) bool { return before(rest[a].part, rest[b].part) })
	}
	sort.SliceStable(l.shared, func(a, b int) bool { return before(first(l.shared[a]), first(l.shared[b])) })
	l.warnShared()
	l.warnUntrusted()
	return l
}

// owners follows each copy that refers, given in order, along its refs to the copy that owns the
// part, the one that refers to nothing given. Refs that go round in a circle lead to no owner, so it
// gives each circle, from the first of its copies given, and that copy stands in for the owner.
func owners(to map[*Map]partAt, referring []partAt) (map[*Map]partAt, [][]partAt) {
	roots := map[*Map]partAt{}
	given := map[*Map]int{}
	for n, c := range referring {
		given[c.part] = n
	}
	var circles [][]partAt
	for _, start := range referring {
		var path []partAt
		on := map[*Map]int{}
		p, root := start, start
		for {
			if known, ok := roots[p.part]; ok {
				root = known
				break
			}
			next, refers := to[p.part]
			if !refers {
				root = p
				break
			}
			if n, ok := on[p.part]; ok {
				circle := path[n:]
				first := 0
				for k, c := range circle {
					if given[c.part] < given[circle[first].part] {
						first = k
					}
				}
				circle = append(append([]partAt{}, circle[first:]...), circle[:first]...)
				circles = append(circles, circle)
				root = circle[0]
				break
			}
			on[p.part] = len(path)
			path = append(path, p)
			p = next
		}
		for _, c := range path {
			roots[c.part] = root
		}
	}
	return roots, circles
}

// disagreements says how a copy differs from the part it refers to: its kind, label or at, or its
// scope, compared by who it holds and its terms rather than by its id.
func disagreements(here *Map, p *Map, there *Map, q *Map) []string {
	var said []string
	quoted := func(m *Map, field string) string {
		if !m.has(field) {
			return "none"
		}
		return fmt.Sprintf(`"%s"`, show(m.get(field)))
	}
	for _, field := range []string{"kind", "label", "at"} {
		if a, b := quoted(p, field), quoted(q, field); a != b {
			said = append(said, fmt.Sprintf("its %s is %s here and %s there", field, a, b))
		}
	}
	// A trigger sits beside what it starts, so its scope is no part of it.
	if str(p, "kind") == "trigger" || str(q, "kind") == "trigger" {
		return said
	}
	a, b := str(p, "in"), str(q, "in")
	switch {
	case a == "" && b == "":
	case a == "" || b == "":
		where := func(scope string) string {
			if scope == "" {
				return "outside every scope"
			}
			return fmt.Sprintf(`in scope "%s"`, scope)
		}
		said = append(said, fmt.Sprintf("it's %s here and %s there", where(a), where(b)))
	default:
		hereScopes, thereScopes := scopeIndex(here), scopeIndex(there)
		// Who a scope holds is compared only where both labels say.
		if x, y := hereScopes.people(a), thereScopes.people(b); x != nil && y != nil && strings.Join(x, ",") != strings.Join(y, ",") {
			said = append(said, fmt.Sprintf("its scope holds %s here and %s there", naturalList(x), naturalList(y)))
		}
		x, y := hereScopes.terms(a), thereScopes.terms(b)
		switch {
		case x == y:
		case x == "" || y == "":
			said = append(said, fmt.Sprintf("its scope's processing is %s here and %s there", orUnsaid(x), orUnsaid(y)))
		default:
			said = append(said, fmt.Sprintf("its scope allows %s here and %s there", x, y))
		}
	}
	return said
}

func orUnsaid(terms string) string {
	if terms == "" {
		return "not given"
	}
	return "given"
}

// scopes indexes a workflow's scopes, so who each holds and its terms can be read.
type scopes struct {
	byID     map[string]*Map
	children map[string][]string
}

func scopeIndex(doc *Map) scopes {
	s := scopes{byID: map[string]*Map{}, children: map[string][]string{}}
	for _, scope := range soundMaps(doc.get("scopes")) {
		s.byID[str(scope, "id")] = scope
		s.children[str(scope, "in")] = append(s.children[str(scope, "in")], str(scope, "id"))
	}
	return s
}

// people gives who a scope holds, sorted, or nil when its label doesn't say.
func (s scopes) people(scope string) []string {
	names, ok := members(s.byID, s.children, scope, map[string]bool{})
	if !ok {
		return nil
	}
	names = append([]string{}, names...)
	sort.Strings(names)
	return names
}

// terms says which companies a scope lets process its content, or "" when it doesn't say.
func (s scopes) terms(scope string) string {
	if s.byID[scope] == nil || !s.byID[scope].has("processing") {
		return ""
	}
	t, _ := readProcessing(s.byID[scope].get("processing"), "")
	if !t.valid {
		return ""
	}
	return terms(t)
}

// leadsInto and leadsOutOf are whether a line in a workflow leads into a part, or out of it.
func leadsInto(doc *Map, p *Map) bool {
	for _, line := range items(doc, "lines") {
		if lineLeads(line) && line.get("to") == p.get("id") {
			return true
		}
	}
	return false
}

func leadsOutOf(doc *Map, p *Map) bool {
	for _, line := range items(doc, "lines") {
		if lineLeads(line) && line.get("from") == p.get("id") {
			return true
		}
	}
	return false
}

// warnShared warns of a shared context that more than one workflow writes, and a shared connector
// that more than one brings items in from, where one can change or take what another relies on.
func (l *linked) warnShared() {
	for _, copies := range l.shared {
		root := copies[0]
		var writers, readers []string
		for _, c := range copies {
			doc, file := l.workflows[c.workflow].Doc, l.workflows[c.workflow].File
			if leadsInto(doc, c.part) && !isOneOf(file, writers...) {
				writers = append(writers, file)
			}
			if leadsOutOf(doc, c.part) && !isOneOf(file, readers...) {
				readers = append(readers, file)
			}
		}
		id := str(root.part, "id")
		switch {
		case str(root.part, "kind") == "context" && len(writers) > 1:
			l.found[root.workflow].add("warn.shared-writers", fmt.Sprintf(`context "%s" is written by %s: each may change what another relies on`, id, naturalList(writers)))
		case str(root.part, "kind") == "connector" && len(readers) > 1:
			l.found[root.workflow].add("warn.shared-readers", fmt.Sprintf(`connector "%s" is read by %s: each may take an item another expects`, id, naturalList(readers)))
		}
	}
}

// joined is the workflows given as one: each part once, a shared part as the copy its refs lead
// back to, with its scope, and every line. Each id is made unique by its workflow, and names gives
// each the name it's reported by: its own id, after its file's name where there are several files.
type joined struct {
	groups, parts, lines []*Map
	from                 map[string]partAt
	names                map[string]string
}

func (l linked) join() joined {
	j := joined{from: map[string]partAt{}, names: map[string]string{}}
	several := len(l.workflows) > 1
	uid := func(w int, id string) string {
		if !several {
			return id
		}
		return fmt.Sprintf("w%d_%s", w, id)
	}
	name := func(w int, id string) string {
		if !several {
			return id
		}
		return strings.TrimSuffix(path.Base(filepath.ToSlash(l.workflows[w].File)), ".yaml") + "#" + id
	}
	for w, workflow := range l.workflows {
		if !IsCanonical(workflow.Doc) {
			continue
		}
		for _, scope := range soundMaps(workflow.Doc.get("scopes")) {
			s := copyMap(scope)
			s.set("id", uid(w, str(scope, "id")))
			j.names[str(s, "id")] = name(w, str(scope, "id"))
			if scope.has("in") {
				s.set("in", uid(w, str(scope, "in")))
			}
			j.groups = append(j.groups, s)
		}
	}
	idOf := func(w int, p *Map) string {
		if root, ok := l.root[p]; ok {
			return uid(root.workflow, str(root.part, "id"))
		}
		return uid(w, str(p, "id"))
	}
	for w, workflow := range l.workflows {
		if !IsCanonical(workflow.Doc) {
			continue
		}
		byID := partsByID(workflow.Doc)
		for _, p := range soundMaps(workflow.Doc.get("parts")) {
			if root, ok := l.root[p]; ok && root.part != p {
				continue
			}
			c := copyMap(l.sharedPart(partAt{w, p}), "ref")
			c.set("id", idOf(w, p))
			j.names[str(c, "id")] = name(w, str(p, "id"))
			j.from[str(c, "id")] = partAt{w, p}
			if p.has("in") {
				c.set("in", uid(w, str(p, "in")))
			}
			j.parts = append(j.parts, c)
		}
		for _, line := range soundMaps(workflow.Doc.get("lines")) {
			from, to := byID[str(line, "from")], byID[str(line, "to")]
			if from == nil || to == nil {
				continue
			}
			c := copyMap(line)
			c.set("from", idOf(w, from))
			c.set("to", idOf(w, to))
			j.lines = append(j.lines, c)
		}
	}
	return j
}

// copiesOf gives every copy of a part, the one every ref leads back to first: just that one, where
// no other workflow shares it.
func (l linked) copiesOf(root partAt) []partAt {
	for _, copies := range l.shared {
		if copies[0].part == root.part {
			return copies
		}
	}
	return []partAt{root}
}

// marked gives the first copy of a part that's marked untrusted, the owner's first, and whether any
// copy is. A part they share is untrusted if any workflow's copy of it is marked untrusted.
func (l linked) marked(root partAt) (partAt, bool) {
	for _, c := range l.copiesOf(root) {
		if c.part.get("untrusted") == true {
			return c, true
		}
	}
	return partAt{}, false
}

// sharedPart gives the part a copy every ref leads back to stands for: the copy, marked untrusted
// where any copy of it is.
func (l linked) sharedPart(root partAt) *Map {
	if _, ok := l.marked(root); !ok || root.part.get("untrusted") == true {
		return root.part
	}
	p := copyMap(root.part)
	p.set("untrusted", true)
	return p
}

// warnedAlone is whether a workflow's own check warns that a part's text reaches one of its outside
// actions: whether its own copy of the part is marked untrusted, and reaches the action within it.
func (l linked) warnedAlone(source, action partAt) bool {
	doc := l.workflows[action.workflow].Doc
	for _, c := range l.copiesOf(source) {
		if c.workflow != action.workflow || c.part.get("untrusted") != true {
			continue
		}
		if alone, _ := reach(c.part, items(doc, "parts"), items(doc, "lines")); alone[action.part] {
			return true
		}
	}
	return false
}

// warnUntrusted warns of each outside action that untrusted text reaches through a part shared with
// another workflow, which no workflow's own check sees. It names the part the text comes from by
// the first copy marked untrusted.
func (l *linked) warnUntrusted() {
	if len(l.shared) == 0 {
		return
	}
	j := l.join()
	from := map[*Map][]partAt{}
	for _, source := range untrustedParts(j.parts) {
		reached, _ := reach(source, j.parts, j.lines)
		root := j.from[str(source, "id")]
		named, _ := l.marked(root)
		for _, p := range j.parts {
			action := j.from[str(p, "id")]
			if reached[p] && p.get("kind") == "outside" && !l.warnedAlone(root, action) {
				from[action.part] = append(from[action.part], named)
			}
		}
	}
	for _, p := range j.parts {
		action := j.from[str(p, "id")]
		sources := from[action.part]
		if sources == nil {
			continue
		}
		var named []string
		for _, s := range sources {
			named = append(named, fmt.Sprintf(`"%s" in %s`, str(s.part, "id"), l.workflows[s.workflow].File))
		}
		what := "part"
		if len(named) > 1 {
			what = "parts"
		}
		l.found[action.workflow].add("warn.untrusted-outside", fmt.Sprintf(`untrusted text from %s %s reaches outside action "%s": a review, a person's own step or a code step marked cleans in between would stop it`,
			what, naturalList(named), str(action.part, "id")))
	}
}

// TogetherMistakes follows each ref between the workflows given, and gives the mistakes in each: a
// ref to a part that isn't there, and a copy that disagrees with the part it refers to. A ref to a
// workflow not given is left alone, as a file checked on its own leaves it.
func TogetherMistakes(workflows []Workflow) [][]string {
	return link(workflows).list(false)
}

// TogetherWarnings gives the warnings in each workflow that only the workflows together show: a
// context more than one writes, a connector more than one brings items in from, and untrusted text
// that reaches an outside action through a part they share.
func TogetherWarnings(workflows []Workflow) [][]string {
	return link(workflows).list(true)
}

func (l linked) list(warnings bool) [][]string {
	out := make([][]string, len(l.found))
	for i, found := range l.found {
		for _, m := range found {
			if strings.HasPrefix(m.check, "warn.") == warnings {
				out[i] = append(out[i], m.message)
			}
		}
	}
	return out
}

// workflowBox is how the map draws a workflow's box: solid, since it's a workflow, not a scope.
const workflowBox = "fill:#f6f7f9,stroke:#5d6472,color:#16171b"

// DrawMap draws workflows given together as their map: a box for each workflow, with its title and
// the triggers it runs on, those with a line out of them, that it shares with none, and each shared
// part once, between the boxes,
// with a line from each workflow that leads into it and to each that it leads into. Each workflow
// must have no mistakes but lines that break a rule.
func DrawMap(workflows []Workflow) string {
	l := link(workflows)
	out := append([]string{}, header...)
	var drawn []*Map
	node := map[*Map]string{}
	for w, workflow := range workflows {
		box := fmt.Sprintf("w%d", w+1)
		out = append(out, fmt.Sprintf(`  subgraph %s["%s"]`, box, text(str(workflow.Doc, "title"))), "    direction TB")
		for _, p := range items(workflow.Doc, "parts") {
			if _, shared := l.root[p]; shared || str(p, "kind") != "trigger" || !leadsOutOf(workflow.Doc, p) {
				continue
			}
			node[p] = box + "_" + str(p, "id")
			drawn = append(drawn, p)
			out = append(out, "    "+node[p]+fmt.Sprintf(shapes["trigger"], label(p, false)))
		}
		out = append(out, "  end")
	}
	for n, copies := range l.shared {
		root := copies[0].part
		node[root] = fmt.Sprintf("s%d", n+1)
		drawn = append(drawn, root)
		out = append(out, "  "+node[root]+fmt.Sprintf(shapes[str(root, "kind")], label(l.sharedPart(copies[0]), false)))
	}
	out = append(out, "")
	for n, copies := range l.shared {
		shared := fmt.Sprintf("s%d", n+1)
		into, outOf := map[int]bool{}, map[int]bool{}
		var order []int
		for _, c := range copies {
			if !isOneOfInt(c.workflow, order) {
				order = append(order, c.workflow)
			}
			doc := workflows[c.workflow].Doc
			into[c.workflow] = into[c.workflow] || leadsInto(doc, c.part)
			outOf[c.workflow] = outOf[c.workflow] || leadsOutOf(doc, c.part)
		}
		sort.Ints(order)
		dotted := str(copies[0].part, "kind") != "trigger"
		for _, w := range order {
			box := fmt.Sprintf("w%d", w+1)
			arrow := map[[2]bool]string{{true, false}: "-->", {false, true}: "-->", {true, true}: "<-->", {false, false}: "---"}[[2]bool{into[w], outOf[w]}]
			if dotted {
				arrow = map[string]string{"-->": "-.->", "<-->": "<-.->", "---": "-.-"}[arrow]
			}
			if outOf[w] && !into[w] {
				out = append(out, fmt.Sprintf("  %s %s %s", shared, arrow, box))
			} else {
				out = append(out, fmt.Sprintf("  %s %s %s", box, arrow, shared))
			}
		}
	}
	out = append(out, defs)
	for _, cls := range []string{"trig", "ctx", "con", "surf"} {
		var ids []string
		for _, p := range drawn {
			if classes[str(p, "kind")] == cls {
				ids = append(ids, node[p])
			}
		}
		if ids != nil {
			out = append(out, fmt.Sprintf("  class %s %s", strings.Join(ids, ","), cls))
		}
	}
	for w := range workflows {
		out = append(out, fmt.Sprintf("  style w%d %s", w+1, workflowBox))
	}
	return strings.Join(out, "\n")
}

func isOneOfInt(v int, choices []int) bool {
	for _, c := range choices {
		if v == c {
			return true
		}
	}
	return false
}

// SharedSummary names a part more than one workflow has, and the workflows that have it.
type SharedSummary struct {
	Label     string   `json:"label"`
	Kind      string   `json:"kind"`
	Workflows []string `json:"workflows"`
}

// SummariseTogether summarises workflows given together: each counted as Summarise counts it, a
// part they share counted once, and untrusted text followed through what they share. It names the
// shared parts. One workflow is summarised as Summarise does.
func SummariseTogether(workflows []Workflow) Summary {
	if len(workflows) == 1 {
		return Summarise(workflows[0].Doc)
	}
	var each []Summary
	for _, w := range workflows {
		each = append(each, Summarise(w.Doc))
	}
	out := TotalSummaries(each...)
	l := link(workflows)
	for _, copies := range l.shared {
		entry := SharedSummary{Label: str(copies[0].part, "label"), Kind: str(copies[0].part, "kind")}
		var order []int
		for _, c := range copies {
			if !isOneOfInt(c.workflow, order) {
				order = append(order, c.workflow)
			}
		}
		sort.Ints(order)
		for _, w := range order {
			entry.Workflows = append(entry.Workflows, str(workflows[w].Doc, "title"))
		}
		out.Shared = append(out.Shared, entry)
		// Summarise counted every copy, but a shared part is one part.
		for range copies[1:] {
			switch str(copies[0].part, "kind") {
			case "trigger":
				out.Triggers--
			case "context":
				out.Contexts--
			case "connector":
				out.Connectors--
			case "surface":
				out.Surfaces--
			}
		}
	}
	j := l.join()
	out.Untrusted = summariseUntrusted(j.parts, j.lines)
	return out
}

// JudgeTogether judges each platform as running every workflow given, with a part they share
// placed once. Where there are several, each part and scope is named after its file, as in
// dinners#plan. One workflow is judged as Judge judges it.
func JudgeTogether(workflows []Workflow, descriptions []*Map) FitReport {
	if len(workflows) == 1 {
		return Judge(workflows[0].Doc, nil, descriptions)
	}
	j := link(workflows).join()
	security := &Map{}
	var groups, parts, lines []any
	for _, g := range j.groups {
		groups = append(groups, g)
	}
	for _, p := range j.parts {
		parts = append(parts, p)
	}
	for _, line := range j.lines {
		lines = append(lines, line)
	}
	security.set("groups", groups)
	security.set("parts", parts)
	security.set("lines", lines)
	model := newFitModel(security)
	model.names = j.names
	results, gaps := judgeModel(model, nil, descriptions)
	if results == nil {
		results = []FitResult{}
	}
	if gaps == nil {
		gaps = []string{}
	}
	return FitReport{Results: results, Gaps: gaps}
}
