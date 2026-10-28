package awml

import (
	"reflect"
	"strings"
	"testing"
)

// A workflow in which a standing approval stands between a code do step and the action it covers,
// and lists each action on the person's own surface.
const standing = `awml: "0.8"
title: Prices
scopes:
  - { id: owner, label: "owner: Ana" }
parts:
  - { id: night, kind: trigger, label: Every night }
  - { id: merge, kind: code, op: do, label: Merge the day's price changes, in: owner }
  - { id: ana_ok, kind: approval, label: "Ana's standing approval: price changes, up to twenty a day", in: owner }
  - { id: prices, kind: outside, label: "Update the prices: Ask", in: owner }
  - { id: list, kind: surface, label: Ana's list, in: owner }
lines:
  - { from: night, to: merge, kind: start }
  - { from: merge, to: ana_ok, kind: start }
  - { from: ana_ok, to: prices, kind: data, label: "[within limits]" }
  - { from: ana_ok, to: list, kind: data, label: "each change, with how to undo it" }
`

func TestAStandingApprovalIsAPartOfAWorkflow(t *testing.T) {
	doc := parsed(t, standing)
	mistakes(t, doc)
	if got := Warnings(doc); len(got) != 0 {
		t.Errorf("warnings: %v", got)
	}
}

func TestAStandingApprovalIsDrawnAsAPersonsYes(t *testing.T) {
	view, err := View(parsed(t, standing), "")
	if err != nil {
		t.Fatal(err)
	}
	drawn := Draw(view)
	want := `ana_ok("<small style='color:rgb(61,67,79)'>👤 📜 standing approval</small><br>Ana#39;s standing approval#58; price changes#44; up to twenty a day")`
	if !strings.Contains(drawn, want) {
		t.Errorf("drawing lacks %s:\n%s", want, drawn)
	}
	if !strings.Contains(drawn, "class ana_ok person") {
		t.Errorf("the standing approval isn't amber:\n%s", drawn)
	}
}

// tiered is a file with the parts every tier test needs, and the lines given.
func tiered(t *testing.T, outside string, lines ...string) *Map {
	t.Helper()
	src := `awml: "0.8"
title: Tiers
parts:
  - { id: go, kind: trigger, label: Go }
  - { id: act, kind: code, op: do, label: Act }
  - { id: me, kind: person, op: do, label: Do it myself }
  - { id: draft, kind: ai, op: draft, label: Draft }
  - { id: check, kind: review, label: Check it }
  - { id: ok, kind: approval, label: "Standing approval: up to three a day" }
  - { id: list, kind: surface, label: List }
  - { id: out, kind: outside, label: "` + outside + `" }
lines:
  - { from: ok, to: list, kind: data }
`
	for _, line := range lines {
		src += "  - " + line + "\n"
	}
	return parsed(t, src)
}

func TestAnAskActionNeedsAPersonInFrontOfIt(t *testing.T) {
	for _, way := range [][]string{
		{"{ from: act, to: check, kind: data }", "{ from: check, to: out, kind: data, label: \"[yes]\" }"},
		{"{ from: me, to: out, kind: data }"},
		{"{ from: act, to: ok, kind: data }", "{ from: ok, to: out, kind: data }"},
	} {
		mistakes(t, tiered(t, "Send it: Ask", way...))
	}
	mistakes(t, tiered(t, "Send it: Ask", "{ from: act, to: out, kind: data }"),
		`outside action "out" is Ask, but part "act" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`)
	mistakes(t, tiered(t, "Send it: Ask", "{ from: act, to: out, kind: start }",
		"{ from: me, to: out, kind: data }"),
		`outside action "out" is Ask, but part "act" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`)
}

func TestAnAllowActionHasNoYesInFrontOfIt(t *testing.T) {
	for _, way := range [][]string{
		{"{ from: act, to: out, kind: data }"},
		{"{ from: me, to: out, kind: data }"},
	} {
		mistakes(t, tiered(t, "File it: Allow under £20", way...))
	}
	mistakes(t, tiered(t, "File it: Allow", "{ from: act, to: check, kind: data }", "{ from: check, to: out, kind: data }"),
		`outside action "out" is Allow, but review "check" stands in front of it: an Allow action just happens, so make it Ask or take the yes away`)
	mistakes(t, tiered(t, "File it: Allow", "{ from: act, to: ok, kind: data }", "{ from: ok, to: out, kind: data }"),
		`outside action "out" is Allow, but standing approval "ok" stands in front of it: an Allow action just happens, so make it Ask or take the yes away`)
}

func TestABlockedActionHasOnlyRedLinesIntoIt(t *testing.T) {
	mistakes(t, tiered(t, "Delete it: Block"))
	mistakes(t, tiered(t, "Delete it: Block", "{ from: act, to: out, kind: data }"),
		`outside action "out" is Block, but part "act" leads to it: a blocked action is never taken, so a line into it is red`)
}

func TestAnOutsideActionsLabelEndsWithItsTier(t *testing.T) {
	mistakes(t, tiered(t, "Send it", "{ from: me, to: out, kind: data }"),
		`outside action "out" has no tier: end its label with Allow, Ask or Block after a colon, as in "Send the mail: Ask"`)
	mistakes(t, tiered(t, "Send it: soon", "{ from: me, to: out, kind: data }"),
		`outside action "out" has no tier: end its label with Allow, Ask or Block after a colon, as in "Send the mail: Ask"`)
}

func TestAStandingApprovalListsItsActionsOnASurface(t *testing.T) {
	doc := parsed(t, strings.Replace(standing, "  - { from: ana_ok, to: list, kind: data, label: \"each change, with how to undo it\" }\n", "", 1))
	mistakes(t, doc, `standing approval "ana_ok" leads to no surface: draw a line from it to the surface where each action it covers is listed`)
}

// A standing approval may cover actions that can't be undone, each listed as sent, so neither the
// check's message nor what it says of itself asks for how to undo each one.
func TestTheStandingApprovalCheckDoesntAskHowToUndoEachAction(t *testing.T) {
	report := CheckSources("dev", []Source{source("prices.yaml", standing)})
	if got := checkResult(t, report, "approvals.listed"); strings.Contains(got.About, "undo") {
		t.Errorf("approvals.listed says %q", got.About)
	}
}

func TestOnlyADoStepLeadsToAnOutsideActionThroughAStandingApproval(t *testing.T) {
	mistakes(t, tiered(t, "Send it: Ask", "{ from: draft, to: ok, kind: data }", "{ from: ok, to: out, kind: data }"),
		`part "draft" leads to outside action "out", but its op is draft: only a do step leads to an outside action`)
}

// The listing surface is where the standing approval reports, not a change the do step makes.
func TestAStandingApprovalsSurfaceIsntAChangeOfTheDoStep(t *testing.T) {
	doc := tiered(t, "Send it: Ask", "{ from: act, to: ok, kind: data }", "{ from: ok, to: out, kind: data }")
	if got := warningsOf(doc, "warn.one-change"); len(got) != 0 {
		t.Errorf("warnings: %v", got)
	}
}

func TestTheSummaryNamesAStandingApprovalAsAPlaceAPersonDecides(t *testing.T) {
	got := Summarise(parsed(t, standing)).People
	want := []PersonSummary{{Label: "Ana's standing approval: price changes, up to twenty a day", Kind: "approval"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("people = %+v, want %+v", got, want)
	}
}

func TestTheTierChecksPassOrFail(t *testing.T) {
	report := CheckSources("dev", []Source{source("prices.yaml", standing)})
	for _, id := range []string{"tiers.written", "tiers.shape", "approvals.listed"} {
		if got := checkResult(t, report, id); got.Result != "pass" {
			t.Errorf("%s gave %s with %+v", id, got.Result, got.Mistakes)
		}
	}
	src := strings.Replace(standing, "Update the prices: Ask", "Update the prices: Allow", 1)
	report = CheckSources("dev", []Source{source("prices.yaml", src)})
	if got := checkResult(t, report, "tiers.shape"); got.Result != "fail" || len(got.Mistakes) != 1 {
		t.Errorf("tiers.shape gave %s with %+v", got.Result, got.Mistakes)
	}
}

func TestAWorkflowIsWrittenInTheCanonicalOrder(t *testing.T) {
	doc := parsed(t, "title: Prices\nparts: []\nawml: \"0.8\"\n")
	got, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "awml: \"0.8\"\ntitle: Prices\nparts: []\n"; string(got) != want {
		t.Errorf("Marshal gave\n%s\nwant\n%s", got, want)
	}
}
