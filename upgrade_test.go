package awml

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The canonical workflow as a 0.4 file, which differs only in its version.
var canonical04 = strings.Replace(canonical, `awml: "0.10"`, `awml: "0.4"`, 1)

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
		"0.5": strings.Replace(canonical, `awml: "0.10"`, `awml: "0.5"`, 1),
		"0.6": strings.Replace(canonical, `awml: "0.10"`, `awml: "0.6"`, 1),
		"0.7": strings.Replace(canonical, `awml: "0.10"`, `awml: "0.7"`, 1),
		"0.8": strings.Replace(canonical, `awml: "0.10"`, `awml: "0.8"`, 1),
		"0.9": strings.Replace(canonical, `awml: "0.10"`, `awml: "0.9"`, 1),
	} {
		mistakes(t, parseCanonical(t, source), `awml is "`+version+`", which this release no longer reads: upgrade the file with awml convert`)
	}
	for _, version := range []string{"0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9"} {
		mistakes(t, parseCanonical(t, strings.Replace(description09, `awml: "0.10"`, `awml: "`+version+`"`, 1)),
			`awml is "`+version+`", which this release no longer reads: upgrade the file with awml convert`)
	}
}

func TestCheckRefusesAnUnknownVersionOrOneThatIsntText(t *testing.T) {
	workflow := canonical
	mistakes(t, parseCanonical(t, strings.Replace(workflow, `awml: "0.10"`, `awml: "1.0"`, 1)), `awml must be "0.10", not "1.0"`)
	mistakes(t, parseCanonical(t, strings.Replace(workflow, `awml: "0.10"`, `awml: 0.10`, 1)), `awml: YAML read it as 0.1, not text: put it in quotes`)
	mistakes(t, parseCanonical(t, strings.Replace(workflow, "awml: \"0.10\"\n", "", 1)), `the file has no awml`)
	mistakes(t, parseCanonical(t, strings.Replace(description09, `awml: "0.10"`, `awml: "1.0"`, 1)), `awml must be "0.10", not "1.0"`)
}

const description09 = `awml: "0.10"
platform: {id: scripts, label: Scripts on a server}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: [notes]
`

func TestUpgradeGivesA09FileUnchanged(t *testing.T) {
	doc := parseCanonical(t, canonical)
	got, err := Upgrade(doc)
	if err != nil || !reflect.DeepEqual(got, doc) {
		t.Fatalf("Upgrade gave %v, %v", got, err)
	}
}

func TestUpgradeFrom04ChangesOnlyTheVersion(t *testing.T) {
	want := `awml: "0.10"
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

// 0.10 changed only a rule, that only a step writes to a context, so a 0.9 file changes only its
// version. convert can't tell which step a line into a context from anything else meant, so it
// leaves the line for the check to report.
func TestUpgradeFrom09ChangesOnlyTheVersion(t *testing.T) {
	if got, want := upgraded(t, strings.Replace(canonical, `awml: "0.10"`, `awml: "0.9"`, 1)), upgraded(t, canonical); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	copied := strings.Replace(reply, "lines:\n", "  - { id: sent, kind: context, label: Sent replies }\nlines:\n", 1) + "  - { from: drafts, to: sent, kind: data }\n"
	data, notes, found, err := ConvertSources([]Source{{Text: []byte(strings.Replace(copied, `awml: "0.10"`, `awml: "0.9"`, 1))}}, "")
	if err != nil || notes != nil || string(data) != upgraded(t, copied) {
		t.Fatalf("convert of a 0.9 file gave\n%s\nnotes %q, %v, want\n%s", data, notes, err, upgraded(t, copied))
	}
	if want := []string{`the line from "drafts" to "sent" goes from context "drafts" into context "sent": only a step writes to a context, so draw a recorded sign-off as a review whose [yes] leads to a code do step that writes it, something brought in from outside as a connector that a bring-in step reads, then writes, and a copy from one context into another as a step that reads one and writes the other`}; !reflect.DeepEqual(found, want) {
		t.Errorf("convert of a 0.9 file found %q, want %q", found, want)
	}
}

// 0.9 changed only a rule, that no line goes from an outside action into a context, so a 0.8 file
// changes only its version, and convert leaves such a line for the check to report.
func TestUpgradeFrom08ChangesOnlyTheVersion(t *testing.T) {
	if got, want := upgraded(t, strings.Replace(canonical, `awml: "0.10"`, `awml: "0.8"`, 1)), upgraded(t, canonical); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	signedOff := reply + "  - { from: send, to: drafts, kind: data }\n"
	data, notes, found, err := ConvertSources([]Source{{Text: []byte(strings.Replace(signedOff, `awml: "0.10"`, `awml: "0.8"`, 1))}}, "")
	if err != nil || notes != nil || string(data) != upgraded(t, signedOff) {
		t.Fatalf("convert of a 0.8 file gave\n%s\nnotes %q, %v, want\n%s", data, notes, err, upgraded(t, signedOff))
	}
	if want := []string{`the line from "send" to "drafts" goes from outside action "send" into context "drafts": only a step writes to a context, so draw a recorded sign-off as a review whose [yes] leads to a code do step that writes it, something brought in from outside as a connector that a bring-in step reads, then writes, and a copy from one context into another as a step that reads one and writes the other`}; !reflect.DeepEqual(found, want) {
		t.Errorf("convert of a 0.8 file found %q, want %q", found, want)
	}
}

// A converted file with errors is still valid, so convert finds nothing wrong with it, and the
// check says its errors.
func TestConvertFindsNoMistakeInADrawingWithErrors(t *testing.T) {
	live := strings.Replace(redLineWorkflow, `awml: "0.10"`, `awml: "0.8"`, 1)
	data, notes, found, err := ConvertSources([]Source{{Text: []byte(live)}}, "")
	if err != nil || notes != nil || found != nil || string(data) != redLineWorkflow {
		t.Errorf("convert gave\n%s\nnotes %q, mistakes %q, %v", data, notes, found, err)
	}
	if _, _, found, _ := ConvertSources([]Source{{Text: []byte(redLineWorkflow)}}, ""); found != nil {
		t.Errorf("convert of a current file finds %q", found)
	}
}

// Apart from dropping forks, 0.8 changed only rules, what a standing approval may cover and that
// untrusted text reaching an action unchecked is a mistake, so a 0.7 file with no fork changes only
// its version, even one that still has a line from a do step into a trigger with a ref, which
// converting a 0.6 file would turn into an outside action.
func TestUpgradeFrom07ChangesOnlyTheVersion(t *testing.T) {
	if got, want := upgraded(t, strings.Replace(canonical, `awml: "0.10"`, `awml: "0.7"`, 1)), upgraded(t, canonical); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	source := strings.Replace(fires06, `awml: "0.6"`, `awml: "0.7"`, 1)
	if got, want := upgraded(t, source), upgraded(t, strings.Replace(fires06, `awml: "0.6"`, `awml: "0.10"`, 1)); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	if _, notes, _, err := ConvertSources([]Source{{Text: []byte(strings.Replace(source, "wait: PT1H, ref:", "wait: PT1H, untrusted: true, ref:", 1))}}, ""); err != nil || notes != nil {
		t.Errorf("convert of a 0.7 file notes %q, %v", notes, err)
	}
}

// The school newsletter as 0.7 drew it, with a fork splitting the work between two steps.
const newsletter07 = `awml: "0.7"
title: The school newsletter
scopes:
  - {id: family, label: "family: Mia, Dev"}
parts:
  - {id: arrives, kind: trigger, label: The newsletter arrives}
  - {id: split, kind: fork, in: family}
  - {id: learn, kind: ai, op: learn, label: Take the term dates, in: family}
  - {id: due, kind: ai, op: check, label: Look for anything due, in: family}
  - {id: chat, kind: surface, label: Family chat, in: family}
lines:
  - {from: arrives, to: split, kind: start}
  - {from: split, to: learn, kind: start}
  - {from: split, to: due, kind: start, label: the newsletter}
  - {from: due, to: chat, kind: data}
implementations:
  home:
    label: At home
    blocks:
      - {id: server, label: Server, parts: [split, learn, due, chat]}
`

// The same work drawn without a fork: two start lines out of the trigger.
const newsletter09 = `awml: "0.10"
title: The school newsletter
scopes:
  - {id: family, label: "family: Mia, Dev"}
parts:
  - {id: arrives, kind: trigger, label: The newsletter arrives}
  - {id: learn, kind: ai, op: learn, label: Take the term dates, in: family}
  - {id: due, kind: ai, op: check, label: Look for anything due, in: family}
  - {id: chat, kind: surface, label: Family chat, in: family}
lines:
  - {from: arrives, to: learn, kind: start}
  - {from: arrives, to: due, kind: start, label: the newsletter}
  - {from: due, to: chat, kind: data}
implementations:
  home:
    label: At home
    blocks:
      - {id: server, label: Server, parts: [learn, due, chat]}
`

// 0.10 has no fork, since several start lines out of one part already show work side by side. So
// converting drops each fork, and joins each line into it to each line out of it, with the line
// out's kind and label. The file checks and draws as the same work drawn without one.
func TestConvertDropsAForkAndJoinsTheLinesThroughIt(t *testing.T) {
	got, want := upgraded(t, newsletter07), upgraded(t, newsletter09)
	if got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
	converted, without := parseCanonical(t, got), parseCanonical(t, newsletter09)
	if !reflect.DeepEqual(Check(converted), Check(without)) || Draw(converted) != Draw(without) {
		t.Errorf("the converted file checks as %q and draws as\n%s", Check(converted), Draw(converted))
	}
	for _, version := range []string{"0.4", "0.5", "0.6"} {
		if got := upgraded(t, strings.Replace(newsletter07, `awml: "0.7"`, `awml: "`+version+`"`, 1)); got != want {
			t.Errorf("Upgrade from %s gave\n%s\nwant\n%s", version, got, want)
		}
	}
}

// Each line into a fork joins each line out of it, through forks in a row, and a joined line the
// file already has isn't drawn twice. A comment tied to a fork is tied instead to the parts the work
// split from, a hidden line at a fork goes, and so does the fork's place in each implementation.
func TestConvertJoinsEveryLineThroughForksInARow(t *testing.T) {
	source := `awml: "0.7"
title: Bread
parts:
  - {id: morning, kind: trigger, label: Morning}
  - {id: order, kind: trigger, label: An order}
  - {id: first, kind: fork}
  - {id: second, kind: fork}
  - {id: bake, kind: code, label: Bake}
  - {id: pack, kind: code, label: Pack}
  - {id: why, kind: comment, label: Both at once}
lines:
  - {from: morning, to: first, kind: start}
  - {from: order, to: first, kind: data}
  - {from: first, to: second, kind: start}
  - {from: second, to: bake, kind: start}
  - {from: second, to: pack, kind: data, label: the order}
  - {from: morning, to: bake, kind: start}
  - {from: why, to: second, kind: tie}
  - {from: pack, to: first, kind: hidden}
implementations:
  home:
    label: At home
    blocks:
      - {id: kitchen, label: Kitchen, parts: [first, second, bake, pack]}
    comments:
      - {id: note, label: One oven, in: kitchen, ties: [first, bake]}
`
	want := `awml: "0.10"
title: Bread
parts:
  - { id: morning, kind: trigger, label: "Morning" }
  - { id: order, kind: trigger, label: "An order" }
  - { id: bake, kind: code, label: "Bake" }
  - { id: pack, kind: code, label: "Pack" }
  - { id: why, kind: comment, label: "Both at once" }
lines:
  - { from: order, to: bake, kind: start }
  - { from: morning, to: pack, kind: data, label: the order }
  - { from: order, to: pack, kind: data, label: the order }
  - { from: morning, to: bake, kind: start }
  - { from: why, to: morning, kind: tie }
  - { from: why, to: order, kind: tie }
implementations:
  home:
    label: At home
    blocks:
      - { id: kitchen, label: "Kitchen", parts: [bake, pack] }
    comments:
      - { id: note, label: "One oven", in: kitchen, ties: [morning, order, bake] }
`
	if got := upgraded(t, source); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
}

// A line out of a fork with no label of its own keeps the label of the line into it, so a review's
// "[yes]" stays on each line its work goes ahead by. A line out's own label wins.
func TestConvertKeepsTheLabelOfTheLineIntoAFork(t *testing.T) {
	source := `awml: "0.7"
title: Post
parts:
  - {id: draft, kind: ai, label: Draft}
  - {id: look, kind: review, label: Look it over}
  - {id: split, kind: fork}
  - {id: send, kind: code, label: Send}
  - {id: file, kind: code, label: File}
lines:
  - {from: draft, to: look, kind: start}
  - {from: look, to: split, kind: start, label: "[yes]"}
  - {from: split, to: send, kind: start}
  - {from: split, to: file, kind: start, label: the post}
`
	want := `awml: "0.10"
title: Post
parts:
  - { id: draft, kind: ai, label: "Draft" }
  - { id: look, kind: review, label: "Look it over" }
  - { id: send, kind: code, label: "Send" }
  - { id: file, kind: code, label: "File" }
lines:
  - { from: draft, to: look, kind: start }
  - { from: look, to: send, kind: start, label: "[yes]" }
  - { from: look, to: file, kind: start, label: the post }
`
	if got := upgraded(t, source); got != want {
		t.Fatalf("Upgrade gave\n%s\nwant\n%s", got, want)
	}
}

// A fork that nothing leads into never ran, so it goes with every line out of it, and convert names
// it, for the author to draw where that work starts by hand. A fork something leads into has nothing
// to note.
func TestConvertNamesAForkThatNothingLeadsInto(t *testing.T) {
	source := `awml: "0.7"
title: Bread
parts:
  - {id: morning, kind: trigger, label: Morning}
  - {id: idle, kind: fork}
  - {id: split, kind: fork}
  - {id: bake, kind: code, label: Bake}
  - {id: pack, kind: code, label: Pack}
lines:
  - {from: idle, to: bake, kind: start}
  - {from: morning, to: split, kind: start}
  - {from: split, to: pack, kind: start}
`
	want := `awml: "0.10"
title: Bread
parts:
  - { id: morning, kind: trigger, label: "Morning" }
  - { id: bake, kind: code, label: "Bake" }
  - { id: pack, kind: code, label: "Pack" }
lines:
  - { from: morning, to: pack, kind: start }
`
	for _, version := range []string{"0.7", "0.6"} {
		data, notes, _, err := ConvertSources([]Source{{Text: []byte(strings.Replace(source, `awml: "0.7"`, `awml: "`+version+`"`, 1))}}, "")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("convert from %s gave\n%s\nwant\n%s", version, data, want)
		}
		if want := []string{`part "idle" was a fork that nothing led into, so it never ran: it went, with every line out of it`}; !reflect.DeepEqual(notes, want) {
			t.Errorf("convert from %s notes %q\nwant %q", version, notes, want)
		}
	}
	if _, notes, _, err := ConvertSources([]Source{{Text: []byte(newsletter07)}}, ""); err != nil || notes != nil {
		t.Errorf("convert notes %q, %v", notes, err)
	}
}

// Each fork's parts are worked out once, so forks in a row, each with two lines into the one before,
// convert at once rather than in a time that doubles with each fork.
func TestConvertWorksOutEachForkOnce(t *testing.T) {
	var source strings.Builder
	source.WriteString(`awml: "0.7"
title: Many
parts:
  - {id: go, kind: trigger, label: Go}
  - {id: work, kind: code, label: Work}
`)
	for n := 1; n <= 40; n++ {
		fmt.Fprintf(&source, "  - {id: f%d, kind: fork}\n", n)
	}
	source.WriteString(`lines:
  - {from: go, to: f40, kind: start}
  - {from: f1, to: work, kind: start}
`)
	for n := 1; n < 40; n++ {
		fmt.Fprintf(&source, "  - {from: f%d, to: f%d, kind: start}\n", n+1, n)
		fmt.Fprintf(&source, "  - {from: f%d, to: f%d, kind: start}\n", n+1, n)
	}
	source.WriteString(`implementations:
  home:
    label: Home
    blocks:
      - {id: desk, label: Desk, parts: [work]}
    comments:
      - {id: note, label: Once, in: desk, ties: [f1]}
`)
	doc := parseCanonical(t, source.String())
	done := make(chan *Map, 1)
	go func() {
		file, _ := Upgrade(doc)
		done <- file
	}()
	select {
	case file := <-done:
		data, err := Marshal(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "ties: [go]") {
			t.Errorf("Upgrade gave\n%s\nwant the note tied to go", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Upgrade took more than five seconds over forty forks in a row")
	}
}

func TestUpgradeFrom06ChangesOnlyTheVersionOfAFileThatFiresNoTrigger(t *testing.T) {
	if got, want := upgraded(t, strings.Replace(canonical, `awml: "0.10"`, `awml: "0.6"`, 1)), upgraded(t, canonical); got != want {
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
	want := `awml: "0.10"
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

// A long comment at the top of an older file said what the workflow was for, as a description now
// does, so convert makes it the description and leaves every other comment at the top.
func TestConvertMakesTheCommentAFileStartsWithItsDescription(t *testing.T) {
	old := `
# Plans the family's dinners each Thursday.
#
#  - The family is fictional.
# Bread comes from the bakery.

# The week starts on Thursday.
awml: "0.5"
title: Dinners
parts:
  - {id: thu, kind: trigger, label: Thursday} # After the school run.
`
	want := `# The week starts on Thursday.
# After the school run.
awml: "0.10"
title: Dinners
description: |
  Plans the family's dinners each Thursday.

   - The family is fictional.
  Bread comes from the bakery.
parts:
  - { id: thu, kind: trigger, label: "Thursday" }
`
	data, _, _, err := ConvertSources([]Source{{Text: []byte(old)}}, "")
	if err != nil || string(data) != want {
		t.Errorf("convert gave %v\n%s\nwant\n%s", err, data, want)
	}
	// A file that already has a description keeps its comment as a comment.
	described := strings.Replace(old, "title: Dinners\n", "title: Dinners\ndescription: Dinners.\n", 1)
	data, _, _, err = ConvertSources([]Source{{Text: []byte(described)}}, "")
	if err != nil || !strings.HasPrefix(string(data), "# Plans the family's dinners each Thursday.\n#\n") || !strings.Contains(string(data), "\ndescription: Dinners.\n") {
		t.Errorf("convert gave %v\n%s", err, data)
	}
	// A platform description has no description, so its comment stays.
	platform := "# The server in the hall.\n" + strings.Replace(description09, `awml: "0.10"`, `awml: "0.7"`, 1)
	data, _, _, err = ConvertSources([]Source{{Text: []byte(platform)}}, "")
	if err != nil || !strings.HasPrefix(string(data), "# The server in the hall.\nawml: \"0.10\"\n") {
		t.Errorf("convert gave %v\n%s", err, data)
	}
	// Empty # lines that end the comment are dropped, so the description is still a literal block.
	trailing := "# Dinners.\n#\n#\nawml: \"0.5\"\ntitle: Dinners\nparts: []\n"
	data, _, _, err = ConvertSources([]Source{{Text: []byte(trailing)}}, "")
	if err != nil || string(data) != "awml: \"0.10\"\ntitle: Dinners\ndescription: |\n  Dinners.\nparts: []\n" {
		t.Errorf("convert gave %v\n%s", err, data)
	}
	// A comment of only empty # lines says nothing, so it stays at the top.
	empty := "#\n#\nawml: \"0.5\"\ntitle: Dinners\nparts: []\n"
	data, _, _, err = ConvertSources([]Source{{Text: []byte(empty)}}, "")
	if err != nil || string(data) != "#\n#\nawml: \"0.10\"\ntitle: Dinners\nparts: []\n" {
		t.Errorf("convert gave %v\n%s", err, data)
	}
	// A comment indented after its # loses the indent its lines share, and a tab becomes spaces, so
	// the description is still a literal block.
	indented := "#   Plans the dinners.\n#\n#     - The family is fictional.\n#\tBread too.\nawml: \"0.5\"\ntitle: Dinners\nparts: []\n"
	data, _, _, err = ConvertSources([]Source{{Text: []byte(indented)}}, "")
	if err != nil || string(data) != "awml: \"0.10\"\ntitle: Dinners\ndescription: |\n  Plans the dinners.\n\n    - The family is fictional.\n   Bread too.\nparts: []\n" {
		t.Errorf("convert gave %v\n%s", err, data)
	}
	// A first line indented more than the rest loses its indent, since a literal block can't start
	// with a space.
	deeper := "#   - Plans the dinners.\n# Bread too.\nawml: \"0.5\"\ntitle: Dinners\nparts: []\n"
	data, _, _, err = ConvertSources([]Source{{Text: []byte(deeper)}}, "")
	if err != nil || string(data) != "awml: \"0.10\"\ntitle: Dinners\ndescription: |\n  - Plans the dinners.\n  Bread too.\nparts: []\n" {
		t.Errorf("convert gave %v\n%s", err, data)
	}
	// A file that starts with its fields, not a comment, has no description to take.
	late := "awml: \"0.5\"\n# Dinners.\ntitle: Dinners\nparts: []\n"
	data, _, _, err = ConvertSources([]Source{{Text: []byte(late)}}, "")
	if err != nil || string(data) != "# Dinners.\nawml: \"0.10\"\ntitle: Dinners\nparts: []\n" {
		t.Errorf("convert gave %v\n%s", err, data)
	}
}

func TestUpgradeFrom05WritesEachPublishAsDataAndDropsBreaksFalse(t *testing.T) {
	want := `awml: "0.10"
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
		want := `awml: "0.10"
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
	for _, version := range []string{"0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9"} {
		source := strings.Replace(description09, `awml: "0.10"`, `awml: "`+version+`"`, 1)
		want := `awml: "0.10"
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
		"awml: \"0.1\"\ntitle: Bread\nparts: []\n":                 `awml convert reads versions 0.2 to 0.10, not "0.1"`,
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
		want := `awml: "0.10"
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
