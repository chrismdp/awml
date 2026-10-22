package awml

import (
	"fmt"
	"strings"
	"testing"
)

// drawing draws a file of the given view whose groups, parts and lines are body.
func drawing(t *testing.T, view, body string) string {
	t.Helper()
	return Draw(parsed(t, fmt.Sprintf("awml: \"0.2\"\ntitle: T\nview: %s\n%s", view, body)))
}

// body is a drawing without its config header.
func body(text string) string {
	return strings.SplitN(text, "---\n", 3)[2]
}

func has(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("the drawing has no %q in\n%s", want, out)
	}
}

func TestEachPartKindHasItsShapeAndColour(t *testing.T) {
	for _, c := range []struct{ kind, node, class string }{
		{"trigger", `p(["Label"])`, "trig"},
		{"ai", `p("<small style='color:rgb(61,67,79)'>🤖</small><br>Label")`, "ai"},
		{"code", `p("<small style='color:rgb(61,67,79)'>⚙️</small><br>Label")`, "code"},
		{"person", `p("<small style='color:rgb(61,67,79)'>👤</small><br>Label")`, "person"},
		{"review", `p("<small style='color:rgb(61,67,79)'>👤 ✅ review</small><br>Label")`, "person"},
		{"fork", `p@{ shape: fork, label: "" }`, "uml"},
		{"context", `p[("Label")]`, "ctx"},
		{"connector", `p{{"Label"}}`, "con"},
		{"surface", `p@{ shape: win-pane, label: "Label" }`, "surf"},
		{"outside", `p>"Label"]`, "out"},
		{"comment", `p@{ shape: notch-rect, label: "Label" }`, "gap"},
	} {
		t.Run(c.kind, func(t *testing.T) {
			out := drawing(t, "security", fmt.Sprintf("parts: [{id: p, kind: %s, label: Label}]", c.kind)) + "\n"
			has(t, out, "\n  "+c.node+"\n")
			has(t, out, "\n  class p "+c.class+"\n")
		})
	}
}

func TestAForkNeedsNoLabel(t *testing.T) {
	has(t, drawing(t, "security", "parts: [{id: f, kind: fork}]"), "\n  f@{ shape: fork, label: \"\" }\n")
}

func TestEachLineKindHasItsArrow(t *testing.T) {
	for kind, arrow := range map[string]string{"start": "-.->", "data": "-->", "publish": "==>", "refused": "--x", "tie": "-.-", "hidden": "~~~"} {
		t.Run(kind, func(t *testing.T) {
			out := drawing(t, "security", fmt.Sprintf(`
parts: [{id: a, kind: code, label: A}, {id: b, kind: comment, label: B}]
lines: [{from: a, to: b, kind: %s}]`, kind))
			has(t, out, "\n  a "+arrow+" b\n")
		})
	}
}

func TestALineCarriesItsLabel(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: a, kind: code, label: A}, {id: b, kind: surface, label: B}]
lines: [{from: a, to: b, kind: data, label: posted}]`)
	has(t, out, "\n  a -->|\"posted\"| b\n")
}

func TestQuotesInLabelsDontBreakTheDrawing(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: a, kind: code, label: 'Say "yes"'}, {id: b, kind: surface, label: B}]
lines: [{from: a, to: b, kind: data, label: '"now"'}]`)
	has(t, out, `a("<small style='color:rgb(61,67,79)'>⚙️</small><br>Say #34;yes#34;")`)
	has(t, out, `a -->|"#34;now#34;"| b`)
}

func TestMarkdownPunctuationInEveryKindOfLabelIsEscaped(t *testing.T) {
	out := drawing(t, "security", `
groups: [{id: g, label: '# Team: noor@example.com'}]
parts:
  - {id: a, kind: code, label: 'Email noor@example.com and visit https://example.org/a', in: g}
  - {id: b, kind: comment, label: '**Keep** ~~this~~ [literal](https://example.com)', in: g}
  - {id: c, kind: surface, label: '1. First - # heading > quote', in: g}
lines: [{from: a, to: c, kind: data, label: 'Read https://example.com'}]`)
	has(t, out, `subgraph g["#35; Team#58; noor#64;example#46;com"]`)
	has(t, out, `a("<small style='color:rgb(61,67,79)'>⚙️</small><br>Email noor#64;example#46;com and visit https#58;#47;#47;example#46;org#47;a")`)
	has(t, out, `b@{ shape: notch-rect, label: "#42;#42;Keep#42;#42; #126;#126;this#126;#126; #91;literal#93;#40;https#58;#47;#47;example#46;com#41;" }`)
	has(t, out, `c@{ shape: win-pane, label: "1#46; First #45; #35; heading #62; quote" }`)
	has(t, out, `a -->|"Read https#58;#47;#47;example#46;com"| c`)
}

func TestMarkdownPunctuationIsAnEntityInLabelText(t *testing.T) {
	const punctuation = `!"#$%&'()*+,-./:;<=>?@[\]^_` + "`" + `{|}~`
	for _, mark := range punctuation {
		want := fmt.Sprintf("#%d;", mark)
		if got := text(string(mark)); got != want {
			t.Errorf("text(%q) = %q, want %q", mark, got, want)
		}
	}
}

func TestRedLinesAreCountedAmongAllLinesHiddenOnesToo(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: a, kind: code, label: a}, {id: b, kind: code, label: b}, {id: c, kind: code, label: c}, {id: d, kind: code, label: d}]
lines:
  - {from: a, to: b, kind: hidden}
  - {from: b, to: c, kind: data, breaks: true}
  - {from: c, to: d, kind: data}
  - {from: a, to: d, kind: publish, breaks: true}`)
	if !strings.HasSuffix(out, "\n  linkStyle 1,3 stroke:#d0342c,stroke-width:2.5px,color:#b0271f") {
		t.Errorf("the drawing ends\n%s", out[strings.LastIndex(out, "\n"):])
	}
}

func TestNoRedLinesMeansNoLinkStyle(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: a, kind: code, label: A}, {id: b, kind: code, label: B}]
lines: [{from: a, to: b, kind: data, breaks: false}]`)
	if strings.Contains(out, "linkStyle") {
		t.Errorf("a linkStyle in\n%s", out)
	}
}

func TestLinesBackToEarlierPartsAreReversedForLayoutWithTheRealArrowheadKept(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: first, kind: code, label: First}, {id: last, kind: code, label: Last}]
lines:
  - {from: last, to: first, kind: data}
  - {from: last, to: first, kind: start}
  - {from: last, to: first, kind: refused}
  - {from: last, to: first, kind: publish}
  - {from: last, to: first, kind: tie}
  - {from: last, to: first, kind: hidden}`)
	has(t, out, "\n  first <--> last\n")
	has(t, out, "\n  first <-.-> last\n")
	has(t, out, "\n  first x--x last\n")
	has(t, out, "\n  first <==> last\n")
	has(t, out, "\n  first -.- last\n")
	has(t, out, "\n  first ~~~ last\n")
	has(t, out, "\n  linkStyle 0,1,2,3 marker-end:none")
}

func TestAReversedRedLineKeepsItsColourAndOnlyItsRealArrowhead(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: first, kind: code, label: First}, {id: last, kind: code, label: Last}]
lines:
  - {from: first, to: last, kind: hidden}
  - {from: last, to: first, kind: data, label: back, breaks: true}
  - {from: first, to: last, kind: data, breaks: true}`)
	has(t, out, `first <-->|"back"| last`)
	if !strings.HasSuffix(out, "\n  linkStyle 1 "+red+",marker-end:none\n  linkStyle 2 "+red) {
		t.Errorf("the drawing ends\n%s", out[strings.LastIndex(out, "\n  linkStyle"):])
	}
}

func TestGroupsNestAndEachKeepsItsDirection(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: a, kind: context, label: Outer notes, in: outer}, {id: b, kind: context, label: Inner notes, in: inner}]
groups: [{id: outer, label: carers}, {id: inner, label: family, in: outer}]`)
	want := `flowchart TB
  subgraph outer["carers"]
    direction TB
    a[("Outer notes")]
    subgraph inner["family"]
      direction TB
      b[("Inner notes")]
    end
  end
`
	if !strings.HasPrefix(body(out), want) {
		t.Errorf("the drawing starts\n%s", body(out))
	}
}

func TestAScopeIsADashedBox(t *testing.T) {
	out := drawing(t, "security", "parts: []\ngroups: [{id: g, label: family}]")
	has(t, out, "\n  style g fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f")
}

func TestAPlatformIsATintedBlockEachInTheNextTint(t *testing.T) {
	groups := "groups:\n"
	for i := 0; i < 8; i++ {
		groups += fmt.Sprintf("  - {id: g%d, label: x}\n", i)
	}
	out := drawing(t, "platform", "parts: []\n"+groups)
	has(t, out, "\n  style g0 fill:#e3ebfa,stroke:#b9c8e8,color:#16171b")
	has(t, out, "\n  style g1 fill:#fbe9df,stroke:#ecc7b2,color:#16171b")
	has(t, out, "\n  style g7 fill:#e3ebfa,stroke:#b9c8e8,color:#16171b")
	if strings.Contains(out, "stroke-dasharray:6 4") {
		t.Errorf("a dashed box in a platform view")
	}
}

func TestATriggerSitsInTheGroupOfWhatItStarts(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: t, kind: trigger, label: "Thursday 19:00"}, {id: s, kind: ai, label: Plan, in: family}]
lines: [{from: t, to: s, kind: start}]
groups: [{id: family, label: family}]`)
	has(t, out, "  subgraph family[\"family\"]\n    direction TB\n    t([\"Thursday 19#58;00\"])\n    s(\"<small style='color:rgb(61,67,79)'>🤖</small><br>Plan\")\n  end")
}

func TestATriggerPlacedByHandStaysPut(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: t, kind: trigger, label: Daily, in: a}, {id: s, kind: ai, label: Plan, in: b}]
lines: [{from: t, to: s, kind: start}]
groups: [{id: a, label: a}, {id: b, label: b}]`)
	has(t, out, "  subgraph a[\"a\"]\n    direction TB\n    t([\"Daily\"])\n  end")
}

func TestACommentSitsInTheGroupOfWhatItIsTiedTo(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: s, kind: ai, label: Plan, in: g}, {id: n, kind: comment, label: An example}]
lines: [{from: s, to: n, kind: tie}]
groups: [{id: g, label: g}]`)
	has(t, out, "    s(\"<small style='color:rgb(61,67,79)'>🤖</small><br>Plan\")\n    n@{ shape: notch-rect, label: \"An example\" }\n  end")
}

func TestHiddenLinesDontPlaceATrigger(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: t, kind: trigger, label: Daily}, {id: s, kind: ai, label: Plan, in: g}]
lines: [{from: t, to: s, kind: hidden}]
groups: [{id: g, label: g}]`)
	if !strings.HasPrefix(body(out), "flowchart TB\n  t([\"Daily\"])\n") {
		t.Errorf("the drawing starts\n%s", body(out))
	}
}

func TestOtherPartsAreNeverMoved(t *testing.T) {
	out := drawing(t, "security", `
parts: [{id: c, kind: context, label: Notes}, {id: s, kind: ai, label: Plan, in: g}]
lines: [{from: c, to: s, kind: data}]
groups: [{id: g, label: g}]`)
	if !strings.HasPrefix(body(out), "flowchart TB\n  c[(\"Notes\")]\n") {
		t.Errorf("the drawing starts\n%s", body(out))
	}
}

func TestAStepsDeciderAndOperationAreAQuietLineAboveItsLabel(t *testing.T) {
	for _, c := range []struct{ kind, who string }{{"ai", "🤖"}, {"code", "⚙️"}, {"person", "👤"}} {
		out := drawing(t, "security", fmt.Sprintf(`parts: [{id: p, kind: %s, op: draft, label: 'Draft the "reply"'}]`, c.kind))
		has(t, out, "\n  p(\"<small style='color:rgb(61,67,79)'>"+c.who+" ✏️ draft</small><br>Draft the #34;reply#34;\")\n")
	}
}

func TestAStepWithNoOperationStillShowsWhoDecidesIt(t *testing.T) {
	for _, c := range []struct{ kind, who string }{{"ai", "🤖"}, {"code", "⚙️"}, {"person", "👤"}} {
		out := drawing(t, "security", fmt.Sprintf("parts: [{id: p, kind: %s, label: Step}]", c.kind))
		has(t, out, "\n  p(\"<small style='color:rgb(61,67,79)'>"+c.who+"</small><br>Step\")\n")
	}
}

func TestAReviewIsARoundedBoxWithItsOwnQuietLineAboveItsLabel(t *testing.T) {
	out := drawing(t, "security", `parts: [{id: p, kind: review, label: 'The "yes" to the reply'}]`)
	has(t, out, "\n  p(\"<small style='color:rgb(61,67,79)'>👤 ✅ review</small><br>The #34;yes#34; to the reply\")\n")
}

func TestEachOperationIsDrawnWithItsEmoji(t *testing.T) {
	for _, c := range []struct{ op, emoji string }{
		{"bring-in", "📥"},
		{"sort", "🗂️"},
		{"learn", "🧠"},
		{"prepare", "📋"},
		{"check", "🔍"},
		{"reflect", "🪞"},
		{"draft", "✏️"},
		{"do", "🛠️"},
		{"answer", "💬"},
		{"other", "❔"},
	} {
		out := drawing(t, "security", fmt.Sprintf("parts: [{id: p, kind: code, op: %s, label: Step}]", c.op))
		has(t, out, "\n  p(\"<small style='color:rgb(61,67,79)'>⚙️ "+c.emoji+" "+c.op+"</small><br>Step\")\n")
	}
}

const questionFile = `
awml: "0.2"
title: A question in the family chat
view: security
groups:
  - {id: zCarers, label: "carers: + June, Beth"}
  - {id: zFamily, label: "family: parents, kids", in: zCarers}
  - {id: zNadia, label: "nadia: Nadia", in: zFamily}
parts:
  - {id: tAsk, kind: trigger, label: Someone in the family chat asks what time swimming is}
  - {id: cCar, kind: context, label: Carers notes, in: zCarers}
  - {id: cFam, kind: context, label: Family notes, in: zFamily}
  - {id: sFind, kind: code, label: Find the notes that speak to it, in: zFamily}
  - {id: sWord, kind: ai, label: Word the answer from those notes alone, in: zFamily}
  - {id: uChat, kind: surface, label: Family chat, in: zFamily}
  - {id: cNadia, kind: context, label: "Nadia's own notes", in: zNadia}
lines:
  - {from: tAsk, to: sFind, kind: start}
  - {from: cFam, to: sFind, kind: data}
  - {from: cCar, to: sFind, kind: data}
  - {from: cNadia, to: sFind, kind: refused, label: never read here}
  - {from: sFind, to: sWord, kind: data}
  - {from: sWord, to: uChat, kind: data, label: "the answer, naming its notes"}
`

// The approved drawing of the question, as the working renderer made it.
const approved = `---
config:
  theme: default
  fontFamily: Inter, sans-serif
  flowchart:
    useMaxWidth: false
    subGraphTitleMargin:
      top: 6
      bottom: 30
---
flowchart TB
  subgraph zCarers["carers#58; #43; June#44; Beth"]
    direction TB
    cCar[("Carers notes")]
    subgraph zFamily["family#58; parents#44; kids"]
      direction TB
      tAsk(["Someone in the family chat asks what time swimming is"])
      cFam[("Family notes")]
      sFind("<small style='color:rgb(61,67,79)'>⚙️</small><br>Find the notes that speak to it")
      sWord("<small style='color:rgb(61,67,79)'>🤖</small><br>Word the answer from those notes alone")
      uChat@{ shape: win-pane, label: "Family chat" }
      subgraph zNadia["nadia#58; Nadia"]
        direction TB
        cNadia[("Nadia#39;s own notes")]
      end
    end
  end

  tAsk -.-> sFind
  cFam --> sFind
  cCar --> sFind
  sFind x--x|"never read here"| cNadia
  sFind --> sWord
  sWord -->|"the answer#44; naming its notes"| uChat
  classDef trig fill:#fbe1ee,stroke:#a33a74,color:#16171b
  classDef ctx fill:#e2eef9,stroke:#3d6fa3,color:#16171b
  classDef con fill:#eceef2,stroke:#5d6472,color:#16171b
  classDef surf fill:#ffffff,stroke:#2a2d35,stroke-width:1.5px,color:#16171b
  classDef out fill:#fde4dc,stroke:#b5532f,color:#16171b
  classDef ai fill:#cdd3fb,stroke:#3b4bb0,color:#16171b
  classDef code fill:#c4e3d1,stroke:#2b7a4f,color:#16171b
  classDef person fill:#f7d79a,stroke:#9a6100,color:#16171b
  classDef uml fill:#16171b,stroke:#16171b,color:#16171b
  classDef gap fill:#ffffff,stroke:#8a93a3,stroke-dasharray:4 3,color:#6b7280
  class tAsk trig
  class cCar,cFam,cNadia ctx
  class uChat surf
  class sWord ai
  class sFind code
  style zCarers fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f
  style zFamily fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f
  style zNadia fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f
  linkStyle 3 marker-end:none`

func TestTheQuestionDrawsAsTheApprovedPageDrewIt(t *testing.T) {
	if got := Draw(parsed(t, questionFile)); got != approved {
		t.Errorf("the question drew as\n%s", got)
	}
}
