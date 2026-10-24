package awml

import (
	"reflect"
	"strings"
	"testing"
)

// replies is a 0.6 workflow whose data lines cross between a private scope that allows any
// company and a business scope that allows only anthropic, and into a venture beside them.
const replies = `awml: "0.6"
title: Replies
scopes:
  - { id: private, label: "private: the consultant", processing: any }
  - { id: business, label: "business: the consultant, an associate", processing: { anthropic: any } }
  - { id: venture, label: "venture: the consultant, a partner", processing: any }
parts:
  - { id: rules, kind: code, op: sort, label: Sort, in: private }
  - { id: draft, kind: ai, op: draft, label: Draft, in: business }
  - { id: drafts, kind: context, label: Drafts, in: business }
  - { id: edit, kind: person, op: do, label: Edit, in: private }
  - { id: pitch, kind: context, label: Pitch, in: venture }
lines:
  - { from: rules, to: draft, kind: data, label: "standing rule: a client's email" }
  - { from: draft, to: drafts, kind: data }
  - { from: drafts, to: edit, kind: data, label: the consultant takes the reply }
  - { from: draft, to: pitch, kind: data }
implementations:
  office:
    label: Office
    blocks:
      - { id: mini, label: Mac mini, parts: [rules, draft, drafts, edit, pitch] }
`

func TestA06FileIsAWorkflow(t *testing.T) {
	if !IsCanonical(parsed(t, replies)) {
		t.Fatal("a 0.6 file isn't read as a workflow")
	}
}

// From 0.6 a data line publishes whenever its scopes make it one, so check doesn't ask for
// publish.
func TestA06DataLineThatPublishesIsNoMistake(t *testing.T) {
	mistakes(t, parsed(t, replies))
}

func TestA06FileHasNoPublishLine(t *testing.T) {
	src := strings.Replace(replies, "{ from: draft, to: drafts, kind: data }", "{ from: draft, to: drafts, kind: publish }", 1)
	mistakes(t, parsed(t, src),
		`the line from "draft" to "drafts" has unknown kind "publish": write it as data, since a data line publishes whenever its scopes make it one`)
}

func TestAnOlderFileKeepsPublishAndItsCheck(t *testing.T) {
	src := strings.Replace(replies, `awml: "0.6"`, `awml: "0.5"`, 1)
	mistakes(t, parsed(t, src),
		`the line from "rules" to "draft" moves content from group "private" into group "business", which isn't inside it: draw it as publish, or as refused if the design refuses the read`,
		`the line from "drafts" to "edit" moves content from group "business" into group "private", which isn't inside it: draw it as publish, or as refused if the design refuses the read`,
		`the line from "draft" to "pitch" moves content from group "business" into group "venture", which isn't inside it: draw it as publish, or as refused if the design refuses the read`)
	old := strings.NewReplacer(`awml: "0.6"`, `awml: "0.5"`, "kind: data, label", "kind: publish, label", "{ from: draft, to: pitch, kind: data }", "{ from: draft, to: pitch, kind: publish }").Replace(replies)
	mistakes(t, parsed(t, old))
	out := Draw(parsed(t, old))
	has(t, out, `rules ==>|"`+text("standing rule: a client's email")+`"| draft`)
	has(t, out, `drafts ==>|"the consultant takes the reply"| edit`)
}

// Each publish is drawn thick, with who it adds and what it loosens after its own label.
func TestA06DrawingSaysWhatEachPublishAddsOrLoosens(t *testing.T) {
	out := Draw(parsed(t, replies))
	has(t, out, `rules ==>|"`+text("standing rule: a client's email")+` · adds an associate"| draft`)
	has(t, out, "\n  draft -.-> drafts\n")
	has(t, out, `drafts ==>|"the consultant takes the reply · loosens anthropic only to any company"| edit`)
	has(t, out, `draft ==>|"adds a partner · loosens anthropic only to any company"| pitch`)
	has(t, out, "linkStyle 2,3 stroke-dasharray:3")
}

func TestThePlatformViewDrawsEachPublishTheSameWay(t *testing.T) {
	view, err := View(parsed(t, replies), "office")
	if err != nil {
		t.Fatal(err)
	}
	out := Draw(view)
	has(t, out, `rules ==>|"`+text("standing rule: a client's email")+` · adds an associate"| draft`)
	has(t, out, `drafts ==>|"the consultant takes the reply · loosens anthropic only to any company"| edit`)
}

func TestTheSummaryListsEachPublishWithWhatItAddsOrLoosens(t *testing.T) {
	got := Summarise(parsed(t, replies)).PublishingLines
	want := PublishingSummary{Count: 3, Labels: []string{
		"standing rule: a client's email · adds an associate",
		"the consultant takes the reply · loosens anthropic only to any company",
		"adds a partner · loosens anthropic only to any company",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// publishingLabels checks a 0.6 file with the scopes, parts and lines given, and gives each
// publishing line's label.
func publishingLabels(t *testing.T, scopes, parts, lines string) []string {
	t.Helper()
	src := "awml: \"0.6\"\ntitle: T\nscopes:\n" + scopes + "parts:\n" + parts + "lines:\n" + lines
	doc := parsed(t, src)
	mistakes(t, doc)
	return Summarise(doc).PublishingLines.Labels
}

func labels(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A scope labelled with + holds the people it names and everyone in the scopes inside it.
func TestAPublishIntoAScopeThatAddsPeopleNamesThem(t *testing.T) {
	labels(t, publishingLabels(t,
		"  - { id: carers, label: \"carers: + June, Beth\" }\n  - { id: family, label: \"family: parents, kids\", in: carers }\n",
		"  - { id: plan, kind: context, label: Plan, in: family }\n  - { id: screen, kind: surface, label: Screen, in: carers }\n",
		"  - { from: plan, to: screen, kind: data, label: \"standing rule: dinners to carers\" }\n  - { from: screen, to: plan, kind: data }\n"),
		"standing rule: dinners to carers · adds June and Beth")
}

func TestAPublishOutOfANarrowerScopeNamesWhoItAdds(t *testing.T) {
	labels(t, publishingLabels(t,
		"  - { id: flat, label: \"flat: Tom, Ana, Joe\" }\n  - { id: tom, label: \"tom: Tom\", in: flat }\n",
		"  - { id: diary, kind: context, label: Diary, in: tom }\n  - { id: answer, kind: ai, label: Answer, in: flat }\n",
		"  - { from: diary, to: answer, kind: data }\n"),
		"adds Ana and Joe")
}

func TestAPublishOutsideEveryScopeSaysSo(t *testing.T) {
	labels(t, publishingLabels(t,
		"  - { id: family, label: \"family: parents, kids\", processing: any }\n",
		"  - { id: plan, kind: context, label: Plan, in: family }\n  - { id: site, kind: surface, label: Site }\n",
		"  - { from: plan, to: site, kind: data, label: posted }\n"),
		"posted · publishes outside every scope")
}

// Where neither the members nor the terms say what a publish adds or loosens, its label names
// the scope it moves into.
func TestAPublishTheScopesCantExplainNamesItsTarget(t *testing.T) {
	labels(t, publishingLabels(t,
		"  - { id: bakery, label: Bakery }\n  - { id: pottery, label: Pottery }\n",
		"  - { id: notes, kind: context, label: Notes, in: bakery }\n  - { id: shop, kind: context, label: Shop, in: pottery }\n",
		"  - { from: notes, to: shop, kind: data }\n"),
		"publishes into Pottery")
}

func TestTermsThatAllowCertainOperationsAreNamed(t *testing.T) {
	labels(t, publishingLabels(t,
		"  - { id: wide, label: \"wide: Jo, Kit\", processing: { anthropic: any, openai: [sort, draft] } }\n  - { id: jo, label: \"jo: Jo\", processing: none }\n",
		"  - { id: diary, kind: context, label: Diary, in: jo }\n  - { id: plan, kind: ai, label: Plan, in: wide }\n",
		"  - { from: diary, to: plan, kind: data }\n"),
		"adds Kit · loosens no company to anthropic and openai for draft and sort only")
}

// A red line draws what a live system did, so it's no publish of the design.
// A 0.6 line that still has breaks gets only the field mistake, and is skipped as before, so it
// isn't a publish either.
func TestARedDataLineIsNoPublish(t *testing.T) {
	doc := parsed(t, "awml: \"0.6\"\ntitle: T\nscopes:\n"+
		"  - { id: bakery, label: \"bakery: Jo\" }\n  - { id: pottery, label: \"pottery: Jo, Kit\" }\n"+
		"parts:\n  - { id: notes, kind: context, label: Notes, in: bakery }\n  - { id: shop, kind: context, label: Shop, in: pottery }\n"+
		"lines:\n  - { from: notes, to: shop, kind: data, breaks: true }\n")
	mistakes(t, doc, `the line from "notes" to "shop" has breaks, but from 0.6 awml check works out which lines break a rule, and awml draw draws them red`)
	labels(t, Summarise(doc).PublishingLines.Labels)
}

func TestTheScopeCheckDoesntApplyTo06Workflows(t *testing.T) {
	report := CheckSources("dev", []Source{source("replies.yaml", replies)})
	if got := checkResult(t, report, "lines.data-narrows"); got.Result != "skipped" || got.SkippedBecause != "no file given is a workflow before 0.6 or a security view" {
		t.Errorf("lines.data-narrows gave %s because %q", got.Result, got.SkippedBecause)
	}
	for _, id := range []string{"tiers.written", "tiers.shape", "approvals.listed"} {
		if got := checkResult(t, report, id); got.Result != "pass" {
			t.Errorf("%s gave %s because %q", id, got.Result, got.SkippedBecause)
		}
	}
}

func TestA06FileKeepsWhat05Allows(t *testing.T) {
	mistakes(t, parsed(t, strings.Replace(standing, `awml: "0.5"`, `awml: "0.6"`, 1)))
	src := strings.Replace(strings.Replace(canonical, `awml: "0.4"`, `awml: "0.6"`, 1), "platform: scripts", "platform: acme/platforms/ops-runner", 1)
	mistakes(t, parseCanonical(t, src))
}

func TestAPlatformDescriptionMayBe06(t *testing.T) {
	mistakes(t, parsed(t, strings.Replace(registeredDescription, `awml: "0.5"`, `awml: "0.6"`, 1)))
}
