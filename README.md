# AWML

AWML, the Agentic Workflow Modelling Language, describes agentic work as diagrams. You write a workflow as a small YAML file: its triggers, the steps that pass work along, the notes they read, and the surfaces and outside actions it ends in. AWML draws it with Mermaid, on the shapes of a UML activity diagram, coloured so you can see at a glance whether a model, a rule or a person decides each step. Dashed boxes show who may see each part, and a second view shows where each part runs. A drawing runs nothing and grants nothing: it's the description a live system is checked against, and a red line marks where the system does something the description doesn't allow.

![A family's dinners, by who may see each part](examples/dinners-scopes.svg)

A fictional family's dinners. On Thursday evening a model plans next week's dinners and posts the plan in the family chat. On Saturday a rule fills the supermarket basket, and checking out waits for a parent's yes to the exact basket. Every morning a rule matches the joint account against the bills expected, which only the parents see. The thick line is the meal plan moving out to a wider scope, so the carers can see it on the kitchen screen. The comment is a gap that drawing it found: nothing keeps that screen current. The same workflow, by where each part runs, is [`examples/dinners-where.svg`](examples/dinners-where.svg).

## Install

```sh
go install github.com/chrismdp/awml/cmd/awml@latest
```

This needs Go 1.22 or later, and puts `awml` in Go's own folder for commands, usually `~/go/bin`.

## Writing a file

One file holds one view of one workflow:

```yaml
awml: "0.1"
title: The family's dinners
view: scopes        # or: where
groups:
  - { id: carers, label: "carers: + June, Beth" }
  - { id: family, label: "family: parents, kids", in: carers }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: favourites, kind: context, label: "Favourites and recipes", in: family }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
  - { id: chat, kind: surface, label: "Family chat", in: family }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: favourites, to: plan, kind: data }
  - { from: plan, to: chat, kind: data, label: posted }
```

The part kinds are `trigger`, `ai`, `rule`, `human`, `review`, `fork`, `context`, `connector`, `surface`, `outside` and `comment`. The line kinds are `start`, `data`, `publish`, `refused`, `tie` and `hidden`, and `breaks: true` draws a line red. A trigger or comment with no `in` is drawn in the group of what it starts or explains. [`SPEC.md`](SPEC.md) gives every field and what each part and line means, and [`examples/`](examples) has three workflows to start from, two of them in both views. [`examples/client-email-where.svg`](examples/client-email-where.svg) shows one small workflow running across five platforms, which is how most agentic work is put together today.

## Drawing it

```sh
awml check examples/question.yaml        # report mistakes; exits 1 on any
awml draw examples/question.yaml         # print the Mermaid
awml page examples/dinners-*.yaml -o dinners.html
```

`page` writes one standalone HTML page that draws each file with Mermaid in the browser.

The Mermaid works anywhere that draws Mermaid 11.4.1. To make an SVG, use the Mermaid CLI, which needs Node:

```sh
awml draw my-workflow.yaml > my-workflow.mmd
npx -y @mermaid-js/mermaid-cli@11.4.1 -b white -i my-workflow.mmd -o my-workflow.svg
```

`examples/render.sh` redraws every example this way. The tests run with `go test ./...`.

## Status

Draft 0.1. The notation and the file format may still change.

## Licence

MIT. See `LICENSE`.
