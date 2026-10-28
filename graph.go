package awml

import (
	"fmt"
	"sort"
	"strings"
)

// A Graph is what a drawing shows, with nothing laid out: its boxes, its lines and its comments,
// each named by the id its Mermaid gives it. awml graph prints it as JSON, for a tool that lays the
// drawing out itself, and Draw and DrawMap draw their Mermaid from it, so the two can't disagree.
// View is security, platform or map.
type Graph struct {
	View     string         `json:"view"`
	Groups   []GraphGroup   `json:"groups"`
	Parts    []GraphPart    `json:"parts"`
	Lines    []GraphLine    `json:"lines"`
	Comments []GraphComment `json:"comments"`
	Places   []GraphPlace   `json:"places"`
}

// A GraphGroup is a box: a scope, a block in a platform view, or a workflow on a map, with the
// file it was given by and, where a ref among the workflows given names it so, its owner/slug. In
// is the box it's drawn inside.
type GraphGroup struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	In        string `json:"in,omitempty"`
	File      string `json:"file,omitempty"`
	Published string `json:"published,omitempty"`
}

// A GraphPart is a part other than a comment, with the box it's drawn in. Decider is who decides
// a step, a review or a standing approval: ai, code or person. Tier is an outside action's tier.
// Untrusted is whether the part is marked untrusted, and Unstopped whether its text reaches an
// outside action with nothing to stop it or accept the risk.
type GraphPart struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	In        string `json:"in,omitempty"`
	Decider   string `json:"decider,omitempty"`
	Op        string `json:"op,omitempty"`
	Tier      string `json:"tier,omitempty"`
	Untrusted bool   `json:"untrusted,omitempty"`
	Unstopped bool   `json:"unstopped,omitempty"`
}

// A GraphLine is a line from one part or box to another. A publish is a data line that Publishes,
// and Widens says what it adds or loosens. Points is which way its arrow points: to its to, both
// ways, or none, as a tie or a hidden line does. Breaks holds the words for each rule a red line
// breaks. Untrusted says how the line is drawn for untrusted text: carries where the text goes
// along it, stopped where it leaves what stopped the text, and unstopped where it's on the text's
// way to an outside action that nothing stops and nobody accepted. On a map, Joins holds the parts
// inside the workflow that a line to its box joins.
type GraphLine struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Kind      string   `json:"kind"`
	Label     string   `json:"label,omitempty"`
	Points    string   `json:"points"`
	Publishes bool     `json:"publishes,omitempty"`
	Widens    []string `json:"widens,omitempty"`
	Breaks    []string `json:"breaks,omitempty"`
	Untrusted string   `json:"untrusted,omitempty"`
	Joins     []string `json:"joins,omitempty"`
}

// A GraphComment is a comment, with the box it's drawn in, whether it marks a gap, which its label
// starting "Missing from the design" says, and the parts it's tied to.
type GraphComment struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	In    string   `json:"in,omitempty"`
	Gap   bool     `json:"gap"`
	Ties  []string `json:"ties"`
}

// A GraphPlace is the colour a platform view gives a block.
type GraphPlace struct {
	ID     string `json:"id"`
	Fill   string `json:"fill"`
	Stroke string `json:"stroke"`
}

// gapLabel starts the label of a comment that marks a gap.
const gapLabel = "Missing from the design"

// GraphOf gives what a workflow's view draws, as View projects it. A workflow is taken as its
// security view. It must have no mistakes but lines that break a rule.
func GraphOf(doc *Map) Graph {
	if IsCanonical(doc) {
		doc, _ = View(doc, "")
	}
	groups, parts, lines := items(doc, "groups"), items(doc, "parts"), items(doc, "lines")
	where := placed(parts, lines)
	g := emptyGraph(str(doc, "view"))
	for _, p := range parts {
		id := str(p, "id")
		if str(p, "kind") == "comment" {
			g.Comments = append(g.Comments, GraphComment{ID: id, Label: str(p, "label"), In: where[id],
				Gap: strings.HasPrefix(str(p, "label"), gapLabel), Ties: tiedTo(id, lines)})
			continue
		}
		part := graphPart(p, id)
		part.In, part.Unstopped = where[id], p.get("unstopped") == true
		g.Parts = append(g.Parts, part)
	}
	for i, group := range groups {
		g.Groups = append(g.Groups, GraphGroup{ID: str(group, "id"), Label: str(group, "label"), In: str(group, "in")})
		if g.View == "platform" {
			fill, stroke, _ := strings.Cut(pastel[i%len(pastel)], ",")
			g.Places = append(g.Places, GraphPlace{ID: str(group, "id"), Fill: strings.TrimPrefix(fill, "fill:"), Stroke: strings.TrimPrefix(stroke, "stroke:")})
		}
	}
	for _, l := range lines {
		line := GraphLine{From: str(l, "from"), To: str(l, "to"), Kind: str(l, "kind"), Label: str(l, "label"), Points: "to", Untrusted: str(l, "untrusted")}
		if line.Kind == "tie" || line.Kind == "hidden" {
			line.Points = "none"
		}
		if line.Kind == "publish" {
			line.Kind, line.Publishes = "data", true
		}
		// A publish's label ends with what it widens, which the line gives apart.
		if widens, ok := l.get("widens").([]any); ok {
			for _, w := range widens {
				line.Widens = append(line.Widens, show(w))
			}
			said := strings.Join(line.Widens, " · ")
			line.Label = strings.TrimSuffix(strings.TrimSuffix(line.Label, said), " · ")
		}
		breaches, _ := l.get("breaks").([]any)
		for _, breach := range breaches {
			line.Breaks = append(line.Breaks, show(breach))
		}
		g.Lines = append(g.Lines, line)
	}
	return g
}

// GraphMap gives what the map of workflows given together draws, as DrawMap describes it. The
// workflows' boxes are w1, w2 and so on, each part drawn on its workflow's box is named by its box
// and its id, as w1_thu, and the shared parts are s1, s2 and so on.
func GraphMap(workflows []Workflow) Graph {
	l := link(workflows)
	g := emptyGraph("map")
	node := map[*Map]string{}
	ends := map[*Map]bool{}
	for _, h := range l.starts {
		ends[h.action.part], ends[h.trigger.part] = true, true
	}
	published := publishedNames(workflows)
	unstopped := l.unstopped()
	var starts []GraphLine
	for w, workflow := range workflows {
		box := fmt.Sprintf("w%d", w+1)
		var inside []GraphPart
		for _, p := range items(workflow.Doc, "parts") {
			trigger := str(p, "kind") == "trigger"
			if _, shared := l.root[p]; shared || !trigger && !ends[p] {
				continue
			}
			node[p] = box + "_" + str(p, "id")
			part := graphPart(p, node[p])
			part.Unstopped = unstopped[p]
			if trigger {
				g.Parts = append(g.Parts, part)
				starts = append(starts, GraphLine{From: node[p], To: box, Kind: "start", Points: "to", Joins: joins(box, workflow.Doc, []partAt{{w, p}})})
			} else {
				part.In = box
				inside = append(inside, part)
			}
		}
		g.Groups = append(g.Groups, GraphGroup{ID: box, Label: str(workflow.Doc, "title"), File: workflow.File, Published: published[w]})
		g.Parts = append(g.Parts, inside...)
	}
	for n, copies := range l.shared {
		part := graphPart(l.sharedPart(copies[0]), fmt.Sprintf("s%d", n+1))
		part.Unstopped = unstopped[copies[0].part]
		g.Parts = append(g.Parts, part)
	}
	g.Lines = append(g.Lines, starts...)
	for n, copies := range l.shared {
		shared := fmt.Sprintf("s%d", n+1)
		into, outOf := map[int]bool{}, map[int]bool{}
		mine := map[int][]partAt{}
		var order []int
		for _, c := range copies {
			if !isOneOfInt(c.workflow, order) {
				order = append(order, c.workflow)
			}
			doc := workflows[c.workflow].Doc
			into[c.workflow] = into[c.workflow] || leadsInto(doc, c.part)
			outOf[c.workflow] = outOf[c.workflow] || leadsOutOf(doc, c.part)
			mine[c.workflow] = append(mine[c.workflow], c)
		}
		sort.Ints(order)
		for _, w := range order {
			box := fmt.Sprintf("w%d", w+1)
			line := GraphLine{From: box, To: shared, Kind: "data", Points: "to", Joins: joins(box, workflows[w].Doc, mine[w])}
			switch {
			case outOf[w] && !into[w]:
				line.From, line.To = shared, box
			case into[w] && outOf[w]:
				line.Points = "both"
			case !into[w] && !outOf[w]:
				line.Points = "none"
			}
			g.Lines = append(g.Lines, line)
		}
	}
	for _, h := range l.starts {
		g.Lines = append(g.Lines, GraphLine{From: node[h.action.part], To: node[h.trigger.part], Kind: "start", Points: "to"})
	}
	return g
}

func emptyGraph(view string) Graph {
	return Graph{View: view, Groups: []GraphGroup{}, Parts: []GraphPart{}, Lines: []GraphLine{}, Comments: []GraphComment{}, Places: []GraphPlace{}}
}

// graphPart gives a part as a graph names it, with who decides it, its operation and its tier.
func graphPart(p *Map, id string) GraphPart {
	part := GraphPart{ID: id, Kind: str(p, "kind"), Label: str(p, "label"), Untrusted: p.get("untrusted") == true}
	switch part.Kind {
	case "ai", "code", "person":
		part.Decider, part.Op = part.Kind, str(p, "op")
	case "review", "approval":
		part.Decider = "person"
	case "outside":
		part.Tier = outsideTier(part.Label)
	}
	return part
}

// tiedTo gives the parts a comment is tied to, in the order of their lines.
func tiedTo(comment string, lines []*Map) []string {
	ties := []string{}
	for _, l := range lines {
		switch {
		case str(l, "kind") != "tie":
		case str(l, "from") == comment:
			ties = append(ties, str(l, "to"))
		case str(l, "to") == comment:
			ties = append(ties, str(l, "from"))
		}
	}
	return ties
}

// joins gives the parts of a workflow, named on its box, that lead into or out of any of the copies
// given, in the order of the workflow's lines.
func joins(box string, doc *Map, copies []partAt) []string {
	var found []string
	for _, line := range items(doc, "lines") {
		if !lineLeads(line) {
			continue
		}
		for _, c := range copies {
			other := ""
			if line.get("from") == c.part.get("id") {
				other = str(line, "to")
			} else if line.get("to") == c.part.get("id") {
				other = str(line, "from")
			}
			if id := box + "_" + other; other != "" && !isOneOf(id, found...) {
				found = append(found, id)
			}
		}
	}
	return found
}

// publishedNames gives each workflow's owner/slug, where a ref from another of the workflows given
// names it so, and "" where none does.
func publishedNames(workflows []Workflow) []string {
	names := make([]string, len(workflows))
	for w, workflow := range workflows {
		if !IsCanonical(workflow.Doc) {
			continue
		}
		for _, p := range items(workflow.Doc, "parts") {
			name, _, ok := readRef(p.get("ref"))
			if !ok || strings.HasSuffix(name, ".yaml") {
				continue
			}
			if t := refersTo(workflows, workflow, name); t >= 0 && t != w && names[t] == "" {
				names[t] = name
			}
		}
	}
	return names
}

// Mermaid draws the graph, so every drawing uses the same shapes and colours. It draws from the
// graph's fields alone, so a graph read back from its JSON draws the same. At each level it writes
// the parts in the order the graph lists them, then the comments, then the groups. It writes each
// line from its from to its to, but a tie from its part to its comment, and a two-way line from its
// part to its box.
func (g Graph) Mermaid() string {
	out := append([]string{}, header...)
	type node struct {
		group         int
		in, id, shape string
	}
	var nodes []node
	kinds := map[string]string{}
	for _, p := range g.Parts {
		kinds[p.ID] = p.Kind
		nodes = append(nodes, node{-1, p.In, p.ID, fmt.Sprintf(shapes[p.Kind], p.mermaidLabel())})
	}
	for _, c := range g.Comments {
		kinds[c.ID] = "comment"
		nodes = append(nodes, node{-1, c.In, c.ID, fmt.Sprintf(shapes["comment"], text(c.Label))})
	}
	for i, group := range g.Groups {
		nodes = append(nodes, node{i, group.In, group.ID, ""})
	}
	var emit func(parent, indent string)
	emit = func(parent, indent string) {
		for _, n := range nodes {
			if n.in != parent {
				continue
			}
			if n.group < 0 {
				out = append(out, indent+n.id+n.shape)
				continue
			}
			out = append(out, fmt.Sprintf(`%ssubgraph %s["%s"]`, indent, n.id, text(g.Groups[n.group].Label)))
			// Mermaid turns a box with no links outside it on its side unless told otherwise.
			out = append(out, indent+"  direction TB")
			emit(n.id, indent+"  ")
			out = append(out, indent+"end")
		}
	}
	emit("", "  ")
	out = append(out, "")
	// A line's style says where it goes: solid from one step to the next, dotted into or out of a
	// context, a connector or a surface.
	aside := func(l GraphLine) bool { return beside[kinds[l.From]] || beside[kinds[l.To]] }
	kindOf := func(l GraphLine) string {
		if l.Publishes {
			return "publish"
		}
		return l.Kind
	}
	for _, l := range g.Lines {
		from, to, kind := l.From, l.To, kindOf(l)
		// A tie has no direction, so it runs from the part to the comment, and the comment hangs
		// below or beside what it explains.
		if kind == "tie" && kinds[from] == "comment" && kinds[to] != "comment" {
			from, to = to, from
		}
		// A two-way line has no direction either, so it runs from the part to the box: from the
		// box, Mermaid can lay it out as a path of NaN, detached from its ends.
		if l.Points == "both" && kinds[from] == "" && kinds[to] != "" {
			from, to = to, from
		}
		arrow := arrows[kind]
		if aside(l) && dottedArrows[kind] != "" {
			arrow = dottedArrows[kind]
		}
		switch {
		case l.Points == "both":
			arrow = twoWay[arrow]
		case l.Points == "none" && headless[arrow] != "":
			arrow = headless[arrow]
		}
		var words []string
		if said := strings.Join(append(nonEmpty(fold(l.Label)), l.Widens...), " · "); said != "" {
			words = append(words, text(said))
		}
		// A line the check reports adds its words for each rule the line breaks, each on a row.
		for _, breach := range l.Breaks {
			words = append(words, text(breach))
		}
		mid := ""
		if words != nil {
			mid = fmt.Sprintf(`|"%s"|`, strings.Join(words, "<br>"))
		}
		out = append(out, fmt.Sprintf("  %s %s%s %s", from, arrow, mid, to))
	}
	out = append(out, defs)
	// A part marked untrusted keeps its own class, and takes a second for its outline: orange, or
	// red where its text reaches an outside action unstopped.
	outline := map[string]string{}
	for _, p := range g.Parts {
		if p.Unstopped {
			outline[p.ID] = "unstopped"
		} else if p.Untrusted {
			outline[p.ID] = "untrusted"
		}
	}
	if len(outline) > 0 {
		out = append(out, "  classDef untrusted "+untrustedOutline, "  classDef unstopped "+unstoppedOutline)
	}
	for _, cls := range []string{"trig", "ctx", "con", "surf", "out", "ai", "code", "person", "gap", "untrusted", "unstopped"} {
		var ids []string
		for _, n := range nodes {
			if n.group < 0 && (classes[kinds[n.id]] == cls || outline[n.id] == cls) {
				ids = append(ids, n.id)
			}
		}
		if ids != nil {
			out = append(out, fmt.Sprintf("  class %s %s", strings.Join(ids, ","), cls))
		}
	}
	places := map[string]GraphPlace{}
	for _, p := range g.Places {
		places[p.ID] = p
	}
	for _, group := range g.Groups {
		look := scope
		if place, ok := places[group.ID]; ok {
			look = fmt.Sprintf("fill:%s,stroke:%s,color:#16171b", place.Fill, place.Stroke)
		} else if g.View == "map" {
			look = workflowBox
		}
		out = append(out, fmt.Sprintf("  style %s %s", group.ID, look))
	}
	// linkStyle counts every line, hidden ones too, and a line's last linkStyle replaces any before
	// it, so each line gets its whole style at once. Lines of the same style share a linkStyle, in
	// the order of the first of them.
	var looks []string
	numbers := map[string][]string{}
	for i, l := range g.Lines {
		var look []string
		switch {
		case len(l.Breaks) > 0 || l.Untrusted == "unstopped":
			look = append(look, red)
		case l.Untrusted == "carries":
			look = append(look, orange)
		case l.Untrusted == "stopped":
			look = append(look, green)
		case l.Kind == "tie":
			look = append(look, grey)
		}
		if l.Publishes && aside(l) {
			look = append(look, dotted)
		}
		if look == nil {
			continue
		}
		key := strings.Join(look, ",")
		if numbers[key] == nil {
			looks = append(looks, key)
		}
		numbers[key] = append(numbers[key], fmt.Sprint(i))
	}
	for _, look := range looks {
		out = append(out, fmt.Sprintf("  linkStyle %s %s", strings.Join(numbers[look], ","), look))
	}
	return strings.Join(out, "\n")
}

// mermaidLabel is a part's label as Mermaid reads it. A step's, a review's or a standing
// approval's sits under its own line.
func (p GraphPart) mermaidLabel() string {
	label := text(p.Label)
	who := deciders[p.Kind]
	if who == "" {
		return label
	}
	if p.Kind == "review" {
		who += " ✅ review"
	} else if p.Kind == "approval" {
		who += " 📜 standing approval"
	} else if p.Op != "" {
		who += " " + ops[p.Op] + " " + p.Op
	}
	return fmt.Sprintf(operation, who) + label
}

// nonEmpty gives a list of the text, or none when it's empty.
func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
