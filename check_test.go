package awml

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const good = `
awml: "0.6"
title: The family's dinners
scopes:
  - { id: carers, label: carers }
  - { id: family, label: family, in: carers }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
  - { id: mealplan, kind: context, label: Meal plan, in: family }
  - { id: split, kind: fork, in: family }
  - { id: note, kind: comment, label: "For example: scored" }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: mealplan, kind: data, label: posted }
  - { from: mealplan, to: split, kind: data }
  - { from: note, to: plan, kind: tie }
  - { from: note, to: thu, kind: hidden }
`

// gone, as an edit's value, removes the field.
type goneField struct{}

var gone = goneField{}

// changed is the good file with edits, given as pairs of a path such as "parts.1.kind" and a value.
func changed(t *testing.T, edits ...any) *Map {
	t.Helper()
	d := parsed(t, good)
	for i := 0; i < len(edits); i += 2 {
		keys := strings.Split(edits[i].(string), ".")
		var target any = d
		for _, k := range keys[:len(keys)-1] {
			target = step(t, target, k)
		}
		last, v := keys[len(keys)-1], edits[i+1]
		switch target := target.(type) {
		case *Map:
			if v == gone {
				delete(target.vals, last)
				for j, k := range target.keys {
					if k == last {
						target.keys = append(target.keys[:j:j], target.keys[j+1:]...)
						break
					}
				}
			} else {
				target.set(last, v)
			}
		case []any:
			n, _ := strconv.Atoi(last)
			target[n] = v
		}
	}
	return d
}

func step(t *testing.T, v any, k string) any {
	t.Helper()
	switch v := v.(type) {
	case *Map:
		return v.get(k)
	case []any:
		n, err := strconv.Atoi(k)
		if err != nil {
			t.Fatalf("%q isn't a position in a list", k)
		}
		return v[n]
	}
	t.Fatalf("can't step into %T", v)
	return nil
}

func mistakes(t *testing.T, doc any, want ...string) {
	t.Helper()
	got := Check(doc)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Check gave\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestASoundFileHasNoMistakes(t *testing.T) {
	mistakes(t, parsed(t, good))
}

func TestLabelsThatMarkdownWouldRecogniseHaveNoMistakes(t *testing.T) {
	mistakes(t, parsed(t, `
awml: "0.6"
title: Literal labels
scopes: [{id: group, label: '# Team'}]
parts:
  - {id: email, kind: trigger, label: noor@example.com, in: group}
  - {id: web, kind: code, label: 'https://example.org/a', in: group}
  - {id: list, kind: comment, label: '1. item - item', in: group}
  - {id: quote, kind: surface, label: '# heading > quote', in: group}
lines:
  - {from: email, to: web, kind: start, label: '**strong**'}
  - {from: list, to: quote, kind: tie}
`))
}

func TestAnUnknownPartKind(t *testing.T) {
	mistakes(t, changed(t, "parts.1.kind", "agent"), `part "plan" has unknown kind "agent"`)
}

func TestAnUnknownLineKind(t *testing.T) {
	mistakes(t, changed(t, "lines.0.kind", "dotted"), `the line from "thu" to "plan" has unknown kind "dotted"`)
}

func TestAWorkflowHasNoView(t *testing.T) {
	mistakes(t, changed(t, "view", "security"), `the file has unknown field "view"`)
}

func TestTheOldNamesForKindsAreUnknown(t *testing.T) {
	mistakes(t, changed(t, "parts.1.kind", "rule"), `part "plan" has unknown kind "rule"`)
	mistakes(t, changed(t, "parts.1.kind", "human"), `part "plan" has unknown kind "human"`)
}

func TestAnUnknownVersion(t *testing.T) {
	mistakes(t, changed(t, "awml", "2"), `awml must be "0.6", not "2"`)
}

func TestAPartInAnUnknownGroup(t *testing.T) {
	mistakes(t, changed(t, "parts.1.in", "house"), `part "plan" is in unknown group "house"`)
}

func TestAGroupInAnUnknownGroup(t *testing.T) {
	mistakes(t, changed(t, "scopes.1.in", "house"), `group "family" is in unknown group "house"`)
}

func TestALineToAnUnknownPart(t *testing.T) {
	mistakes(t, changed(t, "lines.1.to", "plann"), `the line from "plan" to "plann" ends at unknown part "plann"`)
}

func TestALineFromAnUnknownPart(t *testing.T) {
	mistakes(t, changed(t, "lines.0.from", "fri"), `the line from "fri" to "plan" starts at unknown part "fri"`)
}

func TestALineToAGroupIsToAnUnknownPart(t *testing.T) {
	mistakes(t, changed(t, "lines.0.to", "family"), `the line from "thu" to "family" ends at unknown part "family"`)
}

func TestACommentLinkedByAnythingButTie(t *testing.T) {
	for _, kind := range []string{"start", "data", "refused"} {
		t.Run(kind, func(t *testing.T) {
			mistakes(t, changed(t, "lines.3.kind", kind),
				fmt.Sprintf(`comment "note" is linked by a %s line: a comment takes only tie`, kind))
		})
	}
}

func TestATieWithNoComment(t *testing.T) {
	mistakes(t, changed(t, "lines.1.kind", "tie"), `the tie from "plan" to "mealplan" has no comment at either end`)
}

func TestAGroupNestedInItself(t *testing.T) {
	mistakes(t, changed(t, "scopes.0.in", "carers"), `group "carers" is nested in itself`)
}

func TestGroupsNestedInEachOther(t *testing.T) {
	mistakes(t, changed(t, "scopes.0.in", "family"), `group "carers" is nested in itself`, `group "family" is nested in itself`)
}

func TestAnIdUsedTwice(t *testing.T) {
	mistakes(t, changed(t, "parts.2.id", "plan", "lines.1.to", "plan", "lines.2.from", "plan"), `id "plan" is used twice`)
}

func TestAnIdMermaidCantRead(t *testing.T) {
	d := changed(t, "parts.1.id", "plan-it", "lines.0.to", "plan-it", "lines.1.from", "plan-it", "lines.3.to", "plan-it")
	mistakes(t, d, `id "plan-it" must be letters, digits and underscores, starting with a letter`)
}

func TestAMissingField(t *testing.T) {
	mistakes(t, changed(t, "parts.1.label", gone), `part "plan" has no label`)
	mistakes(t, changed(t, "lines.0.kind", gone), `the line from "thu" to "plan" has no kind`)
	mistakes(t, changed(t, "title", gone), "the file has no title")
}

func TestTheVersionIsText(t *testing.T) {
	mistakes(t, changed(t, "awml", 0.6), `awml: YAML read it as 0.6, not text: put it in quotes`)
}

func TestAFileInAVersionNoReleaseReadIsRefused(t *testing.T) {
	mistakes(t, changed(t, "awml", "0.1"), `awml must be "0.6", not "0.1"`)
}

func TestAnUnknownField(t *testing.T) {
	mistakes(t, changed(t, "lines.1.brakes", true), `the line from "plan" to "mealplan" has unknown field "brakes"`)
	mistakes(t, changed(t, "colour", "red"), `the file has unknown field "colour"`)
}

func TestNotAFileOfParts(t *testing.T) {
	mistakes(t, []any{"a", "b"}, "the file must be a mapping of fields, starting with awml")
	mistakes(t, nil, "the file must be a mapping of fields, starting with awml")
	mistakes(t, changed(t, "parts", "none"), "parts must be a list")
	mistakes(t, changed(t, "parts.0", "thu"), "each part must be a mapping of fields")
}

func TestEveryMistakeIsReportedAtOnce(t *testing.T) {
	mistakes(t, changed(t, "colour", "red", "lines.1.to", "plann"),
		`the file has unknown field "colour"`, `the line from "plan" to "plann" ends at unknown part "plann"`)
}

func TestAValueYAMLReadsAsTrueMustBeQuoted(t *testing.T) {
	d := parsed(t, `
awml: "0.6"
title: T
parts:
  - { id: true, kind: review, label: True }
  - { id: send, kind: outside, label: "Send: Ask" }
lines:
  - { from: true, to: send, kind: data }
`)
	mistakes(t, d,
		`part "true": YAML read its id as true, not text: put it in quotes`,
		`part "true": YAML read its label as true, not text: put it in quotes`,
		`the line from "true" to "send": YAML read its from as true, not text: put it in quotes`)
}

func TestANumberMustBeQuoted(t *testing.T) {
	d := parsed(t, `{awml: "0.6", title: T, parts: [{id: t, kind: trigger, label: 12}]}`)
	mistakes(t, d, `part "t": YAML read its label as 12, not text: put it in quotes`)
}

func TestYesNoOnOffAndTimesAreText(t *testing.T) {
	d := parsed(t, `
awml: "0.6"
title: T
parts:
  - { id: t, kind: trigger, label: 19:00 }
  - { id: yes, kind: review, label: yes }
  - { id: no, kind: code, label: no }
  - { id: on, kind: code, label: on }
  - { id: off, kind: code, label: off }
lines:
  - { from: t, to: yes, kind: start }
`)
	mistakes(t, d)
}

const reply = `
awml: "0.6"
title: A reply
parts:
  - { id: write, kind: ai, op: draft, label: Draft the reply }
  - { id: drafts, kind: context, label: Drafts }
  - { id: ask, kind: code, op: do, label: Ask for the send }
  - { id: split, kind: fork }
  - { id: approve, kind: review, label: Yes to the exact reply }
  - { id: send, kind: outside, label: "Send the reply: Ask" }
  - { id: file, kind: outside, label: "File the reply: Allow" }
lines:
  - { from: write, to: drafts, kind: data }
  - { from: drafts, to: ask, kind: data }
  - { from: ask, to: approve, kind: data }
  - { from: approve, to: send, kind: data, label: "[yes]" }
`

// withLines is the reply with more lines.
func withLines(t *testing.T, lines ...string) *Map {
	t.Helper()
	src := reply
	for _, l := range lines {
		src += "  - " + l + "\n"
	}
	return parsed(t, src)
}

func TestEachOperationNamesAStep(t *testing.T) {
	for _, op := range []string{"bring-in", "sort", "learn", "prepare", "check", "reflect", "draft", "do", "answer", "other"} {
		for _, kind := range []string{"ai", "code", "person"} {
			d := parsed(t, fmt.Sprintf(`{awml: "0.6", title: T, parts: [{id: s, kind: %s, op: %s, label: S}]}`, kind, op))
			mistakes(t, d)
		}
	}
}

func TestAnUnknownOperation(t *testing.T) {
	mistakes(t, changed(t, "parts.1.op", "write-up"), `part "plan" has unknown op "write-up"`)
}

func TestAnOperationMustBeText(t *testing.T) {
	mistakes(t, changed(t, "parts.1.op", true),
		`part "plan": YAML read its op as true, not text: put it in quotes`, `part "plan" has unknown op "true"`)
}

func TestOnlyAStepNamesAnOperation(t *testing.T) {
	mistakes(t, changed(t, "parts.2.op", "learn"), `part "mealplan" has an op, but only an ai, code or person step takes one`)
}

func TestADoStepLeadsToAnOutsideActionThroughAReview(t *testing.T) {
	mistakes(t, parsed(t, reply))
}

func TestOnlyADoStepLeadsToAnOutsideAction(t *testing.T) {
	want := `part "write" leads to outside action "send", but its op is draft: only a do step leads to an outside action`
	ask := `outside action "send" is Ask, but part "write" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`
	mistakes(t, withLines(t, "{ from: write, to: send, kind: data }"), want, ask)
	mistakes(t, withLines(t, "{ from: write, to: approve, kind: data }"), want)
	mistakes(t, withLines(t, "{ from: write, to: split, kind: data }", "{ from: split, to: send, kind: data }"), want, ask)
	mistakes(t, withLines(t, "{ from: write, to: split, kind: start }", "{ from: split, to: approve, kind: data }"), want)
}

func TestAStepIsReportedOnceForEachOutsideActionItLeadsTo(t *testing.T) {
	mistakes(t, withLines(t, "{ from: write, to: send, kind: data }", "{ from: write, to: approve, kind: data }",
		"{ from: write, to: file, kind: data }"),
		`part "write" leads to outside action "send", but its op is draft: only a do step leads to an outside action`,
		`part "write" leads to outside action "file", but its op is draft: only a do step leads to an outside action`,
		`outside action "send" is Ask, but part "write" leads to it with no person in between: put a review or a standing approval in front of it, or have a person's own do step take it`)
}

func TestAContextOrASurfaceBetweenStopsTheLead(t *testing.T) {
	mistakes(t, withLines(t, "{ from: drafts, to: approve, kind: data }"))
}

func TestAStepWithNoOperationIsHeldToNone(t *testing.T) {
	src := strings.Replace(reply, "op: draft, ", "", 1) + "  - { from: write, to: file, kind: data }\n"
	mistakes(t, parsed(t, src))
}

func TestAStepWhoseWorkIsOtherIsHeldToNoOperation(t *testing.T) {
	src := strings.Replace(reply, "op: draft, ", "op: other, ", 1) + "  - { from: write, to: file, kind: data }\n  - { from: write, to: drafts, kind: data }\n"
	mistakes(t, parsed(t, src))
	q := strings.Replace(question, "op: answer, ", "op: other, ", 1) + "  - { from: word, to: notes, kind: data }\n"
	mistakes(t, parsed(t, q))
}

func TestHiddenAndRefusedLinesLeadNowhere(t *testing.T) {
	mistakes(t, withLines(t, "{ from: write, to: send, kind: hidden }", "{ from: write, to: file, kind: refused }"))
}

const question = `
awml: "0.6"
title: A question
parts:
  - { id: notes, kind: context, label: Family notes }
  - { id: word, kind: ai, op: answer, label: Word the answer }
  - { id: match, kind: code, op: check, label: Match the bills }
  - { id: chat, kind: surface, label: Family chat }
lines:
  - { from: notes, to: word, kind: data }
  - { from: notes, to: match, kind: data }
  - { from: word, to: chat, kind: data }
  - { from: match, to: chat, kind: data }
`

func TestAnAnswerOrACheckReadsContextsAndSpeaksOnSurfaces(t *testing.T) {
	mistakes(t, parsed(t, question))
}

func TestAnAnswerOrACheckNeverWritesToAContext(t *testing.T) {
	d := parsed(t, question+"  - { from: word, to: notes, kind: data }\n  - { from: match, to: notes, kind: data }\n")
	mistakes(t, d,
		`part "word" writes to context "notes", but its op is answer: an answer step never writes to a context`,
		`part "match" writes to context "notes", but its op is check: a check step never writes to a context`)
	mistakes(t, parsed(t, question+"  - { from: word, to: notes, kind: refused }\n"))
}

func TestACheckPassesItsVerdictOnAndAStepAfterItKeepsIt(t *testing.T) {
	src := strings.Replace(question, "  - { id: chat,", "  - { id: mark, kind: code, op: do, label: Mark the bills checked }\n  - { id: chat,", 1) +
		"  - { from: match, to: mark, kind: start, label: \"[all paid]\" }\n  - { from: mark, to: notes, kind: data }\n"
	mistakes(t, parsed(t, src))
}

func TestOnlyATriggerContextConnectorOrSurfaceTakesARef(t *testing.T) {
	mistakes(t, changed(t, "parts.0.ref", "shopping.yaml#ready", "parts.2.ref", "acme/shopping#list"))
	mistakes(t, changed(t, "parts.1.ref", "shopping.yaml#plan"),
		`part "plan" has a ref, but only a trigger, context, connector or surface takes one`)
}

func TestARefIsWellWritten(t *testing.T) {
	for _, ref := range []string{"shopping.yaml#list", "../shopping/shopping.yaml#list", "acme/shopping#list", "my-co/weekly-shop#list_2"} {
		mistakes(t, changed(t, "parts.2.ref", ref))
	}
	for _, ref := range []string{"list", "shopping.yaml", "/shopping.yaml#list", "shopping.yml#list", "Acme/shopping#list", "acme/shopping#2nd", "acme/shopping/more#list", "./shopping.yaml#list"} {
		mistakes(t, changed(t, "parts.2.ref", ref),
			fmt.Sprintf(`part "mealplan": ref must be owner/slug#id, or a path to a file alongside such as shopping.yaml#list, not "%s"`, ref))
	}
	mistakes(t, changed(t, "parts.2.ref", 3.0),
		`part "mealplan": YAML read its ref as 3, not text: put it in quotes`,
		`part "mealplan": ref must be owner/slug#id, or a path to a file alongside such as shopping.yaml#list, not "3"`)
}

func TestAWorkflowMayHaveADescription(t *testing.T) {
	mistakes(t, changed(t, "description", "The family's *dinners*, planned each Thursday.\n\nThe family is fictional.\n"))
	mistakes(t, changed(t, "description", true), `the file: YAML read its description as true, not text: put it in quotes`)
}
