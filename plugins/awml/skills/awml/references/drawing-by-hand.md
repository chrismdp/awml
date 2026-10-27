# Drawing a file by hand

Use this only where `awml draw` can't run, as in a chat with no shell. It gives the Mermaid that `awml draw` would print, so a hand drawing looks like every other AWML drawing. Check the file first, since `awml draw` refuses a file with mistakes. Say that you drew it by hand, and that `awml draw` gives the exact drawing.

Read `examples/dinners.yaml` beside `examples/dinners.mmd` for its security view and `examples/dinners-home.mmd` for its home implementation. Copy their form exactly.

For the security view, use `scopes` as groups and each part's `in` as its group. The untrusted view, for a file that marks a part `untrusted: true`, uses the same groups, and section 5 gives what it adds. For a platform view, use the chosen implementation's blocks as groups. Replace each part's scope with the block whose `parts` lists it, leaving unplaced triggers and design comments without a group. Keep the design's part and line order. Append implementation comments as comment parts in their blocks, then append their `tie` lines in comment and target order. The steps below work on these groups, parts and lines.

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

Write the top level first, indented two spaces. At each level, write the parts that sit there other than comments, in the file's order, then the comments that sit there, in the file's order, then each group that sits there, in the file's order. A part sits in the group its `in` names, or at the top level if it has none, except a trigger or comment with no `in`: it sits in the group of the part at the other end of its first line that isn't `hidden`.

A group is written as `subgraph id["label"]`, then `direction TB` on the next line, then what sits inside it indented two more spaces, then `end`.

A part is its id followed straight away by its node, with the label in place of `label`:

| Part `kind` | Class | Node, after the id |
|---|---|---|
| `trigger` | `trig` | `(["label"])` |
| `ai` | `ai` | `("label")` |
| `code` | `code` | `("label")` |
| `person` | `person` | `("label")` |
| `review` | `person` | `("label")` |
| `approval` | `person` | `("label")` |
| `fork` | `uml` | `@{ shape: fork, label: "" }` |
| `context` | `ctx` | `[("label")]` |
| `connector` | `con` | `{{"label"}}` |
| `surface` | `surf` | `@{ shape: win-pane, label: "label" }` |
| `outside` | `out` | `>"label"]` |
| `comment` | `gap` | `@{ shape: notch-rect, label: "label" }` |

In every label, of a part, a group or a line, write each run of line breaks, with the whitespace around it, as one space, and trim the label's ends, so each node stays on one row. Then write each ASCII punctuation mark as its decimal Mermaid entity: `@` as `#64;`, `.` as `#46;`, `"` as `#34;`, and so on. Leave letters, numbers, spaces and non-ASCII characters as written. This stops Mermaid reading email addresses, web addresses, lists or other Markdown instead of literal label text. A fork shows no label.

The label of a step, a review or a standing approval starts with a line of its own saying who decides it, `<small style='color:rgb(61,67,79)'>WHO</small><br>`, and then the label. `WHO` is 🤖 for `ai`, ⚙️ for `code` and 👤 for `person`. For a step with an `op`, add its emoji and its name, so an AI draft is `<small style='color:rgb(61,67,79)'>🤖 ✏️ draft</small><br>`. For a review, `WHO` is `👤 ✅ review`, and for a standing approval, `👤 📜 standing approval`.

In every view, a part with `untrusted: true` has a space and ⚠️ after its label, so an order form is `form(["An order on the website#39;s form ⚠️"])`.

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

First find each publish in the security view's scopes. Skip any line that isn't `data`. For each other line, find the group of the part at each end, as in step 2. A part in no group sits outside every group, and counts as wider than any of them. If the target's group is the source's group, or sits inside it, following its `in` upwards, the line narrows and stays `data`. Otherwise it publishes: draw it as `publish`, in the security view and in every platform view, and add to its label, after ` · `, what it does:

- who it adds: the people the target's scope holds that the source's doesn't, as `adds June and Beth`. Read a scope's people from its label, after the colon. A label starting with `+` holds the people it names after everyone in the scopes inside it.
- what it loosens: where both scopes have `processing` and the target's allows a company or an operation the source's doesn't, `loosens SOURCE to TARGET`, writing each as `any company`, `no company`, or its companies in alphabetical order followed by `only`, as `anthropic only` or `anthropic and openai for draft and sort only`.
- both, who it adds first, joined by ` · `.
- where neither can be said, `publishes into NAME`, the target scope's name before its colon, or `publishes outside every scope` when the target sits outside every group.

A line with no label of its own takes only what it does, as `adds an associate`.

After the parts and groups, leave one empty line, then write each line in the file's order, indented two spaces, from its `from` to its `to`, wherever its ends come in `parts`: `from ARROW to`, or with a label, `from ARROW|"label"| to`. A `tie` has no direction, so write it from the part to the comment, whichever way the file lists it, unless both ends are comments. A line is solid from one step to the next, and dotted when either end is a `context`, a `connector` or a `surface`.

| Line `kind` | Arrow | Dotted arrow |
|---|---|---|
| `start` | `-->` | `-.->` |
| `data` | `-->` | `-.->` |
| `publish` | `==>` | `==>` |
| `refused` | `--x` | `-.-x` |
| `tie` | `-.-` | `-.-` |
| `hidden` | `~~~` | `~~~` |

A dotted `publish` line keeps its thick arrow and takes its dots from its style, below.

A line is **red** when it breaks a rule: each line that rule 18, 19 or 21 of [checking by hand](checking-by-hand.md) finds. For 18, that's the lines from the step that isn't a `do` step on its way to the action; for 19, the line into the context; for 21, the lines from each part that breaks the rule on its way to the action. A red line's label adds, for each rule it breaks, `<br>` and the words `awml check` reports for it, up to the colon before its advice, in the order the rules come. For example, `part "write" leads to outside action "send", but its op is draft`. Write the label's own words first, if it has any, and write the punctuation in the added words as entities too, but not the `<br>`.

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

Last, style the lines. Count every line from 0 in the file's order, hidden lines included. A line's style is made of these parts, in this order, joined with commas:

1. For a red line, the red style `stroke:#d0342c,stroke-width:2.5px,color:#b0271f`. Otherwise, for a `tie` line, the grey `stroke:#8a93a3`.
2. For a dotted `publish` line, `stroke-dasharray:3`.

A line with neither has no style. Lines with the same style share one line, `  linkStyle 4,7 STYLE` with their numbers, and these lines come in the order of the first line each one styles. So a red dotted `publish` line and a red line are `  linkStyle 4 stroke:#d0342c,stroke-width:2.5px,color:#b0271f,stroke-dasharray:3` and then `  linkStyle 7 stroke:#d0342c,stroke-width:2.5px,color:#b0271f`.

## 5. The untrusted view

The untrusted view is the security view, with three changes. First find where the text of each part marked `untrusted: true` reaches. It reaches that part, and from any part it reaches it goes along each `start` or `data` line to the part at the other end, red lines included. A `review`, a `person` step or a `code` step with `cleans: true` stops it: the text goes no further than that part, and the part keeps its own class.

1. Each part that stops the text has a space and 🛡️ after its label, after any ⚠️.
2. After the ten `classDef` lines, add these two, unchanged:

   ```
     classDef taint fill:#ead9f5,stroke:#7b3fa0,color:#16171b
     classDef taintout fill:#f9cdc7,stroke:#d0342c,stroke-width:2.5px,color:#16171b
   ```

3. A reached `outside` part takes the class `taintout`, and any other reached part the class `taint`, in place of its own. Write the class lines in the order `trig`, `ctx`, `con`, `surf`, `out`, `ai`, `code`, `person`, `uml`, `gap`, `taint`, `taintout`.

Every group keeps the security view's style.
