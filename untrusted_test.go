package awml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// A bakery's order form, which anyone can write in. What they type is read by an AI step, kept in
// the orders and planned from, and the plan is pinned to the board with nothing to stop the text.
// A code step that keeps only the order number and quantity passes on what the label is made from.
const orderForm = `awml: "0.8"
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

// The words awml check reports for the board in the order form, and the advice after them.
const (
	boardReached  = `untrusted text from part "form" reaches outside action "board" with nothing to stop it or accept the risk`
	untrustedFix  = `a review, a person's own step or a code step marked cleans on the way would stop it, or a standing approval in front of the action would accept the risk`
	untrustedRule = "untrusted.outside"
)

// A box labelled from the order form, with nothing between: the text reaches the label's printer.
const labelForm = `awml: "0.8"
title: Box labels
scopes:
  - { id: bakery, label: "bakery: Ana, Ben", processing: any }
parts:
  - { id: form, kind: trigger, label: "An order on the website's form", untrusted: true }
  - { id: read, kind: ai, op: bring-in, label: "Read the order", in: bakery }
  - { id: tag, kind: code, op: do, label: "Label the box", in: bakery }
  - { id: box, kind: outside, label: "Print the label: Allow", in: bakery }
lines:
  - { from: form, to: read, kind: start }
  - { from: read, to: tag, kind: start }
  - { from: tag, to: box, kind: start }
`

// between puts a part between reading the order and labelling the box.
func between(part string) string {
	src := strings.Replace(labelForm, "  - { id: tag,", "  - { id: stop, "+part+", in: bakery }\n  - { id: tag,", 1)
	return strings.Replace(src, "  - { from: read, to: tag, kind: start }", "  - { from: read, to: stop, kind: start }\n  - { from: stop, to: tag, kind: start }", 1)
}

// untrustedLines gives how a view draws each line for untrusted text, by its ends.
func untrustedLines(t *testing.T, g Graph) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, l := range g.Lines {
		got[l.From+">"+l.To] = l.Untrusted
	}
	return got
}

func unstoppedParts(g Graph) []string {
	var found []string
	for _, p := range g.Parts {
		if p.Unstopped {
			found = append(found, p.ID)
		}
	}
	return found
}

func TestA06FileIsAWorkflowThatMayMarkPartsUntrustedAndCodeStepsCleans(t *testing.T) {
	doc := parsed(t, labelForm)
	if !IsCanonical(doc) {
		t.Fatal("a 0.8 file isn't read as a workflow")
	}
	if got := Unsound(doc); len(got) != 0 {
		t.Errorf("Unsound gave %q", got)
	}
	for _, kind := range []string{"trigger", "connector", "context"} {
		src := strings.Replace(between("kind: code, op: bring-in, label: Keep the order number, cleans: true"),
			"kind: trigger, label: \"An order on the website's form\"", "kind: "+kind+", label: \"An order on the website's form\", in: bakery", 1)
		if kind != "trigger" {
			src = strings.Replace(src, "{ from: form, to: read, kind: start }", "{ from: form, to: read, kind: data }", 1)
		}
		mistakes(t, parsed(t, src))
	}
}

func TestOnlyATriggerConnectorOrContextIsMarkedUntrusted(t *testing.T) {
	src := strings.Replace(orderForm, `label: "Read the order", in: bakery }`, `label: "Read the order", in: bakery, untrusted: true }`, 1)
	mistakes(t, parsed(t, src), `part "read" is marked untrusted, but only a trigger, connector or context takes it`,
		`untrusted text from parts "form" and "read" reaches outside action "board" with nothing to stop it or accept the risk: `+untrustedFix)
	src = strings.Replace(orderForm, "untrusted: true", `untrusted: "yes"`, 1)
	mistakes(t, parsed(t, src), `part "form": untrusted must be true or false, not "yes"`)
}

func TestOnlyACodeStepCleans(t *testing.T) {
	// A step that can't clean lets the text through, so it reaches the label's printer too.
	boxReached := `untrusted text from part "form" reaches outside action "box" with nothing to stop it or accept the risk: ` + untrustedFix
	src := strings.Replace(orderForm, "kind: code, op: bring-in, label: \"Keep only", "kind: ai, op: bring-in, label: \"Keep only", 1)
	mistakes(t, parsed(t, src), `part "keep" cleans, but only a code step takes cleans: strangers' text can steer an AI step however it's written`, boardReached+": "+untrustedFix, boxReached)
	src = strings.Replace(orderForm, "label: Orders, in: bakery }", "label: Orders, in: bakery, cleans: true }", 1)
	mistakes(t, parsed(t, src), `part "orders" cleans, but only a code step takes cleans`, boardReached+": "+untrustedFix)
	src = strings.Replace(orderForm, "cleans: true", "cleans: 1", 1)
	mistakes(t, parsed(t, src), `part "keep": cleans must be true or false, not "1"`, boardReached+": "+untrustedFix, boxReached)
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
	doc := parsed(t, src)
	got := Summarise(doc).Untrusted[0]
	if want := []string{"Read the order", "Keep only the order number and quantity"}; !reflect.DeepEqual(got.Steps, want) {
		t.Errorf("steps = %q, want %q", got.Steps, want)
	}
	if got.OutsideActions != nil {
		t.Errorf("outside actions = %q", got.OutsideActions)
	}
	if lines := untrustedLines(t, GraphOf(doc)); lines["orders>plan"] != "" || lines["orders>tag"] != "" {
		t.Errorf("a refused read is drawn for untrusted text: %q", lines)
	}
}

// A line that brings untrusted text into an outside action, with nothing on the way to stop it or
// accept the risk, breaks a rule. It and every line on the text's way there are red, and so is the
// source's outline.
func TestUntrustedTextThatReachesAnOutsideActionUnstoppedIsAMistakeDrawnRed(t *testing.T) {
	doc := parsed(t, labelForm)
	breach := `untrusted text from part "form" reaches outside action "box" with nothing to stop it or accept the risk`
	mistakes(t, doc, breach+": "+untrustedFix)
	g := GraphOf(doc)
	want := map[string]string{"form>read": "unstopped", "read>tag": "unstopped", "tag>box": "unstopped"}
	if got := untrustedLines(t, g); !reflect.DeepEqual(got, want) {
		t.Errorf("lines are %q, want %q", got, want)
	}
	if got := unstoppedParts(g); !reflect.DeepEqual(got, []string{"form"}) {
		t.Errorf("unstopped parts are %q", got)
	}
	if got := g.Lines[2].Breaks; !reflect.DeepEqual(got, []string{breach}) {
		t.Errorf("the line into the action breaks %q", got)
	}
	for _, l := range g.Lines[:2] {
		if l.Breaks != nil {
			t.Errorf("a line on the way breaks %q", l.Breaks)
		}
	}
	out := Draw(doc) + "\n"
	has(t, out, "\n    form([\"An order on the website#39;s form\"])\n")
	has(t, out, "\n  tag -->|\"untrusted text from part #34;form#34; reaches outside action #34;box#34; with nothing to stop it or accept the risk\"| box\n")
	has(t, out, "\n  classDef untrusted "+untrustedOutline+"\n  classDef unstopped "+unstoppedOutline+"\n")
	has(t, out, "\n  class form trig\n")
	has(t, out, "\n  class form unstopped\n")
	has(t, out, "\n  linkStyle 0,1,2 "+red+"\n")
	for _, never := range []string{"⚠️", "🛡️", "taint", "class form untrusted"} {
		if strings.Contains(out, never) {
			t.Errorf("the drawing has %q:\n%s", never, out)
		}
	}
	report := CheckSources("dev", []Source{source("labels.yaml", labelForm)})
	if got := checkResult(t, report, untrustedRule); got.Result != "fail" || len(got.Mistakes) != 1 {
		t.Errorf("%s gave %s with %+v", untrustedRule, got.Result, got.Mistakes)
	}
	if got := Summarise(doc).RedLines; got != 1 {
		t.Errorf("red lines = %d, want 1", got)
	}
}

// A review, a person's own step or a code step marked cleans stops the text: each line it goes
// along is orange, up to and including the line into what stops it, the line out of that is green,
// and lines after it are drawn as usual.
func TestWhatStopsUntrustedTextEndsItsOrangeLinesWithAGreenOne(t *testing.T) {
	for _, part := range []string{
		"kind: code, op: bring-in, label: Keep the order number, cleans: true",
		"kind: review, label: \"Ana's yes to the order\"",
		"kind: person, op: check, label: Ana checks the order",
	} {
		t.Run(part, func(t *testing.T) {
			doc := parsed(t, between(part))
			mistakes(t, doc)
			g := GraphOf(doc)
			want := map[string]string{"form>read": "carries", "read>stop": "carries", "stop>tag": "stopped", "tag>box": ""}
			if got := untrustedLines(t, g); !reflect.DeepEqual(got, want) {
				t.Errorf("lines are %q, want %q", got, want)
			}
			if got := unstoppedParts(g); got != nil {
				t.Errorf("unstopped parts are %q", got)
			}
			out := Draw(doc) + "\n"
			has(t, out, "\n  class form untrusted\n")
			has(t, out, "\n  linkStyle 0,1 "+orange+"\n  linkStyle 2 "+green+"\n")
			if got := Summarise(doc).Untrusted[0].StoppedBy; len(got) != 1 {
				t.Errorf("stopped by = %q", got)
			}
		})
	}
}

// A standing approval doesn't stop the text, since it never sees it, but one in front of the action
// accepts the risk: its line into the action stays orange, the check passes, and the summary lists
// the risk accepted.
func TestAStandingApprovalInFrontOfTheActionAcceptsTheRisk(t *testing.T) {
	src := strings.Replace(labelForm, "Print the label: Allow", "Print the label: Ask", 1)
	src = strings.Replace(src, "  - { id: box,", "  - { id: yes, kind: approval, label: \"Ana's standing approval: labels only, risk accepted by Ana on last month's orders\", in: bakery }\n  - { id: box,", 1)
	src = strings.Replace(src, "  - { id: box, kind: outside, label: \"Print the label: Ask\", in: bakery }", "  - { id: box, kind: outside, label: \"Print the label: Ask\", in: bakery }\n  - { id: list, kind: surface, label: \"Ana's list\", in: bakery }", 1)
	src = strings.Replace(src, "  - { from: tag, to: box, kind: start }", "  - { from: tag, to: yes, kind: start }\n  - { from: yes, to: box, kind: start }\n  - { from: yes, to: list, kind: data }", 1)
	doc := parsed(t, src)
	mistakes(t, doc)
	g := GraphOf(doc)
	want := map[string]string{"form>read": "carries", "read>tag": "carries", "tag>yes": "carries", "yes>box": "carries", "yes>list": "carries"}
	if got := untrustedLines(t, g); !reflect.DeepEqual(got, want) {
		t.Errorf("lines are %q, want %q", got, want)
	}
	got := Summarise(doc).Untrusted[0]
	accepted := []AcceptedRisk{{OutsideAction: "Print the label: Ask", Approval: "Ana's standing approval: labels only, risk accepted by Ana on last month's orders"}}
	if !reflect.DeepEqual(got.RisksAccepted, accepted) || got.StoppedBy != nil {
		t.Errorf("risks accepted = %+v, stopped by %q", got.RisksAccepted, got.StoppedBy)
	}
	// A standing approval with anything else on the way accepts nothing for that way.
	src = strings.Replace(src, "  - { from: yes, to: box, kind: start }", "  - { from: yes, to: box, kind: start }\n  - { from: read, to: box, kind: data, label: \"read straight out\" }", 1)
	doc = parsed(t, src)
	if got := Check(doc); !reflect.DeepEqual(got[len(got)-1], `untrusted text from part "form" reaches outside action "box" with nothing to stop it or accept the risk: `+untrustedFix) {
		t.Errorf("Check gave %q", got)
	}
	if lines := untrustedLines(t, GraphOf(doc)); lines["yes>box"] != "carries" || lines["read>box"] != "unstopped" || lines["form>read"] != "unstopped" || lines["read>tag"] != "carries" {
		t.Errorf("lines are %q", lines)
	}
}

// A standing approval accepts the risk through a fork too, since a way to an action is followed
// back through any fork.
func TestAStandingApprovalAcceptsTheRiskThroughAFork(t *testing.T) {
	src := strings.Replace(labelForm, "Print the label: Allow", "Print the label: Ask", 1)
	src = strings.Replace(src, "  - { id: box,", "  - { id: yes, kind: approval, label: \"Ana's standing approval, risk accepted by Ana\", in: bakery }\n  - { id: both, kind: fork, in: bakery }\n  - { id: box,", 1)
	src = strings.Replace(src, "  - { id: box, kind: outside, label: \"Print the label: Ask\", in: bakery }", "  - { id: box, kind: outside, label: \"Print the label: Ask\", in: bakery }\n  - { id: list, kind: surface, label: \"Ana's list\", in: bakery }", 1)
	src = strings.Replace(src, "  - { from: tag, to: box, kind: start }", "  - { from: tag, to: yes, kind: start }\n  - { from: yes, to: both, kind: start }\n  - { from: both, to: box, kind: start }\n  - { from: yes, to: list, kind: data }", 1)
	mistakes(t, parsed(t, src))
}

// Where the text is stopped on one branch but reaches an action on another, red wins for the source
// and every line the branches share.
func TestRedWinsForTheLinesTheBranchesShare(t *testing.T) {
	src := strings.Replace(labelForm, "  - { id: tag,", "  - { id: keep, kind: code, op: bring-in, label: Keep the order number, in: bakery, cleans: true }\n  - { id: orders, kind: context, label: Orders, in: bakery }\n  - { id: tag,", 1)
	src = strings.Replace(src, "  - { from: tag, to: box, kind: start }", "  - { from: tag, to: box, kind: start }\n  - { from: read, to: keep, kind: start }\n  - { from: keep, to: orders, kind: data }", 1)
	g := GraphOf(parsed(t, src))
	want := map[string]string{"form>read": "unstopped", "read>tag": "unstopped", "tag>box": "unstopped", "read>keep": "carries", "keep>orders": "stopped"}
	if got := untrustedLines(t, g); !reflect.DeepEqual(got, want) {
		t.Errorf("lines are %q, want %q", got, want)
	}
	has(t, Draw(parsed(t, src))+"\n", "\n  linkStyle 0,1,2 "+red+"\n  linkStyle 3 "+orange+"\n  linkStyle 4 "+green+"\n")
}

func TestUntrustedTextGoesAlongARedLine(t *testing.T) {
	src := strings.Replace(orderForm, "{ from: orders, to: plan, kind: data }", "{ from: orders, to: plan, kind: data }\n  - { from: read, to: box, kind: start, label: \"printed by the live system\" }", 1)
	doc := parsed(t, src)
	got := Summarise(doc).Untrusted[0]
	if want := []string{"Pin the plan to the board: Allow", "Print the label: Allow"}; !reflect.DeepEqual(got.OutsideActions, want) {
		t.Errorf("outside actions = %q, want %q", got.OutsideActions, want)
	}
}

// Every view of a workflow draws untrusted text the same way, the platform view too.
func TestEveryViewDrawsAnUntrustedPartsOutlineAndItsLines(t *testing.T) {
	src := between("kind: code, op: bring-in, label: Keep the order number, cleans: true") +
		"implementations:\n  shop:\n    label: The shop\n    blocks:\n      - { id: till, label: The till, parts: [read, stop, tag, box] }\n"
	doc := parsed(t, src)
	out := Draw(mustView(t, doc, "shop")) + "\n"
	has(t, out, "\n  class form untrusted\n")
	has(t, out, "\n  linkStyle 0,1 "+orange+"\n  linkStyle 2 "+green+"\n")
	if strings.Contains(Draw(parsed(t, soundWorkflow)), "untrusted") {
		t.Error("a file with nothing untrusted draws an untrusted class")
	}
}

func TestCheckNoLongerWarnsOfUntrustedText(t *testing.T) {
	for _, warning := range Warnings(parsed(t, labelForm)) {
		t.Errorf("Warnings gave %q", warning)
	}
	report := CheckSources("dev", []Source{source("orders.yaml", orderForm)})
	for _, check := range report.Checks {
		if check.ID == "warn.untrusted-outside" {
			t.Error("the checks still list warn.untrusted-outside")
		}
	}
	for _, id := range []string{"untrusted.parts", "cleans.code-only"} {
		if got := checkResult(t, report, id); got.Result != "pass" {
			t.Errorf("%s gave %s", id, got.Result)
		}
	}
}

func TestAPageDrawsNoUntrustedView(t *testing.T) {
	doc := parsed(t, orderForm)
	html := Page([]*Map{doc})
	if got, want := drawings(t, html), []string{Draw(doc)}; !reflect.DeepEqual(got, want) {
		t.Errorf("drawings = %q", got)
	}
	if strings.Contains(html, "Untrusted view") {
		t.Error("the page still has an untrusted view")
	}
}

func TestTheUntrustedSummaryHasStableJSONFieldNames(t *testing.T) {
	got, err := json.Marshal(Summarise(parsed(t, orderForm)).Untrusted)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"part":"An order on the website's form","steps":["Read the order","Plan the baking","Note the plan","Keep only the order number and quantity"],"outsideActions":["Pin the plan to the board: Allow"],"stoppedBy":["Keep only the order number and quantity"],"risksAccepted":null}]`
	if string(got) != want {
		t.Errorf("JSON = %s\nwant %s", got, want)
	}
	accepted, err := json.Marshal(AcceptedRisk{OutsideAction: "a", Approval: "b"})
	if err != nil || string(accepted) != `{"outsideAction":"a","approval":"b"}` {
		t.Errorf("JSON = %s", accepted)
	}
}
