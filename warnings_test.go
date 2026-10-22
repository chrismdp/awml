package awml

import (
	"reflect"
	"strings"
	"testing"
)

func warnings(t *testing.T, doc any, want ...string) {
	t.Helper()
	got := Warnings(doc)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Warnings gave\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

const changes = `
awml: "0.3"
title: Changes
view: security
parts:
  - {id: fill, kind: code, op: do, label: Fill}
  - {id: basket, kind: outside, label: "Fill the basket: Allow"}
  - {id: split, kind: fork}
  - {id: approve, kind: review, label: Approve}
  - {id: checkout, kind: outside, label: "Check out: Ask"}
  - {id: filed, kind: context, label: Filed}
  - {id: seen, kind: context, label: Seen}
  - {id: list, kind: surface, label: List}
lines:
`

func withChangeLines(t *testing.T, lines ...string) *Map {
	t.Helper()
	src := changes
	for _, line := range lines {
		src += "  - " + line + "\n"
	}
	return parsed(t, src)
}

func TestAnAIDoStepWarns(t *testing.T) {
	doc := withChangeLines(t)
	parts := doc.get("parts").([]any)
	parts[0].(*Map).set("kind", "ai")
	warnings(t, doc, `step "fill" is a do step decided by AI: a do step should be code acting on what an earlier step decided`)
	if got := Check(doc); len(got) != 0 {
		t.Fatalf("Check returned warnings as mistakes: %v", got)
	}
}

func TestADoStepThatLeadsToMoreThanOneChangeWarns(t *testing.T) {
	warnings(t, withChangeLines(t,
		"{from: fill, to: basket, kind: data}",
		"{from: fill, to: split, kind: start}",
		"{from: split, to: approve, kind: publish}",
		"{from: approve, to: checkout, kind: data}"),
		`do step "fill" leads to 2 changes ("basket", "checkout"): each do step should make one`)
}

func TestAllContextsTogetherAreOneChange(t *testing.T) {
	warnings(t, withChangeLines(t,
		"{from: fill, to: filed, kind: data}",
		"{from: fill, to: seen, kind: data}"))
	warnings(t, withChangeLines(t,
		"{from: fill, to: filed, kind: data}",
		"{from: fill, to: seen, kind: data}",
		"{from: fill, to: list, kind: data}"),
		`do step "fill" leads to 2 changes ("filed", "seen", "list"): each do step should make one`)
}

func TestEachSurfaceIsOneChange(t *testing.T) {
	warnings(t, withChangeLines(t,
		"{from: fill, to: list, kind: data}",
		"{from: fill, to: basket, kind: data}"),
		`do step "fill" leads to 2 changes ("basket", "list"): each do step should make one`)
}

func TestLinesThatDoNotLeadAndRedLinesDoNotCauseWarnings(t *testing.T) {
	warnings(t, withChangeLines(t,
		"{from: fill, to: basket, kind: data}",
		"{from: fill, to: checkout, kind: data, breaks: true}",
		"{from: fill, to: list, kind: hidden}",
		"{from: fill, to: filed, kind: refused}"))
}

func TestWarningsAcceptsAnythingCheckAccepts(t *testing.T) {
	warnings(t, nil)
	var file *Map
	warnings(t, file)
	warnings(t, []any{"not", "a", "file"})
	warnings(t, parsed(t, `awml: "0.3"
platform: {id: worker, label: Worker}
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: []
`))
}
