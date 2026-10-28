package awml

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// pageDoc is a workflow of one AI step, which an implementation places on a server if the view
// asked for is platform.
func pageDoc(t *testing.T, title, view, label string) *Map {
	t.Helper()
	implementations := ""
	if view == "platform" {
		implementations = ", implementations: {home: {label: Home, blocks: [{id: server, label: Server, parts: [a]}]}}"
	}
	return parsed(t, fmt.Sprintf("{awml: \"0.8\", title: %q, parts: [{id: a, kind: ai, label: %q}]%s}", title, label, implementations))
}

// given gives each document as a workflow in a file of its own, as awml page reads them.
func given(docs ...*Map) []Workflow {
	workflows := make([]Workflow, len(docs))
	for i, d := range docs {
		workflows[i] = Workflow{File: fmt.Sprintf("w%d.yaml", i+1), Doc: d}
	}
	return workflows
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
	html := Page(given(pageDoc(t, "Dinners", "security", "Plan")))
	if !strings.HasPrefix(html, "<!doctype html>\n") {
		t.Errorf("the page has no doctype first")
	}
	has(t, html, `<script src="https://cdn.jsdelivr.net/npm/mermaid@11.4.1/dist/mermaid.min.js"></script>`)
	has(t, html, "<title>Dinners</title>")
}

func TestAPageHoldsEachDrawingInOrder(t *testing.T) {
	docs := []*Map{pageDoc(t, "Dinners", "platform", "Plan"), pageDoc(t, "A question", "security", "Plan")}
	html := Page(given(docs...))
	placed, err := View(docs[0], "home")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{DrawMap(given(docs...)), Draw(docs[0]), Draw(placed), Draw(docs[1])}
	if got := drawings(t, html); !reflect.DeepEqual(got, want) {
		t.Errorf("drawings = %q", got)
	}
	if n := strings.Count(html, `<div class="awml"`); n != 4 {
		t.Errorf("%d drawings", n)
	}
	m, a, b, c := strings.Index(html, "Map: how the workflows fit together"), strings.Index(html, "Security view: who may see each part"), strings.Index(html, "Platform view: where each part runs"), strings.Index(html, "<h2>A question</h2>")
	if !(0 <= m && m < a && a < b && b < c) {
		t.Errorf("the drawings are out of order")
	}
}

func TestOneWorkflowHasNoMap(t *testing.T) {
	d := pageDoc(t, "Dinners", "security", "Plan")
	html := Page(given(d))
	if got := drawings(t, html); !reflect.DeepEqual(got, []string{Draw(d)}) {
		t.Errorf("drawings = %q", got)
	}
	if strings.Contains(html, "Map:") {
		t.Errorf("one workflow has a map")
	}
}

func TestAPlatformDescriptionGivenWithAWorkflowMakesNoMap(t *testing.T) {
	d := pageDoc(t, "Dinners", "security", "Plan")
	desc := parsed(t, `{awml: "0.8", platform: {id: scripts, label: Scripts}}`)
	if got := drawings(t, Page(given(d, desc))); !reflect.DeepEqual(got, []string{Draw(d)}) {
		t.Errorf("drawings = %q", got)
	}
}

func TestTheMapIsHeadedWithThePagesName(t *testing.T) {
	html := Page(given(pageDoc(t, "Dinners", "security", "Plan"), pageDoc(t, "A question", "security", "Plan")))
	has(t, html, "<section>\n<h2>Workflows</h2>\n<p class=\"view\">Map: how the workflows fit together</p>\n"+`<div class="panel"><div class="awml" role="img" aria-label="Workflows, map: how the workflows fit together"></div></div>`)
}

func TestEachDrawingIsOnAWhitePanel(t *testing.T) {
	html := Page(given(pageDoc(t, "Dinners", "security", "Plan")))
	if !regexp.MustCompile(`\.panel \{[^}]*background: #ffffff`).MatchString(html) {
		t.Errorf("no white panel")
	}
	has(t, html, `<div class="panel"><div class="awml"`)
}

func TestThePageIsNamedAfterItsWorkflowOrSimplyWorkflows(t *testing.T) {
	has(t, Page(given(pageDoc(t, "Dinners", "platform", "Plan"))), "<title>Dinners</title>")
	has(t, Page(given(pageDoc(t, "Dinners", "security", "Plan"), pageDoc(t, "A question", "security", "Plan"))), "<title>Workflows</title>")
}

func TestAWideDrawingShrinksToFitAWideScreenAndScrollsOnAPhone(t *testing.T) {
	html := Page(given(pageDoc(t, "Dinners", "security", "Plan")))
	has(t, html, "@media (min-width: 900px) { .awml svg { max-width: 100%; height: auto; } }")
	if !regexp.MustCompile(`\.panel \{[^}]*overflow-x: auto`).MatchString(html) {
		t.Errorf("the panel doesn't scroll")
	}
}

func TestTitlesAreEscaped(t *testing.T) {
	has(t, Page(given(pageDoc(t, "Fish & <chips>", "security", "Plan"))), "<h2>Fish &amp; &lt;chips&gt;</h2>")
	has(t, Page(given(pageDoc(t, `The family's "dinners"`, "security", "Plan"))), "<h2>The family&#x27;s &quot;dinners&quot;</h2>")
}

func TestALabelCantEndTheScriptEarly(t *testing.T) {
	d := pageDoc(t, "Dinners", "security", "</script><b>bold</b>")
	html := Page(given(d))
	if n := strings.Count(html, "</script>"); n != 2 {
		t.Errorf("%d script ends", n)
	}
	if got := drawings(t, html); !reflect.DeepEqual(got, []string{Draw(d)}) {
		t.Errorf("drawings = %q", got)
	}
}

func TestTheDrawingsAreWrittenInPlainASCII(t *testing.T) {
	html := Page(given(pageDoc(t, "Dinners", "security", "Fill the basket: Allow under £180 «🛒»"), pageDoc(t, "Dinners", "security", "Plan")))
	has(t, html, `Fill the basket#58; Allow under \u00a3180 \u00ab\ud83d\uded2\u00bb`)
	has(t, html, `bottom: 30\n---\nflowchart TB\n  a(\"\u003csmall style='color:rgb(61,67,79)'>\ud83e\udd16\u003c/small>\u003cbr>Plan\")`)
	has(t, html, `"`+`, "---\nconfig:`)
}
