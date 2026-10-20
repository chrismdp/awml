"""Find the structural mistakes in an AWML document: what would draw wrongly, or not at all."""
import re

from .draw import ARROWS, SHAPES

VERSION = "0.1"
VIEWS = ("scopes", "where")
FIELDS = {
    "file": ({"awml", "title", "view", "parts"}, {"groups", "lines"}),
    "group": ({"id", "label"}, {"in"}),
    "part": ({"id", "kind", "label"}, {"in"}),
    "line": ({"from", "to", "kind"}, {"label", "breaks"}),
}
# Mermaid reads anything else in an id as part of the syntax around it.
ID = re.compile(r"[A-Za-z][A-Za-z0-9_]*\Z")


def fields(what, name, item, errors):
    need, may = FIELDS[what]
    # A fork is a bare bar, so it needs no label.
    if what == "part" and item.get("kind") == "fork":
        need, may = need - {"label"}, may | {"label"}
    errors += [f"{name} has no {f}" for f in sorted(need) if f not in item]
    errors += [f'{name} has unknown field "{f}"' for f in item if f not in need | may]


def check(doc):
    if not isinstance(doc, dict):
        return ["the file must be a mapping of fields, starting with awml"]
    errors = []
    fields("file", "the file", doc, errors)
    if "awml" in doc and str(doc["awml"]) != VERSION:
        errors.append(f'awml must be "{VERSION}", not "{doc["awml"]}"')
    if "view" in doc and doc["view"] not in VIEWS:
        errors.append(f'view must be "scopes" or "where", not "{doc["view"]}"')
    lists, broken = [], []
    for key, what in (("groups", "group"), ("parts", "part"), ("lines", "line")):
        items = doc.get(key) or []
        if not isinstance(items, list):
            broken.append(f"{key} must be a list")
        elif not all(isinstance(i, dict) for i in items):
            broken.append(f"each {what} must be a mapping of fields")
        lists.append(items)
    # Past here every list is sound, or each line would report a part it can't find.
    if broken:
        return errors + broken
    groups, parts, lines = lists

    seen = set()
    for item in groups + parts:
        i = item.get("id")
        if i is None:
            continue
        if i in seen:
            errors.append(f'id "{i}" is used twice')
        seen.add(i)
        if not ID.match(str(i)):
            errors.append(f'id "{i}" must be letters, digits and underscores, starting with a letter')
    group_ids = {g.get("id") for g in groups}
    kinds = {p.get("id"): p.get("kind") for p in parts}

    for g in groups:
        name = f'group "{g.get("id")}"'
        fields("group", name, g, errors)
        if g.get("in") is not None and g["in"] not in group_ids:
            errors.append(f'{name} is in unknown group "{g["in"]}"')
    parent = {g.get("id"): g.get("in") for g in groups}
    for g in groups:
        at, path = parent.get(g.get("id")), set()
        while at is not None and at not in path and at in parent:
            if at == g.get("id"):
                errors.append(f'group "{g["id"]}" is nested in itself')
                break
            path.add(at)
            at = parent[at]

    for p in parts:
        name = f'part "{p.get("id")}"'
        fields("part", name, p, errors)
        if "kind" in p and p["kind"] not in SHAPES:
            errors.append(f'{name} has unknown kind "{p["kind"]}"')
        if p.get("in") is not None and p["in"] not in group_ids:
            errors.append(f'{name} is in unknown group "{p["in"]}"')

    for n, l in enumerate(lines, 1):
        name = f'the line from "{l["from"]}" to "{l["to"]}"' if "from" in l and "to" in l else f"line {n}"
        fields("line", name, l, errors)
        for end, verb in (("from", "starts"), ("to", "ends")):
            if end in l and l[end] not in kinds:
                errors.append(f'{name} {verb} at unknown part "{l[end]}"')
        kind = l.get("kind")
        if kind is None:
            continue
        if kind not in ARROWS:
            errors.append(f'{name} has unknown kind "{kind}"')
            continue
        comments = [l.get(end) for end in ("from", "to") if kinds.get(l.get(end)) == "comment"]
        # A hidden line only steers the layout, so it links nothing.
        if kind not in ("tie", "hidden"):
            errors += [f'comment "{c}" is linked by a {kind} line: a comment takes only tie' for c in comments]
        if kind == "tie" and not comments:
            errors.append(f'the tie from "{l.get("from")}" to "{l.get("to")}" has no comment at either end')
    return errors
