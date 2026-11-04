package awml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// keep is a step whose label is its title, with how, why and decided as its detail.
const keep = `awml: "0.11"
title: Builds
scopes:
  - { id: public, label: "public: anyone", processing: any }
parts:
  - { id: label, kind: trigger, label: "The build label is added" }
  - { id: keep, kind: code, op: bring-in, label: "Keep only what's approved", in: public, how: "writers' comments by collaborator permission", why: "a rebuild reads the referral beside its answer", decided: "The maintainer, in #160 under rule 4" }
  - { id: asked, kind: context, label: "What was asked", in: public, how: "from GitHub's record" }
  - { id: note, kind: comment, label: "For example: an issue's text" }
lines:
  - { from: label, to: keep, kind: start }
  - { from: keep, to: asked, kind: data }
  - { from: note, to: asked, kind: tie }
`

func TestAPartMayHaveHowWhyAndDecided(t *testing.T) {
	mistakes(t, parsed(t, keep))
	warnings(t, parsed(t, keep))
}

// Marshal quotes each detail, as it quotes a part's label, so a comma or a colon in it stays in it.
func TestMarshalQuotesEachDetail(t *testing.T) {
	src := strings.Replace(keep, `how: "writers' comments by collaborator permission"`, `how: "by permission, then: by date"`, 1)
	got, err := Marshal(parsed(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Errorf("Marshal gave\n%s\nwant\n%s", got, src)
	}
}

func TestHowWhyAndDecidedAreText(t *testing.T) {
	mistakes(t, changed(t, "parts.1.how", true, "parts.1.why", 12, "parts.1.decided", false),
		`part "plan": YAML read its how as true, not text: put it in quotes`,
		`part "plan": YAML read its why as 12, not text: put it in quotes`,
		`part "plan": YAML read its decided as false, not text: put it in quotes`)
}

// A comment says all it has to say in its label, so it has no detail of its own.
func TestACommentHasNoHowWhyOrDecided(t *testing.T) {
	mistakes(t, changed(t, "parts.4.why", "it shows the score"),
		`part "note" has a why, but a comment takes no how, why or decided: its label says it all`)
}

func TestALineHasNoHowWhyOrDecided(t *testing.T) {
	mistakes(t, changed(t, "lines.1.why", "posted"), `the line from "plan" to "mealplan" has unknown field "why"`)
}

// A label is a short title, so a label of more than 12 words warns, and a comment's never does.
func TestALongLabelWarns(t *testing.T) {
	twelve := "Keep only the cake, the quantity and the date from the form"
	warnings(t, changed(t, "parts.1.label", twelve))
	thirteen := twelve + " today"
	warnings(t, changed(t, "parts.1.label", thirteen),
		`part "plan" has a label of 13 words: make it a short title, and move the rest into how, why or decided`)
	warnings(t, changed(t, "parts.4.label", "For example: "+thirteen))
}

func TestTheGraphGivesEachFieldApart(t *testing.T) {
	g := graphOf(t, keep, "")
	encoded, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"id":"keep","kind":"code","label":"Keep only what's approved","in":"public","decider":"code","op":"bring-in","how":"writers' comments by collaborator permission","why":"a rebuild reads the referral beside its answer","decided":"The maintainer, in #160 under rule 4"}`,
		`{"id":"asked","kind":"context","label":"What was asked","in":"public","how":"from GitHub's record"}`,
		`{"id":"label","kind":"trigger","label":"The build label is added","in":"public"}`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the JSON has no %s in\n%s", want, encoded)
		}
	}
}

// awml draw draws titles alone, and with --detail a note tied to each part that has a detail: its
// title in bold, then a line each for how, why and decided.
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
	has(t, detailed, "\n    keep_detail@{ shape: notch-rect, label: \"<b>Keep only what#39;s approved</b><br>how: writers#39; comments by collaborator permission<br>why: a rebuild reads the referral beside its answer<br>decided: The maintainer#44; in #35;160 under rule 4\" }\n")
	has(t, detailed, "\n    asked_detail@{ shape: notch-rect, label: \"<b>What was asked</b><br>how: from GitHub#39;s record\" }\n")
	has(t, detailed, "\n  keep -.- keep_detail\n  asked -.- asked_detail\n")
	has(t, detailed, "  class note,keep_detail,asked_detail gap\n")
	has(t, detailed, "  linkStyle 2,3,4 stroke:#8a93a3")
	// Each note comes after the comments, and each tie after the lines, so the rest of the drawing
	// is as it was.
	if without := strings.NewReplacer(
		"    keep_detail@{ shape: notch-rect, label: \"<b>Keep only what#39;s approved</b><br>how: writers#39; comments by collaborator permission<br>why: a rebuild reads the referral beside its answer<br>decided: The maintainer#44; in #35;160 under rule 4\" }\n", "",
		"    asked_detail@{ shape: notch-rect, label: \"<b>What was asked</b><br>how: from GitHub#39;s record\" }\n", "",
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

func TestSummaryListsEachDecidedWithItsTitle(t *testing.T) {
	got := Summarise(parsed(t, keep)).Decided
	if want := []DecidedSummary{{Label: "Keep only what's approved", Decided: "The maintainer, in #160 under rule 4"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("decided = %#v\nwant %#v", got, want)
	}
	if got := Summarise(parsed(t, good)).Decided; got != nil {
		t.Errorf("a workflow with nothing decided lists %#v", got)
	}
}

// A copy of a shared part needn't carry its detail, and where both do, the summary lists it once.
func TestCopiesAgreeWhateverTheirDetail(t *testing.T) {
	owner := strings.Replace(keep, `in: public, how: "from GitHub's record" }`, `in: public, how: "from GitHub's record", decided: "The maintainer" }`, 1)
	copied := `awml: "0.11"
title: Reads
scopes:
  - { id: public, label: "public: anyone", processing: any }
parts:
  - { id: daily, kind: trigger, label: "Every day" }
  - { id: read, kind: ai, op: learn, label: "Read it", in: public }
  - { id: asked, kind: context, label: "What was asked", in: public, ref: builds.yaml#asked, why: "it's the issue as approved", decided: "The maintainer" }
lines:
  - { from: daily, to: read, kind: start }
  - { from: asked, to: read, kind: data }
`
	workflows := together(t, "builds.yaml", owner, "reads.yaml", copied)
	if got := TogetherMistakes(workflows); !reflect.DeepEqual(got, [][]string{nil, nil}) {
		t.Errorf("together they have mistakes %q", got)
	}
	got := SummariseTogether(workflows).Decided
	want := []DecidedSummary{{Label: "Keep only what's approved", Decided: "The maintainer, in #160 under rule 4"}, {Label: "What was asked", Decided: "The maintainer"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decided = %#v\nwant %#v", got, want)
	}
}

// On a map, a shared part has the detail of the copy every ref leads back to, its owner's, as it has
// that copy's title, and a note only where that copy has a detail.
func TestAMapGivesASharedPartItsOwnersDetail(t *testing.T) {
	owner := strings.Replace(keep, `in: public, how: "from GitHub's record" }`, `in: public }`, 1)
	copied := `awml: "0.11"
title: Reads
scopes:
  - { id: public, label: "public: anyone", processing: any }
parts:
  - { id: daily, kind: trigger, label: "Every day" }
  - { id: read, kind: ai, op: learn, label: "Read it", in: public }
  - { id: asked, kind: context, label: "What was asked", in: public, ref: builds.yaml#asked, decided: "The maintainer" }
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
	withHow := strings.Replace(owner, `label: "What was asked", in: public }`, `label: "What was asked", in: public, how: "from GitHub's record" }`, 1)
	g, err := GraphSources(sources(withHow), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.Parts {
		if p.ID == "s1" {
			if p.How != "from GitHub's record" || p.Decided != "" {
				t.Errorf("the shared part has how %q and decided %q, want only its owner's how", p.How, p.Decided)
			}
			return
		}
	}
	t.Errorf("the map has no shared part s1 in %#v", g.Parts)
}

// Only a shared part's copies are listed once: two parts that aren't shared are each listed, however
// alike their title and decided read.
func TestPartsNotSharedAreEachListedThoughTheyReadAlike(t *testing.T) {
	twice := strings.Replace(keep, "  - { id: asked,", `  - { id: again, kind: code, op: bring-in, label: "Keep only what's approved", in: public, decided: "The maintainer, in #160 under rule 4" }
  - { id: asked,`, 1)
	twice = strings.Replace(twice, "  - { from: keep, to: asked, kind: data }", "  - { from: keep, to: asked, kind: data }\n  - { from: label, to: again, kind: start }", 1)
	alone := Summarise(parsed(t, twice)).Decided
	if len(alone) != 2 {
		t.Fatalf("alone, decided = %#v, want both parts", alone)
	}
	other := strings.Replace(keep, "title: Builds", "title: Rebuilds", 1)
	for _, workflows := range [][]Workflow{
		together(t, "builds.yaml", twice, "reads.yaml", good),
		together(t, "builds.yaml", keep, "rebuilds.yaml", other),
	} {
		if got := SummariseTogether(workflows).Decided; len(got) != 2 {
			t.Errorf("together, decided = %#v, want both parts", got)
		}
	}
}
