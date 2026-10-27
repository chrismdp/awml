package awml

import (
	"encoding/json"
	"reflect"
	"sort"
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

// Files given without where they are, as the WebAssembly build gives them, are found from one root,
// whether their names start there or not.
func TestAPathRefIsFollowedFromOneRootWhenNoFileSaysWhereItIs(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	togetherFinds(t, together(t, "/meals/plan.yaml", planWorkflow, "meals/./shop.yaml", shop),
		nil, []string{`part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"`})
}

// A file's Path says where it is, however it was named, and its refs are followed from there.
func TestAPathRefIsFollowedFromWhereTheFileIs(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	workflows := together(t, "plan.yaml", planWorkflow, "../meals/shop.yaml", shop)
	workflows[0].Path, workflows[1].Path = "/home/ana/meals/plan.yaml", "/home/ana/meals/shop.yaml"
	togetherFinds(t, workflows, nil, []string{`part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"`})
	published := strings.Replace(shopWorkflow, "plan.yaml#list", "meals/plan#lists", 1)
	workflows = together(t, "plan.yaml", planWorkflow, "../meals/shop.yaml", published)
	workflows[0].Path, workflows[1].Path = "/home/ana/meals/plan.yaml", "/home/ana/meals/shop.yaml"
	togetherFinds(t, workflows, nil, []string{`part "list" refers to meals/plan#lists, but meals/plan has no part "lists"`})
}

func TestAPublishedRefIsFollowedToAFilePulledIntoItsOwnersFolder(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "ref: plan.yaml#list", "ref: acme/plan#lists", 1)
	togetherFinds(t, together(t, "pulled/acme/plan.yaml", planWorkflow, "shop.yaml", shop),
		nil, []string{`part "list" refers to acme/plan#lists, but acme/plan has no part "lists"`})
	togetherFinds(t, together(t, "pulled/other/plan.yaml", planWorkflow, "shop.yaml", shop))
}

// A ref names a part of another workflow, so one that names its own is a mistake, whether or not the
// part is there.
func TestARefToItsOwnWorkflowIsAMistake(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: ready,", "  - { id: copy, kind: context, label: Shopping list, at: notes/list, in: family, ref: plan.yaml#list }\n"+
		"  - { id: gone, kind: context, label: Old list, in: family, ref: ../meals/plan.yaml#gone }\n  - { id: ready,", 1)
	want := []string{
		`part "copy" refers to plan.yaml#list, which is in its own workflow: a ref names a part of another workflow`,
		`part "gone" refers to ../meals/plan.yaml#gone, which is in its own workflow: a ref names a part of another workflow`,
	}
	togetherFinds(t, together(t, "meals/plan.yaml", plan), want)
	togetherFinds(t, together(t, "meals/plan.yaml", plan, "meals/shop.yaml", shopWorkflow), want, nil)
}

// notes is a workflow that writes notes, which it refers to another workflow for, or owns when ref is
// empty.
func notes(title, ref string) string {
	if ref != "" {
		ref = ", ref: " + ref
	}
	return `awml: "0.6"
title: ` + title + `
scopes:
  - { id: s, label: "s: Ana", processing: any }
parts:
  - { id: go, kind: trigger, label: "Go", wait: PT1H }
  - { id: step, kind: code, op: sort, label: Sort, in: s }
  - { id: notes, kind: context, label: Notes, in: s` + ref + ` }
lines:
  - { from: go, to: step, kind: start }
  - { from: step, to: notes, kind: data }
`
}

// A part's refs lead to the one copy that owns it, so refs that go round in a circle are a mistake
// in each file on the circle.
func TestRefsThatGoRoundInACircleAreAMistake(t *testing.T) {
	togetherFinds(t, together(t, "a.yaml", notes("A", "b.yaml#notes"), "b.yaml", notes("B", "a.yaml#notes")),
		[]string{`part "notes" refers to b.yaml#notes, but its refs go round in a circle through b.yaml, so no workflow owns it: the owner's copy has no ref`},
		[]string{`part "notes" refers to a.yaml#notes, but its refs go round in a circle through a.yaml, so no workflow owns it: the owner's copy has no ref`})
	workflows := together(t, "a.yaml", notes("A", "b.yaml#notes"), "b.yaml", notes("B", "c.yaml#notes"), "c.yaml", notes("C", "a.yaml#notes"), "d.yaml", notes("D", "a.yaml#notes"))
	togetherFinds(t, workflows,
		[]string{`part "notes" refers to b.yaml#notes, but its refs go round in a circle through b.yaml and c.yaml, so no workflow owns it: the owner's copy has no ref`},
		[]string{`part "notes" refers to c.yaml#notes, but its refs go round in a circle through c.yaml and a.yaml, so no workflow owns it: the owner's copy has no ref`},
		[]string{`part "notes" refers to a.yaml#notes, but its refs go round in a circle through a.yaml and b.yaml, so no workflow owns it: the owner's copy has no ref`},
		nil)
	// Every copy is still one part.
	want := [][]string{{`context "notes" is written by a.yaml, b.yaml, c.yaml and d.yaml: each may change what another relies on`}, nil, nil, nil}
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
	}
	// The first copy on the circle given stands in for the owner, wherever the refs are followed from.
	workflows = together(t, "d.yaml", notes("D", "b.yaml#notes"), "a.yaml", notes("A", "b.yaml#notes"), "b.yaml", notes("B", "a.yaml#notes"))
	want = [][]string{nil, {`context "notes" is written by a.yaml, d.yaml and b.yaml: each may change what another relies on`}, nil}
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
	}
	// A chain of refs that ends at an owner is no circle.
	togetherFinds(t, together(t, "a.yaml", notes("A", "b.yaml#notes"), "b.yaml", notes("B", "c.yaml#notes"), "c.yaml", notes("C", "")))
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

// The inbox sorter owns the shop's inbox, and tags each mail and sorts it into a queue. The replier
// reads the queue and sends a reply, and its copy of the inbox marks it untrusted, as anyone can
// write to it.
const sorterWorkflow = `awml: "0.6"
title: Inbox sorter
scopes:
  - { id: office, label: "office: Ana", processing: any }
parts:
  - { id: arrive, kind: trigger, label: "Mail arrives", wait: PT1H }
  - { id: mail, kind: connector, label: The shop inbox, in: office }
  - { id: sort, kind: code, op: sort, label: Sort the mail, in: office }
  - { id: queue, kind: context, label: Sorted mail, at: notes/queue, in: office }
  - { id: tag, kind: code, op: do, label: Tag the mail, in: office }
  - { id: label, kind: outside, label: "Label the mail: Allow", in: office }
lines:
  - { from: arrive, to: sort, kind: start }
  - { from: mail, to: sort, kind: data }
  - { from: sort, to: queue, kind: data }
  - { from: mail, to: tag, kind: data }
  - { from: tag, to: label, kind: data }
`

const replierWorkflow = `awml: "0.6"
title: Replier
scopes:
  - { id: office, label: "office: Ana", processing: any }
parts:
  - { id: hourly, kind: trigger, label: "Every hour", wait: PT1H }
  - { id: mail, kind: connector, label: The shop inbox, in: office, untrusted: true, ref: sorter.yaml#mail }
  - { id: queue, kind: context, label: Sorted mail, at: notes/queue, in: office, ref: sorter.yaml#queue }
  - { id: draft, kind: ai, op: draft, label: Draft a reply, in: office }
  - { id: go, kind: code, op: do, label: Send it, in: office }
  - { id: send, kind: outside, label: "Send the reply: Allow", in: office }
lines:
  - { from: hourly, to: draft, kind: start }
  - { from: queue, to: draft, kind: data }
  - { from: draft, to: go, kind: data }
  - { from: go, to: send, kind: data }
  - { from: send, to: mail, kind: data }
`

// A part they share is untrusted if any workflow's copy of it is marked untrusted, and its text is
// followed from there, into every workflow.
func TestAPartIsUntrustedIfAnyCopyIsMarked(t *testing.T) {
	workflows := together(t, "sorter.yaml", sorterWorkflow, "replier.yaml", replierWorkflow)
	stop := `: a review, a person's own step or a code step marked cleans in between would stop it`
	want := [][]string{
		{`untrusted text from part "mail" in replier.yaml reaches outside action "label"` + stop},
		{`untrusted text from part "mail" in replier.yaml reaches outside action "send"` + stop},
	}
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
	}
	untrusted := []UntrustedSummary{{Part: "The shop inbox", Steps: []string{"Sort the mail", "Tag the mail", "Draft a reply", "Send it"},
		OutsideActions: []string{"Label the mail: Allow", "Send the reply: Allow"}}}
	if got := SummariseTogether(workflows).Untrusted; !reflect.DeepEqual(got, untrusted) {
		t.Errorf("the summary's untrusted parts are %+v\nwant %+v", got, untrusted)
	}
	if got := DrawMap(workflows); !strings.Contains(got, `s1{{"The shop inbox ⚠️"}}`) {
		t.Errorf("the map doesn't mark the inbox untrusted:\n%s", got)
	}
	// Where the owner's copy is marked too, what it reaches in its own workflow is warned of there.
	marked := strings.Replace(sorterWorkflow, "label: The shop inbox, in: office }", "label: The shop inbox, in: office, untrusted: true }", 1)
	workflows = together(t, "sorter.yaml", marked, "replier.yaml", replierWorkflow)
	if got := Warnings(workflows[0].Doc); !reflect.DeepEqual(got, []string{`untrusted text from part "mail" reaches outside action "label"` + stop}) {
		t.Fatalf("the sorter alone warns %q", got)
	}
	want = [][]string{nil, {`untrusted text from part "mail" in sorter.yaml reaches outside action "send"` + stop}}
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, want) {
		t.Errorf("TogetherWarnings gave %q\nwant %q", got, want)
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

// A box holds the triggers its workflow runs on, those with a line out of them, and never one it
// only fires.
func TestTheMapDrawsOnlyTheTriggersAWorkflowRunsOn(t *testing.T) {
	sorting := strings.Replace(notes("Sorting", ""), "  - { id: notes,", "  - { id: done, kind: trigger, label: Sorting is done, ref: elsewhere.yaml#start }\n  - { id: notes,", 1)
	sorting = strings.Replace(sorting, "lines:\n", "lines:\n  - { from: step, to: done, kind: start }\n", 1)
	got := DrawMap(together(t, "plan.yaml", planWorkflow, "sorting.yaml", sorting))
	for _, want := range []string{
		"  subgraph w1[\"Dinners\"]\n    direction TB\n    w1_thu([\"Thursday 19#58;00\"])\n  end\n",
		"  subgraph w2[\"Sorting\"]\n    direction TB\n    w2_go([\"Go\"])\n  end\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the map lacks\n%s\nin\n%s", want, got)
		}
	}
	for _, fired := range []string{"The shopping list is ready", "Sorting is done"} {
		if strings.Contains(got, fired) {
			t.Errorf("the map draws %q, which its workflow only fires:\n%s", fired, got)
		}
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

// What waits on a person is no part of a summary, of one workflow or several.
func TestTheSummaryOfSeveralSaysNothingOfWhatWaitsOnPeople(t *testing.T) {
	shop := strings.Replace(shopWorkflow, `label: "Fill the basket: Allow"`, `label: "Check out: Ask"`, 1)
	shop = strings.Replace(shop, "  - { id: basket,", "  - { id: yes, kind: review, label: Owen's yes to the basket, in: home }\n  - { id: basket,", 1)
	shop = strings.Replace(shop, "{ from: fill, to: basket, kind: data }", "{ from: fill, to: yes, kind: data }\n  - { from: yes, to: basket, kind: data, label: \"[yes]\" }", 1)
	encoded, err := json.Marshal(SummariseTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shop)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"waits"`) {
		t.Errorf("the summary lists what waits on people: %s", encoded)
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

// fitModels gives the AI steps fit names on the laptop, sorted.
func fitModels(t *testing.T, workflows []Workflow) []string {
	t.Helper()
	report := JudgeTogether(workflows, fitDocs(t, laptop))
	if len(report.Gaps) != 0 || len(report.Results) != 1 {
		t.Fatalf("fit gave %+v", report)
	}
	var names []string
	for name := range report.Results[0].Models {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Each workflow is named by the shortest end of its path, without .yaml, that no other file given
// shares, and one given as text alone by where it comes among them.
func TestFitNamesEachWorkflowByTheShortestEndOfItsPathThatsItsOwn(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: ready, kind: trigger, label: The shopping list is ready, ref: shop.yaml#ready }\n", "", 1)
	plan = strings.Replace(plan, "  - { from: plan, to: ready, kind: start }\n", "", 1)
	other := strings.Replace(plan, "Plan next week's dinners", "Plan the week's dinners", 1)
	for _, tc := range []struct {
		files, paths, want []string
	}{
		{[]string{"x/dinners.yaml", "y/dinners.yaml"}, nil, []string{"x/dinners#plan", "y/dinners#plan"}},
		{[]string{"home/x/dinners.yaml", "home/y/dinners.yaml", "plan.yaml"}, nil, []string{"plan#plan", "x/dinners#plan", "y/dinners#plan"}},
		{[]string{"dinners.yaml", "x/dinners.yaml"}, nil, []string{"dinners#plan", "x/dinners#plan"}},
		{[]string{"dinners.yaml", "../work/dinners.yaml"}, []string{"/home/ana/family/dinners.yaml", "/home/ana/work/dinners.yaml"}, []string{"family/dinners#plan", "work/dinners#plan"}},
		{[]string{"", ""}, nil, []string{"1#plan", "2#plan"}},
	} {
		workflows := together(t, tc.files[0], plan, tc.files[1], other)
		if len(tc.files) > 2 {
			workflows = append(workflows, together(t, tc.files[2], plan)...)
		}
		for i, path := range tc.paths {
			workflows[i].Path = path
		}
		if got := fitModels(t, workflows); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: fit names %q, want %q", tc.files, got, tc.want)
		}
	}
}

func TestFitOfOneWorkflowIsAsBefore(t *testing.T) {
	docs := fitDocs(t, fitSecurity, laptop)
	if got, want := JudgeTogether([]Workflow{{File: "mail.yaml", Doc: docs[0]}}, docs[1:]), Judge(docs[0], nil, docs[1:]); !reflect.DeepEqual(got, want) {
		t.Errorf("JudgeTogether gave %+v\nwant %+v", got, want)
	}
}
