package awml

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSummariseEveryExample(t *testing.T) {
	pubClient := PublishingSummary{Count: 2, Labels: []string{"standing rule: a client's email", "the consultant takes the reply"}}
	pubDinners := PublishingSummary{Count: 1, Labels: []string{"standing rule: dinners to carers"}}
	pubGrandma := PublishingSummary{Count: 1, Labels: []string{"standing rule: pickups to Grandma"}}
	pubChecklist := PublishingSummary{Count: 1, Labels: []string{"the checklist, with no names in it"}}
	cases := map[string]Summary{
		"examples/client-email-platform.yaml":     {Steps: StepCounts{AI: 3, Code: 2, Person: 1}, Operations: map[string]int{"bring-in": 1, "sort": 2, "learn": 1, "draft": 1, "do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Contexts: 3, Connectors: 1, Platforms: 4, PublishingLines: pubClient},
		"examples/client-email.yaml":              {Steps: StepCounts{AI: 3, Code: 2, Person: 1}, Operations: map[string]int{"bring-in": 1, "sort": 2, "learn": 1, "draft": 1, "do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Contexts: 3, Connectors: 1, Scopes: 2, PublishingLines: pubClient},
		"examples/dinners-platform.yaml":          {Steps: StepCounts{AI: 1, Code: 2}, Operations: map[string]int{"bring-in": 1, "do": 1}, Reviews: 1, Contexts: 3, Connectors: 2, Surfaces: 2, Platforms: 5},
		"examples/dinners-security.yaml":          {Steps: StepCounts{AI: 1, Code: 3}, Operations: map[string]int{"bring-in": 1, "do": 1, "check": 1}, OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 3, Reviews: 1, Contexts: 3, Connectors: 3, Surfaces: 3, Scopes: 3, PublishingLines: pubDinners},
		"examples/legend.yaml":                    {Steps: StepCounts{AI: 1, Code: 1, Person: 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Reviews: 1, Contexts: 1, Connectors: 1, Surfaces: 1},
		"examples/question.yaml":                  {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"answer": 2}, Triggers: 1, Contexts: 3, Surfaces: 1, Scopes: 3, RefusedReads: 1},
		"examples/tour/01-draft-and-send.yaml":    {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"draft": 1, "do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Reviews: 1, Scopes: 1},
		"examples/tour/02-fork.yaml":              {Steps: StepCounts{AI: 2}, Operations: map[string]int{"learn": 1, "check": 1}, Triggers: 1, Contexts: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/03-refused-read.yaml":      {Steps: StepCounts{AI: 1}, Operations: map[string]int{"answer": 1}, Triggers: 1, Contexts: 2, Surfaces: 1, Scopes: 2, RefusedReads: 1},
		"examples/tour/04-publishing.yaml":        {Steps: StepCounts{Code: 1}, Operations: map[string]int{"bring-in": 1}, Triggers: 1, Contexts: 1, Connectors: 1, Surfaces: 1, Scopes: 2, PublishingLines: pubGrandma},
		"examples/tour/05-side-by-side.yaml":      {Steps: StepCounts{AI: 2}, Operations: map[string]int{"draft": 2}, Contexts: 3, Scopes: 2, PublishingLines: pubChecklist},
		"examples/tour/06-red-line.yaml":          {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"draft": 1, "do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Reviews: 1, Scopes: 1, RedLines: 1},
		"examples/tour/07-person-step.yaml":       {Steps: StepCounts{Code: 1, Person: 1}, Operations: map[string]int{"check": 1, "do": 1}, Triggers: 1, Contexts: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/08-gap.yaml":               {Steps: StepCounts{Code: 1}, Operations: map[string]int{"bring-in": 1}, Triggers: 1, Contexts: 1, Connectors: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/09-platform-view.yaml":     {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"bring-in": 1, "sort": 1}, Triggers: 1, Contexts: 2, Platforms: 2},
		"examples/tour/10-operations.yaml":        {Steps: StepCounts{AI: 3, Code: 1}, Operations: map[string]int{"sort": 1, "learn": 1, "draft": 1, "do": 1}, OutsideActions: TierCounts{Allow: 1}, Triggers: 1, Contexts: 2, Scopes: 1},
		"examples/tour/11-do.yaml":                {Steps: StepCounts{Code: 1}, Operations: map[string]int{"do": 1}, OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 1, Reviews: 1, Scopes: 1},
		"examples/tour/12-standing-approval.yaml": {Steps: StepCounts{Code: 1}, Operations: map[string]int{"do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Connectors: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/13-check-by-code.yaml":     {Steps: StepCounts{Code: 1}, Operations: map[string]int{"check": 1}, Triggers: 1, Contexts: 1, Connectors: 1, Surfaces: 1, Scopes: 1},
	}

	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			got := Summarise(parsed.(*Map))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Summarise() = %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestSummariseDeduplicatesTwoViewsOfOneWorkflow(t *testing.T) {
	security := readExample(t, "examples/dinners-security.yaml")
	platform := readExample(t, "examples/dinners-platform.yaml")
	want := Summary{
		Steps: StepCounts{AI: 1, Code: 3}, Operations: map[string]int{"bring-in": 1, "do": 1, "check": 1},
		OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 3, Reviews: 1, Contexts: 3,
		Connectors: 3, Surfaces: 3, Scopes: 3, Platforms: 5,
		PublishingLines: PublishingSummary{Count: 1, Labels: []string{"standing rule: dinners to carers"}},
	}
	if got := Summarise(security, platform); !reflect.DeepEqual(got, want) {
		t.Errorf("Summarise(two views) = %#v\nwant %#v", got, want)
	}
}

func TestTotalSummariesAddsInventoryCounts(t *testing.T) {
	a := Summarise(readExample(t, "examples/tour/03-refused-read.yaml"))
	b := Summarise(readExample(t, "examples/tour/06-red-line.yaml"))
	got := TotalSummaries(a, b)
	want := Summary{
		Steps: StepCounts{AI: 2, Code: 1}, Operations: map[string]int{"answer": 1, "draft": 1, "do": 1},
		OutsideActions: TierCounts{Ask: 1}, Triggers: 2, Reviews: 1, Contexts: 2,
		Surfaces: 1, Scopes: 3, RefusedReads: 1, RedLines: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TotalSummaries() = %#v\nwant %#v", got, want)
	}
}

func TestSummaryHasStableJSONFieldNames(t *testing.T) {
	got, err := json.Marshal(Summarise(readExample(t, "examples/tour/11-do.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"steps":{"ai":0,"code":1,"person":0},"operations":{"do":1},"outsideActions":{"allow":1,"ask":1,"block":0},"triggers":1,"reviews":1,"contexts":0,"connectors":0,"surfaces":0,"scopes":1,"platforms":0,"publishingLines":{"count":0,"labels":null},"refusedReads":0,"redLines":0}`
	if string(got) != want {
		t.Errorf("JSON = %s\nwant %s", got, want)
	}
}

func TestAnOutsideActionWithoutATierIsNotInvented(t *testing.T) {
	doc := parsed(t, `
awml: "0.2"
title: No tier
view: security
parts:
  - { id: action, kind: outside, label: "" }
`)
	if got := Summarise(doc); got.OutsideActions != (TierCounts{}) {
		t.Errorf("outside actions = %#v", got.OutsideActions)
	}
}

func TestSummariseCountsEveryOutsideActionTier(t *testing.T) {
	doc := parsed(t, `
awml: "0.2"
title: Tiers
view: security
parts:
  - { id: allow, kind: outside, label: "Keep the draft: Allow" }
  - { id: ask, kind: outside, label: "Send the draft: Ask" }
  - { id: block, kind: outside, label: "Delete the record: Block" }
`)
	if got, want := Summarise(doc).OutsideActions, (TierCounts{Allow: 1, Ask: 1, Block: 1}); got != want {
		t.Errorf("outside actions = %#v, want %#v", got, want)
	}
}

func readExample(t *testing.T, path string) *Map {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return doc.(*Map)
}
