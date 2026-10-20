import json
import re

from awml import draw, page


def doc(title, view="scopes", label="Plan"):
    return {"awml": "0.1", "title": title, "view": view, "parts": [{"id": "a", "kind": "ai", "label": label}]}


def drawings(html):
    return json.loads(re.search(r"var DRAWINGS = (\[.*?\]);\n", html).group(1))


def test_a_page_stands_alone_and_draws_with_mermaid_11_4_1():
    html = page([doc("Dinners")])
    assert html.startswith("<!doctype html>\n")
    assert '<script src="https://cdn.jsdelivr.net/npm/mermaid@11.4.1/dist/mermaid.min.js"></script>' in html
    assert "<title>Dinners</title>" in html


def test_a_page_holds_each_drawing_in_order():
    docs = [doc("Dinners"), doc("Dinners", view="where"), doc("A question")]
    html = page(docs)
    assert drawings(html) == [draw(d) for d in docs]
    assert html.count('<div class="awml"') == 3
    assert html.index("Who may see each part") < html.index("Where each part runs") < html.index("<h2>A question</h2>")


def test_each_drawing_is_on_a_white_panel():
    html = page([doc("Dinners")])
    assert re.search(r"\.panel \{[^}]*background: #ffffff", html)
    assert '<div class="panel"><div class="awml"' in html


def test_the_page_is_named_after_its_workflows():
    assert "<title>Dinners</title>" in page([doc("Dinners"), doc("Dinners", view="where")])
    assert "<title>Dinners, A question</title>" in page([doc("Dinners"), doc("A question")])


def test_titles_are_escaped():
    html = page([doc("Fish & <chips>")])
    assert "<h2>Fish &amp; &lt;chips&gt;</h2>" in html


def test_a_label_cant_end_the_script_early():
    d = doc("Dinners", label="</script><b>bold</b>")
    html = page([d])
    assert html.count("</script>") == 2
    assert drawings(html) == [draw(d)]
