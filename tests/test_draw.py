import pytest

from awml import draw


def doc(parts, lines=(), groups=(), view="scopes"):
    return {"awml": "0.1", "title": "T", "view": view, "groups": list(groups), "parts": list(parts), "lines": list(lines)}


def body(text):
    """The drawing without its config header."""
    return text.split("---\n", 2)[2]


@pytest.mark.parametrize("kind, node, cls", [
    ("trigger", 'p(["Label"])', "trig"),
    ("ai", 'p("Label")', "ai"),
    ("rule", 'p("Label")', "rule"),
    ("human", 'p("Label")', "human"),
    ("review", 'p{"Label"}', "human"),
    ("fork", 'p@{ shape: fork, label: "" }', "uml"),
    ("context", 'p[("Label")]', "ctx"),
    ("connector", 'p{{"Label"}}', "con"),
    ("surface", 'p@{ shape: win-pane, label: "Label" }', "surf"),
    ("outside", 'p>"Label"]', "out"),
    ("comment", 'p@{ shape: notch-rect, label: "Label" }', "gap"),
])
def test_each_part_kind_has_its_shape_and_colour(kind, node, cls):
    out = draw(doc([{"id": "p", "kind": kind, "label": "Label"}])) + "\n"
    assert f"\n  {node}\n" in out
    assert f"\n  class p {cls}\n" in out


def test_a_fork_needs_no_label():
    out = draw(doc([{"id": "f", "kind": "fork"}]))
    assert '\n  f@{ shape: fork, label: "" }\n' in out


@pytest.mark.parametrize("kind, arrow", [
    ("start", "-.->"), ("data", "-->"), ("publish", "==>"), ("refused", "--x"), ("tie", "-.-"), ("hidden", "~~~"),
])
def test_each_line_kind_has_its_arrow(kind, arrow):
    out = draw(doc([{"id": "a", "kind": "rule", "label": "A"}, {"id": "b", "kind": "comment", "label": "B"}],
                   [{"from": "a", "to": "b", "kind": kind}]))
    assert f"\n  a {arrow} b\n" in out


def test_a_line_carries_its_label():
    out = draw(doc([{"id": "a", "kind": "rule", "label": "A"}, {"id": "b", "kind": "surface", "label": "B"}],
                   [{"from": "a", "to": "b", "kind": "data", "label": "posted"}]))
    assert '\n  a -->|"posted"| b\n' in out


def test_quotes_in_labels_dont_break_the_drawing():
    out = draw(doc([{"id": "a", "kind": "rule", "label": 'Say "yes"'}, {"id": "b", "kind": "surface", "label": "B"}],
                   [{"from": "a", "to": "b", "kind": "data", "label": '"now"'}]))
    assert 'a("Say #quot;yes#quot;")' in out
    assert 'a -->|"#quot;now#quot;"| b' in out


def test_red_lines_are_counted_among_all_lines_hidden_ones_too():
    parts = [{"id": x, "kind": "rule", "label": x} for x in "abcd"]
    lines = [{"from": "a", "to": "b", "kind": "hidden"}, {"from": "b", "to": "c", "kind": "data", "breaks": True},
             {"from": "c", "to": "d", "kind": "data"}, {"from": "a", "to": "d", "kind": "publish", "breaks": True}]
    out = draw(doc(parts, lines))
    assert out.endswith("\n  linkStyle 1,3 stroke:#d0342c,stroke-width:2.5px,color:#b0271f")


def test_no_red_lines_means_no_link_style():
    out = draw(doc([{"id": "a", "kind": "rule", "label": "A"}, {"id": "b", "kind": "rule", "label": "B"}],
                   [{"from": "a", "to": "b", "kind": "data", "breaks": False}]))
    assert "linkStyle" not in out


def test_groups_nest_and_each_keeps_its_direction():
    out = draw(doc([{"id": "a", "kind": "context", "label": "Outer notes", "in": "outer"},
                    {"id": "b", "kind": "context", "label": "Inner notes", "in": "inner"}],
                   groups=[{"id": "outer", "label": "carers"}, {"id": "inner", "label": "family", "in": "outer"}]))
    assert body(out).startswith("""flowchart TB
  subgraph outer["carers"]
    direction TB
    a[("Outer notes")]
    subgraph inner["family"]
      direction TB
      b[("Inner notes")]
    end
  end
""")


def test_a_scope_is_a_dashed_box():
    out = draw(doc([], groups=[{"id": "g", "label": "family"}]))
    assert "\n  style g fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f" in out


def test_where_it_runs_is_a_tinted_block_each_in_the_next_tint():
    out = draw(doc([], groups=[{"id": f"g{i}", "label": "x"} for i in range(8)], view="where"))
    assert "\n  style g0 fill:#e3ebfa,stroke:#b9c8e8,color:#16171b" in out
    assert "\n  style g1 fill:#fbe9df,stroke:#ecc7b2,color:#16171b" in out
    assert "\n  style g7 fill:#e3ebfa,stroke:#b9c8e8,color:#16171b" in out
    assert "stroke-dasharray:6 4" not in out


def test_a_trigger_sits_in_the_group_of_what_it_starts():
    out = draw(doc([{"id": "t", "kind": "trigger", "label": "Thursday 19:00"},
                    {"id": "s", "kind": "ai", "label": "Plan", "in": "family"}],
                   [{"from": "t", "to": "s", "kind": "start"}], groups=[{"id": "family", "label": "family"}]))
    assert '  subgraph family["family"]\n    direction TB\n    t(["Thursday 19:00"])\n    s("Plan")\n  end' in out


def test_a_trigger_placed_by_hand_stays_put():
    out = draw(doc([{"id": "t", "kind": "trigger", "label": "Daily", "in": "a"},
                    {"id": "s", "kind": "ai", "label": "Plan", "in": "b"}],
                   [{"from": "t", "to": "s", "kind": "start"}], groups=[{"id": "a", "label": "a"}, {"id": "b", "label": "b"}]))
    assert '  subgraph a["a"]\n    direction TB\n    t(["Daily"])\n  end' in out


def test_a_comment_sits_in_the_group_of_what_it_is_tied_to():
    out = draw(doc([{"id": "s", "kind": "ai", "label": "Plan", "in": "g"},
                    {"id": "n", "kind": "comment", "label": "An example"}],
                   [{"from": "s", "to": "n", "kind": "tie"}], groups=[{"id": "g", "label": "g"}]))
    assert '    s("Plan")\n    n@{ shape: notch-rect, label: "An example" }\n  end' in out


def test_hidden_lines_dont_place_a_trigger():
    out = draw(doc([{"id": "t", "kind": "trigger", "label": "Daily"},
                    {"id": "s", "kind": "ai", "label": "Plan", "in": "g"}],
                   [{"from": "t", "to": "s", "kind": "hidden"}], groups=[{"id": "g", "label": "g"}]))
    assert body(out).startswith('flowchart TB\n  t(["Daily"])\n')


def test_other_parts_are_never_moved():
    out = draw(doc([{"id": "c", "kind": "context", "label": "Notes"},
                    {"id": "s", "kind": "ai", "label": "Plan", "in": "g"}],
                   [{"from": "c", "to": "s", "kind": "data"}], groups=[{"id": "g", "label": "g"}]))
    assert body(out).startswith('flowchart TB\n  c[("Notes")]\n')


QUESTION = {
    "awml": "0.1", "title": "A question in the family chat", "view": "scopes",
    "groups": [{"id": "zCarers", "label": "carers: + June, Beth"},
               {"id": "zFamily", "label": "family: parents, kids", "in": "zCarers"},
               {"id": "zNadia", "label": "nadia: Nadia", "in": "zFamily"}],
    "parts": [{"id": "tAsk", "kind": "trigger", "label": "Someone in the family chat asks what time swimming is"},
              {"id": "cCar", "kind": "context", "label": "Carers notes", "in": "zCarers"},
              {"id": "cFam", "kind": "context", "label": "Family notes", "in": "zFamily"},
              {"id": "sFind", "kind": "rule", "label": "Find the notes that speak to it", "in": "zFamily"},
              {"id": "sWord", "kind": "ai", "label": "Word the answer from those notes alone", "in": "zFamily"},
              {"id": "uChat", "kind": "surface", "label": "Family chat", "in": "zFamily"},
              {"id": "cNadia", "kind": "context", "label": "Nadia's own notes", "in": "zNadia"}],
    "lines": [{"from": "tAsk", "to": "sFind", "kind": "start"},
              {"from": "cFam", "to": "sFind", "kind": "data"},
              {"from": "cCar", "to": "sFind", "kind": "data"},
              {"from": "cNadia", "to": "sFind", "kind": "refused", "label": "never read here"},
              {"from": "sFind", "to": "sWord", "kind": "data"},
              {"from": "sWord", "to": "uChat", "kind": "data", "label": "the answer, naming its notes"}],
}

# The approved drawing of the question, as the working renderer made it.
APPROVED = """---
config:
  theme: default
  flowchart:
    useMaxWidth: false
    subGraphTitleMargin:
      top: 6
      bottom: 30
---
flowchart TB
  subgraph zCarers["carers: + June, Beth"]
    direction TB
    cCar[("Carers notes")]
    subgraph zFamily["family: parents, kids"]
      direction TB
      tAsk(["Someone in the family chat asks what time swimming is"])
      cFam[("Family notes")]
      sFind("Find the notes that speak to it")
      sWord("Word the answer from those notes alone")
      uChat@{ shape: win-pane, label: "Family chat" }
      subgraph zNadia["nadia: Nadia"]
        direction TB
        cNadia[("Nadia's own notes")]
      end
    end
  end

  tAsk -.-> sFind
  cFam --> sFind
  cCar --> sFind
  cNadia --x|"never read here"| sFind
  sFind --> sWord
  sWord -->|"the answer, naming its notes"| uChat
  classDef trig fill:#fbe1ee,stroke:#a33a74,color:#16171b
  classDef ctx fill:#e2eef9,stroke:#3d6fa3,color:#16171b
  classDef con fill:#eceef2,stroke:#5d6472,color:#16171b
  classDef surf fill:#ffffff,stroke:#2a2d35,stroke-width:1.5px,color:#16171b
  classDef out fill:#fde4dc,stroke:#b5532f,color:#16171b
  classDef ai fill:#cdd3fb,stroke:#3b4bb0,color:#16171b
  classDef rule fill:#c4e3d1,stroke:#2b7a4f,color:#16171b
  classDef human fill:#f7d79a,stroke:#9a6100,color:#16171b
  classDef uml fill:#16171b,stroke:#16171b,color:#16171b
  classDef gap fill:#ffffff,stroke:#8a93a3,stroke-dasharray:4 3,color:#6b7280
  class tAsk trig
  class cCar,cFam,cNadia ctx
  class uChat surf
  class sWord ai
  class sFind rule
  style zCarers fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f
  style zFamily fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f
  style zNadia fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f"""


def test_the_question_draws_as_the_approved_page_drew_it():
    assert draw(QUESTION) == APPROVED
