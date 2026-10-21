# Checking a file by hand

Use this only where `awml check` can't run, as in a chat with no shell. It covers what `awml check` reports, as the last paragraph of [The file format](spec.md#the-file-format) and the limits under [Operations](spec.md#operations) set it out. Work through every item for each file, list each mistake with the part or line it is in, fix them all, then go through the list again. When you're done, tell the person the file has been checked by hand and not by `awml check`, and that `awml check FILE` will confirm it.

## The file

1. The file is one YAML mapping with `awml`, `title`, `view` and `parts`, and may have `groups` and `lines`. It has no other field.
2. `awml` is `"0.2"`, in quotes. `view` is `security` or `platform`.
3. `parts`, and `groups` and `lines` where given, are lists, and each item in them is a mapping.

## Groups and parts

4. Each group has `id` and `label`, and may have `in`. Nothing else.
5. Each part has `id`, `kind` and `label`, and may have `in` and `op`. A fork needn't have a label. Nothing else.
6. Each id is letters, digits and underscores, starting with a letter: `meal_plan`, never `meal-plan` or `2nd_plan`. No id is used twice, across groups and parts together.
7. Each part's `kind` is one of `trigger`, `ai`, `code`, `person`, `review`, `fork`, `context`, `connector`, `surface`, `outside` and `comment`.
8. Each `in` names a group in the same file. Follow each group's `in` upwards: it never comes back to the group it started from.
9. Only an `ai`, `code` or `person` part has an `op`, and the `op` is one of `bring-in`, `sort`, `learn`, `prepare`, `check`, `reflect`, `draft`, `do`, `answer` and `other`.

## Lines

10. Each line has `from`, `to` and `kind`, and may have `label` and `breaks`. Nothing else.
11. `from` and `to` each name a part in the same file, never a group.
12. Each line's `kind` is one of `start`, `data`, `publish`, `refused`, `tie` and `hidden`.
13. `breaks`, where given, is `true` or `false`, with no quotes.
14. A line with a comment at either end is a `tie` or a `hidden` line. A `tie` has a comment at one end or both.

## Text

15. Every `title`, `view`, `id`, `label`, `kind`, `in`, `from`, `to` and `op` is text. YAML reads some bare values as something else, so each of these is in double quotes: `true` and `false`, numbers such as `12` or `0.5`, and dates such as `2026-10-20`. Quote `yes`, `no`, `on`, `off` and times such as `19:00` too, which YAML 1.1 tools misread.
16. The file is YAML that reads at all. In the `{ id: …, label: … }` form the examples use, a comma ends a value and `: ` starts a new one, so quote any label with a comma, a colon followed by a space, a `#`, or a first character of `[`, `{`, `&`, `*`, `!`, `|`, `>`, `'`, `"`, `%` or `@`. `[yes]` is always quoted.

## Operations

For these two, skip any line with `breaks: true`, and any `hidden`, `refused` or `tie` line: only `start`, `data` and `publish` lines lead anywhere.

17. **Only a `do` step leads to an outside action.** For each `outside` part, walk back along the lines that lead into it. Pass through any review or fork and keep walking back from it. Every step you meet whose `op` is anything but `do` or `other` is a mistake, such as a `draft` step whose line reaches a send through a review. A step with no `op` is held to nothing.
18. **An `answer` or `check` step never writes to a context.** No line runs from a step whose `op` is `answer` or `check` to a `context`.
