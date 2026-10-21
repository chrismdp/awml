package awml

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func pageDoc(t *testing.T, title, view, label string) *Map {
	t.Helper()
	return parsed(t, fmt.Sprintf("{awml: \"0.2\", title: %q, view: %s, parts: [{id: a, kind: ai, label: %q}]}", title, view, label))
}

func drawings(t *testing.T, html string) []string {
	t.Helper()
	m := regexp.MustCompile(`var DRAWINGS = (\[.*?\]);\n`).FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("no drawings in\n%s", html)
	}
	var out []string
	if err := json.Unmarshal([]byte(m[1]), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAPageStandsAloneAndDrawsWithMermaid1141(t *testing.T) {
	html := Page([]*Map{pageDoc(t, "Dinners", "security", "Plan")})
	if !strings.HasPrefix(html, "<!doctype html>\n") {
		t.Errorf("the page has no doctype first")
	}
	has(t, html, `<script src="https://cdn.jsdelivr.net/npm/mermaid@11.4.1/dist/mermaid.min.js"></script>`)
	has(t, html, "<title>Dinners</title>")
}

func TestAPageHoldsEachDrawingInOrder(t *testing.T) {
	docs := []*Map{pageDoc(t, "Dinners", "security", "Plan"), pageDoc(t, "Dinners", "platform", "Plan"), pageDoc(t, "A question", "security", "Plan")}
	html := Page(docs)
	var want []string
	for _, d := range docs {
		want = append(want, Draw(d))
	}
	if got := drawings(t, html); !reflect.DeepEqual(got, want) {
		t.Errorf("drawings = %q", got)
	}
	if n := strings.Count(html, `<div class="awml"`); n != 3 {
		t.Errorf("%d drawings", n)
	}
	a, b, c := strings.Index(html, "Security view: who may see each part"), strings.Index(html, "Platform view: where each part runs"), strings.Index(html, "<h2>A question</h2>")
	if !(a < b && b < c) {
		t.Errorf("the drawings are out of order")
	}
}

func TestEachDrawingIsOnAWhitePanel(t *testing.T) {
	html := Page([]*Map{pageDoc(t, "Dinners", "security", "Plan")})
	if !regexp.MustCompile(`\.panel \{[^}]*background: #ffffff`).MatchString(html) {
		t.Errorf("no white panel")
	}
	has(t, html, `<div class="panel"><div class="awml"`)
}

func TestThePageIsNamedAfterItsWorkflowOrSimplyWorkflows(t *testing.T) {
	has(t, Page([]*Map{pageDoc(t, "Dinners", "security", "Plan"), pageDoc(t, "Dinners", "platform", "Plan")}), "<title>Dinners</title>")
	has(t, Page([]*Map{pageDoc(t, "Dinners", "security", "Plan"), pageDoc(t, "A question", "security", "Plan")}), "<title>Workflows</title>")
}

func TestAWideDrawingShrinksToFitAWideScreenAndScrollsOnAPhone(t *testing.T) {
	html := Page([]*Map{pageDoc(t, "Dinners", "security", "Plan")})
	has(t, html, "@media (min-width: 900px) { .awml svg { max-width: 100%; height: auto; } }")
	if !regexp.MustCompile(`\.panel \{[^}]*overflow-x: auto`).MatchString(html) {
		t.Errorf("the panel doesn't scroll")
	}
}

func TestTitlesAreEscaped(t *testing.T) {
	has(t, Page([]*Map{pageDoc(t, "Fish & <chips>", "security", "Plan")}), "<h2>Fish &amp; &lt;chips&gt;</h2>")
	has(t, Page([]*Map{pageDoc(t, `The family's "dinners"`, "security", "Plan")}), "<h2>The family&#x27;s &quot;dinners&quot;</h2>")
}

func TestALabelCantEndTheScriptEarly(t *testing.T) {
	d := pageDoc(t, "Dinners", "security", "</script><b>bold</b>")
	html := Page([]*Map{d})
	if n := strings.Count(html, "</script>"); n != 2 {
		t.Errorf("%d script ends", n)
	}
	if got := drawings(t, html); !reflect.DeepEqual(got, []string{Draw(d)}) {
		t.Errorf("drawings = %q", got)
	}
}

func TestTheDrawingsAreWrittenInPlainASCII(t *testing.T) {
	html := Page([]*Map{pageDoc(t, "Dinners", "security", "Fill the basket: Allow under £180 «🛒»"), pageDoc(t, "Dinners", "platform", "Plan")})
	has(t, html, `Fill the basket#58; Allow under \u00a3180 \u00ab\ud83d\uded2\u00bb`)
	has(t, html, `bottom: 30\n---\nflowchart TB\n  a(\"\u003csmall style='color:rgb(61,67,79)'>\ud83e\udd16\u003c/small>\u003cbr>Plan\")`)
	has(t, html, `"`+`, "---\nconfig:`)
}
