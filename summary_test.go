package awml

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSummariseEveryExample(t *testing.T) {
	pubClient := PublishingSummary{Count: 2, Labels: []string{"standing rule: a client's email · adds an associate", "the consultant takes the reply · loosens anthropic only to any company"}}
	pubDinners := PublishingSummary{Count: 1, Labels: []string{"standing rule: dinners to carers · adds June and Beth"}}
	pubGrandma := PublishingSummary{Count: 1, Labels: []string{"standing rule: pickups to Grandma · adds Grandma Rose"}}
	pubChecklist := PublishingSummary{Count: 1, Labels: []string{"the checklist, with no names in it · adds Kit"}}
	untrustedLegend := []UntrustedSummary{{Part: "Text anyone can write, marked untrusted: a thick orange outline", StoppedBy: []string{"A review: a person's yes"}}}
	cases := map[string]Summary{
		"examples/client-email.yaml":              {Steps: StepCounts{AI: 3, Code: 3, Person: 1}, Operations: map[string]int{"bring-in": 1, "sort": 2, "learn": 1, "draft": 1, "do": 2}, OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 1, Contexts: 2, Connectors: 1, Scopes: 2, Platforms: 4, PublishingLines: pubClient},
		"examples/dinners.yaml":                   {Steps: StepCounts{AI: 1}, Triggers: 1, Contexts: 2, Surfaces: 2, Scopes: 2, Platforms: 3, PublishingLines: pubDinners},
		"examples/shopping.yaml":                  {Steps: StepCounts{Code: 1}, Operations: map[string]int{"do": 1}, OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 1, Reviews: 1, Contexts: 2, Connectors: 1, Scopes: 2, Platforms: 3},
		"examples/bills.yaml":                     {Steps: StepCounts{Code: 1}, Operations: map[string]int{"check": 1}, Triggers: 1, Connectors: 1, Surfaces: 1, Scopes: 3, Platforms: 3},
		"examples/pickups.yaml":                   {Steps: StepCounts{Code: 1}, Operations: map[string]int{"bring-in": 1}, Triggers: 1, Contexts: 1, Connectors: 1, Surfaces: 1, Scopes: 2, Platforms: 3},
		"examples/legend.yaml":                    {Steps: StepCounts{AI: 1, Code: 1, Person: 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Reviews: 1, Contexts: 1, Connectors: 2, Surfaces: 1, Platforms: 1, Untrusted: untrustedLegend},
		"examples/question.yaml":                  {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"answer": 2}, Triggers: 1, Contexts: 3, Surfaces: 1, Scopes: 3, RefusedReads: 1},
		"examples/tour/01-draft-and-send.yaml":    {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"draft": 1, "do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Reviews: 1, Scopes: 1},
		"examples/tour/02-two-start-lines.yaml":   {Steps: StepCounts{AI: 2}, Operations: map[string]int{"learn": 1, "check": 1}, Triggers: 1, Contexts: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/03-refused-read.yaml":      {Steps: StepCounts{AI: 1}, Operations: map[string]int{"answer": 1}, Triggers: 1, Contexts: 2, Surfaces: 1, Scopes: 2, RefusedReads: 1},
		"examples/tour/04-publishing.yaml":        {Steps: StepCounts{Code: 1}, Operations: map[string]int{"bring-in": 1}, Triggers: 1, Contexts: 1, Connectors: 1, Surfaces: 1, Scopes: 2, PublishingLines: pubGrandma},
		"examples/tour/05-side-by-side.yaml":      {Steps: StepCounts{AI: 2, Code: 2}, Operations: map[string]int{"draft": 2, "do": 2}, Triggers: 1, Contexts: 3, Scopes: 2, PublishingLines: pubChecklist},
		"examples/live/red-line.yaml":             {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"draft": 1, "do": 1}, OutsideActions: TierCounts{Ask: 1}, Triggers: 1, Reviews: 1, Scopes: 1, RedLines: 1},
		"examples/tour/07-person-step.yaml":       {Steps: StepCounts{Code: 1, Person: 1}, Operations: map[string]int{"check": 1, "do": 1}, Triggers: 1, Contexts: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/08-gap.yaml":               {Steps: StepCounts{Code: 1}, Operations: map[string]int{"bring-in": 1}, Triggers: 1, Contexts: 1, Connectors: 1, Surfaces: 1, Scopes: 1},
		"examples/tour/09-platform-view.yaml":     {Steps: StepCounts{AI: 1, Code: 1}, Operations: map[string]int{"bring-in": 1, "sort": 1}, Triggers: 1, Contexts: 2, Scopes: 1, Platforms: 2},
		"examples/tour/10-operations.yaml":        {Steps: StepCounts{AI: 3, Code: 1}, Operations: map[string]int{"sort": 1, "learn": 1, "draft": 1, "do": 1}, OutsideActions: TierCounts{Allow: 1}, Triggers: 1, Contexts: 2, Scopes: 1},
		"examples/tour/11-do.yaml":                {Steps: StepCounts{Code: 2}, Operations: map[string]int{"do": 2}, OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 1, Reviews: 1, Scopes: 1},
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
			// The lists of names have tests of their own below; this table checks the counts.
			got.AISteps, got.People, got.ScopeNames, got.PlatformNames = nil, nil, nil, nil
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Summarise() = %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestSummariseCountsAWorkflowGivenTwiceOnce(t *testing.T) {
	file := readExample(t, "examples/shopping.yaml")
	want := Summary{
		Steps: StepCounts{Code: 1}, Operations: map[string]int{"do": 1},
		OutsideActions: TierCounts{Allow: 1, Ask: 1}, Triggers: 1, Reviews: 1, Contexts: 2,
		Connectors: 1, Scopes: 2, Platforms: 3,
		People:        []PersonSummary{{Label: "Owen's yes to the exact basket", Kind: "review"}},
		ScopeNames:    []string{"carers", "family"},
		PlatformNames: []string{"Server", "The family messaging app", "The supermarket"},
	}
	if got := Summarise(file, file); !reflect.DeepEqual(got, want) {
		t.Errorf("Summarise(the workflow twice) = %#v\nwant %#v", got, want)
	}
}

func TestTotalSummariesAddsInventoryCounts(t *testing.T) {
	a := Summarise(readExample(t, "examples/tour/03-refused-read.yaml"))
	b := Summarise(readExample(t, "examples/live/red-line.yaml"))
	got := TotalSummaries(a, b)
	want := Summary{
		Steps: StepCounts{AI: 2, Code: 1}, Operations: map[string]int{"answer": 1, "draft": 1, "do": 1},
		OutsideActions: TierCounts{Ask: 1}, Triggers: 2, Reviews: 1, Contexts: 2,
		Surfaces: 1, Scopes: 3, RefusedReads: 1, RedLines: 1,
		AISteps:    []string{"Answer from the flat's notes alone", "Draft the week's message to the parents"},
		People:     []PersonSummary{{Label: "Ella's yes to the exact message", Kind: "review"}},
		ScopeNames: []string{"flat", "tom", "ella"},
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
	want := `{"steps":{"ai":0,"code":2,"person":0},"operations":{"do":2},"outsideActions":{"allow":1,"ask":1,"block":0},"triggers":1,"reviews":1,"contexts":0,"connectors":0,"surfaces":0,"scopes":1,"platforms":0,"publishingLines":{"count":0,"labels":null},"refusedReads":0,"redLines":0,"aiSteps":null,"people":[{"label":"Dev's yes to the exact payment","kind":"review"}],"scopeNames":["parents"],"platformNames":null,"untrusted":null}`
	if string(got) != want {
		t.Errorf("JSON = %s\nwant %s", got, want)
	}
}

func TestAnOutsideActionWithoutATierIsNotInvented(t *testing.T) {
	doc := parsed(t, `
awml: "0.10"
title: No tier
parts:
  - { id: action, kind: outside, label: "" }
`)
	if got := Summarise(doc); got.OutsideActions != (TierCounts{}) {
		t.Errorf("outside actions = %#v", got.OutsideActions)
	}
}

func TestSummariseCountsEveryOutsideActionTier(t *testing.T) {
	doc := parsed(t, `
awml: "0.10"
title: Tiers
parts:
  - { id: allow, kind: outside, label: "Keep the draft: Allow" }
  - { id: ask, kind: outside, label: "Send the draft: Ask" }
  - { id: block, kind: outside, label: "Delete the record: Block" }
`)
	if got, want := Summarise(doc).OutsideActions, (TierCounts{Allow: 1, Ask: 1, Block: 1}); got != want {
		t.Errorf("outside actions = %#v, want %#v", got, want)
	}
}

func TestSummariseListsTheAIStepsPeopleAndGroupsByName(t *testing.T) {
	got := Summarise(readExample(t, "examples/client-email.yaml"))
	if want := []string{"Choose the engagement the email belongs to", "Update the engagement's project with what the email says", "Draft a reply"}; !reflect.DeepEqual(got.AISteps, want) {
		t.Errorf("AI steps = %q, want %q", got.AISteps, want)
	}
	if want := []PersonSummary{{Label: "Edit the reply and send it", Kind: "person"}}; !reflect.DeepEqual(got.People, want) {
		t.Errorf("people = %#v, want %#v", got.People, want)
	}
	if want := []string{"private", "business"}; !reflect.DeepEqual(got.ScopeNames, want) {
		t.Errorf("scope names = %q, want %q", got.ScopeNames, want)
	}
	if want := []string{"Claude Cowork, Mac mini", "Synced Markdown notes", "The consultant's phone", "Gmail"}; !reflect.DeepEqual(got.PlatformNames, want) {
		t.Errorf("platform names = %q, want %q", got.PlatformNames, want)
	}
}

func TestSummariseListsPersonStepsReviewsAndStandingApprovalsInFileOrder(t *testing.T) {
	got := Summarise(readExample(t, "examples/legend.yaml")).People
	want := []PersonSummary{{Label: "A step decided by a person", Kind: "person"}, {Label: "A review: a person's yes", Kind: "review"},
		{Label: "A standing approval: a person's yes, given once", Kind: "approval"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("people = %#v, want %#v", got, want)
	}
}

func TestAScopeWithoutMembersIsNamedByItsWholeLabel(t *testing.T) {
	doc := parsed(t, `
awml: "0.10"
title: Bare scope
scopes:
  - { id: team, label: "The team" }
parts:
  - { id: step, kind: code, label: "Do it", in: team }
`)
	if got, want := Summarise(doc).ScopeNames, []string{"The team"}; !reflect.DeepEqual(got, want) {
		t.Errorf("scope names = %q, want %q", got, want)
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
