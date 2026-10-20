"""Turn an AWML document into Mermaid, so every drawing uses the same shapes and colours."""

# UML activity shapes where Mermaid has them; our own where UML has nothing.
SHAPES = {
    "trigger": '(["{}"])',                                  # accept event or accept time event, drawn as a pill
    "context": '[("{}")]',                                  # data store node, drawn as a cylinder
    "connector": '{{{{"{}"}}}}',                            # connector: ours
    "surface": '@{{ shape: win-pane, label: "{}" }}',       # surface: ours
    "outside": '>"{}"]',                                    # send signal action, drawn as a flag
    "ai": '("{}")', "rule": '("{}")', "human": '("{}")',    # actions
    "review": '{{"{}"}}',                                   # decision node
    "fork": '@{{ shape: fork, label: "" }}',                # fork node
    "comment": '@{{ shape: notch-rect, label: "{}" }}',     # comment
}
CLASS = {"trigger": "trig", "context": "ctx", "connector": "con", "surface": "surf", "outside": "out",
         "ai": "ai", "rule": "rule", "human": "human", "review": "human", "fork": "uml", "comment": "gap"}
ARROWS = {"start": "-.->", "data": "-->", "publish": "==>", "refused": "--x", "tie": "-.-", "hidden": "~~~"}
DEFS = """  classDef trig fill:#fbe1ee,stroke:#a33a74,color:#16171b
  classDef ctx fill:#e2eef9,stroke:#3d6fa3,color:#16171b
  classDef con fill:#eceef2,stroke:#5d6472,color:#16171b
  classDef surf fill:#ffffff,stroke:#2a2d35,stroke-width:1.5px,color:#16171b
  classDef out fill:#fde4dc,stroke:#b5532f,color:#16171b
  classDef ai fill:#cdd3fb,stroke:#3b4bb0,color:#16171b
  classDef rule fill:#c4e3d1,stroke:#2b7a4f,color:#16171b
  classDef human fill:#f7d79a,stroke:#9a6100,color:#16171b
  classDef uml fill:#16171b,stroke:#16171b,color:#16171b
  classDef gap fill:#ffffff,stroke:#8a93a3,stroke-dasharray:4 3,color:#6b7280"""
SCOPE = "fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f"
PASTEL = ["fill:#e3ebfa,stroke:#b9c8e8", "fill:#fbe9df,stroke:#ecc7b2", "fill:#efe6fa,stroke:#d3c2ea",
          "fill:#fbf3d9,stroke:#e8d9a6", "fill:#dff3ef,stroke:#b4ddd4", "fill:#f6e3ea,stroke:#e3bccb", "fill:#e9f2dc,stroke:#c7dcae"]
RED = "stroke:#d0342c,stroke-width:2.5px,color:#b0271f"
HEADER = ["---", "config:", "  theme: default", "  flowchart:", "    useMaxWidth: false",
          "    subGraphTitleMargin:", "      top: 6", "      bottom: 30", "---", "flowchart TB"]


def text(s):
    # Mermaid ends a label at a double quote, and reads #quot; as one.
    return str(s).replace('"', "#quot;")


def placed(doc):
    """Each part's group. A trigger or comment with none sits in the group of what it starts or explains,
    so it doesn't stretch the drawing."""
    parts, lines = doc.get("parts") or [], doc.get("lines") or []
    home = {p["id"]: p.get("in") for p in parts}
    out = dict(home)
    for p in parts:
        if p.get("in") is None and p["kind"] in ("trigger", "comment"):
            out[p["id"]] = next((home.get(l["to"] if l["from"] == p["id"] else l["from"]) for l in lines
                                 if p["id"] in (l["from"], l["to"]) and l["kind"] != "hidden"), None)
    return out


def draw(doc):
    groups, parts, lines = doc.get("groups") or [], doc.get("parts") or [], doc.get("lines") or []
    where = placed(doc)
    out = list(HEADER)

    def emit(parent, indent):
        for p in parts:
            if where[p["id"]] == parent:
                out.append(f"{indent}{p['id']}{SHAPES[p['kind']].format(text(p.get('label', '')))}")
        for g in groups:
            if g.get("in") == parent:
                out.append(f'{indent}subgraph {g["id"]}["{text(g["label"])}"]')
                # Mermaid turns a box with no links outside it on its side unless told otherwise.
                out.append(f"{indent}  direction TB")
                emit(g["id"], indent + "  ")
                out.append(f"{indent}end")
    emit(None, "  ")
    out.append("")
    for l in lines:
        mid = f'|"{text(l["label"])}"|' if l.get("label") else ""
        out.append(f"  {l['from']} {ARROWS[l['kind']]}{mid} {l['to']}")
    out.append(DEFS)
    for cls in ("trig", "ctx", "con", "surf", "out", "ai", "rule", "human", "uml", "gap"):
        ids = [p["id"] for p in parts if CLASS[p["kind"]] == cls]
        if ids:
            out.append(f"  class {','.join(ids)} {cls}")
    for i, g in enumerate(groups):
        look = SCOPE if doc["view"] == "scopes" else f"{PASTEL[i % len(PASTEL)]},color:#16171b"
        out.append(f"  style {g['id']} {look}")
    # linkStyle counts every line, hidden ones too.
    reds = [str(i) for i, l in enumerate(lines) if l.get("breaks")]
    if reds:
        out.append(f"  linkStyle {','.join(reds)} {RED}")
    return "\n".join(out)
