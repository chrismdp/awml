# Checking a file by hand

Use this only where `awml check` can't run, as in a chat with no shell. It covers what `awml check` reports, as [The file format](spec.md#the-file-format), the limits under [Operations](spec.md#operations) and "Moving content" under [The security view](spec.md#the-security-view-who-may-see-each-part) set it out. Work through every item for each file, list each mistake with the part or line it is in, fix them all, then go through the list again. When you're done, tell the person the file has been checked by hand and not by `awml check`, and that `awml check FILE` will confirm it.

## The file

1. The file is one YAML mapping with `awml`, `title` and `parts`, and may have `scopes`, `lines` and `implementations`. It has no other field.
2. `awml` is `"0.6"`, in quotes. There is no `view`.
3. `parts`, and `scopes` and `lines` where given, are lists, and each item in them is a mapping.

## Scopes and parts

4. Each scope has `id` and `label`, and may have `in` and `processing`. Nothing else.
5. Each part has `id`, `kind` and `label`, and may have `in`, `op`, `wait`, `at`, `untrusted` and `cleans`. A fork needn't have a label. Nothing else. `untrusted` and `cleans`, where given, are `true` or `false`, with no quotes. Only a `trigger`, `connector` or `context` has `untrusted: true`, and only a `code` step has `cleans: true`.
6. Each id is letters, digits and underscores, starting with a letter: `meal_plan`, never `meal-plan` or `2nd_plan`. No id is used twice, across scopes, parts, blocks and implementation comments.
7. Each part's `kind` is one of `trigger`, `ai`, `code`, `person`, `review`, `approval`, `fork`, `context`, `connector`, `surface`, `outside` and `comment`.
8. A design part or scope's `in` names a scope. Follow each scope's `in` upwards: it never comes back to the scope it started from.
9. Only an `ai`, `code` or `person` part has an `op`, and the `op` is one of `bring-in`, `sort`, `learn`, `prepare`, `check`, `reflect`, `draft`, `do`, `answer` and `other`.
## Lines

10. Each line has `from`, `to` and `kind`, and may have `label`. Nothing else.
11. `from` and `to` each name a part in the same file, never a group.
12. Each line's `kind` is one of `start`, `data`, `refused`, `tie` and `hidden`. There is no `publish`: a `data` line publishes whenever its scopes make it one.
13. No line has `breaks`. A line that breaks rule 17, 18 or 20 is a mistake, and the drawing shows it red, so a file never marks one by hand.
14. A line with a comment at either end is a `tie` or a `hidden` line. A `tie` has a comment at one end or both.

## Text

15. Every `title`, `id`, `label`, `kind`, `in`, `from`, `to` and `op` is text. YAML reads some bare values as something else, so each of these is in double quotes: `true` and `false`, numbers such as `12` or `0.5`, and dates such as `2026-10-20`. Quote `yes`, `no`, `on`, `off` and times such as `19:00` too, which YAML 1.1 tools misread.
16. The file is YAML that reads at all. In the `{ id: …, label: … }` form the examples use, a comma ends a value and `: ` starts a new one, so quote any label with a comma, a colon followed by a space, a `#`, or a first character of `[`, `{`, `&`, `*`, `!`, `|`, `>`, `'`, `"`, `%` or `@`. `[yes]` is always quoted.

## Operations and tiers

For these, skip any `hidden`, `refused` or `tie` line: only `start` and `data` lines lead anywhere. In a drawing of a live system, a line that breaks one of these rules may be what the system really did. It stays a mistake, and the drawing shows it red, so leave it there until the system changes.

17. **Only a `do` step leads to an outside action.** For each `outside` part, walk back along the lines that lead into it. Pass through any review, standing approval or fork and keep walking back from it. Every step you meet whose `op` is anything but `do` or `other` is a mistake, such as a `draft` step whose line reaches a send through a review. A step with no `op` is held to nothing.
18. **An `answer` or `check` step never writes to a context.** No line runs from a step whose `op` is `answer` or `check` to a `context`.
19. **Each outside action's label gives its tier.** The first word after the label's last colon is `Allow`, `Ask` or `Block`, as in `"Send the mail: Ask"` or `"Fill the basket: Allow under £180"`.
20. **Each tier agrees with what stands in front of it.** For each `outside` part, find the parts whose lines lead into it, passing back through any fork. For an Ask action, each of them is a `review`, an `approval` or a `person` step. For an Allow action, none of them is a `review` or an `approval`. For a Block action, there are none.
21. **Each standing approval lists its actions.** A line leads from each `approval` part straight to a `surface`.

## Placement and fit fields

A scope's `processing` is `any`, `none` or a map from company ids to `any` or known operations. An inner scope never permits more than its outer scope. Only a trigger has `wait`, a fixed ISO 8601 duration; only a context has `at`, a relative path with no `.` or `..` names.

`implementations` is a map from ids to entries with `label` and `blocks`, and optional `comments`. Each block has `id`, `label` and `parts`, a list of known design part ids. Only blocks may have `platform`, which is a company id or `owner/platforms/slug` for a published platform, with the owner and slug each lower-case letters, digits and hyphens, up to 40 characters, starting and ending with a letter or a digit. A block's optional `in` names another block in the same implementation, and following its parents never comes back to itself. Each part except a trigger or design comment appears in exactly one block per implementation. No part appears twice.

Each implementation comment has `id`, `label`, `in`, a block in that implementation, and `ties`, a list of known design part ids. It appears only in that implementation's drawing.
