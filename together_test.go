package awml

import (
	"reflect"
	"strings"
	"testing"
)

// Two workflows that hand work on: the plan writes the shopping list and fires the trigger the
// shopping starts on. The shopping owns the trigger and the plan owns the list, so each refers to
// the other's part.
const planWorkflow = `awml: "0.6"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan next week's dinners", in: family }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: family }
  - { id: ready, kind: trigger, label: The shopping list is ready, ref: shop.yaml#ready }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: list, kind: data }
  - { from: plan, to: ready, kind: start }
`

const shopWorkflow = `awml: "0.6"
title: Shopping
scopes:
  - { id: home, label: "home: parents, kids", processing: any }
parts:
  - { id: ready, kind: trigger, label: The shopping list is ready, wait: PT1H }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: home, ref: plan.yaml#list }
  - { id: fill, kind: code, op: do, label: Fill the basket, in: home }
  - { id: basket, kind: outside, label: "Fill the basket: Allow", in: home }
lines:
  - { from: ready, to: fill, kind: start }
  - { from: list, to: fill, kind: data }
  - { from: fill, to: basket, kind: data }
`

func together(t *testing.T, files ...string) []Workflow {
	t.Helper()
	var workflows []Workflow
	for i := 0; i < len(files); i += 2 {
		doc := parsed(t, files[i+1])
		if mistakes := Unsound(doc); len(mistakes) != 0 {
			t.Fatalf("%s: %v", files[i], mistakes)
		}
		workflows = append(workflows, Workflow{File: files[i], Doc: doc})
	}
	return workflows
}

func togetherFinds(t *testing.T, workflows []Workflow, want ...[]string) {
	t.Helper()
	got := TogetherMistakes(workflows)
	if len(want) == 0 {
		want = make([][]string, len(workflows))
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherMistakes gave\n  %q\nwant\n  %q", got, want)
	}
}

func TestWorkflowsThatReferToEachOthersPartsAgree(t *testing.T) {
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	togetherFinds(t, workflows)
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, [][]string{nil, nil}) {
		t.Errorf("TogetherWarnings gave %q", got)
	}
}

func TestARefToAPartThatIsntThereIsAMistake(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "ref: plan.yaml#list", "ref: plan.yaml#lists", 1)
	togetherFinds(t, together(t, "plan.yaml", planWorkflow, "shop.yaml", shop),
		nil, []string{`part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"`})
}

func TestARefToAFileNotGivenIsCheckedAlone(t *testing.T) {
	togetherFinds(t, together(t, "shop.yaml", shopWorkflow))
	togetherFinds(t, together(t, "plan.yaml", planWorkflow, "other/shop.yaml", shopWorkflow))
}

func TestAPathRefIsFollowedFromTheReferringFilesFolder(t *testing.T) {
	plan := strings.Replace(planWorkflow, "ref: shop.yaml#ready", "ref: ../shopping/shop.yaml#readied", 1)
	togetherFinds(t, together(t, "meals/plan.yaml", plan, "shopping/shop.yaml", shopWorkflow),
		[]string{`part "ready" refers to ../shopping/shop.yaml#readied, but ../shopping/shop.yaml has no part "readied"`}, nil)
}

func TestAPublishedRefIsFollowedToAFilePulledIntoItsOwnersFolder(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "ref: plan.yaml#list", "ref: acme/plan#lists", 1)
	togetherFinds(t, together(t, "pulled/acme/plan.yaml", planWorkflow, "shop.yaml", shop),
		nil, []string{`part "list" refers to acme/plan#lists, but acme/plan has no part "lists"`})
	togetherFinds(t, together(t, "pulled/other/plan.yaml", planWorkflow, "shop.yaml", shop))
}

func TestACopyThatDisagreesWithThePartItRefersToIsAMistake(t *testing.T) {
	for _, tc := range []struct{ from, to, want string }{
		{"label: Shopping list, at", "label: The list, at", `part "list" refers to plan.yaml#list, but its label is "The list" here and "Shopping list" there`},
		{"kind: context, label: Shopping list, at: notes/list", "kind: surface, label: Shopping list", `part "list" refers to plan.yaml#list, but its kind is "surface" here and "context" there` + "\n" +
			`part "list" refers to plan.yaml#list, but its at is none here and "notes/list" there`},
		{"at: notes/list, in: home", "at: notes/shop, in: home", `part "list" refers to plan.yaml#list, but its at is "notes/shop" here and "notes/list" there`},
		{"at: notes/list, in: home", "in: home", `part "list" refers to plan.yaml#list, but its at is none here and "notes/list" there`},
		{`"home: parents, kids"`, `"home: parents"`, `part "list" refers to plan.yaml#list, but its scope holds parents here and kids and parents there`},
		{"processing: any", "processing: { anthropic: any }", `part "list" refers to plan.yaml#list, but its scope allows anthropic only here and any company there`},
		{"at: notes/list, in: home, ref", "at: notes/list, ref", `part "list" refers to plan.yaml#list, but it's outside every scope here and in scope "family" there`},
	} {
		t.Run(tc.want, func(t *testing.T) {
			shop := strings.Replace(shopWorkflow, tc.from, tc.to, 1)
			togetherFinds(t, together(t, "plan.yaml", planWorkflow, "shop.yaml", shop), nil, strings.Split(tc.want, "\n"))
		})
	}
}

// A scope is compared by who it holds and its terms, never by its id.
func TestScopesWithTheSamePeopleAndTermsAgreeWhateverTheirIds(t *testing.T) {
	shop := strings.Replace(shopWorkflow, `"home: parents, kids"`, `"Home: kids, parents"`, 1)
	togetherFinds(t, together(t, "plan.yaml", planWorkflow, "shop.yaml", shop))
}

func TestPartsWithTheSameIdAndNoRefAreNotLinked(t *testing.T) {
	shop := strings.Replace(shopWorkflow, ", ref: plan.yaml#list", "", 1)
	shop = strings.Replace(shop, "label: Shopping list", "label: Something else", 1)
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shop)
	togetherFinds(t, workflows)
	if strings.Contains(DrawMap(workflows), "Something else") {
		t.Error("the map draws a part no ref links")
	}
}

func TestAContextSeveralWorkflowsWriteIsAWarning(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "  - { from: fill, to: basket", "  - { from: fill, to: list, kind: data }\n  - { from: fill, to: basket", 1)
	got := TogetherWarnings(together(t, "plan.yaml", planWorkflow, "shop.yaml", shop))
	want := [][]string{{`context "list" is written by plan.yaml and shop.yaml: each may change what another relies on`}, nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
	}
}

func TestAConnectorSeveralWorkflowsReadIsAWarning(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: list,", "  - { id: inbox, kind: connector, label: The orders inbox, in: family }\n  - { id: list,", 1)
	plan = strings.Replace(plan, "lines:\n", "lines:\n  - { from: inbox, to: plan, kind: data }\n", 1)
	shop := strings.Replace(shopWorkflow, "  - { id: fill,", "  - { id: inbox, kind: connector, label: The orders inbox, in: home, ref: plan.yaml#inbox }\n  - { id: fill,", 1)
	shop = strings.Replace(shop, "lines:\n", "lines:\n  - { from: inbox, to: fill, kind: data }\n", 1)
	got := TogetherWarnings(together(t, "plan.yaml", plan, "shop.yaml", shop))
	want := [][]string{{`connector "inbox" is read by plan.yaml and shop.yaml: each may take an item another expects`}, nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
	}
}

func TestUntrustedTextIsFollowedAcrossWorkflows(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: list,", "  - { id: form, kind: connector, label: The order form, untrusted: true, in: family }\n  - { id: list,", 1)
	plan = strings.Replace(plan, "lines:\n", "lines:\n  - { from: form, to: plan, kind: data }\n", 1)
	workflows := together(t, "plan.yaml", plan, "shop.yaml", shopWorkflow)
	if got := Warnings(workflows[0].Doc); len(got) != 0 {
		t.Fatalf("the plan alone warns %q", got)
	}
	got := TogetherWarnings(workflows)
	want := [][]string{nil, {`untrusted text from part "form" in plan.yaml reaches outside action "basket": a review, a person's own step or a code step marked cleans in between would stop it`}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
	}
	summary := SummariseTogether(workflows)
	if len(summary.Untrusted) != 1 || !reflect.DeepEqual(summary.Untrusted[0].OutsideActions, []string{"Fill the basket: Allow"}) {
		t.Errorf("the summary's untrusted parts are %+v", summary.Untrusted)
	}
}

func TestTheMapDrawsABoxForEachWorkflowAndEachSharedPartOnce(t *testing.T) {
	got := DrawMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow))
	for _, want := range []string{
		"  subgraph w1[\"Dinners\"]\n    direction TB\n    w1_thu([\"Thursday 19#58;00\"])\n  end\n",
		"  subgraph w2[\"Shopping\"]\n    direction TB\n  end\n",
		"  s1[(\"Shopping list\")]\n  s2([\"The shopping list is ready\"])\n",
		"  w1 -.-> s1\n  s1 -.-> w2\n  w1 --> s2\n  s2 --> w2\n",
		"  class w1_thu,s2 trig\n  class s1 ctx\n",
		"  style w1 " + workflowBox + "\n  style w2 " + workflowBox,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the map lacks\n%s\nin\n%s", want, got)
		}
	}
	if strings.Count(got, "Shopping list") != 1 {
		t.Errorf("the shared list is drawn more than once:\n%s", got)
	}
}

func TestTheSummaryOfSeveralCountsASharedPartOnceAndNamesIt(t *testing.T) {
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	got := SummariseTogether(workflows)
	if got.Triggers != 2 || got.Contexts != 1 || got.Scopes != 2 || got.Steps != (StepCounts{AI: 1, Code: 1}) {
		t.Errorf("the summary counts %+v", got)
	}
	want := []SharedSummary{
		{Label: "Shopping list", Kind: "context", Workflows: []string{"Dinners", "Shopping"}},
		{Label: "The shopping list is ready", Kind: "trigger", Workflows: []string{"Dinners", "Shopping"}},
	}
	if !reflect.DeepEqual(got.Shared, want) {
		t.Errorf("shared parts are %+v\nwant %+v", got.Shared, want)
	}
}

func TestTheSummaryOfSeveralListsWhatWaitsOnPeopleWithTheTriggers(t *testing.T) {
	shop := strings.Replace(shopWorkflow, `label: "Fill the basket: Allow"`, `label: "Check out: Ask"`, 1)
	shop = strings.Replace(shop, "  - { id: basket,", "  - { id: yes, kind: review, label: Owen's yes to the basket, in: home }\n  - { id: basket,", 1)
	shop = strings.Replace(shop, "{ from: fill, to: basket, kind: data }", "{ from: fill, to: yes, kind: data }\n  - { from: yes, to: basket, kind: data, label: \"[yes]\" }", 1)
	got := SummariseTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shop)).Waits
	want := []WaitSummary{{Label: "Owen's yes to the basket", Kind: "review", Workflow: "Shopping", Triggers: []string{"The shopping list is ready"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("waits are %+v\nwant %+v", got, want)
	}
}

func TestTheSummaryOfOneWorkflowIsItsOwn(t *testing.T) {
	for _, path := range []string{"examples/dinners.yaml", "examples/client-email.yaml", "examples/tour/14-untrusted-text.yaml"} {
		doc := readExample(t, path)
		if got, want := SummariseTogether([]Workflow{{File: path, Doc: doc}}), Summarise(doc); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: SummariseTogether gave %+v\nwant %+v", path, got, want)
		}
	}
}

const laptop = `awml: "0.6"
platform: { id: laptop, label: A laptop }
off_for_up_to: PT0S
steps: agent
agents: [anthropic]
calls: []
holds: []
`

func TestFitJudgesEachPlatformAsRunningEveryWorkflowGiven(t *testing.T) {
	report := JudgeTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow), fitDocs(t, laptop))
	want := []FitResult{{Platform: "laptop", Judged: true, Reasons: []string{
		"plan#list is kept at notes/list, which the platform doesn't hold",
		"shop#fill is a code step, which anthropic's agent would carry out",
	}, Models: map[string][]string{"plan#plan": {"anthropic"}}, steps: []string{"plan#plan"}}}
	if len(report.Gaps) != 0 || !reflect.DeepEqual(report.Results, want) {
		t.Errorf("fit gave %+v\nwant %+v", report, want)
	}
}

func TestFitOfSeveralNamesEachGapByItsWorkflow(t *testing.T) {
	shop := strings.Replace(shopWorkflow, ", wait: PT1H", "", 1)
	report := JudgeTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shop), fitDocs(t, laptop))
	if want := []string{`trigger "shop#ready" has no wait`}; !reflect.DeepEqual(report.Gaps, want) {
		t.Errorf("gaps are %q, want %q", report.Gaps, want)
	}
}

func TestFitOfOneWorkflowIsAsBefore(t *testing.T) {
	docs := fitDocs(t, fitSecurity, laptop)
	if got, want := JudgeTogether([]Workflow{{File: "mail.yaml", Doc: docs[0]}}, docs[1:]), Judge(docs[0], nil, docs[1:]); !reflect.DeepEqual(got, want) {
		t.Errorf("JudgeTogether gave %+v\nwant %+v", got, want)
	}
}
