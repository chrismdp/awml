package awml

import (
	"fmt"
	"strings"
)

// Draw turns an AWML document into Mermaid, so every drawing uses the same shapes and colours. The
// document must have no mistakes: Check it first.
func Draw(doc *Map) string {
	groups, parts, lines := items(doc, "groups"), items(doc, "parts"), items(doc, "lines")
	where := placed(parts, lines)
	out := append([]string{}, header...)

	var emit func(parent, indent string)
	emit = func(parent, indent string) {
		for _, p := range parts {
			if where[str(p, "id")] == parent {
				out = append(out, indent+str(p, "id")+fmt.Sprintf(shapes[str(p, "kind")], text(str(p, "label"))))
			}
		}
		for _, g := range groups {
			if str(g, "in") == parent {
				out = append(out, fmt.Sprintf(`%ssubgraph %s["%s"]`, indent, str(g, "id"), text(str(g, "label"))))
				// Mermaid turns a box with no links outside it on its side unless told otherwise.
				out = append(out, indent+"  direction TB")
				emit(str(g, "id"), indent+"  ")
				out = append(out, indent+"end")
			}
		}
	}
	emit("", "  ")
	out = append(out, "")
	for _, l := range lines {
		mid := ""
		if str(l, "label") != "" {
			mid = fmt.Sprintf(`|"%s"|`, text(str(l, "label")))
		}
		out = append(out, fmt.Sprintf("  %s %s%s %s", str(l, "from"), arrows[str(l, "kind")], mid, str(l, "to")))
	}
	out = append(out, defs)
	for _, cls := range []string{"trig", "ctx", "con", "surf", "out", "ai", "rule", "human", "uml", "gap"} {
		var ids []string
		for _, p := range parts {
			if classes[str(p, "kind")] == cls {
				ids = append(ids, str(p, "id"))
			}
		}
		if ids != nil {
			out = append(out, fmt.Sprintf("  class %s %s", strings.Join(ids, ","), cls))
		}
	}
	for i, g := range groups {
		look := scope
		if str(doc, "view") != "scopes" {
			look = pastel[i%len(pastel)] + ",color:#16171b"
		}
		out = append(out, fmt.Sprintf("  style %s %s", str(g, "id"), look))
	}
	// linkStyle counts every line, hidden ones too.
	var reds []string
	for i, l := range lines {
		if l.get("breaks") == true {
			reds = append(reds, fmt.Sprint(i))
		}
	}
	if reds != nil {
		out = append(out, fmt.Sprintf("  linkStyle %s %s", strings.Join(reds, ","), red))
	}
	return strings.Join(out, "\n")
}

// UML activity shapes where Mermaid has them; our own where UML has nothing.
var shapes = map[string]string{
	"trigger":   `(["%s"])`,                          // accept event or accept time event, drawn as a pill
	"context":   `[("%s")]`,                          // data store node, drawn as a cylinder
	"connector": `{{"%s"}}`,                          // connector: ours
	"surface":   `@{ shape: win-pane, label: "%s" }`, // surface: ours
	"outside":   `>"%s"]`,                            // send signal action, drawn as a flag
	"ai":        `("%s")`,                            // actions
	"rule":      `("%s")`,
	"human":     `("%s")`,
	"review":    `{"%s"}`,                              // decision node
	"fork":      `@{ shape: fork, label: "" }%.0s`,     // fork node, which shows no label
	"comment":   `@{ shape: notch-rect, label: "%s" }`, // comment
}

var classes = map[string]string{"trigger": "trig", "context": "ctx", "connector": "con", "surface": "surf", "outside": "out",
	"ai": "ai", "rule": "rule", "human": "human", "review": "human", "fork": "uml", "comment": "gap"}

var arrows = map[string]string{"start": "-.->", "data": "-->", "publish": "==>", "refused": "--x", "tie": "-.-", "hidden": "~~~"}

const defs = `  classDef trig fill:#fbe1ee,stroke:#a33a74,color:#16171b
  classDef ctx fill:#e2eef9,stroke:#3d6fa3,color:#16171b
  classDef con fill:#eceef2,stroke:#5d6472,color:#16171b
  classDef surf fill:#ffffff,stroke:#2a2d35,stroke-width:1.5px,color:#16171b
  classDef out fill:#fde4dc,stroke:#b5532f,color:#16171b
  classDef ai fill:#cdd3fb,stroke:#3b4bb0,color:#16171b
  classDef rule fill:#c4e3d1,stroke:#2b7a4f,color:#16171b
  classDef human fill:#f7d79a,stroke:#9a6100,color:#16171b
  classDef uml fill:#16171b,stroke:#16171b,color:#16171b
  classDef gap fill:#ffffff,stroke:#8a93a3,stroke-dasharray:4 3,color:#6b7280`

const scope = "fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f"

var pastel = []string{"fill:#e3ebfa,stroke:#b9c8e8", "fill:#fbe9df,stroke:#ecc7b2", "fill:#efe6fa,stroke:#d3c2ea",
	"fill:#fbf3d9,stroke:#e8d9a6", "fill:#dff3ef,stroke:#b4ddd4", "fill:#f6e3ea,stroke:#e3bccb", "fill:#e9f2dc,stroke:#c7dcae"}

const red = "stroke:#d0342c,stroke-width:2.5px,color:#b0271f"

var header = []string{"---", "config:", "  theme: default", "  flowchart:", "    useMaxWidth: false",
	"    subGraphTitleMargin:", "      top: 6", "      bottom: 30", "---", "flowchart TB"}

// text keeps a label inside its quotes: Mermaid ends a label at a double quote, and reads #quot; as one.
func text(s string) string {
	return strings.ReplaceAll(s, `"`, "#quot;")
}

// placed gives each part's group, "" for none. A trigger or comment with none sits in the group of
// what it starts or explains, so it doesn't stretch the drawing.
func placed(parts, lines []*Map) map[string]string {
	home, out := map[string]string{}, map[string]string{}
	for _, p := range parts {
		home[str(p, "id")] = str(p, "in")
		out[str(p, "id")] = str(p, "in")
	}
	for _, p := range parts {
		self := str(p, "id")
		if home[self] != "" || (str(p, "kind") != "trigger" && str(p, "kind") != "comment") {
			continue
		}
		for _, l := range lines {
			if str(l, "kind") == "hidden" || (str(l, "from") != self && str(l, "to") != self) {
				continue
			}
			other := str(l, "from")
			if other == self {
				other = str(l, "to")
			}
			out[self] = home[other]
			break
		}
	}
	return out
}

// items gives a file's groups, parts or lines.
func items(doc *Map, key string) []*Map {
	list, _ := doc.get(key).([]any)
	maps := make([]*Map, 0, len(list))
	for _, item := range list {
		maps = append(maps, item.(*Map))
	}
	return maps
}

// str gives a field as text, or "" when it's missing or null.
func str(m *Map, key string) string {
	s, _ := m.get(key).(string)
	return s
}
