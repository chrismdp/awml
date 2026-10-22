package awml

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

const publishSkill = "plugins/awml/skills/awml-publish"

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var manifests = []string{"plugins/awml/.claude-plugin/plugin.json", "plugins/awml/.codex-plugin/plugin.json"}

var marketplaces = []string{".claude-plugin/marketplace.json", ".agents/plugins/marketplace.json"}

// Installing the plugin connects to nothing: it declares no MCP server, so the awml skill works
// offline and a registry is reached only once the person connects one.
func TestThePluginDeclaresNoMCPServer(t *testing.T) {
	for _, path := range []string{"plugins/awml/.mcp.json", "plugins/awml/.claude-plugin/.mcp.json", "plugins/awml/.codex-plugin/.mcp.json"} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s declares an MCP server; installing the plugin must connect to nothing", path)
		}
	}
	for _, path := range manifests {
		if strings.Contains(readText(t, path), "mcpServers") {
			t.Errorf("%s declares an MCP server; installing the plugin must connect to nothing", path)
		}
	}
}

// The awml skill describes, checks and draws offline. It names no registry and no address, and
// leaves sharing to awml-publish.
func TestTheAwmlSkillStaysOffline(t *testing.T) {
	text := readText(t, filepath.Join(skill, "SKILL.md"))
	for _, word := range []string{"registry", "awml.io", "mcp"} {
		if strings.Contains(strings.ToLower(text), word) {
			t.Errorf("the awml skill names %q; it works offline and leaves publishing to awml-publish", word)
		}
	}
	if !strings.Contains(text, "awml-publish") {
		t.Error("the awml skill doesn't say the awml-publish skill can share the workflow")
	}
}

// awml-publish is for publishing only. It uses registry tools someone already connected without
// asking which registry, starts from AWML_REGISTRY_URL or awml.io when there are none, and gives
// the connect commands that were run against a registry, word for word.
func TestThePublishSkill(t *testing.T) {
	text := readText(t, filepath.Join(publishSkill, "SKILL.md"))
	front, body, ok := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	if !ok {
		t.Fatal("awml-publish has no front matter")
	}
	for _, want := range []string{"name: awml-publish\n", "publish", "Never use it to describe, check or draw"} {
		if !strings.Contains(front, want) {
			t.Errorf("awml-publish's front matter lacks %q", want)
		}
	}
	for _, want := range []string{
		"AWML_REGISTRY_URL",
		"https://awml.io",
		"use them straight away and never ask which registry",
		"`check`",
		"`publish`",
		"claude mcp add --transport http awml https://awml.io/mcp",
		"codex mcp add awml --url https://awml.io/mcp --bearer-token-env-var AWML_TOKEN",
		"[mcp_servers.awml]\nurl = \"https://awml.io/mcp\"\nbearer_token_env_var = \"AWML_TOKEN\"",
		"awml login",
		"awml push",
		"Only an organisation can keep a workflow private",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("awml-publish lacks %q", want)
		}
	}
	if n, never := strings.Count(body, "which registry"), strings.Count(body, "never ask which registry"); n != never {
		t.Error("awml-publish asks which registry; it uses the connected tools, or AWML_REGISTRY_URL, or awml.io")
	}
}

// Both skills keep the voice of the rest: no dashes as punctuation.
func TestTheSkillsUseNoDashesAsPunctuation(t *testing.T) {
	for _, path := range []string{filepath.Join(skill, "SKILL.md"), filepath.Join(publishSkill, "SKILL.md")} {
		text := readText(t, path)
		for _, dash := range []string{"—", "–", " - ", " -- "} {
			if strings.Contains(text, dash) {
				t.Errorf("%s uses %q as punctuation", path, dash)
			}
		}
	}
}

// Both manifests and both marketplaces give the plugin the same version.
func TestThePluginHasOneVersion(t *testing.T) {
	version := regexp.MustCompile(`"version":\s*"([^"]+)"`)
	want := version.FindStringSubmatch(readText(t, manifests[0]))
	if want == nil {
		t.Fatalf("%s gives no version", manifests[0])
	}
	for _, path := range append(append([]string{}, manifests...), marketplaces...) {
		m := version.FindStringSubmatch(readText(t, path))
		if m == nil || m[1] != want[1] {
			t.Errorf("%s doesn't give the plugin's version, %s", path, want[1])
		}
	}
}
