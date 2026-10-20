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

func TestThereAreExamples(t *testing.T) {
	var names []string
	for _, p := range examples(t) {
		names = append(names, strings.TrimSuffix(strings.TrimPrefix(p, "examples/"), ".yaml"))
	}
	want := []string{"client-email-where", "client-email", "dinners-scopes", "dinners-where", "question"}
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
