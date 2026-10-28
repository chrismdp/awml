package awml

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func examples(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, pattern := range []string{"examples/*.yaml", "examples/tour/*.yaml"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, found...)
	}
	return paths
}

func platformExamples(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("examples/platforms/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestThereAreExamples(t *testing.T) {
	var names []string
	for _, p := range examples(t) {
		names = append(names, strings.TrimSuffix(strings.TrimPrefix(p, "examples/"), ".yaml"))
	}
	want := []string{"bakery-order", "bakery-stock", "bills", "client-email", "dinners", "legend", "pickups", "question", "shopping",
		"tour/01-draft-and-send", "tour/02-fork", "tour/03-refused-read", "tour/04-publishing", "tour/05-side-by-side",
		"tour/07-person-step", "tour/08-gap", "tour/09-platform-view", "tour/10-operations",
		"tour/11-do", "tour/12-standing-approval", "tour/13-check-by-code", "tour/14-untrusted-text",
		"tour/15-several-workflows", "tour/15-split-count", "tour/15-split-order", "tour/16-accepted-risk", "tour/17-untrusted-risk-accepted"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("examples = %q\nwant %q", names, want)
	}
}

// The security view shows where untrusted text goes, so no example keeps an untrusted view.
func TestNoExampleHasAnUntrustedView(t *testing.T) {
	var found []string
	for _, pattern := range []string{"examples/*untrusted-view*", "examples/*/*untrusted-view*"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		found = append(found, paths...)
	}
	if found != nil {
		t.Errorf("examples still has %q", found)
	}
}

// A drawing of a live system shows where it breaks the design, so awml check fails on it by
// design. Its examples sit apart from the rest, which every check passes.
func liveExamples(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("examples/live/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestThereAreLiveExamples(t *testing.T) {
	if got, want := liveExamples(t), []string{"examples/live/red-line.yaml", "examples/live/untrusted-text.yaml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("live examples = %q\nwant %q", got, want)
	}
}

func TestEachExampleIsSoundAndItsDrawingsAreCurrent(t *testing.T) {
	for _, path := range append(examples(t), liveExamples(t)...) {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(path, "examples/live/") {
				// Each of its mistakes is a line that breaks a rule, which it draws red.
				if len(Check(doc)) == 0 || len(Unsound(doc)) != 0 {
					t.Errorf("%s: Check gave %q and Unsound %q", path, Check(doc), Unsound(doc))
				}
			} else {
				mistakes(t, doc)
			}
			var want []string
			got := Warnings(doc)
			switch path {
			case "examples/legend.yaml":
				// The legend is a key to the shapes, not a workflow, so nothing starts its parts and
				// its hidden lines only lay them out. It's held to every warning but those of shape.
				got = warningsOf(doc, "warn.do-by-code", "warn.one-change")
			case "examples/tour/15-several-workflows.yaml":
				// The tour shows this warning, and the split that answers it.
				want = []string{`this file holds 2 workflows: the one started by "Every day 18:00" and the one started by "Every day 07:00": split it at its triggers into workflows that refer to each other`}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s has warnings %q, want %q", path, got, want)
			}
			file := doc.(*Map)
			if !IsCanonical(file) {
				t.Fatal("example is not a workflow")
			}
			for _, name := range ImplementationIDs(file) {
				view, err := View(file, name)
				if err != nil {
					t.Fatal(err)
				}
				stem := strings.TrimSuffix(path, ".yaml") + "-" + name
				mmd, err := os.ReadFile(stem + ".mmd")
				if err != nil {
					t.Fatal(err)
				}
				if string(mmd) != Draw(view)+"\n" {
					t.Errorf("%s.mmd is stale", stem)
				}
				if _, err := os.Stat(stem + ".svg"); err != nil {
					t.Error(err)
				}
			}
			stem := strings.TrimSuffix(path, ".yaml")
			mmd, err := os.ReadFile(stem + ".mmd")
			if err != nil {
				t.Fatal(err)
			}
			if got := Draw(doc.(*Map)) + "\n"; got != string(mmd) {
				t.Errorf("%s.mmd isn't what awml draws now: run examples/render.sh", stem)
			}
			svg, err := os.ReadFile(stem + ".svg")
			if err != nil || !strings.HasPrefix(string(svg), "<svg") {
				t.Errorf("%s.svg isn't an SVG", stem)
			}
		})
	}
}

// The family's workflows share the meal plan, the favourites, the kitchen screen and the family
// chat, the bakery's stock count shares its stock levels with the supplier order and starts it, and
// the tour's split count and order share the stock levels, so each set is drawn together as a map.
// examples/render.sh draws their SVGs.
func TestTheExamplesThatShareAPartAreDrawnTogether(t *testing.T) {
	for _, example := range []struct {
		drawing string
		paths   []string
		starts  bool
	}{
		{"examples/family-map.mmd", []string{"examples/dinners.yaml", "examples/shopping.yaml", "examples/bills.yaml",
			"examples/pickups.yaml", "examples/question.yaml"}, false},
		{"examples/bakery-map.mmd", []string{"examples/bakery-stock.yaml", "examples/bakery-order.yaml"}, true},
		{"examples/tour/15-split-map.mmd", []string{"examples/tour/15-split-count.yaml", "examples/tour/15-split-order.yaml"}, false},
	} {
		var workflows []Workflow
		for _, path := range example.paths {
			workflows = append(workflows, Workflow{File: path, Doc: readExample(t, path)})
		}
		none := make([][]string, len(workflows))
		if got := TogetherMistakes(workflows); !reflect.DeepEqual(got, none) {
			t.Errorf("%s: together they have mistakes: %q", example.drawing, got)
		}
		if got := TogetherWarnings(workflows); !reflect.DeepEqual(got, none) {
			t.Errorf("%s: together they have warnings: %q", example.drawing, got)
		}
		if got := SummariseTogether(workflows).Shared; len(got) == 0 {
			t.Errorf("%s: they share no part", example.drawing)
		}
		if got := len(link(workflows).starts) > 0; got != example.starts {
			t.Errorf("%s: one starts the other is %v, want %v", example.drawing, got, example.starts)
		}
		draw := "awml draw " + strings.Join(example.paths, " ")
		mmd, err := os.ReadFile(example.drawing)
		if err != nil || string(mmd) != DrawMap(workflows)+"\n" {
			t.Errorf("%s isn't what %s draws now: run examples/render.sh", example.drawing, draw)
		}
		script, err := os.ReadFile("examples/render.sh")
		if err != nil || !strings.Contains(string(script), "-map "+example.drawing+" "+strings.Join(example.paths, " ")) {
			t.Errorf("examples/render.sh doesn't draw %s", example.drawing)
		}
	}
}

// The Mermaid CLI takes any Mermaid 11, so a new release could redraw every example unless
// render.sh pins Mermaid itself.
func TestRenderPinsMermaid(t *testing.T) {
	src, err := os.ReadFile("examples/render.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`npx -y -p @mermaid-js/mermaid-cli@\d+\.\d+\.\d+ -p mermaid@\d+\.\d+\.\d+ mmdc `).Match(src) {
		t.Error("render.sh doesn't pin both mermaid-cli and mermaid to exact versions")
	}
}

func TestEachExampleIsWrittenInTheHouseStyle(t *testing.T) {
	for _, path := range append(append(examples(t), liveExamples(t)...), platformExamples(t)...) {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			value, err := Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			doc, ok := value.(*Map)
			if !ok {
				t.Fatalf("example is %T, not a mapping", value)
			}
			got, err := Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(src) {
				t.Errorf("Marshal changed %s\ngot:\n%s\nwant:\n%s", path, got, src)
			}
		})
	}
}

func TestEachPlatformExampleIsSound(t *testing.T) {
	for _, path := range platformExamples(t) {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			mistakes(t, doc)
		})
	}
}

func TestClientEmailFitMatchesTheSpecExample(t *testing.T) {
	read := func(path string) *Map {
		t.Helper()
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		if errors := Check(doc); len(errors) != 0 {
			t.Fatalf("%s: %v", path, errors)
		}
		return doc.(*Map)
	}
	security := read("examples/client-email.yaml")
	var platforms []*Map
	for _, path := range []string{"cowork-mac-mini", "scripts-mac-mini", "codex-laptop", "worker"} {
		platforms = append(platforms, read("examples/platforms/"+path+".yaml"))
	}
	lines, gaps := Fit(security, nil, platforms)
	if len(gaps) != 0 {
		t.Fatalf("fit gaps: %v", gaps)
	}
	want := `cowork-mac-mini   no: bring, rules and save are code steps, which anthropic's agent would carry out
scripts-mac-mini  fits: choose, learn and draft may use anthropic
codex-laptop      no: weekdays may wait PT2H, but the platform may be off for P14D
                  no: openai's agents run here, and business doesn't allow openai
worker            no: engagements is kept at notes/engagements, which the platform doesn't hold
                  no: project is kept at notes/projects, which the platform doesn't hold`
	got := strings.Join(lines, "\n")
	if got != want {
		t.Errorf("fit gave\n%s\nwant\n%s", got, want)
	}
	spec, err := os.ReadFile("SPEC.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(spec), want) {
		t.Error("SPEC.md doesn't contain the client email fit output")
	}
}
