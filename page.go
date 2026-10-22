package awml

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// Page writes one standalone HTML page that draws AWML documents with Mermaid in the browser. Each
// document must have no mistakes.
func Page(docs []*Map) string {
	docs = workflowViews(docs)
	// Two views of one workflow share its title; several workflows each have their own heading.
	titles := map[string]bool{}
	for _, d := range docs {
		titles[str(d, "title")] = true
	}
	name := "Workflows"
	if len(titles) == 1 {
		name = escape(str(docs[0], "title"))
	}
	out := []string{"<!doctype html>", `<html lang="en">`, `<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		"<title>" + name + "</title>", "<style>\n" + style + "\n</style>", "<main>", "<h1>" + name + "</h1>"}
	sources := make([]string, len(docs))
	for i, d := range docs {
		title, view := escape(str(d, "title")), views[str(d, "view")]
		if label := str(d, "implementationLabel"); label != "" {
			view += ": " + escape(label)
		}
		out = append(out, "<section>", "<h2>"+title+"</h2>", `<p class="view">`+view+"</p>",
			fmt.Sprintf(`<div class="panel"><div class="awml" role="img" aria-label="%s, %s"></div></div>`, title, strings.ToLower(view)),
			"</section>")
		sources[i] = quote(Draw(d))
	}
	out = append(out, "</main>")
	// Inside a script, "<" is the only thing that can end it early, so none is left in the drawings.
	drawings := strings.ReplaceAll("["+strings.Join(sources, ", ")+"]", "<", `\u003c`)
	out = append(out, strings.Replace(runtime, "var DRAWINGS = {};", "var DRAWINGS = "+drawings+";", 1))
	return strings.Join(out, "\n") + "\n"
}

var views = map[string]string{"security": "Security view: who may see each part", "platform": "Platform view: where each part runs"}

const style = `:root { color-scheme: light; }
body { margin: 0; background: #f6f7f9; color: #1a1c21; font: 16px/1.6 system-ui, "Segoe UI", sans-serif; }
main { max-width: 1500px; margin: 0 auto; padding: 32px 16px 64px; display: grid; gap: 32px; }
h1 { font-size: clamp(28px, 5vw, 40px); line-height: 1.1; margin: 0; }
h2 { font-size: 22px; line-height: 1.25; margin: 0; }
section { display: grid; gap: 10px; }
.view { margin: 0; color: #5b6270; }
.panel { box-sizing: border-box; background: #ffffff; border: 1px solid #dde1e7; border-radius: 10px; padding: 16px; overflow-x: auto; }
.awml { min-height: 120px; color: #3d434f; font-size: 14px; }
.awml svg { display: block; margin: 0 auto; }
@media (min-width: 900px) { .awml svg { max-width: 100%; height: auto; } }`

const runtime = `<script src="https://cdn.jsdelivr.net/npm/mermaid@11.4.1/dist/mermaid.min.js"></script>
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
</script>`

// escape makes text safe in HTML, quotes included.
var escape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;").Replace

// quote writes text as a JSON string in plain ASCII, with anything else as a \u escape, so the page
// reads the same in any encoding.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < ' ' || r > '~':
			if r > 0xffff {
				hi, lo := utf16.EncodeRune(r)
				fmt.Fprintf(&b, `\u%04x\u%04x`, hi, lo)
			} else {
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
