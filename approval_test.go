package awml

import (
	"reflect"
	"strings"
	"testing"
)

// A 0.5 workflow in which a standing approval stands between a code do step and the action it
// covers, and lists each action on the person's own surface.
const standing = `awml: "0.5"
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
  - { from: merge, to: ana_ok, kind: data }
  - { from: ana_ok, to: prices, kind: data, label: "[within limits]" }
  - { from: ana_ok, to: list, kind: data, label: "each change, with how to undo it" }
`

func TestAStandingApprovalIsAPartOfA05Workflow(t *testing.T) {
	doc := parsed(t, standing)
	if !IsCanonical(doc) {
		t.Fatal("a 0.5 workflow isn't canonical")
	}
	mistakes(t, doc)
	if got := Warnings(doc); len(got) != 0 {
		t.Errorf("warnings: %v", got)
	}
}

func TestAStandingApprovalIsDrawnAsAPersonsYes(t *testing.T) {
	drawn := Draw(parsed(t, standing))
	want := `ana_ok("<small style='color:rgb(61,67,79)'>👤 📜 standing approval</small><br>Ana#39;s standing approval#58; price changes#44; up to twenty a day")`
	if !strings.Contains(drawn, want) {
		t.Errorf("drawing lacks %s:\n%s", want, drawn)
	}
	if !strings.Contains(drawn, "class ana_ok person") {
		t.Errorf("the standing approval isn't amber:\n%s", drawn)
	}
}

// Only a 0.5 file may hold a standing approval.
func TestAStandingApprovalIsUnknownBefore05(t *testing.T) {
	doc := parsed(t, strings.Replace(standing, `awml: "0.5"`, `awml: "0.4"`, 1))
	if !IsCanonical(doc) {
		t.Fatal("a 0.4 workflow isn't canonical")
	}
	mistakes(t, doc, `part "ana_ok" has unknown kind "approval"`)
	legacy := parsed(t, `awml: "0.3"
title: Prices
view: security
parts:
  - { id: ana_ok, kind: approval, label: Ana's standing approval }
`)
	mistakes(t, legacy, `part "ana_ok" has unknown kind "approval"`)
}

// tiered is a 0.5 file with the parts every tier test needs, and the lines given.
func tiered(t *testing.T, outside string, lines ...string) *Map {
	t.Helper()
	src := `awml: "0.5"
title: Tiers
parts:
  - { id: go, kind: trigger, label: Go }
  - { id: act, kind: code, op: do, label: Act }
  - { id: me, kind: person, op: do, label: Do it myself }
  - { id: draft, kind: ai, op: draft, label: Draft }
  - { id: check, kind: review, label: Check it }
  - { id: ok, kind: approval, label: "Standing approval: up to three a day" }
  - { id: split, kind: fork }
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
		{"{ from: act, to: check, kind: data }", "{ from: check, to: split, kind: data }", "{ from: split, to: out, kind: data }"},
		{"{ from: act, to: out, kind: data, breaks: true }"},
	} {
		mistakes(t, tiered(t, "Send it: Ask", way...))
	}
	mistakes(t, tiered(t, "Send it: Ask", "{ from: act, to: out, kind: data }"),
		`outside action "out" is Ask, but part "act" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`)
	mistakes(t, tiered(t, "Send it: Ask", "{ from: act, to: split, kind: start }", "{ from: split, to: out, kind: data }",
		"{ from: me, to: out, kind: data }"),
		`outside action "out" is Ask, but part "act" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`)
}

func TestAnAllowActionHasNoYesInFrontOfIt(t *testing.T) {
	for _, way := range [][]string{
		{"{ from: act, to: out, kind: data }"},
		{"{ from: me, to: out, kind: data }"},
		{"{ from: act, to: split, kind: data }", "{ from: split, to: out, kind: data }"},
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
	mistakes(t, tiered(t, "Delete it: Block", "{ from: act, to: out, kind: data, breaks: true }"))
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
	mistakes(t, doc, `standing approval "ana_ok" leads to no surface: draw a line from it to the surface where each action it covers is listed with how to undo it`)
}

func TestOnlyADoStepLeadsToAnOutsideActionThroughAStandingApproval(t *testing.T) {
	mistakes(t, tiered(t, "Send it: Ask", "{ from: draft, to: ok, kind: data }", "{ from: ok, to: out, kind: data }"),
		`part "draft" leads to outside action "out", but its op is draft: only a do step leads to an outside action`)
}

// A 0.4 file keeps its tiers as words in a label, as it did before.
func TestTiersArentCheckedBefore05(t *testing.T) {
	doc := tiered(t, "Send it: Ask", "{ from: act, to: out, kind: data }")
	doc.set("awml", "0.4")
	var parts []any
	for _, part := range items(doc, "parts") {
		if str(part, "kind") != "approval" {
			parts = append(parts, part)
		}
	}
	doc.set("parts", parts)
	var lines []any
	for _, line := range items(doc, "lines") {
		if str(line, "from") != "ok" {
			lines = append(lines, line)
		}
	}
	doc.set("lines", lines)
	mistakes(t, doc)
}

// The listing surface is where the standing approval reports, not a change the do step makes.
func TestAStandingApprovalsSurfaceIsntAChangeOfTheDoStep(t *testing.T) {
	doc := tiered(t, "Send it: Ask", "{ from: act, to: ok, kind: data }", "{ from: ok, to: out, kind: data }")
	if got := Warnings(doc); len(got) != 0 {
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

func TestTheTierChecksApplyOnlyTo05Workflows(t *testing.T) {
	report := CheckSources("dev", []Source{source("prices.yaml", standing)})
	for _, id := range []string{"tiers.written", "tiers.shape", "approvals.listed"} {
		if got := checkResult(t, report, id); got.Result != "pass" {
			t.Errorf("%s gave %s with %+v", id, got.Result, got.Mistakes)
		}
	}
	old := strings.Replace(soundWorkflow, `awml: "0.5"`, `awml: "0.4"`, 1)
	report = CheckSources("dev", []Source{source("dinners.yaml", old)})
	for _, id := range []string{"tiers.written", "tiers.shape", "approvals.listed"} {
		if got := checkResult(t, report, id); got.Result != "skipped" || got.SkippedBecause != "no file given is a 0.5 workflow" {
			t.Errorf("%s gave %s because %q", id, got.Result, got.SkippedBecause)
		}
	}
	src := strings.Replace(standing, "Update the prices: Ask", "Update the prices: Allow", 1)
	report = CheckSources("dev", []Source{source("prices.yaml", src)})
	if got := checkResult(t, report, "tiers.shape"); got.Result != "fail" || len(got.Mistakes) != 1 {
		t.Errorf("tiers.shape gave %s with %+v", got.Result, got.Mistakes)
	}
}

func TestAPlatformDescriptionMayBe05(t *testing.T) {
	mistakes(t, parsed(t, strings.Replace(soundDescription, `awml: "0.4"`, `awml: "0.5"`, 1)))
}

func TestA05WorkflowIsWrittenInTheCanonicalOrder(t *testing.T) {
	doc := parsed(t, "title: Prices\nparts: []\nawml: \"0.5\"\n")
	got, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "awml: \"0.5\"\ntitle: Prices\nparts: []\n"; string(got) != want {
		t.Errorf("Marshal gave\n%s\nwant\n%s", got, want)
	}
}
