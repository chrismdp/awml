package awml

import (
	"fmt"
	"strings"
)

// Draw turns an AWML document into Mermaid, so every drawing uses the same shapes and colours. The
// document must have no mistakes: Check it first.
func Draw(doc *Map) string {
	doc, _ = View(doc, "")
	groups, parts, lines := items(doc, "groups"), items(doc, "parts"), items(doc, "lines")
	where := placed(parts, lines)
	partOrder := map[string]int{}
	for i, p := range parts {
		partOrder[str(p, "id")] = i
	}
	out := append([]string{}, header...)

	var emit func(parent, indent string)
	emit = func(parent, indent string) {
		for _, p := range parts {
			if where[str(p, "id")] == parent {
				out = append(out, indent+str(p, "id")+fmt.Sprintf(shapes[str(p, "kind")], label(p)))
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
		from, to, arrow := str(l, "from"), str(l, "to"), arrows[str(l, "kind")]
		if partOrder[from] > partOrder[to] {
			from, to = to, from
			if flipped := backwardsArrows[str(l, "kind")]; flipped != "" {
				arrow = flipped
			}
		}
		mid := ""
		if str(l, "label") != "" {
			mid = fmt.Sprintf(`|"%s"|`, text(str(l, "label")))
		}
		out = append(out, fmt.Sprintf("  %s %s%s %s", from, arrow, mid, to))
	}
	out = append(out, defs)
	for _, cls := range []string{"trig", "ctx", "con", "surf", "out", "ai", "code", "person", "uml", "gap"} {
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
		if str(doc, "view") != "security" {
			look = pastel[i%len(pastel)] + ",color:#16171b"
		}
		out = append(out, fmt.Sprintf("  style %s %s", str(g, "id"), look))
	}
	// linkStyle counts every line, hidden ones too. A backwards arrow is drawn with a head at both
	// ends, then loses the head at its drawn end, leaving the one at the real target.
	var backwards, redBackwards, reds []string
	for i, l := range lines {
		back := partOrder[str(l, "from")] > partOrder[str(l, "to")] && backwardsArrows[str(l, "kind")] != ""
		broken := l.get("breaks") == true
		switch {
		case back && broken:
			redBackwards = append(redBackwards, fmt.Sprint(i))
		case back:
			backwards = append(backwards, fmt.Sprint(i))
		case broken:
			reds = append(reds, fmt.Sprint(i))
		}
	}
	if backwards != nil {
		out = append(out, fmt.Sprintf("  linkStyle %s marker-end:none", strings.Join(backwards, ",")))
	}
	if redBackwards != nil {
		out = append(out, fmt.Sprintf("  linkStyle %s %s,marker-end:none", strings.Join(redBackwards, ","), red))
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
	"code":      `("%s")`,
	"person":    `("%s")`,
	"review":    `("%s")`,                              // an action too, since its one way out is [yes]
	"fork":      `@{ shape: fork, label: "" }%.0s`,     // fork node, which shows no label
	"comment":   `@{ shape: notch-rect, label: "%s" }`, // comment
}

var classes = map[string]string{"trigger": "trig", "context": "ctx", "connector": "con", "surface": "surf", "outside": "out",
	"ai": "ai", "code": "code", "person": "person", "review": "person", "fork": "uml", "comment": "gap"}

var arrows = map[string]string{"start": "-.->", "data": "-->", "publish": "==>", "refused": "--x", "tie": "-.-", "hidden": "~~~"}

// Mermaid's layout follows an edge's written direction. For a line back to an earlier part, draw
// the edge in file order with two heads; linkStyle removes the head at the real source.
var backwardsArrows = map[string]string{"start": "<-.->", "data": "<-->", "publish": "<==>", "refused": "x--x"}

const defs = `  classDef trig fill:#fbe1ee,stroke:#a33a74,color:#16171b
  classDef ctx fill:#e2eef9,stroke:#3d6fa3,color:#16171b
  classDef con fill:#eceef2,stroke:#5d6472,color:#16171b
  classDef surf fill:#ffffff,stroke:#2a2d35,stroke-width:1.5px,color:#16171b
  classDef out fill:#fde4dc,stroke:#b5532f,color:#16171b
  classDef ai fill:#cdd3fb,stroke:#3b4bb0,color:#16171b
  classDef code fill:#c4e3d1,stroke:#2b7a4f,color:#16171b
  classDef person fill:#f7d79a,stroke:#9a6100,color:#16171b
  classDef uml fill:#16171b,stroke:#16171b,color:#16171b
  classDef gap fill:#ffffff,stroke:#8a93a3,stroke-dasharray:4 3,color:#6b7280`

const scope = "fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f"

var pastel = []string{"fill:#e3ebfa,stroke:#b9c8e8", "fill:#fbe9df,stroke:#ecc7b2", "fill:#efe6fa,stroke:#d3c2ea",
	"fill:#fbf3d9,stroke:#e8d9a6", "fill:#dff3ef,stroke:#b4ddd4", "fill:#f6e3ea,stroke:#e3bccb", "fill:#e9f2dc,stroke:#c7dcae"}

const red = "stroke:#d0342c,stroke-width:2.5px,color:#b0271f"

var header = []string{"---", "config:", "  theme: default",
	"  fontFamily: Inter, sans-serif", "  flowchart:", "    useMaxWidth: false",
	"    subGraphTitleMargin:", "      top: 6", "      bottom: 30", "---", "flowchart TB"}

// Who decides a step or review, as an emoji, so a reader needn't tell the colours apart. Code is a
// gear rather than a computer, since a computer reads as a machine, which the platform view is about.
var deciders = map[string]string{"ai": "🤖", "code": "⚙️", "person": "👤", "review": "👤"}

// A step's own line holds who decides it, then its operation's emoji and name if it has one,
// smaller and softer than the label beneath it. A review's holds ✅ review, which no operation uses.
const operation = "<small style='color:rgb(61,67,79)'>%s</small><br>"

// label is a part's label as Mermaid reads it. A step's or a review's sits under its own line.
func label(p *Map) string {
	who, words := deciders[str(p, "kind")], text(str(p, "label"))
	if who == "" {
		return words
	}
	if str(p, "kind") == "review" {
		who += " ✅ review"
	} else if op := str(p, "op"); op != "" {
		who += " " + ops[op] + " " + op
	}
	return fmt.Sprintf(operation, who) + words
}

// text keeps Mermaid from reading a label as Markdown or HTML. CommonMark syntax is made entirely
// from ASCII punctuation, so entity-encoding every such mark leaves no syntax for its parser while
// Mermaid still renders the original character. The operation HTML is added after this function.
func text(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r >= '!' && r <= '/' || r >= ':' && r <= '@' || r >= '[' && r <= '`' || r >= '{' && r <= '~' {
			fmt.Fprintf(&out, "#%d;", r)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
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
