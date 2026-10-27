package awml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// A bakery's order form, which anyone can write in. What they type is read by an AI step, kept in
// the orders and planned from, and a code step that keeps only the order number and quantity
// passes on what the label is made from.
const orderForm = `awml: "0.7"
title: Cake orders
scopes:
  - { id: bakery, label: "bakery: Ana, Ben", processing: any }
parts:
  - { id: form, kind: trigger, label: "An order on the website's form", untrusted: true }
  - { id: read, kind: ai, op: bring-in, label: "Read the order", in: bakery }
  - { id: orders, kind: context, label: Orders, in: bakery }
  - { id: plan, kind: ai, op: prepare, label: "Plan the baking", in: bakery }
  - { id: split, kind: fork, in: bakery }
  - { id: note, kind: code, op: do, label: "Note the plan", in: bakery }
  - { id: board, kind: outside, label: "Pin the plan to the board: Allow", in: bakery }
  - { id: keep, kind: code, op: bring-in, label: "Keep only the order number and quantity", in: bakery, cleans: true }
  - { id: tag, kind: code, op: do, label: "Label the box", in: bakery }
  - { id: box, kind: outside, label: "Print the label: Allow", in: bakery }
lines:
  - { from: form, to: read, kind: start }
  - { from: read, to: orders, kind: data }
  - { from: orders, to: plan, kind: data }
  - { from: plan, to: split, kind: start }
  - { from: split, to: note, kind: start }
  - { from: note, to: board, kind: start }
  - { from: form, to: keep, kind: start }
  - { from: keep, to: tag, kind: start }
  - { from: tag, to: box, kind: start }
`

func TestA06FileIsAWorkflowThatMayMarkPartsUntrustedAndCodeStepsCleans(t *testing.T) {
	doc := parsed(t, orderForm)
	if !IsCanonical(doc) {
		t.Fatal("a 0.7 file isn't read as a workflow")
	}
	mistakes(t, doc)
	for _, kind := range []string{"trigger", "connector", "context"} {
		src := strings.Replace(orderForm, "kind: trigger, label: \"An order on the website's form\"", "kind: "+kind+", label: \"An order on the website's form\", in: bakery", 1)
		if kind != "trigger" {
			src = strings.Replace(src, "{ from: form, to: read, kind: start }", "{ from: form, to: read, kind: data }", 1)
			src = strings.Replace(src, "{ from: form, to: keep, kind: start }", "{ from: form, to: keep, kind: data }", 1)
		}
		mistakes(t, parsed(t, src))
	}
}

func TestOnlyATriggerConnectorOrContextIsMarkedUntrusted(t *testing.T) {
	src := strings.Replace(orderForm, `label: "Read the order", in: bakery }`, `label: "Read the order", in: bakery, untrusted: true }`, 1)
	mistakes(t, parsed(t, src), `part "read" is marked untrusted, but only a trigger, connector or context takes it`)
	src = strings.Replace(orderForm, "untrusted: true", `untrusted: "yes"`, 1)
	mistakes(t, parsed(t, src), `part "form": untrusted must be true or false, not "yes"`)
}

func TestOnlyACodeStepCleans(t *testing.T) {
	src := strings.Replace(orderForm, "kind: code, op: bring-in, label: \"Keep only", "kind: ai, op: bring-in, label: \"Keep only", 1)
	mistakes(t, parsed(t, src), `part "keep" cleans, but only a code step takes cleans: strangers' text can steer an AI step however it's written`)
	src = strings.Replace(orderForm, "label: Orders, in: bakery }", "label: Orders, in: bakery, cleans: true }", 1)
	mistakes(t, parsed(t, src), `part "orders" cleans, but only a code step takes cleans`)
	src = strings.Replace(orderForm, "cleans: true", "cleans: 1", 1)
	mistakes(t, parsed(t, src), `part "keep": cleans must be true or false, not "1"`)
}

func TestUntrustedTextSpreadsAlongEveryLineThatLeadsOnUntilSomethingStopsIt(t *testing.T) {
	got := Summarise(parsed(t, orderForm)).Untrusted
	want := []UntrustedSummary{{
		Part:           "An order on the website's form",
		Steps:          []string{"Read the order", "Plan the baking", "Note the plan", "Keep only the order number and quantity"},
		OutsideActions: []string{"Pin the plan to the board: Allow"},
		StoppedBy:      []string{"Keep only the order number and quantity"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("untrusted = %#v\nwant %#v", got, want)
	}
}

func TestARefusedReadCarriesNoUntrustedText(t *testing.T) {
	src := strings.Replace(orderForm, "{ from: orders, to: plan, kind: data }", "{ from: orders, to: plan, kind: refused }\n  - { from: orders, to: tag, kind: refused }", 1)
	got := Summarise(parsed(t, src)).Untrusted[0]
	if want := []string{"Read the order", "Keep only the order number and quantity"}; !reflect.DeepEqual(got.Steps, want) {
		t.Errorf("steps = %q, want %q", got.Steps, want)
	}
	if got.OutsideActions != nil {
		t.Errorf("outside actions = %q", got.OutsideActions)
	}
}

func TestAReviewOrAPersonsStepStopsUntrustedTextButAStandingApprovalDoesnt(t *testing.T) {
	for _, c := range []struct {
		kind, extra string
		stops       bool
	}{{"review", "", true}, {"person", ", op: check", true}, {"approval", "", false}} {
		t.Run(c.kind, func(t *testing.T) {
			src := strings.Replace(orderForm, "  - { id: board,", "  - { id: yes, kind: "+c.kind+c.extra+", label: Ana, in: bakery }\n  - { id: board,", 1)
			src = strings.Replace(src, "{ from: note, to: board, kind: start }", "{ from: note, to: yes, kind: start }\n  - { from: yes, to: board, kind: start }", 1)
			got := Summarise(parsed(t, src)).Untrusted[0]
			if reached := len(got.OutsideActions) == 1; reached == c.stops {
				t.Errorf("outside actions = %q", got.OutsideActions)
			}
			if stopped := reflect.DeepEqual(got.StoppedBy, []string{"Ana", "Keep only the order number and quantity"}); stopped != c.stops {
				t.Errorf("stopped by = %q", got.StoppedBy)
			}
		})
	}
}

func TestACodeStepThatDoesntCleanLetsUntrustedTextThrough(t *testing.T) {
	src := strings.Replace(orderForm, ", cleans: true", "", 1)
	got := Summarise(parsed(t, src)).Untrusted[0]
	if want := []string{"Pin the plan to the board: Allow", "Print the label: Allow"}; !reflect.DeepEqual(got.OutsideActions, want) {
		t.Errorf("outside actions = %q, want %q", got.OutsideActions, want)
	}
}

func TestCheckWarnsOfEachOutsideActionUntrustedTextReaches(t *testing.T) {
	want := []string{`untrusted text from part "form" reaches outside action "board": a review, a person's own step or a code step marked cleans in between would stop it`}
	if got := warningsOf(parsed(t, orderForm), "warn.untrusted-outside"); !reflect.DeepEqual(got, want) {
		t.Errorf("warnings = %q\nwant %q", got, want)
	}
	report := CheckSources("dev", []Source{source("orders.yaml", orderForm)})
	if got := checkResult(t, report, "warn.untrusted-outside"); got.Result != "warn" || len(got.Mistakes) != 1 {
		t.Errorf("warn.untrusted-outside gave %s with %+v", got.Result, got.Mistakes)
	}
	for _, id := range []string{"untrusted.parts", "cleans.code-only"} {
		if got := checkResult(t, report, id); got.Result != "pass" {
			t.Errorf("%s gave %s", id, got.Result)
		}
	}
}

func TestEveryViewMarksAnUntrustedPartAfterItsLabel(t *testing.T) {
	src := orderForm + "implementations:\n  shop:\n    label: The shop\n    blocks:\n      - { id: till, label: The till, parts: [read, orders, plan, split, note, board, keep, tag, box] }\n"
	doc := parsed(t, src)
	has(t, Draw(doc), "\n    form([\"An order on the website#39;s form ⚠️\"])\n")
	view, err := View(doc, "shop")
	if err != nil {
		t.Fatal(err)
	}
	has(t, Draw(view), "form([\"An order on the website#39;s form ⚠️\"])")
	if strings.Contains(Draw(doc), "🛡️") || strings.Contains(Draw(doc), "taint") {
		t.Error("the security view shows more than the ⚠️")
	}
}

func TestTheUntrustedViewTintsWhatTheTextReachesMarksWhatStopsItAndReddensTheOutsideActions(t *testing.T) {
	view, err := UntrustedView(parsed(t, orderForm))
	if err != nil {
		t.Fatal(err)
	}
	out := Draw(view) + "\n"
	has(t, out, "\n  subgraph bakery[\"bakery#58; Ana#44; Ben\"]\n")
	has(t, out, "\n  style bakery "+scope+"\n")
	has(t, out, "\n    keep(\"<small style='color:rgb(61,67,79)'>⚙️ 📥 bring-in</small><br>Keep only the order number and quantity 🛡️\")\n")
	has(t, out, "\n  classDef taint "+taint+"\n")
	has(t, out, "\n  classDef taintout "+taintOut+"\n")
	has(t, out, "\n  class form,read,orders,plan,split,note taint\n")
	has(t, out, "\n  class board taintout\n")
	has(t, out, "\n  class keep,tag code\n")
	has(t, out, "\n  class box out\n")
	if strings.Count(out, "🛡️") != 1 {
		t.Errorf("only the step that stops the text is marked:\n%s", out)
	}
}

func TestOnlyAFileThatMarksAPartUntrustedHasAnUntrustedView(t *testing.T) {
	if _, err := UntrustedView(parsed(t, soundWorkflow)); err == nil || err.Error() != "the file marks no part untrusted, so it has no untrusted view" {
		t.Errorf("err = %v", err)
	}
}

func TestAPageDrawsTheUntrustedViewAfterTheSecurityView(t *testing.T) {
	doc := parsed(t, orderForm)
	view, _ := UntrustedView(doc)
	html := Page([]*Map{doc})
	if got, want := drawings(t, html), []string{Draw(doc), Draw(view)}; !reflect.DeepEqual(got, want) {
		t.Errorf("drawings = %q", got)
	}
	has(t, html, `<p class="view">Untrusted view: where strangers' text reaches</p>`)
	if got := drawings(t, Page([]*Map{parsed(t, soundWorkflow)})); len(got) != 2 {
		t.Errorf("a file with nothing untrusted drew %d views", len(got))
	}
}

func TestTheUntrustedSummaryHasStableJSONFieldNames(t *testing.T) {
	got, err := json.Marshal(Summarise(parsed(t, orderForm)).Untrusted)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"part":"An order on the website's form","steps":["Read the order","Plan the baking","Note the plan","Keep only the order number and quantity"],"outsideActions":["Pin the plan to the board: Allow"],"stoppedBy":["Keep only the order number and quantity"]}]`
	if string(got) != want {
		t.Errorf("JSON = %s\nwant %s", got, want)
	}
}
