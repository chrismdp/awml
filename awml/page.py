"""Write one standalone HTML page that draws AWML documents with Mermaid in the browser."""
import html
import json

from .draw import draw

VIEWS = {"scopes": "Who may see each part", "where": "Where each part runs"}
STYLE = """:root { color-scheme: light; }
body { margin: 0; background: #f6f7f9; color: #1a1c21; font: 16px/1.6 system-ui, "Segoe UI", sans-serif; }
main { max-width: 1500px; margin: 0 auto; padding: 32px 16px 64px; display: grid; gap: 32px; }
h1 { font-size: clamp(28px, 5vw, 40px); line-height: 1.1; margin: 0; }
h2 { font-size: 22px; line-height: 1.25; margin: 0; }
section { display: grid; gap: 10px; }
.view { margin: 0; color: #5b6270; }
.panel { box-sizing: border-box; background: #ffffff; border: 1px solid #dde1e7; border-radius: 10px; padding: 16px; overflow-x: auto; }
.awml { min-height: 120px; color: #3d434f; font-size: 14px; }
.awml svg { display: block; margin: 0 auto; }"""
RUNTIME = """<script src="https://cdn.jsdelivr.net/npm/mermaid@11.4.1/dist/mermaid.min.js"></script>
<script>
var DRAWINGS = {};
(async function () {
  var boxes = document.querySelectorAll(".awml");
  if (!window.mermaid) { boxes.forEach(function (el) { el.textContent = "The diagrams couldn't load. Reload the page to try again."; }); return; }
  mermaid.initialize({ startOnLoad: false, theme: "default", securityLevel: "strict" });
  for (var i = 0; i < boxes.length; i++) {
    try { var r = await mermaid.render("awml" + i, DRAWINGS[i]); boxes[i].innerHTML = r.svg; }
    catch (e) { boxes[i].textContent = "This diagram couldn't be drawn."; }
  }
})();
</script>"""


def page(docs):
    titles = list(dict.fromkeys(d["title"] for d in docs))
    out = ["<!doctype html>", '<html lang="en">', '<meta charset="utf-8">',
           '<meta name="viewport" content="width=device-width, initial-scale=1">',
           f"<title>{html.escape(', '.join(titles))}</title>", f"<style>\n{STYLE}\n</style>",
           "<main>", f"<h1>{html.escape(', '.join(titles))}</h1>"]
    for d in docs:
        title = html.escape(d["title"])
        out += ["<section>", f"<h2>{title}</h2>", f'<p class="view">{VIEWS[d["view"]]}</p>',
                f'<div class="panel"><div class="awml" role="img" aria-label="{title}: {VIEWS[d["view"]].lower()}"></div></div>',
                "</section>"]
    out.append("</main>")
    # Inside a script, "<" is the only thing that can end it early, so none is left in the drawings.
    sources = json.dumps([draw(d) for d in docs]).replace("<", "\\u003c")
    out.append(RUNTIME.replace("var DRAWINGS = {};", f"var DRAWINGS = {sources};"))
    return "\n".join(out) + "\n"
