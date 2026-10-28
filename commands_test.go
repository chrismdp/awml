package awml

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A workflow whose line into a Block action breaks a rule: check fails, and draw draws it red.
const redLineWorkflow = `awml: "0.8"
title: The bakery's old files
scopes:
  - { id: bakery, label: "bakery: Ana, Ben", processing: any }
parts:
  - { id: night, kind: trigger, label: "Every night" }
  - { id: tidy, kind: code, op: do, label: "Tidy the old files", in: bakery }
  - { id: wipe, kind: outside, label: "Delete a file: Block", in: bakery }
lines:
  - { from: night, to: tidy, kind: start }
  - { from: tidy, to: wipe, kind: start, label: "one night in June" }
`

// refused gives the words of a command's refusal, failing when it gave something else.
func refused(t *testing.T, err error) string {
	t.Helper()
	var refusal Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("gave %v, want a refusal", err)
	}
	return err.Error()
}

// mistaken gives the mistakes a command said, failing when it gave something else.
func mistaken(t *testing.T, err error) Mistakes {
	t.Helper()
	var mistakes Mistakes
	if !errors.As(err, &mistakes) {
		t.Fatalf("gave %v, want mistakes", err)
	}
	return mistakes
}

func TestReadingSaysEachFilesOwnMistakesThenThoseOfItsRefsAfterItsName(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: thu,", "  - { id: odd, kind: bogus, label: Odd }\n  - { id: thu,", 1)
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	_, err := ReadSources([]Source{source("plan.yaml", plan), source("shop.yaml", shop)}, false)
	got := mistaken(t, err)
	want := Mistakes{
		{"plan.yaml", Check(parsed(t, plan))[0]},
		{"shop.yaml", `part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mistakes are %q\nwant %q", got, want)
	}
	if text := err.Error(); text != "plan.yaml: "+want[0].Message+"\nshop.yaml: "+want[1].Message {
		t.Errorf("the error says %q", text)
	}
}

func TestReadingGoesOnPastAFileThatCantBeRead(t *testing.T) {
	_, err := ReadSources([]Source{
		{File: "missing.yaml", Unreadable: "can't be read: No such file or directory"},
		source("broken.yaml", "parts: [\n"),
		source("list.yaml", "- a list\n"),
		source("dinners.yaml", soundWorkflow),
	}, true)
	got := mistaken(t, err)
	if len(got) != 3 || got[0] != (FileMistake{"missing.yaml", "can't be read: No such file or directory"}) ||
		got[1].File != "broken.yaml" || !strings.HasPrefix(got[1].Message, "isn't YAML: ") ||
		got[2] != (FileMistake{"list.yaml", "the file must be a mapping of fields, starting with awml"}) {
		t.Errorf("mistakes are %q", got)
	}
}

func TestAFileGivenWithoutANameHasItsMistakesSaidAlone(t *testing.T) {
	_, err := ReadSources([]Source{{Text: []byte("- a list\n")}}, false)
	if err == nil || err.Error() != "the file must be a mapping of fields, starting with awml" {
		t.Errorf("gave %v", err)
	}
}

func TestReadingForADrawingLetsALineThatBreaksARuleThrough(t *testing.T) {
	if _, err := ReadSources([]Source{source("red.yaml", redLineWorkflow)}, false); err == nil {
		t.Error("a line that breaks a rule passes the check")
	}
	workflows, err := ReadSources([]Source{source("red.yaml", redLineWorkflow)}, true)
	if err != nil || len(workflows) != 1 || workflows[0].File != "red.yaml" || str(workflows[0].Doc, "title") != "The bakery's old files" {
		t.Errorf("gave %+v and %v", workflows, err)
	}
}

func TestAFileGivenTwiceIsReadOnceWhereItsFirstGiven(t *testing.T) {
	workflows, err := ReadSources([]Source{
		source("plan.yaml", planWorkflow),
		{File: "here/shop.yaml", Path: "/home/shop.yaml", Text: []byte(shopWorkflow)},
		source("./plan.yaml", planWorkflow),
		{File: "shop.yaml", Path: "/home/./shop.yaml", Text: []byte(shopWorkflow)},
		{Text: []byte(soundWorkflow)},
		{Text: []byte(soundWorkflow)},
	}, false)
	var files []string
	for _, w := range workflows {
		files = append(files, w.File)
	}
	if want := []string{"plan.yaml", "here/shop.yaml", "", ""}; err != nil || !reflect.DeepEqual(files, want) {
		t.Errorf("read %q with %v, want %q", files, err, want)
	}
	report := CheckSources("dev", []Source{source("dinners.yaml", soundWorkflow), source("/dinners.yaml", soundWorkflow)})
	if !reflect.DeepEqual(report.Files, []CheckedFile{{File: "dinners.yaml", View: "workflow"}}) {
		t.Errorf("checked %+v", report.Files)
	}
}

func TestTheCheckSaysEachFilesWarningsThenItsMistakes(t *testing.T) {
	warned := strings.Replace(soundWorkflow, "kind: ai, op: draft", "kind: ai, op: do", 1)
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	report := CheckSources("dev", []Source{source("dinners.yaml", warned), source("plan.yaml", planWorkflow), source("shop.yaml", shop)})
	for i, want := range [][2][]string{
		{Warnings(parsed(t, warned)), nil},
		{nil, nil},
		{nil, {`part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"`}},
	} {
		warnings, mistakes := report.Said(i)
		if !reflect.DeepEqual(warnings, want[0]) || !reflect.DeepEqual(mistakes, want[1]) {
			t.Errorf("file %d: warnings %q and mistakes %q, want %q and %q", i, warnings, mistakes, want[0], want[1])
		}
	}
}

func TestTheCheckSaysAFilesOwnMistakesBeforeThoseOfItsRefs(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	shop = strings.Replace(shop, "kind: code, op: do, label: Fill", "kind: code, op: bogus, label: Fill", 1)
	report := CheckSources("dev", []Source{source("plan.yaml", planWorkflow), source("shop.yaml", shop)})
	_, mistakes := report.Said(1)
	want := append(Check(parsed(t, shop)), `part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"`)
	if !reflect.DeepEqual(mistakes, want) {
		t.Errorf("mistakes are %q\nwant %q", mistakes, want)
	}
}

func TestDrawingOneFileDrawsItsView(t *testing.T) {
	for _, implementation := range []string{"", "home"} {
		view, err := View(parsed(t, soundWorkflow), implementation)
		if err != nil {
			t.Fatal(err)
		}
		drawn, err := DrawSources([]Source{source("dinners.yaml", soundWorkflow)}, implementation)
		if err != nil || drawn != Draw(view) {
			t.Errorf("%q: drew %q with %v", implementation, drawn, err)
		}
		graph, err := GraphSources([]Source{source("dinners.yaml", soundWorkflow)}, implementation)
		if err != nil || !reflect.DeepEqual(graph, GraphOf(view)) {
			t.Errorf("%q: graph %+v with %v", implementation, graph, err)
		}
	}
}

func TestDrawingOneFileSaysWhyItHasNoView(t *testing.T) {
	for _, tc := range []struct{ name, text, implementation, want string }{
		{"a platform description", soundDescription, "", "a platform description has no drawing"},
		{"an unknown implementation", soundWorkflow, "office", ""},
	} {
		_, want := View(parsed(t, tc.text), tc.implementation)
		if _, err := DrawSources([]Source{source("x.yaml", tc.text)}, tc.implementation); err == nil || err.Error() != want.Error() {
			t.Errorf("%s: draw gave %v, want %v", tc.name, err, want)
		}
		if _, err := GraphSources([]Source{source("x.yaml", tc.text)}, tc.implementation); err == nil || err.Error() != want.Error() {
			t.Errorf("%s: graph gave %v, want %v", tc.name, err, want)
		}
	}
}

func TestDrawingSeveralFilesDrawsTheirMap(t *testing.T) {
	sources := []Source{source("plan.yaml", planWorkflow), source("shop.yaml", shopWorkflow)}
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	if drawn, err := DrawSources(sources, ""); err != nil || drawn != DrawMap(workflows) {
		t.Errorf("drew %q with %v", drawn, err)
	}
	if graph, err := GraphSources(sources, ""); err != nil || !reflect.DeepEqual(graph, GraphMap(workflows)) {
		t.Errorf("graph %+v with %v", graph, err)
	}
}

func TestTheMapRefusesAFileThatCantBeOnIt(t *testing.T) {
	sources := []Source{source("plan.yaml", planWorkflow), source("laptop.yaml", laptop)}
	want := "laptop.yaml: a platform description has no drawing"
	if _, err := DrawSources(sources, ""); err == nil || err.Error() != want {
		t.Errorf("draw gave %v, want %q", err, want)
	}
	if _, err := GraphSources(sources, ""); err == nil || err.Error() != want {
		t.Errorf("graph gave %v, want %q", err, want)
	}
}

func TestDrawingAnImplementationTakesOneWorkflow(t *testing.T) {
	sources := []Source{source("plan.yaml", planWorkflow), {File: "missing.yaml", Unreadable: "can't be read: No such file or directory"}}
	if _, err := DrawSources(sources, "home"); refused(t, err) != "argument --implementation: draws one workflow, not several" {
		t.Errorf("draw refused with %q", err)
	}
	if _, err := GraphSources(sources, "home"); refused(t, err) != "argument --implementation: takes one workflow, not several" {
		t.Errorf("graph refused with %q", err)
	}
	// A file given twice is one workflow.
	if _, err := DrawSources([]Source{source("dinners.yaml", soundWorkflow), source("./dinners.yaml", soundWorkflow)}, "home"); err != nil {
		t.Errorf("draw of one file given twice gave %v", err)
	}
}

func TestDrawingSaysTheMistakesOfEveryFile(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	sources := []Source{source("plan.yaml", planWorkflow), source("shop.yaml", shop)}
	_, want := ReadSources(sources, true)
	if _, err := DrawSources(sources, ""); err == nil || err.Error() != want.Error() {
		t.Errorf("draw gave %v, want %v", err, want)
	}
	if _, err := DrawSources([]Source{source("red.yaml", redLineWorkflow)}, ""); err != nil {
		t.Errorf("draw of a line that breaks a rule gave %v", err)
	}
	if _, err := DrawSources(nil, ""); refused(t, err) != "no workflow was given" {
		t.Errorf("draw of nothing gave %v", err)
	}
}

func TestFitTellsDescriptionsFromWorkflows(t *testing.T) {
	report, err := FitSources([]Source{source("laptop.yaml", laptop), source("plan.yaml", planWorkflow), source("shop.yaml", shopWorkflow)}, "")
	want := JudgeTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow), fitDocs(t, laptop))
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Errorf("fit gave %+v with %v\nwant %+v", report, err, want)
	}
}

func TestFitJudgesOneImplementation(t *testing.T) {
	doc := parsed(t, soundWorkflow)
	placement, err := View(doc, "home")
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptions := range [][]string{{}, {soundDescription}} {
		sources := []Source{source("dinners.yaml", soundWorkflow)}
		for _, d := range descriptions {
			sources = append(sources, source("scripts.yaml", d))
		}
		report, err := FitSources(sources, "home")
		if want := Judge(doc, placement, fitDocs(t, descriptions...)); err != nil || !reflect.DeepEqual(report, want) {
			t.Errorf("fit with %d descriptions gave %+v with %v\nwant %+v", len(descriptions), report, err, want)
		}
	}
	_, want := View(doc, "office")
	if _, err := FitSources([]Source{source("dinners.yaml", soundWorkflow), source("laptop.yaml", laptop)}, "office"); err == nil || err.Error() != want.Error() {
		t.Errorf("fit of an unknown implementation gave %v, want %v", err, want)
	}
}

func TestFitRefusesWhatItCantJudge(t *testing.T) {
	for _, tc := range []struct {
		name           string
		sources        []Source
		implementation string
		want           string
	}{
		{"no workflow", []Source{source("laptop.yaml", laptop)}, "", "no workflow was given"},
		{"no descriptions", []Source{source("plan.yaml", planWorkflow)}, "", "no platform descriptions were given"},
		{"an implementation of several", []Source{source("plan.yaml", planWorkflow), source("shop.yaml", shopWorkflow), source("laptop.yaml", laptop)}, "home",
			"argument --implementation: judges one workflow, not several"},
	} {
		if _, err := FitSources(tc.sources, tc.implementation); refused(t, err) != tc.want {
			t.Errorf("%s: refused with %q, want %q", tc.name, err, tc.want)
		}
	}
	_, want := ReadSources([]Source{source("red.yaml", redLineWorkflow)}, false)
	if _, err := FitSources([]Source{source("red.yaml", redLineWorkflow), source("laptop.yaml", laptop)}, ""); err == nil || err.Error() != want.Error() {
		t.Errorf("fit of a line that breaks a rule gave %v, want %v", err, want)
	}
}

func TestTheSummaryCountsTheWorkflowsTogether(t *testing.T) {
	summary, err := SummariseSources([]Source{source("plan.yaml", planWorkflow), source("shop.yaml", shopWorkflow)})
	want := SummariseTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow))
	if err != nil || !reflect.DeepEqual(summary, want) {
		t.Errorf("summary is %+v with %v\nwant %+v", summary, err, want)
	}
	// Workflows given together list the parts they share, even when they share none, and one given
	// twice is one workflow, which lists none.
	if summary, err := SummariseSources([]Source{source("dinners.yaml", soundWorkflow), source("red.yaml", redLineWorkflow)}); err != nil || summary.Shared == nil || len(summary.Shared) != 0 {
		t.Errorf("summary of two workflows that share nothing is %+v with %v", summary, err)
	}
	if summary, err := SummariseSources([]Source{source("plan.yaml", planWorkflow), source("./plan.yaml", planWorkflow)}); err != nil || summary.Shared != nil {
		t.Errorf("summary of one workflow given twice is %+v with %v", summary, err)
	}
	if summary, err := SummariseSources([]Source{source("red.yaml", redLineWorkflow)}); err != nil || summary.RedLines != 1 {
		t.Errorf("summary of a line that breaks a rule is %+v with %v", summary, err)
	}
	if _, err := SummariseSources([]Source{source("list.yaml", "- a list\n")}); err == nil || err.Error() != "list.yaml: the file must be a mapping of fields, starting with awml" {
		t.Errorf("summary of a list gave %v", err)
	}
}
