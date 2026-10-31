package awml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// A reply the bakery's owner drafts and sends: its line to the outside action breaks two rules, its
// line to the shop's board publishes into the wider scope, a comment marks a gap, and its home
// implementation places the parts on two platforms, with a comment tied to the draft.
const bakeryReplies = `awml: "0.10"
title: Bakery replies
scopes:
  - { id: shop, label: "shop: Ana, Ben" }
  - { id: owner, label: "owner: Ana", in: shop }
parts:
  - { id: mail, kind: trigger, label: "A customer emails", untrusted: true }
  - { id: draft, kind: ai, op: draft, label: "Draft a reply", in: owner }
  - { id: notes, kind: context, label: "Ana's notes", in: owner }
  - { id: send, kind: outside, label: "Send the reply: Ask, once a day", in: owner }
  - { id: board, kind: surface, label: "The shop's board", in: shop }
  - { id: gap, kind: comment, label: "Missing from the design: nobody reads the replies" }
lines:
  - { from: mail, to: draft, kind: start }
  - { from: notes, to: draft, kind: data }
  - { from: draft, to: send, kind: data, label: the reply }
  - { from: draft, to: board, kind: data, label: posted }
  - { from: gap, to: send, kind: tie }
implementations:
  home:
    label: At home
    blocks:
      - { id: laptop, label: "Ana's laptop", parts: [draft, notes] }
      - { id: mailer, label: The mail service, parts: [send, board] }
    comments:
      - { id: tried, label: "For example: tried on a second model", in: laptop, ties: [draft] }
`

func graphOf(t *testing.T, src, implementation string) Graph {
	t.Helper()
	doc := parsed(t, src)
	if mistakes := Unsound(doc); len(mistakes) != 0 {
		t.Fatal(mistakes)
	}
	view, err := View(doc, implementation)
	if err != nil {
		t.Fatal(err)
	}
	return GraphOf(view)
}

func TestTheGraphOfAWorkflowHasItsParts(t *testing.T) {
	g := graphOf(t, bakeryReplies, "")
	if g.View != "security" {
		t.Errorf("view is %q", g.View)
	}
	want := []GraphPart{
		{ID: "mail", Kind: "trigger", Label: "A customer emails", In: "owner", Untrusted: true, Unstopped: true},
		{ID: "draft", Kind: "ai", Label: "Draft a reply", In: "owner", Decider: "ai", Op: "draft"},
		{ID: "notes", Kind: "context", Label: "Ana's notes", In: "owner"},
		{ID: "send", Kind: "outside", Label: "Send the reply: Ask, once a day", In: "owner", Tier: "Ask"},
		{ID: "board", Kind: "surface", Label: "The shop's board", In: "shop"},
	}
	if !reflect.DeepEqual(g.Parts, want) {
		t.Errorf("parts are\n%+v", g.Parts)
	}
	review := graphOf(t, `awml: "0.10"
title: T
parts:
  - { id: ok, kind: review, label: "Ana's yes" }
`, "").Parts
	if review[0].Decider != "person" {
		t.Errorf("a review is %+v", review)
	}
}

func TestTheGraphOfAWorkflowHasItsGroups(t *testing.T) {
	want := []GraphGroup{
		{ID: "shop", Label: "shop: Ana, Ben", Stroke: "#127c8c", Color: "#0d5c68"},
		{ID: "owner", Label: "owner: Ana", In: "shop", Stroke: "#8c2f6b", Color: "#6b2251"},
	}
	if g := graphOf(t, bakeryReplies, ""); !reflect.DeepEqual(g.Groups, want) || len(g.Places) != 0 {
		t.Errorf("groups are %+v and places %+v", g.Groups, g.Places)
	}
}

func TestTheGraphOfAWorkflowHasItsLines(t *testing.T) {
	g := graphOf(t, bakeryReplies, "")
	want := []GraphLine{
		{From: "mail", To: "draft", Kind: "start", Points: "to", Untrusted: "unstopped"},
		{From: "notes", To: "draft", Kind: "data", Points: "to"},
		{From: "draft", To: "send", Kind: "data", Label: "the reply", Points: "to", Breaks: g.Lines[2].Breaks, Untrusted: "unstopped"},
		{From: "draft", To: "board", Kind: "data", Label: "posted", Points: "to", Publishes: true, Widens: []string{"adds Ben"}, Untrusted: "carries"},
		{From: "gap", To: "send", Kind: "tie", Points: "none"},
	}
	if !reflect.DeepEqual(g.Lines, want) {
		t.Errorf("lines are\n%+v", g.Lines)
	}
	if breaks := g.Lines[2].Breaks; len(breaks) != 3 || !strings.Contains(breaks[0], "draft") || !strings.Contains(breaks[2], `untrusted text from part "mail"`) {
		t.Errorf("the red line breaks %q", breaks)
	}
}

func TestTheGraphOfAWorkflowHasItsComments(t *testing.T) {
	want := []GraphComment{{ID: "gap", Label: "Missing from the design: nobody reads the replies", In: "owner", Gap: true, Ties: []string{"send"}}}
	if g := graphOf(t, bakeryReplies, ""); !reflect.DeepEqual(g.Comments, want) {
		t.Errorf("comments are %+v", g.Comments)
	}
}

func TestTheGraphOfAPlatformViewHasItsBlocksAndPlaces(t *testing.T) {
	g := graphOf(t, bakeryReplies, "home")
	if g.View != "platform" {
		t.Errorf("view is %q", g.View)
	}
	groups := []GraphGroup{{ID: "laptop", Label: "Ana's laptop"}, {ID: "mailer", Label: "The mail service"}}
	places := []GraphPlace{{ID: "laptop", Fill: "#e3ebfa", Stroke: "#b9c8e8"}, {ID: "mailer", Fill: "#fbe9df", Stroke: "#ecc7b2"}}
	if !reflect.DeepEqual(g.Groups, groups) || !reflect.DeepEqual(g.Places, places) {
		t.Errorf("groups are %+v and places %+v", g.Groups, g.Places)
	}
	if g.Parts[1].In != "laptop" || g.Parts[3].In != "mailer" || g.Parts[4].In != "mailer" {
		t.Errorf("parts are %+v", g.Parts)
	}
	comments := []GraphComment{
		{ID: "gap", Label: "Missing from the design: nobody reads the replies", In: "mailer", Gap: true, Ties: []string{"send"}},
		{ID: "tried", Label: "For example: tried on a second model", In: "laptop", Ties: []string{"draft"}},
	}
	if !reflect.DeepEqual(g.Comments, comments) {
		t.Errorf("comments are %+v", g.Comments)
	}
	if last := g.Lines[len(g.Lines)-1]; !reflect.DeepEqual(last, GraphLine{From: "tried", To: "draft", Kind: "tie", Points: "none"}) {
		t.Errorf("the last line is %+v", last)
	}
}

// Each line says how it's drawn for untrusted text, and the part it comes from whether it reaches an
// outside action unstopped.
func TestTheGraphSaysWhereUntrustedTextGoes(t *testing.T) {
	g := graphOf(t, ordersForGraph, "")
	want := map[string]string{"form>keep": "carries", "keep>book": "stopped", "book>rota": "", "form>draft": "unstopped", "draft>send": "unstopped"}
	if got := untrustedLines(t, g); !reflect.DeepEqual(got, want) {
		t.Errorf("lines are %q, want %q", got, want)
	}
	if got := unstoppedParts(g); !reflect.DeepEqual(got, []string{"form"}) {
		t.Errorf("unstopped parts are %q", got)
	}
	encoded, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"untrusted":true,"unstopped":true`, `"untrusted":"carries"`, `"untrusted":"stopped"`, `"untrusted":"unstopped"`} {
		if !strings.Contains(string(encoded), field) {
			t.Errorf("the graph's JSON has no %s:\n%s", field, encoded)
		}
	}
	if strings.Contains(string(encoded), `"reached"`) || strings.Contains(string(encoded), `"stops"`) {
		t.Errorf("the graph's JSON still says what the untrusted view did:\n%s", encoded)
	}
}

const ordersForGraph = `awml: "0.10"
title: Orders
scopes:
  - { id: bakery, label: "bakery: Ana, Ben" }
parts:
  - { id: form, kind: trigger, label: "An order on the form", untrusted: true }
  - { id: keep, kind: code, op: bring-in, label: "Keep only the cake", in: bakery, cleans: true }
  - { id: book, kind: code, op: do, label: "Book the bake", in: bakery }
  - { id: rota, kind: outside, label: "Add it to the rota: Allow", in: bakery }
  - { id: draft, kind: code, op: do, label: "Write the reply", in: bakery }
  - { id: send, kind: outside, label: "Send the reply: Allow", in: bakery }
lines:
  - { from: form, to: keep, kind: start }
  - { from: keep, to: book, kind: data }
  - { from: book, to: rota, kind: data }
  - { from: form, to: draft, kind: start }
  - { from: draft, to: send, kind: data }
`

func mustView(t *testing.T, doc *Map, implementation string) *Map {
	t.Helper()
	view, err := View(doc, implementation)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestTheGraphOfAMapHasEachWorkflowsBox(t *testing.T) {
	g := GraphMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow))
	if g.View != "map" {
		t.Errorf("view is %q", g.View)
	}
	groups := []GraphGroup{{ID: "w1", Label: "Dinners", File: "plan.yaml"}, {ID: "w2", Label: "Shopping", File: "shop.yaml"}}
	if !reflect.DeepEqual(g.Groups, groups) {
		t.Errorf("groups are %+v", g.Groups)
	}
	parts := []GraphPart{
		{ID: "w1_thu", Kind: "trigger", Label: "Thursday 19:00"},
		{ID: "w1_ready", Kind: "outside", Label: "Start the shopping: Allow", In: "w1", Tier: "Allow"},
		{ID: "w2_ready", Kind: "trigger", Label: "The shopping list is ready"},
		{ID: "s1", Kind: "context", Label: "Shopping list"},
	}
	if !reflect.DeepEqual(g.Parts, parts) {
		t.Errorf("parts are\n%+v", g.Parts)
	}
	lines := []GraphLine{
		{From: "w1_thu", To: "w1", Kind: "start", Points: "to", Joins: []string{"w1_plan"}},
		{From: "w2_ready", To: "w2", Kind: "start", Points: "to", Joins: []string{"w2_fill"}},
		{From: "w1", To: "s1", Kind: "data", Points: "to", Joins: []string{"w1_plan"}},
		{From: "s1", To: "w2", Kind: "data", Points: "to", Joins: []string{"w2_fill"}},
		{From: "w1_ready", To: "w2_ready", Kind: "start", Points: "to"},
	}
	if !reflect.DeepEqual(g.Lines, lines) {
		t.Errorf("lines are\n%+v", g.Lines)
	}
	if len(g.Comments) != 0 || len(g.Places) != 0 {
		t.Errorf("a map has comments %+v and places %+v", g.Comments, g.Places)
	}
}

func TestTheGraphOfAMapSaysWhichWayALinePoints(t *testing.T) {
	both := strings.Replace(shopWorkflow, "  - { from: fill, to: basket, kind: data }\n", "  - { from: fill, to: basket, kind: data }\n  - { from: fill, to: list, kind: data }\n", 1)
	g := GraphMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", both))
	if l := g.Lines[3]; l.From != "w2" || l.To != "s1" || l.Points != "both" || !reflect.DeepEqual(l.Joins, []string{"w2_fill"}) {
		t.Errorf("a two-way line is %+v", l)
	}
	has(t, g.Mermaid(), "\n  s1 <-.-> w2\n")
	neither := strings.Replace(shopWorkflow, "  - { from: list, to: fill, kind: data }\n", "  - { from: list, to: fill, kind: hidden }\n", 1)
	g = GraphMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", neither))
	if l := g.Lines[3]; l.From != "w2" || l.To != "s1" || l.Points != "none" || l.Joins != nil {
		t.Errorf("a line with no way is %+v", l)
	}
	has(t, g.Mermaid(), "\n  w2 -.- s1\n")
}

func TestTheGraphOfAMapNamesAWorkflowARefPublishes(t *testing.T) {
	plan := strings.Replace(planWorkflow, "ref: shop.yaml#ready", "ref: bakery/shopping#ready", 1)
	shop := strings.Replace(shopWorkflow, "ref: plan.yaml#list", "ref: ../plan.yaml#list", 1)
	g := GraphMap(together(t, "plan.yaml", plan, "bakery/shopping.yaml", shop))
	if g.Groups[0].Published != "" || g.Groups[1].Published != "bakery/shopping" {
		t.Errorf("groups are %+v", g.Groups)
	}
}

// The JSON names each field as the spec lists it, and leaves out what a part, group or line
// doesn't have.
func TestTheGraphsJSONNamesItsFields(t *testing.T) {
	encoded, err := json.Marshal(graphOf(t, bakeryReplies, "home"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`{"view":"platform","groups":[{"id":"laptop","label":"Ana's laptop"}`,
		`{"id":"send","kind":"outside","label":"Send the reply: Ask, once a day","in":"mailer","tier":"Ask"}`,
		`{"id":"draft","kind":"ai","label":"Draft a reply","in":"laptop","decider":"ai","op":"draft"}`,
		`{"from":"draft","to":"board","kind":"data","label":"posted","points":"to","publishes":true,"widens":["adds Ben"],"untrusted":"carries"}`,
		`{"from":"draft","to":"send","kind":"data","label":"the reply","points":"to","breaks":["part \"draft\" leads to outside action \"send\", but its op is draft",`,
		`{"from":"gap","to":"send","kind":"tie","points":"none"}`,
		`"comments":[{"id":"gap","label":"Missing from the design: nobody reads the replies","in":"mailer","gap":true,"ties":["send"]}`,
		`"places":[{"id":"laptop","fill":"#e3ebfa","stroke":"#b9c8e8"}`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the JSON has no %s in\n%s", want, encoded)
		}
	}
	empty, _ := json.Marshal(graphOf(t, "awml: \"0.10\"\ntitle: T\nparts:\n  - { id: a, kind: ai, label: A }\n", ""))
	if want := `{"view":"security","groups":[],"parts":[{"id":"a","kind":"ai","label":"A","decider":"ai"}],"lines":[],"comments":[],"places":[]}`; string(empty) != want {
		t.Errorf("the JSON of a lone step is\n%s", empty)
	}
}

// Draw draws the graph, so the Mermaid and the JSON can't disagree.
func TestDrawDrawsTheGraph(t *testing.T) {
	view := mustView(t, parsed(t, bakeryReplies), "home")
	if Draw(view) != GraphOf(view).Mermaid() {
		t.Error("Draw doesn't draw the graph")
	}
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	if DrawMap(workflows) != GraphMap(workflows).Mermaid() {
		t.Error("DrawMap doesn't draw the graph")
	}
}

// The Mermaid is drawn from what the graph lists, in the order it lists it, so a graph read back
// from awml graph's JSON draws just as awml draw does.
func TestAGraphReadBackFromItsJSONDrawsTheSame(t *testing.T) {
	question := parsed(t, questionFile)
	graphs := map[string]Graph{
		"security": GraphOf(mustView(t, parsed(t, bakeryReplies), "")),
		"platform": GraphOf(mustView(t, parsed(t, bakeryReplies), "home")),
		"question": GraphOf(question),
		"map":      GraphMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)),
	}
	for name, g := range graphs {
		encoded, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		var read Graph
		if err := json.Unmarshal(encoded, &read); err != nil {
			t.Fatal(err)
		}
		if got, want := read.Mermaid(), g.Mermaid(); got != want {
			t.Errorf("%s read back draws\n%s\nnot\n%s", name, got, want)
		}
	}
}

// At each level, the Mermaid writes the parts in the order the graph lists them, then the
// comments, then the groups, and each line from its from to its to.
func TestTheMermaidWritesWhatTheGraphListsInItsOrder(t *testing.T) {
	g := Graph{
		View:     "security",
		Groups:   []GraphGroup{{ID: "inner", Label: "Inner", In: "outer", Stroke: "#127c8c", Color: "#0d5c68"}, {ID: "outer", Label: "Outer", Stroke: "#8c2f6b", Color: "#6b2251"}},
		Parts:    []GraphPart{{ID: "b", Kind: "context", Label: "B", In: "outer"}, {ID: "a", Kind: "trigger", Label: "A"}, {ID: "d", Kind: "context", Label: "D", In: "inner"}},
		Comments: []GraphComment{{ID: "c", Label: "C", In: "outer", Ties: []string{"b"}}},
		Lines:    []GraphLine{{From: "b", To: "a", Kind: "data", Points: "to"}, {From: "c", To: "b", Kind: "tie", Points: "none"}},
	}
	has(t, g.Mermaid(), `flowchart TB
  a("A")
  subgraph outer["Outer"]
    direction TB
    b[("B")]
    c@{ shape: notch-rect, label: "C" }
    subgraph inner["Inner"]
      direction TB
      d[("D")]
    end
  end

  b -.-> a
  b -.- c
`)
}

// The graph gives each label as the file wrote it, so a tool that shows it escapes it itself, as
// the Mermaid does.
func TestTheGraphGivesLabelsAsWrittenAndTheMermaidEscapesThem(t *testing.T) {
	const label = `Say "yes" & <b>now</b>`
	g := graphOf(t, "awml: \"0.10\"\ntitle: T\nparts:\n  - { id: a, kind: ai, label: '"+label+"' }\n", "")
	if g.Parts[0].Label != label {
		t.Errorf("the label is %q", g.Parts[0].Label)
	}
	if out := g.Mermaid(); strings.Contains(out, label) || !strings.Contains(out, text(label)) {
		t.Errorf("the Mermaid doesn't escape the label:\n%s", out)
	}
}
