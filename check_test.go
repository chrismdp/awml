package awml

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const good = `
awml: "0.2"
title: The family's dinners
view: security
groups:
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
  - { from: plan, to: mealplan, kind: data, label: posted, breaks: true }
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
awml: "0.2"
title: Literal labels
view: platform
groups: [{id: group, label: '# Team'}]
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

func TestAnUnknownView(t *testing.T) {
	mistakes(t, changed(t, "view", "who"), `view must be "security" or "platform", not "who"`)
}

func TestTheOldNamesForViewsAndKindsAreUnknown(t *testing.T) {
	mistakes(t, changed(t, "view", "scopes"), `view must be "security" or "platform", not "scopes"`)
	mistakes(t, changed(t, "view", "where"), `view must be "security" or "platform", not "where"`)
	mistakes(t, changed(t, "parts.1.kind", "rule"), `part "plan" has unknown kind "rule"`)
	mistakes(t, changed(t, "parts.1.kind", "human"), `part "plan" has unknown kind "human"`)
}

func TestAnUnknownVersion(t *testing.T) {
	mistakes(t, changed(t, "awml", "2"), `awml must be "0.2" or "0.3", not "2"`)
}

func TestAPartInAnUnknownGroup(t *testing.T) {
	mistakes(t, changed(t, "parts.1.in", "house"), `part "plan" is in unknown group "house"`)
}

func TestAGroupInAnUnknownGroup(t *testing.T) {
	mistakes(t, changed(t, "groups.1.in", "house"), `group "family" is in unknown group "house"`)
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
	for _, kind := range []string{"start", "data", "publish", "refused"} {
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
	mistakes(t, changed(t, "groups.0.in", "carers"), `group "carers" is nested in itself`)
}

func TestGroupsNestedInEachOther(t *testing.T) {
	mistakes(t, changed(t, "groups.0.in", "family"), `group "carers" is nested in itself`, `group "family" is nested in itself`)
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

func TestTheVersionMayBeWrittenAsANumber(t *testing.T) {
	mistakes(t, changed(t, "awml", 0.2))
}

func TestAFileInTheOldVersionIsRefused(t *testing.T) {
	mistakes(t, changed(t, "awml", "0.1"), `awml must be "0.2" or "0.3", not "0.1"`)
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
	mistakes(t, changed(t, "awml", "2", "lines.1.to", "plann"),
		`awml must be "0.2" or "0.3", not "2"`, `the line from "plan" to "plann" ends at unknown part "plann"`)
}

func TestAValueYAMLReadsAsTrueMustBeQuoted(t *testing.T) {
	d := parsed(t, `
awml: "0.2"
title: T
view: security
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
	d := parsed(t, `{awml: "0.2", title: T, view: security, parts: [{id: t, kind: trigger, label: 12}]}`)
	mistakes(t, d, `part "t": YAML read its label as 12, not text: put it in quotes`)
}

func TestYesNoOnOffAndTimesAreText(t *testing.T) {
	d := parsed(t, `
awml: "0.2"
title: T
view: security
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

func TestBreaksIsTrueOrFalse(t *testing.T) {
	mistakes(t, changed(t, "lines.1.breaks", false))
	mistakes(t, changed(t, "lines.1.breaks", "no"), `the line from "plan" to "mealplan": breaks must be true or false, not "no"`)
}

const reply = `
awml: "0.2"
title: A reply
view: security
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
			d := parsed(t, fmt.Sprintf(`{awml: "0.2", title: T, view: security, parts: [{id: s, kind: %s, op: %s, label: S}]}`, kind, op))
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
	mistakes(t, withLines(t, "{ from: write, to: send, kind: data }"), want)
	mistakes(t, withLines(t, "{ from: write, to: approve, kind: data }"), want)
	mistakes(t, withLines(t, "{ from: write, to: split, kind: data }", "{ from: split, to: send, kind: data }"), want)
	mistakes(t, withLines(t, "{ from: write, to: split, kind: start }", "{ from: split, to: approve, kind: data }"), want)
}

func TestAStepIsReportedOnceForEachOutsideActionItLeadsTo(t *testing.T) {
	mistakes(t, withLines(t, "{ from: write, to: send, kind: data }", "{ from: write, to: approve, kind: data }",
		"{ from: write, to: file, kind: data }"),
		`part "write" leads to outside action "send", but its op is draft: only a do step leads to an outside action`,
		`part "write" leads to outside action "file", but its op is draft: only a do step leads to an outside action`)
}

func TestAContextOrASurfaceBetweenStopsTheLead(t *testing.T) {
	mistakes(t, withLines(t, "{ from: drafts, to: approve, kind: data }"))
}

func TestAStepWithNoOperationIsHeldToNone(t *testing.T) {
	src := strings.Replace(reply, "op: draft, ", "", 1) + "  - { from: write, to: send, kind: data }\n"
	mistakes(t, parsed(t, src))
}

func TestAStepWhoseWorkIsOtherIsHeldToNoOperation(t *testing.T) {
	src := strings.Replace(reply, "op: draft, ", "op: other, ", 1) + "  - { from: write, to: send, kind: data }\n  - { from: write, to: drafts, kind: data }\n"
	mistakes(t, parsed(t, src))
	q := strings.Replace(question, "op: answer, ", "op: other, ", 1) + "  - { from: word, to: notes, kind: data }\n"
	mistakes(t, parsed(t, q))
}

func TestHiddenAndRefusedLinesLeadNowhere(t *testing.T) {
	mistakes(t, withLines(t, "{ from: write, to: send, kind: hidden }", "{ from: write, to: file, kind: refused }"))
}

func TestARedLineIsHeldToNoOperation(t *testing.T) {
	mistakes(t, withLines(t, "{ from: write, to: send, kind: data, breaks: true }"))
}

const question = `
awml: "0.2"
title: A question
view: security
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
	d := parsed(t, question+"  - { from: word, to: notes, kind: data }\n  - { from: match, to: notes, kind: publish }\n")
	mistakes(t, d,
		`part "word" writes to context "notes", but its op is answer: an answer step never writes to a context`,
		`part "match" writes to context "notes", but its op is check: a check step never writes to a context`)
	mistakes(t, parsed(t, question+"  - { from: word, to: notes, kind: data, breaks: true }\n"))
	mistakes(t, parsed(t, question+"  - { from: word, to: notes, kind: refused }\n"))
}

const scoped = `
awml: "0.2"
title: The family's dinners
view: security
groups:
  - { id: carers, label: "carers: + June, Beth" }
  - { id: family, label: "family: parents, kids", in: carers }
  - { id: household, label: "household: parents", in: family }
  - { id: club, label: "club: the kids, their coach" }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
  - { id: mealplan, kind: context, label: Meal plan, in: family }
  - { id: screen, kind: surface, label: Kitchen screen, in: carers }
  - { id: budget, kind: context, label: The food budget, in: household }
  - { id: fixtures, kind: context, label: The club's fixtures, in: club }
  - { id: site, kind: surface, label: The family's public page }
  - { id: note, kind: comment, label: "For example: last week's plan", in: family }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: mealplan, kind: data }
`

// withScopedLines is the scoped file with more lines.
func withScopedLines(t *testing.T, lines ...string) *Map {
	t.Helper()
	src := scoped
	for _, l := range lines {
		src += "  - " + l + "\n"
	}
	return parsed(t, src)
}

func TestADataLineWithinOneBoxIsRouting(t *testing.T) {
	mistakes(t, parsed(t, scoped))
}

func TestADataLineIntoABoxInsideItsOwnIsRouting(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: mealplan, to: budget, kind: data }", "{ from: screen, to: budget, kind: data }"))
}

func TestADataLineIntoAWiderBoxIsPublishing(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: mealplan, to: screen, kind: data }"),
		`the line from "mealplan" to "screen" moves content from group "family" into group "carers", which isn't inside it: draw it as publish, or as refused if the design refuses the read`)
}

func TestADataLineFromANarrowerBoxIsPublishing(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: budget, to: plan, kind: data }"),
		`the line from "budget" to "plan" moves content from group "household" into group "family", which isn't inside it: draw it as publish, or as refused if the design refuses the read`)
}

func TestADataLineIntoABoxSideBySideIsPublishing(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: fixtures, to: plan, kind: data }", "{ from: mealplan, to: fixtures, kind: data }"),
		`the line from "fixtures" to "plan" moves content from group "club" into group "family", which isn't inside it: draw it as publish, or as refused if the design refuses the read`,
		`the line from "mealplan" to "fixtures" moves content from group "family" into group "club", which isn't inside it: draw it as publish, or as refused if the design refuses the read`)
}

func TestOnlyADataLineIsHeldToItsScopes(t *testing.T) {
	for _, kind := range []string{"start", "publish", "refused", "hidden"} {
		t.Run(kind, func(t *testing.T) {
			mistakes(t, withScopedLines(t, fmt.Sprintf("{ from: mealplan, to: screen, kind: %s }", kind)))
		})
	}
	mistakes(t, withScopedLines(t, "{ from: note, to: screen, kind: tie }"))
}

func TestARedLineIsHeldToNoScope(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: mealplan, to: screen, kind: data, breaks: true }"))
}

func TestThePlatformViewIsHeldToNoScope(t *testing.T) {
	src := strings.Replace(scoped, "view: security", "view: platform", 1) + "  - { from: mealplan, to: screen, kind: data }\n"
	mistakes(t, parsed(t, src))
}

func TestATriggerWithNoGroupIsInTheBoxOfWhatItStarts(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: thu, to: budget, kind: data }"))
	mistakes(t, withScopedLines(t, "{ from: thu, to: screen, kind: data }"),
		`the line from "thu" to "screen" moves content from group "family" into group "carers", which isn't inside it: draw it as publish, or as refused if the design refuses the read`)
}

func TestThePageOutsideEveryBoxIsWiderThanAnyOfThem(t *testing.T) {
	mistakes(t, withScopedLines(t, "{ from: site, to: mealplan, kind: data }", "{ from: site, to: budget, kind: data }"))
	mistakes(t, withScopedLines(t, "{ from: mealplan, to: site, kind: data }"),
		`the line from "mealplan" to "site" moves content from group "family" to the top level, outside every group: draw it as publish, or as refused if the design refuses the read`)
}
