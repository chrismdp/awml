package awml

import (
	"strings"
	"testing"
)

// A bakery counts its stock each evening and orders each morning, each from its own trigger.
const twoTriggers = `
awml: "0.8"
title: The bakery
parts:
  - {id: evening, kind: trigger, label: "Every day 18:00"}
  - {id: morning, kind: trigger, label: "Every day 07:00"}
  - {id: count, kind: code, op: bring-in, label: Count the stock}
  - {id: stock, kind: context, label: Stock levels}
  - {id: order, kind: ai, op: draft, label: Draft the order}
lines:
  - {from: evening, to: count, kind: start}
  - {from: count, to: stock, kind: data}
  - {from: stock, to: order, kind: data}
`

func shaped(t *testing.T, src string, lines ...string) *Map {
	t.Helper()
	for _, line := range lines {
		src += "  - " + line + "\n"
	}
	return parsed(t, src)
}

const several = `this file holds 2 workflows: the one started by "Every day 18:00" and the one started by "Every day 07:00": split it at its triggers into workflows that refer to each other`

func TestTwoTriggersWhoseRunsShareOnlyAContextAreTwoWorkflows(t *testing.T) {
	warnings(t, shaped(t, twoTriggers, "{from: morning, to: order, kind: start}"), several)
}

func TestTwoTriggersWhoseRunsShareAStepAreOneWorkflow(t *testing.T) {
	warnings(t, shaped(t, twoTriggers,
		"{from: morning, to: count, kind: start}",
		"{from: count, to: order, kind: start}"))
}

func TestARunCarriesOnThroughAnyPartAStartLineReaches(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: order,", "  - {id: approve, kind: review, label: Ana's yes}\n  - {id: order,", 1)
	warnings(t, shaped(t, src,
		"{from: count, to: approve, kind: start}",
		"{from: approve, to: order, kind: start, label: \"[yes]\"}",
		"{from: morning, to: order, kind: start}"))
}

func TestAFileWithOneTriggerIsOneWorkflow(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: morning, kind: trigger, label: \"Every day 07:00\"}\n", "", 1)
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}"))
}

func TestEachGroupIsNamedByItsTriggers(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: count,", "  - {id: asked, kind: trigger, label: Sam asks for a count}\n  - {id: count,", 1)
	warnings(t, shaped(t, src,
		"{from: asked, to: count, kind: start}",
		"{from: morning, to: order, kind: start}"),
		`this file holds 2 workflows: the one started by "Every day 18:00" or "Sam asks for a count" and the one started by "Every day 07:00": split it at its triggers into workflows that refer to each other`)
}

func TestAPartNoLineLinksToATriggerWarns(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: morning, kind: trigger, label: \"Every day 07:00\"}\n", "", 1)
	src = strings.Replace(src, "parts:\n", "parts:\n  - {id: till, kind: connector, label: The till's sales}\n", 1)
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}"),
		`part "till" has no line linking it to a trigger: connect it or remove it`)
	// The till is linked once the count reads it, and the stock levels through the count.
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}", "{from: till, to: count, kind: data}"))
}

func TestACommentWithNoTieWarns(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: morning, kind: trigger, label: \"Every day 07:00\"}\n", "", 1)
	src = strings.Replace(src, "lines:\n", "  - {id: gap, kind: comment, label: \"Missing from the design: who reads the order\"}\nlines:\n", 1)
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}"),
		`comment "gap" is tied to no part: tie it to what it explains, or remove it`)
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}", "{from: gap, to: order, kind: tie}"))
}

func TestAStepThatOnlyReadsWhatAnotherWroteIsStartedByNoTrigger(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: morning, kind: trigger, label: \"Every day 07:00\"}\n", "", 1)
	warnings(t, parsed(t, src),
		`step "order" is started by no trigger: add the start line it's missing, or split its work into a workflow of its own that an outside action starts`)
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}"))
}

func TestAReviewReachedOnlyByDataIsStartedByNoTrigger(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: morning, kind: trigger, label: \"Every day 07:00\"}\n", "", 1)
	src = strings.Replace(src, "lines:\n", "  - {id: approve, kind: review, label: Ana's yes}\nlines:\n", 1)
	warnings(t, shaped(t, src,
		"{from: count, to: order, kind: start}",
		"{from: order, to: approve, kind: data}"),
		`review "approve" is started by no trigger: add the start line it's missing, or split its work into a workflow of its own that an outside action starts`)
}

func TestALooseStepIsUnlinkedAndStartedByNoTrigger(t *testing.T) {
	src := strings.Replace(twoTriggers, "  - {id: morning, kind: trigger, label: \"Every day 07:00\"}\n", "", 1)
	src = strings.Replace(src, "lines:\n", "  - {id: tidy, kind: code, label: Tidy the shelves}\nlines:\n", 1)
	warnings(t, shaped(t, src, "{from: count, to: order, kind: start}"),
		`part "tidy" has no line linking it to a trigger: connect it or remove it`,
		`step "tidy" is started by no trigger: add the start line it's missing, or split its work into a workflow of its own that an outside action starts`)
}

func TestTheShapeWarningsLeaveTheExitStatusAlone(t *testing.T) {
	doc := shaped(t, twoTriggers, "{from: morning, to: order, kind: start}")
	if got := Check(doc); len(got) != 0 {
		t.Fatalf("Check returned warnings as mistakes: %v", got)
	}
}
