package awml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The skill travels without the repository, so it carries its own copies of the spec, the guide and
// some examples. Each must be the same, byte for byte, as its source.
const skill = "plugins/awml/skills/awml"

func TestTheSkillsCopiesAreCurrent(t *testing.T) {
	copies := [][2]string{{"SPEC.md", "references/spec.md"}, {"GUIDE.md", "references/guide.md"}}
	for _, name := range []string{"client-email", "client-email-platform", "dinners-security", "dinners-platform"} {
		for _, ext := range []string{".yaml", ".mmd"} {
			copies = append(copies, [2]string{"examples/" + name + ext, "references/examples/" + name + ext})
		}
	}
	for _, c := range copies {
		src, dst := c[0], filepath.Join(skill, c[1])
		want, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(dst)
		if err != nil || string(got) != string(want) {
			t.Errorf("%s isn't the same as %s: copy it again with cp %s %s", dst, src, src, dst)
		}
	}
}

// Drawing by hand, where awml can't run, must give what awml draw gives, so the guide to it holds
// every shape, arrow and style that Draw uses.
func TestTheGuideToDrawingByHandMatchesDraw(t *testing.T) {
	path := filepath.Join(skill, "references/drawing-by-hand.md")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	guide := string(src)
	want := []string{strings.Join(header, "\n"), defs, scope, red, fmt.Sprintf(operation, "🤖 ✏️ draft")}
	for _, shape := range shapes {
		want = append(want, fmt.Sprintf(shape, "label"))
	}
	for _, arrow := range arrows {
		want = append(want, "`"+arrow+"`")
	}
	for _, arrow := range backwardsArrows {
		want = append(want, "`"+arrow+"`")
	}
	want = append(want, "marker-end:none")
	want = append(want, pastel...)
	for op, emoji := range ops {
		want = append(want, fmt.Sprintf("| %s | `%s` |", emoji, op))
	}
	for kind, class := range classes {
		want = append(want, fmt.Sprintf("| `%s` | `%s` |", kind, class))
	}
	for _, w := range want {
		if !strings.Contains(guide, w) {
			t.Errorf("%s doesn't hold what awml draws: %q", path, w)
		}
	}
}
