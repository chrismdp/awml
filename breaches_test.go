package awml

import (
	"reflect"
	"strings"
	"testing"
)

// The swimming club's Friday message as the live system ran it one week, written in 0.8. The design
// waits for Ella's yes, but the system sent the draft straight out. Ella is fictional.
const fridaySent = `awml: "0.8"
title: The club's Friday message, one week
scopes:
  - { id: ella, label: "ella: Ella" }
parts:
  - { id: fri, kind: trigger, label: "Friday 16:00" }
  - { id: write, kind: ai, op: draft, label: "Draft the week's message to the parents", in: ella }
  - { id: ask, kind: code, op: do, label: "Ask Ella to send it", in: ella }
  - { id: approve, kind: review, label: "Ella's yes to the exact message", in: ella }
  - { id: send, kind: outside, label: "Send it to the parents: Ask", in: ella }
lines:
  - { from: fri, to: write, kind: start }
  - { from: write, to: ask, kind: data, label: the draft }
  - { from: ask, to: approve, kind: data }
  - { from: approve, to: send, kind: data, label: "[yes]" }
  - { from: write, to: send, kind: data, label: "sent without Ella's yes" }
implementations:
  club:
    label: The club's laptop
    blocks:
      - { id: laptop, label: "Club laptop: claude-sonnet-4-5", parts: [write, ask, approve, send] }
`

var fridayBreaches = []string{
	`part "write" leads to outside action "send", but its op is draft: only a do step leads to an outside action`,
	`outside action "send" is Ask, but part "write" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`,
}

// The red line's label: its own words, then the check's words for each rule it breaks.
const fridayRedLabel = `|"sent without Ella#39;s yes<br>part #34;write#34; leads to outside action #34;send#34;#44; but its op is draft<br>outside action #34;send#34; is Ask#44; but part #34;write#34; leads to it with no person in between"|`

func TestCheckStillReportsEachLineThatBreaksARule(t *testing.T) {
	mistakes(t, parsed(t, fridaySent), fridayBreaches...)
}

func TestA06LineHasNoBreaks(t *testing.T) {
	src := strings.Replace(fridaySent, `label: "sent without Ella's yes" }`, `label: "sent without Ella's yes", breaks: true }`, 1)
	mistakes(t, parsed(t, src), append([]string{
		`the line from "write" to "send" has breaks, but awml check works out which lines break a rule, and awml draw draws them red`},
		fridayBreaches...)...)
}

func TestOnlyALineThatBreaksARuleLeavesAFileDrawable(t *testing.T) {
	if got := Unsound(parsed(t, fridaySent)); len(got) != 0 {
		t.Errorf("Unsound gave %q", got)
	}
	bad := strings.Replace(fridaySent, "op: draft", "op: drafting", 1)
	if got := Unsound(parsed(t, bad)); !reflect.DeepEqual(got, []string{`part "write" has unknown op "drafting"`}) {
		t.Errorf("Unsound gave %q", got)
	}
}

func TestDrawDrawsRedEachLineTheCheckReports(t *testing.T) {
	doc := parsed(t, fridaySent)
	for _, implementation := range []string{"", "club"} {
		view, err := View(doc, implementation)
		if err != nil {
			t.Fatal(err)
		}
		out := Draw(view)
		has(t, out, "\n  write -->"+fridayRedLabel+" send\n")
		has(t, out, "\n  linkStyle 4 "+red)
	}
}

func TestARedLineWithNoLabelHasTheChecksWordsAlone(t *testing.T) {
	src := strings.Replace(fridaySent, `, label: "sent without Ella's yes" }`, " }", 1)
	has(t, Draw(parsed(t, src)), `write -->|"part #34;write#34; leads`)
}

func TestEachRuleALineBreaksDrawsItRed(t *testing.T) {
	for _, c := range []struct{ name, src, line, words, style string }{
		{"a line into a Block action", `awml: "0.8"
title: T
parts:
  - { id: act, kind: code, op: do, label: Act }
  - { id: out, kind: outside, label: "Delete it: Block" }
lines:
  - { from: act, to: out, kind: start }
`, "act -->", `outside action #34;out#34; is Block#44; but part #34;act#34; leads to it`, "0"},
		{"an Allow action behind a review", `awml: "0.8"
title: T
parts:
  - { id: act, kind: code, op: do, label: Act }
  - { id: yes, kind: review, label: Yes }
  - { id: out, kind: outside, label: "Note it: Allow" }
lines:
  - { from: act, to: yes, kind: start }
  - { from: yes, to: out, kind: start }
`, "yes -->", `outside action #34;out#34; is Allow#44; but review #34;yes#34; stands in front of it`, "1"},
		{"an answer writing to a context", `awml: "0.8"
title: T
parts:
  - { id: answer, kind: ai, op: answer, label: Answer }
  - { id: notes, kind: context, label: Notes }
lines:
  - { from: answer, to: notes, kind: data }
`, "answer -.->", `part #34;answer#34; writes to context #34;notes#34;#44; but its op is answer`, "0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := parsed(t, c.src)
			if len(Check(doc)) != 1 || len(Unsound(doc)) != 0 {
				t.Fatalf("Check gave %q and Unsound %q", Check(doc), Unsound(doc))
			}
			out := Draw(doc)
			has(t, out, "\n  "+c.line+`|"`+c.words+`"|`)
			has(t, out, "\n  linkStyle "+c.style+" "+red)
		})
	}
}

func TestAMistakeThatIsntALinesIsDrawnAsBefore(t *testing.T) {
	// A standing approval with no line to a surface is a mistake of the part, so no line is red.
	src := strings.Replace(standing, "  - { from: ana_ok, to: list, kind: data, label: \"each change, with how to undo it\" }\n", "", 1)
	if len(Check(parsed(t, src))) != 1 || strings.Contains(Draw(parsed(t, src)), red) {
		t.Error("a mistake that isn't a line's draws a red line")
	}
}

func TestUntrustedTextGoesAlongALineThatBreaksARule(t *testing.T) {
	src := strings.Replace(orderForm, "{ from: orders, to: plan, kind: data }", "{ from: orders, to: plan, kind: data }\n  - { from: read, to: box, kind: start, label: \"printed by the live system\" }", 1)
	doc := parsed(t, src)
	got := Summarise(doc).Untrusted[0]
	if want := []string{"Pin the plan to the board: Allow", "Print the label: Allow"}; !reflect.DeepEqual(got.OutsideActions, want) {
		t.Errorf("outside actions = %q, want %q", got.OutsideActions, want)
	}
	if !reflect.DeepEqual(warningsOf(doc, "warn.untrusted-outside"), []string{
		`untrusted text from part "form" reaches outside action "board": a review, a person's own step or a code step marked cleans in between would stop it`,
		`untrusted text from part "form" reaches outside action "box": a review, a person's own step or a code step marked cleans in between would stop it`,
	}) {
		t.Errorf("Warnings gave %q", Warnings(doc))
	}
}

func TestADoStepsWarningsCountALineThatBreaksARule(t *testing.T) {
	doc := parsed(t, `awml: "0.8"
title: T
parts:
  - { id: act, kind: code, op: do, label: Act }
  - { id: chat, kind: surface, label: Chat }
  - { id: out, kind: outside, label: "Delete it: Block" }
lines:
  - { from: act, to: chat, kind: data }
  - { from: act, to: out, kind: start, label: "done by the live system" }
`)
	if got := warningsOf(doc, "warn.one-change"); !reflect.DeepEqual(got, []string{`do step "act" leads to 2 changes ("chat", "out"): each do step should make one`}) {
		t.Errorf("Warnings gave %q", got)
	}
}

func TestSummaryCountsEachLineThatBreaksARuleAsARedLine(t *testing.T) {
	if got := Summarise(parsed(t, fridaySent)).RedLines; got != 1 {
		t.Errorf("red lines = %d, want 1", got)
	}
}

// The bakery's specials, drafted in the kitchen and pinned straight up in the shop, so the one red
// line also publishes. Sam is fictional.
const specialsPinned = `awml: "0.8"
title: The bakery's specials, pinned up
scopes:
  - { id: kitchen, label: "kitchen: Sam" }
  - { id: shop, label: "shop: Sam, the staff" }
parts:
  - { id: monday, kind: trigger, label: "Monday 07:00" }
  - { id: write, kind: ai, op: draft, label: "Draft the week's specials", in: kitchen }
  - { id: pin, kind: outside, label: "Pin them on the shop board: Ask", in: shop }
lines:
  - { from: monday, to: write, kind: start }
  - { from: write, to: pin, kind: data, label: the specials }
implementations:
  bakery:
    label: The bakery
    blocks:
      - { id: tablet, label: Shop tablet, parts: [write, pin] }
`

func TestARedLineThatPublishesIsDrawnRedAndThick(t *testing.T) {
	doc := parsed(t, specialsPinned)
	mistakes(t, doc,
		`part "write" leads to outside action "pin", but its op is draft: only a do step leads to an outside action`,
		`outside action "pin" is Ask, but part "write" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`)
	for _, implementation := range []string{"", "bakery"} {
		view, err := View(doc, implementation)
		if err != nil {
			t.Fatal(err)
		}
		out := Draw(view)
		has(t, out, `
  write ==>|"the specials · adds the staff<br>part #34;write#34; leads to outside action #34;pin#34;#44; but its op is draft<br>outside action #34;pin#34; is Ask#44; but part #34;write#34; leads to it with no person in between"| pin
`)
		has(t, out, "\n  linkStyle 1 "+red)
	}
}

func TestSummaryCountsARedLineThatPublishesAsARedLine(t *testing.T) {
	got := Summarise(parsed(t, specialsPinned))
	if got.RedLines != 1 || got.PublishingLines.Count != 0 {
		t.Errorf("red lines = %d and publishing lines = %d, want 1 and 0", got.RedLines, got.PublishingLines.Count)
	}
}
