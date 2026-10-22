package awml

import (
	"os"
	"path/filepath"
	"reflect"
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
	want := []string{"client-email", "dinners", "legend", "question",
		"tour/01-draft-and-send", "tour/02-fork", "tour/03-refused-read", "tour/04-publishing", "tour/05-side-by-side",
		"tour/06-red-line", "tour/07-person-step", "tour/08-gap", "tour/09-platform-view", "tour/10-operations",
		"tour/11-do", "tour/12-standing-approval", "tour/13-check-by-code"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("examples = %q\nwant %q", names, want)
	}
}

func TestEachExampleIsSoundAndItsDrawingsAreCurrent(t *testing.T) {
	for _, path := range examples(t) {
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
			if got := Warnings(doc); len(got) != 0 {
				t.Errorf("%s has warnings: %v", path, got)
			}
			file := doc.(*Map)
			if !IsCanonical(file) {
				t.Fatal("example is not 0.4")
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

func TestEachExampleIsWrittenInTheHouseStyle(t *testing.T) {
	for _, path := range append(examples(t), platformExamples(t)...) {
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
	want := `cowork-mac-mini   no: bring and rules are code steps, which anthropic's agent would carry out
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
