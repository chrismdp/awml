# Drawing a file by hand

Use this only where `awml draw` can't run, as in a chat with no shell. It gives the Mermaid that `awml draw` would print, so a hand drawing looks like every other AWML drawing. Check the file first, since `awml draw` refuses a file with mistakes. Say that you drew it by hand, and that `awml draw` gives the exact drawing.

Read one example and its drawing side by side before you start: `examples/dinners-security.yaml` with `examples/dinners-security.mmd`, and `examples/dinners-platform.yaml` with `examples/dinners-platform.mmd`. Copy their form exactly.

## 1. The header

Start with these lines, unchanged:

```
---
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
```

## 2. The parts and groups

Write the top level first, indented two spaces. At each level, write the parts that sit there, in the file's order, then each group that sits there, in the file's order. A part sits in the group its `in` names, or at the top level if it has none, except a trigger or comment with no `in`: it sits in the group of the part at the other end of its first line that isn't `hidden`.

A group is written as `subgraph id["label"]`, then `direction TB` on the next line, then what sits inside it indented two more spaces, then `end`.

A part is its id followed straight away by its node, with the label in place of `label`:

| Part `kind` | Class | Node, after the id |
|---|---|---|
| `trigger` | `trig` | `(["label"])` |
| `ai` | `ai` | `("label")` |
| `code` | `code` | `("label")` |
| `person` | `person` | `("label")` |
| `review` | `person` | `("label")` |
| `fork` | `uml` | `@{ shape: fork, label: "" }` |
| `context` | `ctx` | `[("label")]` |
| `connector` | `con` | `{{"label"}}` |
| `surface` | `surf` | `@{ shape: win-pane, label: "label" }` |
| `outside` | `out` | `>"label"]` |
| `comment` | `gap` | `@{ shape: notch-rect, label: "label" }` |

In every label, of a part, a group or a line, write each ASCII punctuation mark as its decimal Mermaid entity: `@` as `#64;`, `.` as `#46;`, `"` as `#34;`, and so on. Leave letters, numbers, spaces and non-ASCII characters as written. This stops Mermaid reading email addresses, web addresses, lists or other Markdown instead of literal label text. A fork shows no label.

The label of a step or a review starts with a line of its own saying who decides it, `<small style='color:rgb(61,67,79)'>WHO</small><br>`, and then the label. `WHO` is 🤖 for `ai`, ⚙️ for `code` and 👤 for `person`. For a step with an `op`, add its emoji and its name, so an AI draft is `<small style='color:rgb(61,67,79)'>🤖 ✏️ draft</small><br>`. For a review, `WHO` is `👤 ✅ review`.

| Emoji | `op` |
|---|---|
| 📥 | `bring-in` |
| 🗂️ | `sort` |
| 🧠 | `learn` |
| 📋 | `prepare` |
| 🔍 | `check` |
| 🪞 | `reflect` |
| ✏️ | `draft` |
| 🛠️ | `do` |
| 💬 | `answer` |
| ❔ | `other` |

## 3. The lines

After the parts and groups, leave one empty line, then write each line in the file's order, indented two spaces: `from ARROW to`, or with a label, `from ARROW|"label"| to`. Find the position of each end in `parts`. When `from` comes after `to`, swap the written ends and use the backwards arrow in the table. For `tie` and `hidden`, which have no arrowhead, swap the ends but keep the same line.

| Line `kind` | Arrow | Backwards arrow |
|---|---|---|
| `start` | `-.->` | `<-.->` |
| `data` | `-->` | `<-->` |
| `publish` | `==>` | `<==>` |
| `refused` | `--x` | `x--x` |
| `tie` | `-.-` | `-.-` |
| `hidden` | `~~~` | `~~~` |

## 4. The colours

Then write these lines, unchanged:

```
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
```

Then one line for each class that any part has, in the order `trig`, `ctx`, `con`, `surf`, `out`, `ai`, `code`, `person`, `uml`, `gap`, naming its parts in the file's order: `  class plan ai`, or `  class approve,edit person`.

Then one line for each group, in the file's order: `  style id STYLE`. In the security view, every group's `STYLE` is:

```
fill:transparent,stroke:#8a93a3,stroke-dasharray:6 4,color:#3d434f
```

In the platform view, the first group takes the first of these, the second group the second, and so on, starting again after the seventh, each followed by `,color:#16171b`:

1. `fill:#e3ebfa,stroke:#b9c8e8`
2. `fill:#fbe9df,stroke:#ecc7b2`
3. `fill:#efe6fa,stroke:#d3c2ea`
4. `fill:#fbf3d9,stroke:#e8d9a6`
5. `fill:#dff3ef,stroke:#b4ddd4`
6. `fill:#f6e3ea,stroke:#e3bccb`
7. `fill:#e9f2dc,stroke:#c7dcae`

Last, style backwards arrowed lines and red lines. Count every line from 0 in the file's order, hidden lines included. For backwards `start`, `data`, `publish` and `refused` lines, write `  linkStyle 4,7 marker-end:none` with their numbers: this hides the spare head and leaves the head pointing at the real `to` part. For lines with `breaks: true`, write `  linkStyle 4,7 STYLE` with their numbers and this `STYLE`:

```
stroke:#d0342c,stroke-width:2.5px,color:#b0271f
```

When a line is both backwards and red, merge the two on one line: `  linkStyle 4 stroke:#d0342c,stroke-width:2.5px,color:#b0271f,marker-end:none`.
