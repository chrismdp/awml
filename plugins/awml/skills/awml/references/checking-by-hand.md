# Checking a file by hand

Use this only where `awml check` can't run, as in a chat with no shell. It covers what `awml check` reports, as [The file format](spec.md#the-file-format), the limits under [Operations](spec.md#operations) and "Moving content" under [The security view](spec.md#the-security-view-who-may-see-each-part) set it out. Work through every item for each file, list each mistake with the part or line it is in, fix them all, then go through the list again. When you're done, tell the person the file has been checked by hand and not by `awml check`, and that `awml check FILE` will confirm it.

## The file

1. The file is one YAML mapping with `awml`, `title` and `parts`, and may have `description`, `scopes`, `lines` and `implementations`. It has no other field.
2. `awml` is `"0.10"`, in quotes. There is no `view`.
3. `parts`, and `scopes` and `lines` where given, are lists, and each item in them is a mapping.

## Scopes and parts

4. Each scope has `id` and `label`, and may have `in` and `processing`. Nothing else.
5. Each part has `id`, `kind` and `label`, and may have `in`, `op`, `wait`, `at`, `untrusted`, `cleans` and `ref`. Nothing else. `untrusted` and `cleans`, where given, are `true` or `false`, with no quotes. Only a `trigger`, `connector` or `context` has `untrusted: true`, and only a `code` step has `cleans: true`. Only a `context`, `connector`, `surface` or `outside` part has a `ref`, never a `trigger`, written `owner/slug#id` or as a path to a file alongside, ending `.yaml`, then `#` and the part's id, as `shopping.yaml#list`, and never naming its own file. Following refs from file to file ends at a copy with no `ref`, never coming back round. Where the file it names is to hand, that file has a part with that id. For an `outside` part, it's a `trigger`, the one the action starts. For any other, it's of the same kind, label and `at`, in a scope with the same people and terms.
6. Each id is letters, digits and underscores, starting with a letter: `meal_plan`, never `meal-plan` or `2nd_plan`. No id is used twice, across scopes, parts, blocks and implementation comments.
7. Each part's `kind` is one of `trigger`, `ai`, `code`, `person`, `review`, `approval`, `context`, `connector`, `surface`, `outside` and `comment`.
8. A design part or scope's `in` names a scope. Follow each scope's `in` upwards: it never comes back to the scope it started from.
9. Only an `ai`, `code` or `person` part has an `op`, and the `op` is one of `bring-in`, `sort`, `learn`, `prepare`, `check`, `reflect`, `draft`, `do`, `answer` and `other`.
## Lines

10. Each line has `from`, `to` and `kind`, and may have `label`. Nothing else.
11. `from` and `to` each name a part in the same file, never a group.
12. Each line's `kind` is one of `start`, `data`, `refused`, `tie` and `hidden`. There is no `publish`: a `data` line publishes whenever its scopes make it one.
13. No line has `breaks`. A line that breaks rule 18, 19, 21 or 23 is an error, and the drawing shows it red, so a file never marks one by hand.
14. A line with a comment at either end is a `tie` or a `hidden` line. A `tie` has a comment at one end or both.
15. No line but a `tie` or a `hidden` one goes into a `trigger`. A trigger only starts its own workflow, so to start another, a `do` step leads to an `outside` part whose `ref` names the other's trigger. Nor does any line but a `tie` or a `hidden` one go into a `context` from anything but an `ai`, `code` or `person` step. Only a step writes to a context, so a recorded sign-off is a `review` whose `[yes]` leads to a `code` `do` step that writes it, something brought in from outside is a `connector` that a `bring-in` step reads, then writes, and a copy from one context into another is a step that reads one and writes the other.

## Text

16. Every `title`, `description`, `id`, `label`, `kind`, `in`, `from`, `to`, `op` and `ref` is text. YAML reads some bare values as something else, so each of these is in double quotes: `true` and `false`, numbers such as `12` or `0.5`, and dates such as `2026-10-20`. Quote `yes`, `no`, `on`, `off` and times such as `19:00` too, which YAML 1.1 tools misread.
17. The file is YAML that reads at all. In the `{ id: …, label: … }` form the examples use, a comma ends a value and `: ` starts a new one, so quote any label with a comma, a colon followed by a space, a `#`, or a first character of `[`, `{`, `&`, `*`, `!`, `|`, `>`, `'`, `"`, `%` or `@`. `[yes]` is always quoted.

## Operations, tiers and untrusted text

For these, skip any `hidden`, `refused` or `tie` line: only `start` and `data` lines lead anywhere. A line that breaks rule 18, 19, 21 or 23 is an error. In a drawing of a live system, it may be what the system really did. It stays an error, and the drawing shows it red, so leave it there until the system changes. A mistake against rule 20 or 22, and every other mistake on this list, makes the file invalid, and every command refuses an invalid file, but a drawing with errors is valid, and `awml check` passes it, saying each error.

18. **Only a `do` step leads to an outside action.** For each `outside` part, walk back along the lines that lead into it. Pass through any review or standing approval and keep walking back from it. Every step you meet whose `op` is anything but `do` or `other` is a mistake, such as a `draft` step whose line reaches a send through a review. A step with no `op` is held to nothing.
19. **An `answer` or `check` step never writes to a context.** No line runs from a step whose `op` is `answer` or `check` to a `context`.
20. **Each outside action's label gives its tier.** The first word after the label's last colon is `Allow`, `Ask` or `Block`, as in `"Send the mail: Ask"` or `"Fill the basket: Allow under £180"`.
21. **Each tier agrees with what stands in front of it.** For each `outside` part, find the parts whose lines lead into it. For an Ask action, each of them is a `review`, an `approval` or a `person` step. For an Allow action, none of them is a `review` or an `approval`. For a Block action, there are none.
22. **Each standing approval lists its actions.** A line leads from each `approval` part straight to a `surface`.
23. **No untrusted text reaches an outside action unstopped.** From each part with `untrusted: true`, follow every line that leads on, through any part, but stop at a `review`, a `person` step or a `code` step with `cleans: true`, which stops the text. Each line you follow into an `outside` part is a mistake, unless it comes straight from an `approval`, which accepts the risk. Say which part the text came from, and that a review, a person's own step or a `cleans` step on the way would stop it, or a standing approval in front of the action would accept the risk. Where several files are given together, follow the text through the parts they share and from each `outside` part into the trigger its `ref` names.

## Warnings

These are warnings, which `awml check` prints without failing. Tell the person of each one you find. A trigger's **run** is every part you reach from it by following `start` lines forward, through any kind of part. A `data` line never carries a run on, even from one step straight to another.

24. **The file holds one workflow.** Put two triggers together when their runs share any part. If the triggers fall into more than one group, the file holds several workflows: split it at its triggers into files that refer to each other. A file with one trigger is one workflow.
25. **Every part is linked to a trigger.** From the triggers, follow every line either way, through any part, leaving out comments and their ties. Each part you never reach has no line linking it to a trigger: connect it or remove it.
26. **Every comment is tied to a part.** Each `comment` has a `tie` to a part. Tie it to what it explains, or remove it.
27. **Every step is started by a trigger.** Each `ai`, `code` or `person` step, `review` and `approval` is in some trigger's run, whether item 25 found it or not. For one that isn't, add the start line it's missing, or split its work into a workflow of its own that an outside action starts.
28. **No untrusted text reaches a surface unstopped.** Follow the text from each part with `untrusted: true` as item 23 does. Each `surface` you reach is a warning, unless every line you followed into it comes straight from an `approval`, whose line lists what went out. Say which part the text came from, that people reading the surface see text a stranger could have written, and that a review, a person's own step or a `cleans` step on the way would stop it.

## Placement and fit fields

A scope's `processing` is `any`, `none` or a map from company ids to `any` or known operations. An inner scope never permits more than its outer scope. Only a trigger has `wait`, a fixed ISO 8601 duration; only a context has `at`, a relative path with no `.` or `..` names.

`implementations` is a map from ids to entries with `label` and `blocks`, and optional `comments`. Each block has `id`, `label` and `parts`, a list of known design part ids. Only blocks may have `platform`, which is a company id or `owner/platforms/slug` for a published platform, with the owner and slug each lower-case letters, digits and hyphens, up to 40 characters, starting and ending with a letter or a digit. A block's optional `in` names another block in the same implementation, and following its parents never comes back to itself. Each part except a trigger or design comment appears in exactly one block per implementation. No part appears twice.

Each implementation comment has `id`, `label`, `in`, a block in that implementation, and `ties`, a list of known design part ids. It appears only in that implementation's drawing.
