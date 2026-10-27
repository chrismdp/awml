package awml

import (
	"fmt"
	"regexp"
	"strings"
)

// Draw turns a workflow, or one of its views, into Mermaid, so every drawing uses the same shapes
// and colours. A workflow is drawn as its security view. It must have no mistakes but lines that
// break a rule: check it with Unsound first. It draws what GraphOf gives.
func Draw(doc *Map) string {
	return GraphOf(doc).Mermaid()
}

// drawnRed is whether a view draws a line red: one that View found breaking a rule.
func drawnRed(l *Map) bool {
	breaches, _ := l.get("breaks").([]any)
	return len(breaches) > 0
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
	"approval":  `("%s")`,                              // a standing approval: an action, like a review
	"fork":      `@{ shape: fork, label: "" }%.0s`,     // fork node, which shows no label
	"comment":   `@{ shape: notch-rect, label: "%s" }`, // comment
}

var classes = map[string]string{"trigger": "trig", "context": "ctx", "connector": "con", "surface": "surf", "outside": "out",
	"ai": "ai", "code": "code", "person": "person", "review": "person", "approval": "person", "fork": "uml", "comment": "gap"}

var arrows = map[string]string{"start": "-->", "data": "-->", "publish": "==>", "refused": "--x", "tie": "-.-", "hidden": "~~~"}

// The parts a line is dotted into or out of, since what they hold sits beside the path the work takes.
var beside = map[string]bool{"context": true, "connector": true, "surface": true}

// The dotted arrows. A publish line stays thick, and takes its dashes from linkStyle.
var dottedArrows = map[string]string{"start": "-.->", "data": "-.->", "refused": "-.-x"}

// A map's line between a workflow and a part it shares has a head at each end where lines in the
// workflow lead both into the part and out of it, and no head where none leads into it or out of it.
var twoWay = map[string]string{"-->": "<-->", "-.->": "<-.->"}
var headless = map[string]string{"-->": "---", "-.->": "-.-"}

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

// In the untrusted view, a part untrusted text reaches is tinted violet, and an outside action it
// reaches is drawn in the red of a red line.
const taint, taintOut = "fill:#ead9f5,stroke:#7b3fa0,color:#16171b", "fill:#f9cdc7,stroke:#d0342c,stroke-width:2.5px,color:#16171b"

// A tie is grey, the colour of a comment's border. Mermaid has no thick dotted arrow, so a dotted
// publish line is a thick one given dashes.
const grey, dotted = "stroke:#8a93a3", "stroke-dasharray:3"

var header = []string{"---", "config:", "  theme: default",
	"  fontFamily: Inter, sans-serif", "  flowchart:", "    useMaxWidth: false",
	"    subGraphTitleMargin:", "      top: 6", "      bottom: 30", "---", "flowchart TB"}

// Who decides a step or review, as an emoji, so a reader needn't tell the colours apart. Code is a
// gear rather than a computer, since a computer reads as a machine, which the platform view is about.
var deciders = map[string]string{"ai": "🤖", "code": "⚙️", "person": "👤", "review": "👤", "approval": "👤"}

// A step's own line holds who decides it, then its operation's emoji and name if it has one,
// smaller and softer than the label beneath it. A review's holds ✅ review, and a standing
// approval's 📜 standing approval, which no operation uses.
const operation = "<small style='color:rgb(61,67,79)'>%s</small><br>"

// lineBreak is a run of line breaks, with the whitespace around it.
var lineBreak = regexp.MustCompile(`[\s\v\x{85}\pZ]*[\n\r\x{2028}\x{2029}][\s\v\x{85}\pZ]*`)

// fold makes each run of line breaks in a label one space, and trims its ends. Mermaid ends a row at
// a line break, so a label that kept one would break its node's row, and one holding end on a row
// of its own would close its subgraph.
func fold(s string) string {
	return strings.TrimSpace(lineBreak.ReplaceAllString(s, " "))
}

// text keeps Mermaid from reading a label as Markdown or HTML, once its line breaks are folded.
// CommonMark syntax is made entirely from ASCII punctuation, so entity-encoding every such mark
// leaves no syntax for its parser while Mermaid still renders the original character. The operation
// HTML is added after this function.
func text(s string) string {
	var out strings.Builder
	for _, r := range fold(s) {
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
