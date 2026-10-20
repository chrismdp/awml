package awml

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const good = `
awml: "0.1"
title: The family's dinners
view: scopes
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

func TestAnUnknownPartKind(t *testing.T) {
	mistakes(t, changed(t, "parts.1.kind", "agent"), `part "plan" has unknown kind "agent"`)
}

func TestAnUnknownLineKind(t *testing.T) {
	mistakes(t, changed(t, "lines.0.kind", "dotted"), `the line from "thu" to "plan" has unknown kind "dotted"`)
}

func TestAnUnknownView(t *testing.T) {
	mistakes(t, changed(t, "view", "who"), `view must be "scopes" or "where", not "who"`)
}

func TestAnUnknownVersion(t *testing.T) {
	mistakes(t, changed(t, "awml", "2"), `awml must be "0.1", not "2"`)
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
	mistakes(t, changed(t, "awml", 0.1))
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
		`awml must be "0.1", not "2"`, `the line from "plan" to "plann" ends at unknown part "plann"`)
}

func TestAValueYAMLReadsAsTrueMustBeQuoted(t *testing.T) {
	d := parsed(t, `
awml: "0.1"
title: T
view: scopes
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
	d := parsed(t, `{awml: "0.1", title: T, view: scopes, parts: [{id: t, kind: trigger, label: 12}]}`)
	mistakes(t, d, `part "t": YAML read its label as 12, not text: put it in quotes`)
}

func TestYesNoOnOffAndTimesAreText(t *testing.T) {
	d := parsed(t, `
awml: "0.1"
title: T
view: scopes
parts:
  - { id: t, kind: trigger, label: 19:00 }
  - { id: yes, kind: review, label: yes }
  - { id: no, kind: rule, label: no }
  - { id: on, kind: rule, label: on }
  - { id: off, kind: rule, label: off }
lines:
  - { from: t, to: yes, kind: start }
`)
	mistakes(t, d)
}

func TestBreaksIsTrueOrFalse(t *testing.T) {
	mistakes(t, changed(t, "lines.1.breaks", false))
	mistakes(t, changed(t, "lines.1.breaks", "no"), `the line from "plan" to "mealplan": breaks must be true or false, not "no"`)
}
