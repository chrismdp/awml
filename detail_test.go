package awml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// keep is a step whose label is its title, with a description for its detail.
const keep = currentAwml + `
title: Builds
scopes:
  - { id: public, label: "public: anyone", processing: any }
parts:
  - { id: label, kind: trigger, label: "The build label is added" }
  - { id: keep, kind: code, op: bring-in, label: "Keep only what's approved", in: public, description: "writers' comments by collaborator permission; a rebuild reads the referral beside its answer" }
  - { id: asked, kind: context, label: "What was asked", in: public, description: "from GitHub's record" }
  - { id: note, kind: comment, label: "For example: an issue's text" }
lines:
  - { from: label, to: keep, kind: start }
  - { from: keep, to: asked, kind: data }
  - { from: note, to: asked, kind: tie }
`

func TestAPartMayHaveADescription(t *testing.T) {
	mistakes(t, parsed(t, keep))
	warnings(t, parsed(t, keep))
}

// Marshal quotes each detail, as it quotes a part's label, so a comma or a colon in it stays in it.
func TestMarshalQuotesEachDetail(t *testing.T) {
	src := strings.Replace(keep, `description: "writers' comments by collaborator permission; a rebuild reads the referral beside its answer"`, `description: "by permission, then: by date"`, 1)
	got, err := Marshal(parsed(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Errorf("Marshal gave\n%s\nwant\n%s", got, src)
	}
}

func TestDescriptionIsText(t *testing.T) {
	for _, value := range []any{true, 12, false} {
		mistakes(t, changed(t, "parts.1.description", value),
			`part "plan": YAML read its description as `+show(value)+`, not text: put it in quotes`)
	}
}

func TestOldDetailFieldsAreUnknown(t *testing.T) {
	for _, field := range []string{"how", "why", "decided"} {
		mistakes(t, changed(t, "parts.1."+field, "a rule"),
			`part "plan" has unknown field "`+field+`"`)
	}
}

// A comment says all it has to say in its label.
func TestACommentHasNoDescription(t *testing.T) {
	mistakes(t, changed(t, "parts.4.description", "it shows the score"),
		`part "note" has a description, but a comment takes no description: its label says it all`)
}

func TestALineHasNoDescription(t *testing.T) {
	mistakes(t, changed(t, "lines.1.description", "posted"), `the line from "plan" to "mealplan" has unknown field "description"`)
}

// A label is a short title, so a label of more than 12 words warns, and a comment's never does.
func TestALongLabelWarns(t *testing.T) {
	twelve := "Keep only the cake, the quantity and the date from the form"
	warnings(t, changed(t, "parts.1.label", twelve))
	thirteen := twelve + " today"
	warnings(t, changed(t, "parts.1.label", thirteen),
		`part "plan" has a label of 13 words: make it a short title, and move the rest into description`)
	warnings(t, changed(t, "parts.4.label", "For example: "+thirteen))
}

func TestTheGraphGivesEachFieldApart(t *testing.T) {
	g := graphOf(t, keep, "")
	encoded, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"id":"keep","kind":"code","label":"Keep only what's approved","in":"public","decider":"code","op":"bring-in","description":"writers' comments by collaborator permission; a rebuild reads the referral beside its answer"}`,
		`{"id":"asked","kind":"context","label":"What was asked","in":"public","description":"from GitHub's record"}`,
		`{"id":"label","kind":"trigger","label":"The build label is added","in":"public"}`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the JSON has no %s in\n%s", want, encoded)
		}
	}
}

// awml draw draws titles alone, and with --detail a note tied to each part that has a detail: its
// title in bold, then its description.
func TestDrawShowsTheDetailOnlyWhenAsked(t *testing.T) {
	source := []Source{{File: "keep.yaml", Text: []byte(keep)}}
	plain, err := DrawSources(source, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "collaborator") || strings.Contains(plain, "_detail") {
		t.Errorf("the drawing shows the detail:\n%s", plain)
	}
	detailed, err := DrawSources(source, "", true)
	if err != nil {
		t.Fatal(err)
	}
	has(t, detailed, "\n    keep_detail@{ shape: notch-rect, label: \"<b>Keep only what#39;s approved</b><br>writers#39; comments by collaborator permission#59; a rebuild reads the referral beside its answer\" }\n")
	has(t, detailed, "\n    asked_detail@{ shape: notch-rect, label: \"<b>What was asked</b><br>from GitHub#39;s record\" }\n")
	has(t, detailed, "\n  keep -.- keep_detail\n  asked -.- asked_detail\n")
	has(t, detailed, "  class note,keep_detail,asked_detail gap\n")
	has(t, detailed, "  linkStyle 2,3,4 stroke:#8a93a3")
	// Each note comes after the comments, and each tie after the lines, so the rest of the drawing
	// is as it was.
	if without := strings.NewReplacer(
		"    keep_detail@{ shape: notch-rect, label: \"<b>Keep only what#39;s approved</b><br>writers#39; comments by collaborator permission#59; a rebuild reads the referral beside its answer\" }\n", "",
		"    asked_detail@{ shape: notch-rect, label: \"<b>What was asked</b><br>from GitHub#39;s record\" }\n", "",
		"  keep -.- keep_detail\n  asked -.- asked_detail\n", "",
		"note,keep_detail,asked_detail", "note",
		"linkStyle 2,3,4", "linkStyle 2").Replace(detailed); without != plain {
		t.Errorf("--detail changed more than its notes:\n%s\nwant\n%s", without, plain)
	}
}

// A note's id never takes a part's.
func TestADetailNoteTakesAnIdNoPartHas(t *testing.T) {
	src := strings.Replace(keep, "{ id: note,", "{ id: keep_detail,", 1)
	src = strings.Replace(src, "{ from: note,", "{ from: keep_detail,", 1)
	detailed, err := DrawSources([]Source{{File: "keep.yaml", Text: []byte(src)}}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	has(t, detailed, "\n  keep -.- keep_detail_detail\n")
}

func TestSummaryDoesNotListDescriptions(t *testing.T) {
	with := parsed(t, keep)
	without := parsed(t, keep)
	for _, p := range items(without, "parts") {
		p.set("description", "")
	}
	if !reflect.DeepEqual(Summarise(with), Summarise(without)) {
		t.Error("a description changes the summary")
	}
}

// A note's id also skips ids taken by earlier notes.
func TestDetailNotesHaveDistinctIds(t *testing.T) {
	src := currentAwml + `
title: Notes
parts:
  - { id: a, kind: trigger, label: First, description: "The first part" }
  - { id: a_detail, kind: code, op: do, label: Second, description: "The second part" }
lines:
  - { from: a, to: a_detail, kind: start }
`
	detailed, err := DrawSources([]Source{{Text: []byte(src)}}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	has(t, detailed, "\n  a -.- a_detail_detail\n")
	has(t, detailed, "\n  a_detail -.- a_detail_detail_detail\n")
	for _, id := range []string{"a_detail_detail", "a_detail_detail_detail"} {
		if strings.Count(detailed, id+"@{") != 1 {
			t.Errorf("note %s is not drawn once:\n%s", id, detailed)
		}
	}
}

// On a map, a shared part has the detail of the copy every ref leads back to, its owner's, as it has
// that copy's title, and a note only where that copy has a detail.
func TestAMapGivesASharedPartItsOwnersDetail(t *testing.T) {
	owner := strings.Replace(keep, `in: public, description: "from GitHub's record" }`, `in: public }`, 1)
	copied := currentAwml + `
title: Reads
scopes:
  - { id: public, label: "public: anyone", processing: any }
parts:
  - { id: daily, kind: trigger, label: "Every day" }
  - { id: read, kind: ai, op: learn, label: "Read it", in: public }
  - { id: asked, kind: context, label: "What was asked", in: public, ref: builds.yaml#asked, description: "The copy's explanation" }
lines:
  - { from: daily, to: read, kind: start }
  - { from: asked, to: read, kind: data }
`
	sources := func(owner string) []Source {
		return []Source{{File: "builds.yaml", Text: []byte(owner)}, {File: "reads.yaml", Text: []byte(copied)}}
	}
	detailed, err := DrawSources(sources(owner), "", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(detailed, "s1_detail") {
		t.Errorf("the map draws the copy's detail for the shared part:\n%s", detailed)
	}
	withDescription := strings.Replace(owner, `label: "What was asked", in: public }`, `label: "What was asked", in: public, description: "from GitHub's record" }`, 1)
	g, err := GraphSources(sources(withDescription), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.Parts {
		if p.ID == "s1" {
			encoded, _ := json.Marshal(p)
			if !strings.Contains(string(encoded), `"description":"from GitHub's record"`) || strings.Contains(string(encoded), "copy's explanation") {
				t.Errorf("the shared part does not have its owner's description: %s", encoded)
			}
			detailed, err := DrawSources(sources(withDescription), "", true)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(detailed, "s1_detail@{") != 1 {
				t.Errorf("the shared part's note is not drawn once:\n%s", detailed)
			}
			return
		}
	}
	t.Errorf("the map has no shared part s1 in %#v", g.Parts)
}

// Shared copies can omit or change their description without changing their agreement.
func TestCopiesAgreeWhateverTheirDescription(t *testing.T) {
	for _, description := range []string{"", `, description: "from GitHub's record"`, `, description: "A different explanation"`} {
		copied := strings.Replace(keep, `in: public, description: "from GitHub's record" }`, `in: public, ref: builds.yaml#asked`+description+` }`, 1)
		workflows := together(t, "builds.yaml", keep, "reads.yaml", copied)
		if got := TogetherMistakes(workflows); !reflect.DeepEqual(got, [][]string{nil, nil}) {
			t.Errorf("together they have mistakes %q", got)
		}
		sources := []Source{{File: "builds.yaml", Text: []byte(keep)}, {File: "reads.yaml", Text: []byte(copied)}}
		detailed, err := DrawSources(sources, "", true)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(detailed, "s1_detail@{") != 1 {
			t.Errorf("the shared part's note is not drawn once:\n%s", detailed)
		}
	}
}
