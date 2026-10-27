package awml

import (
	"reflect"
	"strings"
	"testing"
)

// The canonical workflow as a 0.4 file, which differs only in its version.
var canonical04 = strings.Replace(canonical, `awml: "0.7"`, `awml: "0.4"`, 1)

// upgraded upgrades a file and gives it as awml convert writes it.
func upgraded(t *testing.T, source string) string {
	t.Helper()
	doc, err := Upgrade(parseCanonical(t, source))
	if err != nil {
		t.Fatal(err)
	}
	data, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCheckRefusesEachOlderVersionAndSaysToConvertIt(t *testing.T) {
	for version, source := range map[string]string{
		"0.2": "awml: \"0.2\"\ntitle: Bread\nview: security\nparts: [{id: plan, kind: ai, label: Plan}]\n",
		"0.3": "awml: \"0.3\"\ntitle: Bread\nview: platform\ngroups: [{id: server, label: Server, platform: scripts}]\nparts: [{id: plan, kind: ai, label: Plan, in: server}]\n",
		"0.4": canonical04,
		"0.5": strings.Replace(canonical, `awml: "0.7"`, `awml: "0.5"`, 1),
		"0.6": strings.Replace(canonical, `awml: "0.7"`, `awml: "0.6"`, 1),
	} {
		mistakes(t, parseCanonical(t, source), `awml is "`+version+`", which this release no longer reads: upgrade the file with awml convert`)
	}
	for _, version := range []string{"0.3", "0.4", "0.5", "0.6"} {
		mistakes(t, parseCanonical(t, strings.Replace(description07, `awml: "0.7"`, `awml: "`+version+`"`, 1)),
			`awml is "`+version+`", which this release no longer reads: upgrade the file with awml convert`)
	}
}

func TestCheckRefusesAnUnknownVersionOrOneThatIsntText(t *testing.T) {
	workflow := canonical
	mistakes(t, parseCanonical(t, strings.Replace(workflow, `awml: "0.7"`, `awml: "0.8"`, 1)), `awml must be "0.7", not "0.8"`)
	mistakes(t, parseCanonical(t, strings.Replace(workflow, `awml: "0.7"`, `awml: 0.7`, 1)), `awml: YAML read it as 0.7, not text: put it in quotes`)
	mistakes(t, parseCanonical(t, strings.Replace(workflow, "awml: \"0.7\"\n", "", 1)), `the file has no awml`)
	mistakes(t, parseCanonical(t, strings.Replace(description07, `awml: "0.7"`, `awml: "0.8"`, 1)), `awml must be "0.7", not "0.8"`)
}

const description07 = `awml: "0.7"
platform: {id: scripts, label: Scripts on a server}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: [notes]
`

func TestUpgradeGivesA07FileUnchanged(t *testing.T) {
	doc := parseCanonical(t, canonical)
	got, err := Upgrade(doc)
	if err != nil || !reflect.DeepEqual(got, doc) {
		t.Fatalf("Upgrade gave %v, %v", got, err)
	}
}

func TestUpgradeFrom04ChangesOnlyTheVersion(t *testing.T) {
	want := `awml: "0.7"
title: Bread
scopes:
  - { id: bakery, label: "Bakery", processing: any }
parts:
  - { id: morning, kind: trigger, label: "Morning", wait: PT1H }
  - { id: plan, kind: ai, label: "Plan", in: bakery }
  - { id: notes, kind: context, label: "Notes", in: bakery, at: notes }
  - { id: gap, kind: comment, label: "Gap" }
lines:
  - { from: morning, to: plan, kind: start }
  - { from: plan, to: notes, kind: data }
  - { from: gap, to: notes, kind: tie }
implementations:
  home:
    label: At home
    blocks:
      - { id: server, label: "Server", platform: scripts, parts: [plan, notes] }
    comments:
      - { id: keys, label: "Keys", in: server, ties: [plan] }
`
	if got := upgraded(t, canonical04); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	mistakes(t, parseCanonical(t, want))
}

const workflow05 = `awml: "0.5"
title: Renewals
scopes:
  - {id: sam, label: "sam: Sam", processing: any}
  - {id: club, label: "club: Sam, members"}
parts:
  - {id: monday, kind: trigger, label: Monday}
  - {id: renew, kind: code, op: do, label: Renew the books, in: sam}
  - {id: standing, kind: approval, label: "Sam's standing approval: renewals only", in: sam}
  - {id: library, kind: outside, label: "Renew at the library: Ask", in: sam}
  - {id: phone, kind: surface, label: Sam's phone, in: sam}
  - {id: board, kind: surface, label: The club's board, in: club}
lines:
  - {from: monday, to: renew, kind: start}
  - {from: renew, to: standing, kind: data, breaks: false}
  - {from: standing, to: library, kind: data}
  - {from: standing, to: phone, kind: data}
  - {from: renew, to: board, kind: publish, label: what was renewed}
implementations:
  home:
    label: At home
    blocks:
      - {id: runner, label: Runner, platform: acme/platforms/ops-runner, parts: [renew, standing, library, phone, board]}
`

func TestUpgradeFrom06ChangesOnlyTheVersionOfAFileThatFiresNoTrigger(t *testing.T) {
	if got, want := upgraded(t, strings.Replace(canonical, `awml: "0.7"`, `awml: "0.6"`, 1)), upgraded(t, canonical); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
}

// A 0.6 workflow that fires the shopping's trigger from a do step, and another workflow's trigger
// from a draft step.
const fires06 = `awml: "0.6"
title: Dinners
scopes:
  - {id: family, label: "family: parents, kids"}
parts:
  - {id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H}
  - {id: plan, kind: ai, op: draft, label: Plan the dinners, in: family}
  - {id: tell, kind: code, op: do, label: Tell the shopping, in: family}
  - {id: ready, kind: trigger, label: The shopping list is ready, wait: PT1H, ref: shopping.yaml#ready}
  - {id: done, kind: trigger, label: The plan is done, ref: notify.yaml#done}
lines:
  - {from: thu, to: plan, kind: start}
  - {from: plan, to: tell, kind: start}
  - {from: tell, to: ready, kind: start}
  - {from: plan, to: done, kind: start}
implementations:
  home:
    label: At home
    blocks:
      - {id: server, label: Server, parts: [plan, tell]}
`

// From 0.7 a trigger is never a destination. A line from a do step into another workflow's trigger
// becomes the do step's outside action, which refers to that trigger. Any other line into a trigger
// is left for the author to redraw, and the check reports it.
func TestUpgradeFrom06MakesADoStepsLineIntoATriggerAnOutsideAction(t *testing.T) {
	want := `awml: "0.7"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids" }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan the dinners", in: family }
  - { id: tell, kind: code, op: do, label: "Tell the shopping", in: family }
  - { id: ready, kind: outside, label: "The shopping list is ready: Allow", ref: shopping.yaml#ready, in: family }
  - { id: done, kind: trigger, label: "The plan is done", ref: notify.yaml#done }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: tell, kind: start }
  - { from: tell, to: ready, kind: start }
  - { from: plan, to: done, kind: start }
implementations:
  home:
    label: At home
    blocks:
      - { id: server, label: "Server", parts: [plan, tell, ready] }
`
	got := upgraded(t, fires06)
	if got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	mistakes(t, parseCanonical(t, got),
		`part "done" has a ref, but a trigger belongs to the workflow it starts: start another workflow with an outside action whose ref names its trigger`,
		`the line from "plan" to "done" goes into trigger "done": a trigger only starts its own workflow, so start another workflow with an outside action whose ref names its trigger`)
}

// Only a trigger, connector or context is marked untrusted, so the mark on a trigger a do step
// fired is dropped, and convert says it belongs on the trigger of the workflow the action starts.
func TestConvertSaysAnUntrustedTriggerItTurnsIntoAnOutsideActionIsMarkedThere(t *testing.T) {
	source := strings.Replace(fires06, "wait: PT1H, ref: shopping.yaml#ready", "wait: PT1H, untrusted: true, ref: shopping.yaml#ready", 1)
	data, notes, found, err := ConvertSources([]Source{{File: "dinners.yaml", Text: []byte(source)}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := upgraded(t, fires06); string(data) != want {
		t.Errorf("convert gave\n%s\nwant\n%s", data, want)
	}
	if want := []string{`part "ready" was a trigger marked untrusted, and an outside action isn't: mark the trigger it starts, shopping.yaml#ready, untrusted instead`}; !reflect.DeepEqual(notes, want) {
		t.Errorf("convert notes %q\nwant %q", notes, want)
	}
	if len(found) != 2 {
		t.Errorf("convert finds %q, want the two mistakes of the trigger the draft fires", found)
	}
	// A file that loses nothing has nothing to note.
	if _, notes, _, err := ConvertSources([]Source{{Text: []byte(fires06)}}, ""); err != nil || notes != nil {
		t.Errorf("convert notes %q, %v", notes, err)
	}
}

func TestUpgradeFrom05WritesEachPublishAsDataAndDropsBreaksFalse(t *testing.T) {
	want := `awml: "0.7"
title: Renewals
scopes:
  - { id: sam, label: "sam: Sam", processing: any }
  - { id: club, label: "club: Sam, members" }
parts:
  - { id: monday, kind: trigger, label: "Monday" }
  - { id: renew, kind: code, op: do, label: "Renew the books", in: sam }
  - { id: standing, kind: approval, label: "Sam's standing approval: renewals only", in: sam }
  - { id: library, kind: outside, label: "Renew at the library: Ask", in: sam }
  - { id: phone, kind: surface, label: "Sam's phone", in: sam }
  - { id: board, kind: surface, label: "The club's board", in: club }
lines:
  - { from: monday, to: renew, kind: start }
  - { from: renew, to: standing, kind: data }
  - { from: standing, to: library, kind: data }
  - { from: standing, to: phone, kind: data }
  - { from: renew, to: board, kind: data, label: what was renewed }
implementations:
  home:
    label: At home
    blocks:
      - { id: runner, label: "Runner", platform: acme/platforms/ops-runner, parts: [renew, standing, library, phone, board] }
`
	got := upgraded(t, workflow05)
	if got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	doc := parseCanonical(t, got)
	mistakes(t, doc)
	view, _ := View(doc, "")
	if line := items(view, "lines")[4]; str(line, "kind") != "publish" || !strings.HasPrefix(str(line, "label"), "what was renewed · ") {
		t.Fatalf("the publish isn't drawn as one: %v", line)
	}
}

const redLine05 = `awml: "0.5"
title: The club's Friday message
scopes:
  - {id: ella, label: "ella: Ella"}
parts:
  - {id: fri, kind: trigger, label: "Friday 16:00"}
  - {id: write, kind: ai, op: draft, label: Draft the message, in: ella}
  - {id: ask, kind: code, op: do, label: Ask Ella to send it, in: ella}
  - {id: approve, kind: review, label: Ella's yes, in: ella}
  - {id: send, kind: outside, label: "Send it: Ask", in: ella}
lines:
  - {from: fri, to: write, kind: start}
  - {from: write, to: ask, kind: data}
  - {from: ask, to: approve, kind: data}
  - {from: approve, to: send, kind: data, label: "[yes]"}
  - {from: write, to: send, kind: data, label: sent without Ella's yes, breaks: true}
`

func TestUpgradeKeepsARedLineTheCheckFinds(t *testing.T) {
	got := parseCanonical(t, upgraded(t, redLine05))
	if parts := items(got, "parts"); len(parts) != 5 {
		t.Fatalf("Upgrade added a part for a red line the check finds: %v", parts)
	}
	if got := Unsound(got); len(got) != 0 {
		t.Fatal(got)
	}
	view, _ := View(got, "")
	if !drawnRed(items(view, "lines")[4]) {
		t.Fatal("the red line isn't red")
	}
}

func TestUpgradeTiesACommentToARedLineTheCheckCantFind(t *testing.T) {
	source := strings.Replace(redLine05, "{from: write, to: ask, kind: data}", "{from: write, to: ask, kind: data, label: read the whole inbox, breaks: true}", 1)
	got := upgraded(t, source)
	for _, want := range []string{
		"  - { id: red_write_ask, kind: comment, label: \"Red line: read the whole inbox\" }\n",
		"  - { from: write, to: ask, kind: data, label: read the whole inbox }\n",
		"  - { from: red_write_ask, to: write, kind: tie }\n  - { from: red_write_ask, to: ask, kind: tie }\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Upgrade gave\n%s\nwant it to hold\n%s", got, want)
		}
	}
	if got := Unsound(parseCanonical(t, got)); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestUpgradeNamesARedLineWithNoLabelByItsEnds(t *testing.T) {
	source := strings.Replace(redLine05, "{from: write, to: ask, kind: data}", "{from: write, to: ask, kind: data, breaks: true}", 1)
	source = strings.Replace(source, "  - {id: fri,", "  - {id: red_write_ask, kind: comment, label: Taken}\n  - {id: fri,", 1)
	got := upgraded(t, source)
	if !strings.Contains(got, "  - { id: red_write_ask_2, kind: comment, label: \"Red line from \\\"write\\\" to \\\"ask\\\"\" }\n") {
		t.Fatalf("Upgrade gave\n%s", got)
	}
}

func TestUpgradeFrom03Or02SecurityViewMakesAWorkflowWithNoImplementation(t *testing.T) {
	for _, version := range []string{"0.2", "0.3"} {
		source := `# The family's dinners.
awml: "` + version + `"
title: Dinners
view: security
groups:
  - {id: family, label: "family: parents, kids"}
parts:
  - {id: thu, kind: trigger, label: Thursday}
  - {id: plan, kind: ai, label: Plan, in: family}
  - {id: board, kind: surface, label: The board}
lines:
  - {from: thu, to: plan, kind: start}
  - {from: plan, to: board, kind: publish, label: posted}
`
		want := `awml: "0.7"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids" }
parts:
  - { id: thu, kind: trigger, label: "Thursday" }
  - { id: plan, kind: ai, label: "Plan", in: family }
  - { id: board, kind: surface, label: "The board" }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: board, kind: data, label: posted }
`
		if got := upgraded(t, source); got != want {
			t.Errorf("Upgrade of %s gave\n%s\nwant\n%s", version, got, want)
		}
	}
}

func TestUpgradeMovesAPlatformDescriptionToTheCurrentVersion(t *testing.T) {
	for _, version := range []string{"0.3", "0.4", "0.5", "0.6"} {
		source := strings.Replace(description07,`awml: "0.7"`, `awml: "`+version+`"`, 1)
		want := `awml: "0.7"
platform: { id: scripts, label: "Scripts on a server" }
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: [notes]
`
		got := upgraded(t, source)
		if got != want {
			t.Errorf("Upgrade of %s gave\n%s\nwant\n%s", version, got, want)
		}
		mistakes(t, parseCanonical(t, got))
	}
}

func TestUpgradeRefusesWhatItCantConvertAlone(t *testing.T) {
	for source, want := range map[string]string{
		"awml: \"0.3\"\ntitle: Bread\nview: platform\nparts: []\n": "a platform view converts only with its security view: awml convert security.yaml platform.yaml",
		"awml: \"0.1\"\ntitle: Bread\nparts: []\n":                 `awml convert reads versions 0.2 to 0.7, not "0.1"`,
		"awml: \"0.5\"\ntitle: Bread\nparts: 3\n":                  "parts must be a list of mappings",
		"- a list\n": "the file must be a mapping of fields, starting with awml",
	} {
		doc, err := Parse([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Upgrade(doc); err == nil || err.Error() != want {
			t.Errorf("Upgrade of\n%s\ngave %v, want %s", source, err, want)
		}
	}
}

func TestConvertMakesA07FileFromALegacyPair(t *testing.T) {
	for _, version := range []string{"0.2", "0.3"} {
		security := parseCanonical(t, `awml: "`+version+`"
title: Bread
view: security
groups: [{id: bakery, label: Bakery}]
parts:
  - {id: plan, kind: ai, label: Plan, in: bakery}
  - {id: list, kind: surface, label: The list}
lines: [{from: plan, to: list, kind: publish, breaks: false}]
`)
		platform := parseCanonical(t, `awml: "`+version+`"
title: Bread
view: platform
groups: [{id: server, label: Server}]
parts:
  - {id: plan, kind: ai, label: Plan, in: server}
  - {id: list, kind: surface, label: The list, in: server}
lines: [{from: plan, to: list, kind: publish}]
`)
		converted, err := Convert(security, platform)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := Marshal(converted)
		want := `awml: "0.7"
title: Bread
scopes:
  - { id: bakery, label: "Bakery" }
parts:
  - { id: plan, kind: ai, label: "Plan", in: bakery }
  - { id: list, kind: surface, label: "The list" }
lines:
  - { from: plan, to: list, kind: data }
implementations:
  default:
    label: Default
    blocks:
      - { id: server, label: "Server", parts: [plan, list] }
`
		if string(data) != want {
			t.Errorf("Convert of %s gave\n%s\nwant\n%s", version, data, want)
		}
	}
}
