package awml

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Two workflows that hand work on: the plan writes the shopping list, which the shopping refers
// to, and a do step asks the shopping to start, with an outside action that names its trigger.
const planWorkflow = `awml: "0.8"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan next week's dinners", in: family }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: family }
  - { id: tell, kind: code, op: do, label: Tell the shopping the list is ready, in: family }
  - { id: ready, kind: outside, label: "Start the shopping: Allow", in: family, ref: shop.yaml#ready }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: list, kind: data }
  - { from: plan, to: tell, kind: start }
  - { from: tell, to: ready, kind: start }
`

const shopWorkflow = `awml: "0.8"
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
	return `awml: "0.8"
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
		// A scope records its people only in its label, so a label that doesn't list them is compared as written.
		{`"home: parents, kids"`, `"The family"`, `part "list" refers to plan.yaml#list, but its scope's label is "The family" here and "family: parents, kids" there`},
	} {
		t.Run(tc.want, func(t *testing.T) {
			shop := strings.Replace(shopWorkflow, tc.from, tc.to, 1)
			togetherFinds(t, together(t, "plan.yaml", planWorkflow, "shop.yaml", shop), nil, strings.Split(tc.want, "\n"))
		})
	}
}

// An outside action that starts another workflow names that workflow's trigger. It's a part of its
// own, not a copy, so its kind, label and scope are its own.
func TestAnOutsideActionsRefNamesTheTriggerOfTheWorkflowItStarts(t *testing.T) {
	plan := strings.Replace(planWorkflow, "ref: shop.yaml#ready", "ref: shop.yaml#fill", 1)
	togetherFinds(t, together(t, "plan.yaml", plan, "shop.yaml", shopWorkflow), []string{
		`part "ready" refers to shop.yaml#fill, but that's a code step: an outside action's ref names the trigger of the workflow it starts`,
	}, nil)
	plan = strings.Replace(planWorkflow, "ref: shop.yaml#ready", "ref: shop.yaml#readied", 1)
	togetherFinds(t, together(t, "plan.yaml", plan, "shop.yaml", shopWorkflow), []string{
		`part "ready" refers to shop.yaml#readied, but shop.yaml has no part "readied"`,
	}, nil)
	plan = strings.Replace(planWorkflow, "ref: shop.yaml#ready", "ref: plan.yaml#thu", 1)
	togetherFinds(t, together(t, "plan.yaml", plan, "shop.yaml", shopWorkflow), []string{
		`part "ready" refers to plan.yaml#thu, which is in its own workflow: a ref names a part of another workflow`,
	}, nil)
	// The action and the trigger it starts are two parts, so neither counts as shared.
	if got := SummariseTogether(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)).Shared; len(got) != 1 || got[0].Kind != "context" {
		t.Errorf("the shared parts are %+v, want the list alone", got)
	}
}

// breachOf is the mistake awml check reports where text from a part reaches an outside action
// with nothing to stop it or accept the risk.
func breachOf(from, action string) string {
	return `untrusted text from part ` + from + ` reaches outside action "` + action + `" with nothing to stop it or accept the risk: ` + untrustedFix
}

// Given together, untrusted text that reaches an outside action carries on into the trigger it
// starts, and through the workflow it starts, and a line that brings it into an action there with
// nothing to stop it or accept the risk is a mistake. Checked alone, the started workflow doesn't
// know.
func TestUntrustedTextFollowsAnOutsideActionIntoTheWorkflowItStarts(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: tell,", "  - { id: form, kind: connector, label: The order form, untrusted: true, in: family }\n  - { id: tell,", 1)
	plan = strings.Replace(plan, "lines:\n", "lines:\n  - { from: form, to: tell, kind: data }\n", 1)
	workflows := together(t, "plan.yaml", plan, "shop.yaml", shopWorkflow)
	mistakes(t, workflows[0].Doc, breachOf(`"form"`, "ready"))
	mistakes(t, workflows[1].Doc)
	togetherFinds(t, workflows, nil, []string{breachOf(`"form" in plan.yaml`, "basket")})
	// It's a line that breaks a rule, so the map is still drawn.
	if got := TogetherUnsound(workflows); !reflect.DeepEqual(got, [][]string{nil, nil}) {
		t.Errorf("TogetherUnsound gave %q", got)
	}
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, [][]string{nil, nil}) {
		t.Errorf("TogetherWarnings gave %q", got)
	}
	untrusted := []UntrustedSummary{{Part: "The order form", Steps: []string{"Tell the shopping the list is ready", "Fill the basket"},
		OutsideActions: []string{"Start the shopping: Allow", "Fill the basket: Allow"}}}
	if got := SummariseTogether(workflows).Untrusted; !reflect.DeepEqual(got, untrusted) {
		t.Errorf("the summary's untrusted parts are %+v\nwant %+v", got, untrusted)
	}
	// A review in the started workflow stops it there.
	shop := strings.Replace(shopWorkflow, "  - { from: ready, to: fill, kind: start }", "  - { from: ready, to: ok, kind: start }\n  - { from: ok, to: fill, kind: start }", 1)
	shop = strings.Replace(shop, "  - { id: fill,", "  - { id: ok, kind: review, label: Check the list, in: home }\n  - { id: fill,", 1)
	togetherFinds(t, together(t, "plan.yaml", plan, "shop.yaml", shop))
	// A standing approval in front of the action there accepts the risk, and the summary lists it.
	shop = strings.Replace(shopWorkflow, "  - { from: fill, to: basket, kind: data }", "  - { from: fill, to: yes, kind: start }\n  - { from: yes, to: basket, kind: data }\n  - { from: yes, to: orders, kind: data }", 1)
	shop = strings.Replace(shop, `  - { id: basket, kind: outside, label: "Fill the basket: Allow", in: home }`, `  - { id: yes, kind: approval, label: "The parents' standing approval, risk accepted by the parents", in: home }`+"\n"+
		`  - { id: basket, kind: outside, label: "Fill the basket: Ask", in: home }`+"\n"+`  - { id: orders, kind: surface, label: "The parents' orders", in: home }`, 1)
	workflows = together(t, "plan.yaml", plan, "shop.yaml", shop)
	togetherFinds(t, workflows)
	accepted := []AcceptedRisk{{OutsideAction: "Fill the basket: Ask", Approval: "The parents' standing approval, risk accepted by the parents"}}
	if got := SummariseTogether(workflows).Untrusted[0].RisksAccepted; !reflect.DeepEqual(got, accepted) {
		t.Errorf("the risks accepted are %+v", got)
	}
}

// Labels that list no one are the same scope when they're written the same.
func TestScopesWhoseLabelsListNoOneAgreeWhenTheLabelsDo(t *testing.T) {
	plan := strings.Replace(planWorkflow, `"family: parents, kids"`, `"The family"`, 1)
	shop := strings.Replace(shopWorkflow, `"home: parents, kids"`, `"The family"`, 1)
	togetherFinds(t, together(t, "plan.yaml", plan, "shop.yaml", shop))
}

// A scope is compared by who it holds and its terms, never by its id.
func TestScopesWithTheSamePeopleAndTermsAgreeWhateverTheirIds(t *testing.T) {
	shop := strings.Replace(shopWorkflow, `"home: parents, kids"`, `"Home: kids, parents"`, 1)
	togetherFinds(t, together(t, "plan.yaml", planWorkflow, "shop.yaml", shop))
}

// Two parts with the same id, kind, label and scope, and no ref, are two parts: nothing reports them
// shared, counts them once or warns that both workflows write the one list.
func TestPartsWithTheSameIdAndNoRefAreNotLinked(t *testing.T) {
	shop := strings.Replace(shopWorkflow, ", ref: plan.yaml#list", "", 1)
	shop = strings.Replace(shop, "  - { from: fill, to: basket", "  - { from: fill, to: list, kind: data }\n  - { from: fill, to: basket", 1)
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shop)
	togetherFinds(t, workflows)
	if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, [][]string{nil, nil}) {
		t.Errorf("TogetherWarnings gave %q", got)
	}
	summary := SummariseTogether(workflows)
	if summary.Shared != nil {
		t.Errorf("the shared parts are %+v, want none", summary.Shared)
	}
	if summary.Contexts != 2 {
		t.Errorf("the summary counts %d contexts, want 2", summary.Contexts)
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
	mistakes(t, workflows[0].Doc, breachOf(`"form"`, "ready"))
	togetherFinds(t, workflows, nil, []string{breachOf(`"form" in plan.yaml`, "basket")})
	summary := SummariseTogether(workflows)
	if len(summary.Untrusted) != 1 || !reflect.DeepEqual(summary.Untrusted[0].OutsideActions, []string{"Start the shopping: Allow", "Fill the basket: Allow"}) {
		t.Errorf("the summary's untrusted parts are %+v", summary.Untrusted)
	}
}

// The inbox sorter owns the shop's inbox, and tags each mail and sorts it into a queue. The replier
// reads the queue and sends a reply, and its copy of the inbox marks it untrusted, as anyone can
// write to it.
const sorterWorkflow = `awml: "0.8"
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

const replierWorkflow = `awml: "0.8"
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
	togetherFinds(t, workflows,
		[]string{breachOf(`"mail" in replier.yaml`, "label")},
		[]string{breachOf(`"mail" in replier.yaml`, "send")})
	untrusted := []UntrustedSummary{{Part: "The shop inbox", Steps: []string{"Sort the mail", "Tag the mail", "Draft a reply", "Send it"},
		OutsideActions: []string{"Label the mail: Allow", "Send the reply: Allow"}}}
	if got := SummariseTogether(workflows).Untrusted; !reflect.DeepEqual(got, untrusted) {
		t.Errorf("the summary's untrusted parts are %+v\nwant %+v", got, untrusted)
	}
	// The map draws the inbox with the red outline of text that reaches an action unstopped.
	got := DrawMap(workflows) + "\n"
	has(t, got, `s1{{"The shop inbox"}}`)
	has(t, got, "\n  class s1 unstopped\n")
	// Where the owner's copy is marked too, what it reaches in its own workflow is reported there.
	marked := strings.Replace(sorterWorkflow, "label: The shop inbox, in: office }", "label: The shop inbox, in: office, untrusted: true }", 1)
	workflows = together(t, "sorter.yaml", marked, "replier.yaml", replierWorkflow)
	mistakes(t, workflows[0].Doc, breachOf(`"mail"`, "label"))
	togetherFinds(t, workflows, nil, []string{breachOf(`"mail" in sorter.yaml`, "send")})
}

// On a map, a part marked untrusted whose text reaches no action unstopped has the orange outline.
func TestTheMapDrawsAnUntrustedPartsOutline(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "label: The shopping list is ready, wait: PT1H }", "label: The shopping list is ready, wait: PT1H, untrusted: true }", 1)
	shop = strings.Replace(shop, "label: Fill the basket, in: home }", "label: Fill the basket, in: home, cleans: true }", 1)
	got := DrawMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shop)) + "\n"
	has(t, got, "\n  classDef untrusted "+untrustedOutline+"\n")
	has(t, got, "\n  class w2_ready untrusted\n")
}

func TestTheMapDrawsABoxForEachWorkflowAndEachSharedPartOnce(t *testing.T) {
	got := DrawMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow))
	for _, want := range []string{
		"  w1_thu([\"Thursday 19#58;00\"])\n  w2_ready([\"The shopping list is ready\"])\n  s1[(\"Shopping list\")]\n" +
			"  subgraph w1[\"Dinners\"]\n    direction TB\n    w1_ready>\"Start the shopping#58; Allow\"]\n  end\n" +
			"  subgraph w2[\"Shopping\"]\n    direction TB\n  end\n",
		"  w1_thu --> w1\n  w2_ready --> w2\n  w1 -.-> s1\n  s1 -.-> w2\n  w1_ready --> w2_ready\n",
		"  class w1_thu,w2_ready trig\n  class s1 ctx\n  class w1_ready out\n",
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

// Each trigger sits outside its workflow's box, with a line into the box, so what starts a workflow
// reads as coming from outside it. An outside action that starts a workflow given leads to that
// trigger, outside the box of the workflow it starts.
func TestTheMapDrawsEachTriggerOutsideItsBox(t *testing.T) {
	got := DrawMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow))
	boxes := regexp.MustCompile(`(?s)  subgraph .*?\n  end\n`).FindAllString(got, -1)
	if len(boxes) != 2 {
		t.Fatalf("the map has %d boxes:\n%s", len(boxes), got)
	}
	for _, box := range boxes {
		if strings.Contains(box, "w1_thu") || strings.Contains(box, "w2_ready") {
			t.Errorf("a trigger is drawn inside its box:\n%s", box)
		}
	}
	for _, want := range []string{"  w1_thu --> w1\n", "  w2_ready --> w2\n", "  w1_ready --> w2_ready\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("the map lacks\n%s\nin\n%s", want, got)
		}
	}
}

// A box holds an outside action only where it starts a workflow given: the map draws the hand-off as
// a line from the action to the trigger it starts.
func TestTheMapDrawsAnOutsideActionOnlyWhereItStartsAWorkflowGiven(t *testing.T) {
	sorting := strings.Replace(notes("Sorting", ""), "  - { id: notes,", "  - { id: tell, kind: code, op: do, label: Say sorting is done, in: s }\n"+
		"  - { id: done, kind: outside, label: \"Start the filing: Allow\", in: s, ref: elsewhere.yaml#start }\n  - { id: notes,", 1)
	sorting = strings.Replace(sorting, "lines:\n", "lines:\n  - { from: step, to: tell, kind: start }\n  - { from: tell, to: done, kind: start }\n", 1)
	got := DrawMap(together(t, "plan.yaml", planWorkflow, "sorting.yaml", sorting))
	for _, want := range []string{
		"  w1_thu([\"Thursday 19#58;00\"])\n  w2_go([\"Go\"])\n" +
			"  subgraph w1[\"Dinners\"]\n    direction TB\n  end\n  subgraph w2[\"Sorting\"]\n    direction TB\n  end\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the map lacks\n%s\nin\n%s", want, got)
		}
	}
	if strings.Contains(got, "w2_done") {
		t.Errorf("the map draws a hand-off to a workflow not given:\n%s", got)
	}
}

// Every trigger is one its workflow runs on, so the map draws it even with no line out of it.
func TestTheMapDrawsEveryTrigger(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: plan,", "  - { id: sun, kind: trigger, label: Sunday, wait: PT1H }\n  - { id: plan,", 1)
	got := DrawMap(together(t, "plan.yaml", plan, "shop.yaml", shopWorkflow))
	if want := "  w1_thu([\"Thursday 19#58;00\"])\n  w1_sun([\"Sunday\"])\n"; !strings.Contains(got, want) {
		t.Errorf("the map lacks\n%s\nin\n%s", want, got)
	}
}

func TestTheSummaryOfSeveralCountsASharedPartOnceAndNamesIt(t *testing.T) {
	workflows := together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	got := SummariseTogether(workflows)
	if got.Triggers != 2 || got.Contexts != 1 || got.Scopes != 2 || got.Steps != (StepCounts{AI: 1, Code: 2}) {
		t.Errorf("the summary counts %+v", got)
	}
	want := []SharedSummary{
		{Label: "Shopping list", Kind: "context", Workflows: []string{"Dinners", "Shopping"}},
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

const laptop = `awml: "0.8"
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
		"plan#tell and shop#fill are code steps, which anthropic's agent would carry out",
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
	plan := planWorkflow
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

// specHandOff gives the spec's two workflows that hand work on, each with the family's scope, which
// the spec leaves out.
func specHandOff(t *testing.T) (dinners, shopping string) {
	t.Helper()
	spec, err := os.ReadFile("SPEC.md")
	if err != nil {
		t.Fatal(err)
	}
	block := func(name, title string) string {
		_, after, found := strings.Cut(string(spec), "```yaml\n# "+name+"\n")
		if !found {
			t.Fatalf("SPEC.md has no %s", name)
		}
		parts, _, _ := strings.Cut(after, "```")
		return `awml: "0.8"
title: ` + title + `
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
` + parts
	}
	return block("dinners.yaml", "Dinners"), block("shopping.yaml", "Shopping")
}

const codeServer = `awml: "0.8"
platform: { id: server, label: Scripts on a server }
off_for_up_to: P1D
steps: code
agents: []
calls: any
holds: []
`

// The spec's two workflows check together. Starting the shopping is an outside action of the
// dinners, which waits for nothing in them, so the shopping's trigger alone is judged against the
// platform.
func TestTheSpecsHandOffStartsTheShoppingWithAnOutsideAction(t *testing.T) {
	dinners, shopping := specHandOff(t)
	workflows := together(t, "dinners.yaml", dinners, "shopping.yaml", shopping)
	togetherFinds(t, workflows)
	want := FitReport{Results: []FitResult{{Platform: "server", Judged: true, Fits: true, Reasons: []string{},
		Models: map[string][]string{"plan": {"any"}}, steps: []string{"plan"}}}, Gaps: []string{}}
	if got := JudgeTogether(workflows[:1], fitDocs(t, codeServer)); !reflect.DeepEqual(got, want) {
		t.Errorf("fit of the dinners gave %+v\nwant %+v", got, want)
	}
	if got := JudgeTogether(workflows[1:], fitDocs(t, codeServer)); len(got.Gaps) != 0 || len(got.Results) != 1 || got.Results[0].Fits {
		t.Errorf("fit of the shopping gave %+v, but it may wait PT1H on a platform off for P1D", got)
	}
	got := JudgeTogether(workflows, fitDocs(t, strings.Replace(codeServer, "P1D", "PT0S", 1)))
	if len(got.Gaps) != 0 || len(got.Results) != 1 || !got.Results[0].Fits {
		t.Errorf("fit of both gave %+v", got)
	}
}

func TestFitOfOneWorkflowIsAsBefore(t *testing.T) {
	docs := fitDocs(t, fitSecurity, laptop)
	if got, want := JudgeTogether([]Workflow{{File: "mail.yaml", Doc: docs[0]}}, docs[1:]), Judge(docs[0], nil, docs[1:]); !reflect.DeepEqual(got, want) {
		t.Errorf("JudgeTogether gave %+v\nwant %+v", got, want)
	}
}

// A file can hold an id YAML reads as a list, which is a mistake of its own, and the check still
// follows untrusted text across the workflows given without failing on it.
func TestUntrustedTextAcrossWorkflowsSurvivesAnIDThatIsAList(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: thu,", "  - { id: odd, kind: trigger, label: The order form, untrusted: true }\n  - { id: [x], kind: outside, label: \"Send it: Allow\", in: family }\n  - { id: thu,", 1)
	plan = strings.Replace(plan, "lines:\n", "lines:\n  - { from: odd, to: [x], kind: start }\n", 1)
	workflows := []Workflow{
		{File: "plan.yaml", Doc: parsed(t, plan)},
		{File: "shop.yaml", Doc: parsed(t, shopWorkflow)},
	}
	if len(Check(workflows[0].Doc)) == 0 {
		t.Fatal("the plan's list id isn't a mistake of its own")
	}
	TogetherMistakes(workflows)
}
