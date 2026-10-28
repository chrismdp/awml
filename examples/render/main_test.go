package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// copyExample puts the example at path, changed by edit, in a directory of its own, so drawing it
// leaves the repository alone.
func copyExample(t *testing.T, path string, edit func(string) string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(out, []byte(edit(string(src))), 0644); err != nil {
		t.Fatal(err)
	}
	return out
}

// A live example's only mistakes are the red lines awml draw draws, so the helper draws it too.
func TestTheHelperDrawsAFileWhoseOnlyMistakesAreRedLines(t *testing.T) {
	path := copyExample(t, "../live/red-line.yaml", func(s string) string { return s })
	drawings, err := drawExample(path)
	if err != nil {
		t.Fatal(err)
	}
	stem := strings.TrimSuffix(path, ".yaml")
	if want := []string{stem + ".mmd"}; !reflect.DeepEqual(drawings, want) {
		t.Errorf("drawings = %q, want %q", drawings, want)
	}
	got, err := os.ReadFile(stem + ".mmd")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../live/red-line.mmd")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("the helper drew\n%s\nwant\n%s", got, want)
	}
}

// The security view shows where untrusted text goes, so the helper draws no untrusted view.
func TestTheHelperDrawsOnlyTheSecurityViewOfAFileThatMarksAPartUntrusted(t *testing.T) {
	path := copyExample(t, "../tour/14-untrusted-text.yaml", func(s string) string { return s })
	drawings, err := drawExample(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{strings.TrimSuffix(path, ".yaml") + ".mmd"}; !reflect.DeepEqual(drawings, want) {
		t.Errorf("drawings = %q, want %q", drawings, want)
	}
}

func TestTheHelperRefusesAFileWithAMistakeThatIsntALine(t *testing.T) {
	path := copyExample(t, "../live/red-line.yaml", func(s string) string {
		return strings.Replace(s, "kind: trigger", "kind: nonsense", 1)
	})
	if _, err := drawExample(path); err == nil {
		t.Error("the helper drew a file with a part of no known kind")
	}
	if _, err := os.Stat(strings.TrimSuffix(path, ".yaml") + ".mmd"); err == nil {
		t.Error("the helper wrote a drawing of a file it refused")
	}
}

func TestTheHelperRefusesAMapOfAFileWithAMistakeThatIsntALine(t *testing.T) {
	path := copyExample(t, "../live/red-line.yaml", func(s string) string {
		return strings.Replace(s, "kind: trigger", "kind: nonsense", 1)
	})
	out := filepath.Join(t.TempDir(), "map.mmd")
	if err := drawMap(out, []string{path}); err == nil {
		t.Error("the helper drew a map of a file with a part of no known kind")
	}
}

// set -e doesn't fire for a command substitution in a for list or in a command's arguments, so
// render.sh takes the helper's output in an assignment, which fails the script when the helper does.
func TestRenderFailsWhenTheHelperFails(t *testing.T) {
	src, err := os.ReadFile("../render.sh")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, `$("$tmp/draw"`) {
			continue
		}
		calls++
		if !strings.HasPrefix(strings.TrimSpace(line), `drawings=$("$tmp/draw" `) {
			t.Errorf("render.sh carries on when the helper fails in: %s", strings.TrimSpace(line))
		}
	}
	if calls == 0 {
		t.Error("render.sh never runs the helper")
	}
}
