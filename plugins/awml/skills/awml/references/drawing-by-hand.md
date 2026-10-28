# Drawing a file by hand

Use this only where `awml draw` can't run, as in a chat with no shell. It gives the Mermaid that `awml draw` would print, so a hand drawing looks like every other AWML drawing. Check the file first, since `awml draw` refuses a file with mistakes. Say that you drew it by hand, and that `awml draw` gives the exact drawing.

Read `examples/dinners.yaml` beside `examples/dinners.mmd` for its security view and `examples/dinners-home.mmd` for its home implementation. Copy their form exactly.

For the security view, use `scopes` as groups and each part's `in` as its group. For a platform view, use the chosen implementation's blocks as groups. Replace each part's scope with the block whose `parts` lists it, leaving unplaced triggers and design comments without a group. Keep the design's part and line order. Append implementation comments as comment parts in their blocks, then append their `tie` lines in comment and target order. The steps below work on these groups, parts and lines.

Several workflows given together are drawn as their map, which section 6 describes. Check them together first, since `awml draw` refuses them when following their refs finds a mistake.

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
| `context` | `ctx` | `[("label")]` |
| `connector` | `con` | `{{"label"}}` |
| `surface` | `surf` | `@{ shape: win-pane, label: "label" }` |
| `outside` | `out` | `>"label"]` |
| `comment` | `gap` | `@{ shape: notch-rect, label: "label" }` |

In every label, of a part, a group or a line, write each run of line breaks, with the whitespace around it, as one space, and trim the label's ends, so each node stays on one row. Then write each ASCII punctuation mark as its decimal Mermaid entity: `@` as `#64;`, `.` as `#46;`, `"` as `#34;`, and so on. Leave letters, numbers, spaces and non-ASCII characters as written. This stops Mermaid reading email addresses, web addresses, lists or other Markdown instead of literal label text.

The label of a step, a review or a standing approval starts with a line of its own saying who decides it, `<small style='color:rgb(61,67,79)'>WHO</small><br>`, and then the label. `WHO` is 🤖 for `ai`, ⚙️ for `code` and 👤 for `person`. For a step with an `op`, add its emoji and its name, so an AI draft is `<small style='color:rgb(61,67,79)'>🤖 ✏️ draft</small><br>`. For a review, `WHO` is `👤 ✅ review`, and for a standing approval, `👤 📜 standing approval`.

A part with `untrusted: true` is written like any other, and section 5 gives its outline.

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

A line is **red** when it breaks a rule: each line that rule 18, 19, 21 or 23 of [checking by hand](checking-by-hand.md) finds. For 18, that's the lines from the step that isn't a `do` step on its way to the action; for 19, the line into the context; for 21, the lines from each part that breaks the rule on its way to the action; for 23, each line that brings the text into the action. A red line's label adds, for each rule it breaks, `<br>` and the words `awml check` reports for it, up to the colon before its advice, in the order the rules come. For example, `part "write" leads to outside action "send", but its op is draft`. Write the label's own words first, if it has any, and write the punctuation in the added words as entities too, but not the `<br>`.

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
  classDef gap fill:#ffffff,stroke:#8a93a3,stroke-dasharray:4 3,color:#6b7280
```

Where any part has `untrusted: true`, add the two lines of section 5 here.

Then one line for each class that any part has, in the order `trig`, `ctx`, `con`, `surf`, `out`, `ai`, `code`, `person`, `gap`, `untrusted`, `unstopped`, naming its parts in the file's order: `  class plan ai`, or `  class approve,edit person`. A part is in the class of its kind, and a part with `untrusted: true` is in `untrusted` or `unstopped` too, as section 5 says.

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

1. For a red line, or a line section 5 makes red, the red style `stroke:#d0342c,stroke-width:2.5px,color:#b0271f`. Otherwise, for a line section 5 makes orange, `stroke:#d97a00,stroke-width:2.5px`; for one it makes green, `stroke:#109a74,stroke-width:2.5px`; and for a `tie` line, the grey `stroke:#8a93a3`.
2. For a dotted `publish` line, `stroke-dasharray:3`.

A line with neither has no style. Lines with the same style share one line, `  linkStyle 4,7 STYLE` with their numbers, and these lines come in the order of the first line each one styles. So a red dotted `publish` line and a red line are `  linkStyle 4 stroke:#d0342c,stroke-width:2.5px,color:#b0271f,stroke-dasharray:3` and then `  linkStyle 7 stroke:#d0342c,stroke-width:2.5px,color:#b0271f`.

## 5. Untrusted text

Every view of a file that marks a part `untrusted: true` shows where that part's text goes, in its lines and the part's outline. Follow the text of each such part. From any part it has reached, it goes along each `start` or `data` line to the part at the other end, red lines included. A `review`, a `person` step or a `code` step with `cleans: true` stops it: the text reaches that part, but goes no further. Where the text goes into an `outside` part straight from an `approval`, the standing approval accepts the risk. Any other line that brings the text into an `outside` part breaks rule 23 of [checking by hand](checking-by-hand.md).

1. Each line the text goes along is orange.
2. Each `start` or `data` line out of a part that stopped the text is green.
3. Each line that breaks rule 23 is red, and so is each line the text goes along on its way there from the part marked untrusted. Where the text is stopped on one branch but goes on to break rule 23 on another, the lines the branches share are red.
4. A line that is red for one part's text, or for one way, and orange or green for another, is red. Any other line is drawn as usual.
5. After the ten `classDef` lines, add these two, unchanged:

   ```
     classDef untrusted stroke:#d97a00,stroke-width:3px
     classDef unstopped stroke:#d0342c,stroke-width:3px
   ```

6. A part marked `untrusted: true` keeps the class of its kind, and is in `unstopped` too where its text breaks rule 23, or in `untrusted` otherwise. That gives it a thick red or orange outline over its own fill.

So an order form whose text reaches a printed label with nothing between has `  class form trig`, then later `  class form unstopped`.

On a map of several workflows, a part marked untrusted in any workflow's copy takes `untrusted` or `unstopped` the same way, following the text through the parts they share and from each outside action into the trigger it starts, and no line is coloured.

## 6. A map of several workflows

A map draws one box for each workflow and only what links them. Number the workflows in the order they're given. The first one's box is `w1`, the second's `w2`, and so on, and a part drawn for a workflow is named by its box and its id, as `w1_thu`.

A part is shared when a context, connector or surface in one workflow has a `ref` naming a part of another workflow given. Follow the refs to the copy that has none. That copy owns the part, and its label is the one drawn. Number the shared parts `s1`, `s2` and so on, in the order of the first copy of each, taking the workflows in the order given and each one's parts in its file's order. An outside action whose `ref` names the trigger of a workflow given starts that workflow, and is a hand-off. The map draws no other part.

After the header of section 1, write at the top level, indented two spaces, each workflow's triggers, taking the workflows in the order given and each one's triggers in its file's order, then each shared part in its order, each with its node from section 2. Then write each workflow's box: `subgraph w1["title"]`, with the workflow's `title` as its label, then `direction TB`, then each of its hand-offs in its file's order, indented two more spaces, then `end`. Write every label as section 2 says.

Leave one empty line, then write the lines, indented two spaces. None has a label.

1. For each workflow in the order given, a line from each of its triggers into its box, as `w1_thu --> w1`.
2. For each shared part in its order, a line for each workflow with a copy of it, in the order given. Read that workflow's own `start`, `data` and `publish` lines to and from its copy. Where some lead into the copy and none out of it, write `w1 -.-> s1`. Where some lead out and none in, write `s1 -.-> w1`. Where some lead each way, write `s1 <-.-> w1`, and where none do, `w1 -.- s1`. A shared part is a context, a connector or a surface, so each of these lines is dotted.
3. For each hand-off, in the order of the workflows and their parts, a line from the action to the trigger it starts, as `w1_start --> w2_low`.

Then write the `classDef` lines of section 4, and the two of section 5 where any part is marked untrusted as section 5 says for a map. Then the `class` lines, as section 4 says, naming the triggers and hand-offs in the order of their workflows and then the shared parts in their order, as `  class w1_close,w2_low trig`. Last, write one line for each box, in the order given. A box is a workflow, not a scope, so it's solid:

```
  style w1 fill:#f6f7f9,stroke:#5d6472,color:#16171b
```

No line on a map is styled, so a map has no `linkStyle`.
