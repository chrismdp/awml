package awml

import (
	"reflect"
	"strings"
	"testing"
)

const canonical = `awml: "0.4"
title: Bread
scopes:
  - {id: bakery, label: Bakery, processing: any}
parts:
  - {id: morning, kind: trigger, label: Morning, wait: PT1H}
  - {id: plan, kind: ai, label: Plan, in: bakery}
  - {id: notes, kind: context, label: Notes, in: bakery, at: notes}
  - {id: gap, kind: comment, label: Gap}
lines:
  - {from: morning, to: plan, kind: start}
  - {from: plan, to: notes, kind: data}
  - {from: gap, to: notes, kind: tie}
implementations:
  home:
    label: At home
    blocks:
      - {id: server, label: Server, platform: scripts, parts: [plan, notes]}
    comments:
      - {id: keys, label: Keys, in: server, ties: [plan]}
`

func parseCanonical(t *testing.T, source string) *Map {
	t.Helper()
	doc, err := Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return doc.(*Map)
}

func TestCanonicalCheck(t *testing.T) {
	if got := Check(parseCanonical(t, canonical)); len(got) != 0 {
		t.Fatal(got)
	}
	cases := []struct{ old, new, want string }{
		{`awml: "0.4"`, "awml: \"0.4\"\nview: security", `the file has unknown field "view"`},
		{"scopes:", "groups:", `the file has unknown field "groups"`},
		{"parts: [plan, notes]", "parts: [plan, plan, notes]", `part "plan" is placed twice in implementation "home"`},
		{"parts: [plan, notes]", "parts: [plan]", `part "notes" is not placed in implementation "home"`},
		{"parts: [plan, notes]", "parts: [plan, notes, missing]", `block "server" names unknown part "missing"`},
		{"platform: scripts, parts", "platform: scripts, in: missing, parts", `group "server" is in unknown group "missing"`},
		{"platform: scripts, parts", "platform: scripts, in: server, parts", `group "server" is nested in itself`},
		{"label: Keys, in: server", "label: Keys, in: missing", `part "keys" is in unknown group "missing"`},
		{"ties: [plan]", "ties: [missing]", `the line from "keys" to "missing" ends at unknown part "missing"`},
		{"id: keys", "id: plan", `id "plan" is used twice`},
		{"id: server", "id: bakery", `id "bakery" is used twice`},
		{"label: Bakery, processing: any", "label: Bakery, processing: any, platform: scripts", `group "bakery" has a platform, but only a group in the platform view takes one`},
		{"label: Server, platform", "label: Server, processing: any, platform", `block "server" has unknown field "processing"`},
		{"wait: PT1H", "wait: P1M", `wait must be an ISO 8601 duration`},
		{"at: notes", "at: ../notes", `at must be a path`},
		{"label: Plan, in: bakery", "label: Plan, op: answer, in: bakery", `an answer step never writes to a context`},
		{"label: Notes, in: bakery", "label: Notes", `draw it as publish`},
		{"blocks:\n      - {id: server, label: Server, platform: scripts, parts: [plan, notes]}", "blocks: nope", "blocks must be a list"},
		{"ties: [plan]", "ties: plan", "ties must be a list"},
		{"parts: [plan, notes]", "parts: plan", "parts must be a list"},
		{"implementations:\n  home:", "implementations:\n  12:", "implementation id"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := Check(parseCanonical(t, strings.Replace(canonical, tc.old, tc.new, 1)))
			if !strings.Contains(strings.Join(got, "\n"), tc.want) {
				t.Fatalf("got %v; want %s", got, tc.want)
			}
		})
	}
}

func TestCanonicalViews(t *testing.T) {
	doc := parseCanonical(t, canonical)
	security, err := View(doc, "")
	if err != nil {
		t.Fatal(err)
	}
	platform, err := View(doc, "home")
	if err != nil {
		t.Fatal(err)
	}
	expected := parseCanonical(t, `awml: "0.3"
title: Bread
view: platform
groups:
  - {id: server, label: Server, platform: scripts}
parts:
  - {id: morning, kind: trigger, label: Morning, wait: PT1H}
  - {id: plan, kind: ai, label: Plan, in: server}
  - {id: notes, kind: context, label: Notes, in: server, at: notes}
  - {id: gap, kind: comment, label: Gap}
  - {id: keys, kind: comment, label: Keys, in: server}
lines:
  - {from: morning, to: plan, kind: start}
  - {from: plan, to: notes, kind: data}
  - {from: gap, to: notes, kind: tie}
  - {from: keys, to: plan, kind: tie}
`)
	if Draw(platform) != Draw(expected) {
		t.Fatal("placement Mermaid differs from 0.3")
	}
	if Draw(doc) != Draw(security) {
		t.Fatal("default drawing isn't security")
	}
	if _, err := View(doc, "missing"); err == nil {
		t.Fatal("unknown implementation accepted")
	}
	if _, err := View(security, "home"); err == nil {
		t.Fatal("implementation accepted on 0.3")
	}
	if !reflect.DeepEqual(Summarise(doc), Summarise(security, expected)) {
		t.Fatal("summary differs from pair")
	}
	page := Page([]*Map{doc})
	if strings.Count(page, "<section>") != 2 || !strings.Contains(page, "At home") {
		t.Fatal("page lacks implementation")
	}
	if str(doc, "awml") != "0.4" || str(items(doc, "parts")[1], "in") != "bakery" {
		t.Fatal("projection mutated input")
	}
	description := parseCanonical(t, `awml: "0.4"
platform: {id: scripts, label: Scripts}
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: [notes]
`)
	if got := Check(description); len(got) != 0 {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(Judge(doc, platform, []*Map{description}), Judge(security, expected, []*Map{description})) {
		t.Fatal("fit differs")
	}
	if got := Warnings(parseCanonical(t, strings.Replace(canonical, "label: Plan", "label: Plan, op: do", 1))); len(got) != 1 {
		t.Fatal(got)
	}
}

func TestConvertCanonical(t *testing.T) {
	doc := parseCanonical(t, canonical)
	security, _ := View(doc, "")
	platform, _ := View(doc, "home")
	converted, err := Convert(security, platform)
	if err != nil {
		t.Fatal(err)
	}
	if got := Check(converted); len(got) != 0 {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(Summarise(doc), Summarise(converted)) {
		t.Fatal("conversion changed summary")
	}
	bytes, err := Marshal(converted)
	if err != nil {
		t.Fatal(err)
	}
	if got := Check(parseCanonical(t, string(bytes))); len(got) != 0 {
		t.Fatal(got)
	}
	items(platform, "parts")[1].set("kind", "code")
	if _, err := Convert(security, platform); err == nil || !strings.Contains(err.Error(), `has kind "code"`) {
		t.Fatalf("wrong mismatch error: %v", err)
	}
}

func TestConvertKeepsDesignTitleAndRenamesClashingBlocks(t *testing.T) {
	security := parseCanonical(t, `awml: "0.3"
title: Bread
view: security
groups: [{id: home, label: Home}]
parts: [{id: plan, kind: ai, label: Plan, in: home}]
`)
	platform := parseCanonical(t, `awml: "0.3"
title: Bread on a server
view: platform
groups: [{id: home, label: Server}, {id: child, label: Child, in: home}]
parts: [{id: plan, kind: ai, label: Plan, in: child}]
`)
	converted, err := Convert(security, platform)
	if err != nil {
		t.Fatal(err)
	}
	if str(converted, "title") != "Bread" {
		t.Fatal("lost design title")
	}
	view, err := View(converted, "default")
	if err != nil {
		t.Fatal(err)
	}
	if str(items(view, "groups")[0], "id") != "block_home" || str(items(view, "groups")[1], "in") != "block_home" {
		t.Fatal("bad block rename")
	}
}

func TestCanonicalMultipleImplementationsAndNestedAccess(t *testing.T) {
	source := canonical + `  work:
    label: At work
    blocks:
      - {id: machine, label: Machine, platform: scripts, parts: []}
      - {id: session, label: Session, in: machine, parts: [plan, notes]}
`
	file := parseCanonical(t, source)
	if got := Check(file); len(got) != 0 {
		t.Fatal(got)
	}
	if got := Summarise(file); got.Platforms != 3 || got.Steps.AI != 1 || got.Contexts != 1 {
		t.Fatalf("%+v", got)
	}
	if page := Page([]*Map{file}); strings.Count(page, "<section>") != 3 {
		t.Fatal("not all implementations drawn")
	}
	if got := Check(parseCanonical(t, strings.Split(canonical, "implementations:")[0])); len(got) != 0 {
		t.Fatal(got)
	}
	// These are new fields, so malformed collections must report errors without panicking.
	for _, source := range []string{
		strings.Replace(canonical, "implementations:\n  home:", "implementations: []\nunused:", 1),
		strings.Replace(canonical, "    comments:\n      - {id: keys, label: Keys, in: server, ties: [plan]}", "    comments: [null]", 1),
		strings.Replace(canonical, "scopes:\n  - {id: bakery, label: Bakery, processing: any}", "scopes: [null]", 1),
		strings.Replace(canonical, "label: Keys, in: server", "label: Keys, in: null", 1),
	} {
		if got := Check(parseCanonical(t, source)); len(got) == 0 {
			t.Fatalf("accepted malformed input:\n%s", source)
		}
	}
}

func TestConvertRenamesImplementationCommentsThatClashWithScopes(t *testing.T) {
	security := parseCanonical(t, `awml: "0.3"
title: Bread
view: security
groups: [{id: keys, label: Bakery}]
parts: [{id: plan, kind: ai, label: Plan, in: keys}]
`)
	platform := parseCanonical(t, `awml: "0.3"
title: Bread
view: platform
groups: [{id: server, label: Server}]
parts:
  - {id: plan, kind: ai, label: Plan, in: server}
  - {id: keys, kind: comment, label: Keys, in: server}
lines: [{from: keys, to: plan, kind: tie}]
`)
	converted, err := Convert(security, platform)
	if err != nil {
		t.Fatal(err)
	}
	view, _ := View(converted, "default")
	if str(items(view, "parts")[1], "id") != "comment_keys" || str(items(view, "lines")[0], "from") != "comment_keys" {
		t.Fatal("comment was not renamed with its ties")
	}
}
