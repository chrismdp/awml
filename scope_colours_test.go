package awml

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

var wantScopeColours = [][2]string{
	{"#127c8c", "#0d5c68"}, // teal
	{"#8c2f6b", "#6b2251"}, // plum
	{"#52677c", "#394a5c"}, // slate
	{"#6c7026", "#4e511b"}, // olive
	{"#655294", "#493b6d"}, // violet
	{"#66616b", "#49464d"}, // graphite
}

// The first scope is inside the second, so file order differs from drawing order.
// More than two rounds check every colour and the return to the start of the list.
func manyScopes() string {
	var src strings.Builder
	src.WriteString("awml: \"0.10\"\ntitle: Scope colours\nscopes:\n")
	for i := 0; i < 2*len(wantScopeColours)+1; i++ {
		parent := ""
		if i == 0 {
			parent = ", in: g1"
		} else if i > 1 {
			parent = fmt.Sprintf(", in: g%d", i-1)
		}
		fmt.Fprintf(&src, "  - {id: g%d, label: Scope %d%s}\n", i, i, parent)
	}
	src.WriteString("parts: []\nlines: []\n")
	return src.String()
}

func scopeStyleLines(out string) []string {
	var styles []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  style ") {
			styles = append(styles, line)
		}
	}
	return styles
}

func TestScopeColoursFollowFileOrderAndCycle(t *testing.T) {
	styles := scopeStyleLines(Draw(parsed(t, manyScopes())))
	if len(styles) != 2*len(wantScopeColours)+1 {
		t.Fatalf("got %d scope styles: %v", len(styles), styles)
	}
	for i, got := range styles {
		colour := wantScopeColours[i%len(wantScopeColours)]
		want := fmt.Sprintf("  style g%d fill:transparent,stroke:%s,stroke-width:1.5px,stroke-dasharray:6 4,color:%s", i, colour[0], colour[1])
		if got != want {
			t.Errorf("scope %d: got %q, want %q", i, got, want)
		}
	}
}

func TestScopesHaveNoFillAndKeepTheirDashes(t *testing.T) {
	for _, line := range scopeStyleLines(Draw(parsed(t, manyScopes()))) {
		has(t, line+",", "fill:transparent,")
		has(t, line+",", "stroke-dasharray:6 4,")
		if strings.Count(line, "fill:") != 1 {
			t.Errorf("scope has another fill: %s", line)
		}
	}
}

func TestEveryScopeColourHasContrastOnWhite(t *testing.T) {
	for _, line := range scopeStyleLines(Draw(parsed(t, manyScopes()))) {
		for _, field := range strings.Split(line, ",") {
			key, colour, _ := strings.Cut(field, ":")
			minimum := 3.0
			if key == "color" {
				minimum = 4.5
			} else if key != "stroke" {
				continue
			}
			contrast := contrastOnWhite(t, colour)
			if contrast < minimum {
				t.Errorf("%s %s has %.2f:1 contrast on white, want at least %.1f:1", key, colour, contrast, minimum)
			}
			t.Logf("%s %s: %.2f:1", key, colour, contrast)
		}
	}
}

// WCAG relative luminance linearises each sRGB channel before weighting it.
func contrastOnWhite(t *testing.T, colour string) float64 {
	t.Helper()
	if len(colour) != 7 || colour[0] != '#' {
		t.Fatalf("not an RGB colour: %q", colour)
	}
	var channels [3]float64
	for i := range channels {
		n, err := strconv.ParseUint(colour[1+2*i:3+2*i], 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		c := float64(n) / 255
		if c <= 0.04045 {
			channels[i] = c / 12.92
		} else {
			channels[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	luminance := 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2]
	return 1.05 / (luminance + 0.05)
}

func TestSecurityGraphGroupsGiveTheDrawnColours(t *testing.T) {
	g := GraphOf(parsed(t, manyScopes()))
	encoded, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var read struct {
		Groups []struct {
			ID, Stroke, Color string
			Fill              *string
		}
	}
	if err := json.Unmarshal(encoded, &read); err != nil {
		t.Fatal(err)
	}
	for i, group := range read.Groups {
		colour := wantScopeColours[i%len(wantScopeColours)]
		if group.Stroke != colour[0] || group.Color != colour[1] || group.Fill != nil {
			t.Errorf("scope %d has JSON group %+v", i, group)
		}
		has(t, g.Mermaid(), fmt.Sprintf("  style %s fill:transparent,stroke:%s,stroke-width:1.5px,stroke-dasharray:6 4,color:%s", group.ID, group.Stroke, group.Color))
	}
	if len(read.Groups) != 2*len(wantScopeColours)+1 || len(g.Places) != 0 {
		t.Errorf("security graph has %d groups and places %+v", len(read.Groups), g.Places)
	}
}

func TestOnlySecurityGroupsGainScopeColours(t *testing.T) {
	for name, g := range map[string]Graph{
		"platform": graphOf(t, bakeryReplies, "home"),
		"map":      GraphMap(together(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)),
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(g.Groups)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), `"stroke"`) || strings.Contains(string(encoded), `"color"`) {
				t.Errorf("%s groups gained scope colours: %s", name, encoded)
			}
			styles := scopeStyleLines(g.Mermaid())
			want := []string{
				"  style laptop fill:#e3ebfa,stroke:#b9c8e8,color:#16171b",
				"  style mailer fill:#fbe9df,stroke:#ecc7b2,color:#16171b",
			}
			if name == "map" {
				want = []string{
					"  style w1 fill:#f6f7f9,stroke:#5d6472,color:#16171b",
					"  style w2 fill:#f6f7f9,stroke:#5d6472,color:#16171b",
				}
			}
			if strings.Join(styles, "\n") != strings.Join(want, "\n") {
				t.Errorf("%s styles changed: %v", name, styles)
			}
		})
	}
}

func TestTheMermaidUsesSecurityGroupColoursFromJSON(t *testing.T) {
	var g Graph
	if err := json.Unmarshal([]byte(`{"view":"security","groups":[{"id":"g","label":"Scope","stroke":"#314159","color":"#271828"}]}`), &g); err != nil {
		t.Fatal(err)
	}
	has(t, g.Mermaid(), "  style g fill:transparent,stroke:#314159,stroke-width:1.5px,stroke-dasharray:6 4,color:#271828")
}
