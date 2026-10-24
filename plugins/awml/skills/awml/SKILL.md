---
name: awml
description: Describe, check and draw agentic and AI workflows in AWML, the Agentic Workflow Modelling Language. Works out the workflow with the person, writes its design and implementations in one AWML YAML file, checks it with awml check, draws them as Mermaid, and points out what the drawing reveals, such as content published to a wider audience without a yes. Use it whenever someone wants to describe, map, draw, diagram, document or audit an agentic workflow, an AI workflow, an AI agent's process or an automation that uses AI, even if they never say AWML. Also use it whenever they mention AWML or the awml command, or have an AWML YAML file to write, fix, check or explain.
license: MIT
---

# AWML

AWML describes one workflow in one YAML file, with a design and its implementations, and draws its views as Mermaid diagrams on the shapes of a UML activity diagram. A drawing runs nothing and grants nothing: it is the description a live system is checked against. Your job is to get the person's workflow into AWML correctly, then show them what the drawing reveals.

Before you write or fix a file, read `references/spec.md` in full. It is short, and it is the authority: wherever this skill and the spec differ, the spec is right. Then read the examples nearest the person's workflow, in both views:

- `references/examples/client-email.yaml`: a consultant's email, with two scopes side by side and a send the consultant does themselves.
- `references/examples/dinners.yaml`: a family's dinners and money, with nested scopes, a review before checkout, a standing rule and a gap.

Each example's security drawing sits beside it as `.mmd`, and each implementation drawing adds `-ID.mmd`. The question in `references/examples/question.yaml` and photos in `references/examples/tour/09-platform-view.yaml` give smaller examples.

## 1. Find out the workflow

If the person already runs the workflow, as scheduled tasks, an agent or automations, find it and trace a real run with steps 1 to 4 of `references/guide.md` before you write anything. If they've named the workflow they want, that's the one step 3 picks. Wherever you can reach the live sources, such as the list of scheduled tasks, the scripts and the run records, read them yourself rather than asking the person. The guide's `SPEC.md` is `references/spec.md`, and its examples are in `references/examples/`.

You need each of these, and each becomes part of the file:

| Find out | It becomes |
|---|---|
| What starts it: a time, or an event such as an email arriving | a `trigger` |
| Each step, and who decides it: AI, code (which does the same thing every time) or a person | an `ai`, `code` or `person` step, with its `op` |
| The notes each step reads and writes | a `context` for each folder of notes, joined by `data` lines |
| The outside services it reaches, and whose access each one is | a `connector`, labelled with whose it is |
| Where people see the results, such as a chat, a list or a screen | a `surface` |
| What it changes in the world, and the tier of each change | an `outside` action, its label ending with its tier, as `"Send the reply: Ask"` |
| Who said yes once to a task's actions, to which kinds and within what limits | an `approval`, labelled with whose it is and its limits |
| Who may see what: each group of people, its members, and which companies may process its content | a scope in `scopes` |
| Where each part runs: the machines and services, the model each uses, and when each machine is on | a block in an implementation |

Start from what the person has already told you. Infer what you reasonably can, ask only for what's still missing, and say plainly what you inferred so they can correct it. A first draft they can react to often gets better answers than a long list of questions, so ask a few questions at most before you write one. If they run several workflows, draw one with them at a time, or a few that hand work to each other, each in its own conversation, and ask that workflow's questions as you reach them, never as one long list across workflows.

Some judgements need care:

- **Code or AI.** A step is code if it gives the same output for the same input every time, such as copying, filtering by a rule or matching against a list. It's AI if it makes a judgement call. If you can't tell, ask.
- **Allow or Ask.** An action is Allow only if it can be undone when it's taken, is seen only within its scope, and stays within the design's spending limits. Anything else is Ask, unless it is blocked. A send to a person, a payment and anything public are never Allow, and no standing approval covers them. An Ask action needs a person in front of it in the drawing: a `review`, an `approval` or the person's own `do` step.
- **Delivery isn't an outside action.** Posting to a shared surface for the people it serves, such as the family chat, is a `data` line to a `surface`, with no flag and no review.
- **Scopes.** Name each with its members, as `family: parents, kids`, or with `+` and the people it adds to the scope inside it, as `carers: + June, Beth`. Ask which companies may process a scope's content where it matters, as for something told in confidence.
- **Names.** Use the names the person uses. If the files will be shared or published, ask whether to use roles or made-up names instead.

## 2. Write one workflow file

Write the design first, then add its implementations in the same file. Name it after the workflow, as `standup.yaml`, and save it where the person keeps their work. In a chat with no files, give it in a `yaml` code block.

Start with a comment saying what the file shows, then `awml: "0.6"`, `title`, `scopes`, `parts` and `lines`. Write the parts in the order the work flows. Quote every label. Ids are short words joined by underscores, such as `meal_plan`. Give each step an `op` where one fits, using the spec's table of operations for what each produces and must not do. A scope's `processing`, a trigger's `wait` and a context's `at` let `awml fit` judge where the design can run. Mark a trigger, connector or context that takes in text anyone can write `untrusted: true`, and a code step that passes on only what's safe from it `cleans: true`, as the spec's "Untrusted text" says.

Add `implementations` as a map of ids to entries with a `label` and `blocks`. Each block has `id`, `label` and `parts`, a list of design part ids; it may have `platform`, the id of its platform description, and `in`, its parent block. Every part except a trigger or comment needs exactly one block per implementation. Keep the design's parts and lines once. An implementation may add `comments`, each with `id`, `label`, `in`, its block, and `ties`, the design part ids it explains. Scope, part, block and comment ids must be unique across the file.

**In the security view**, each scope is a box. The spec's "Where a part sits" is the rule:

- A context sits in its scope's box, and a surface in the box of the scope its viewers make up.
- A step sits in the box of the scope it writes into, with the reviews and outside actions it leads to.
- A standing approval sits in the box of the scope of the person who gives it.
- A connector sits in the box of the scope its inputs take: who could see what it brings in.
- One box goes inside another only when every member of the inner scope is in the outer one, and its terms are at least as strict. Otherwise the boxes sit side by side.
- A trigger or comment may have no `in`, and is then drawn beside what it starts or explains.

Choose each line's kind by what it does:

- `start` runs from a trigger to the step it starts.
- `data` moves content within a scope, or into a narrower one.
- `publish` moves content into a wider scope: a box it isn't already inside, or one with looser terms. Label it with the standing rule that allows it and whose words it moves, who must have agreed to that target, as `"co-op notes, under the bakery's rule, for customers who ticked the box"`, or say what yes releases it, as `"the consultant takes the reply"`.
- `refused` is a read from a narrower scope that the design refuses, with a label such as `never read here`.
- The one line out of each review is labelled `"[yes]"`.
- A standing approval sits between the `do` step and the actions it covers, with a `data` line to the surface where each action is listed with how to undo it.
- `tie` links a comment to what it explains, and `hidden` only steers the layout.

**In the platform view**, each implementation block is a platform: a machine or service that brings its own surfaces, harness and models, and often its own triggers.

- Each part sits in the block of the platform it runs on or lives in. A trigger sits where it is fired, a context where its notes are kept, a connector in the block of the service it reaches, a review where the person gives their yes, and a standing approval where its limits are kept.
- Label each block with what the implementation chose there, such as its model, and when the machine is on, if that isn't always: `"Server: claude-sonnet-4-5"`, `"Noor's laptop: on some evenings"`.
- A model on its own isn't a platform: it's a choice a platform makes.

## 3. Check

Check every file you write or change, and fix every mistake before you show a drawing.

If you have a shell, see whether `awml` is there and which release it is with `awml --version`. If it isn't, tell the person you're installing it, then run `go install github.com/chrismdp/awml/cmd/awml@latest`, which needs Go 1.22 or later and puts `awml` in `~/go/bin`. Without Go, download the archive for the system from https://github.com/chrismdp/awml/releases/latest, unpack it and run the `awml` inside. Then check the file:

```sh
awml check standup.yaml
```

It prints `ok` for each sound file, or one line for each mistake, naming the part or line it is in. Fix every one and run it again until the file prints `ok`.

If you have no shell, or `awml` can't be installed, go through `references/checking-by-hand.md` for each file instead. Then say plainly that the files were checked by hand, not by `awml check`, and that `awml check` will confirm them.

In the security view, `awml check` also reports publishing drawn as routing: a `data` line into a box that isn't the source's or one inside it. It reads the boxes, not their labels, so it can't see a box drawn in the wrong place. So look at each box inside another yourself: if it adds anyone, or its terms are looser, it belongs beside the other, and a `data` line into it is really publishing. Drawn inside the business, the consultant's private scope would hide that taking the reply there loosens the terms it is kept under.

## 4. Draw

- `awml draw standup.yaml` prints the file's Mermaid, which needs Mermaid 11.4.1 or later. Where the host renders Mermaid, show it in a `mermaid` code block.
- `awml draw --implementation ID standup.yaml` prints that implementation's platform view.
- `awml draw --untrusted standup.yaml` prints the untrusted view, for a file that marks a part untrusted.
- `awml page standup.yaml -o standup.html` writes one standalone HTML page with the security view and every implementation, to share or open in a browser.
- For an SVG, save the Mermaid to `standup.mmd` and run `npx -y @mermaid-js/mermaid-cli@11.4.1 -b white -i standup.mmd -o standup.svg`, which needs Node.

Without `awml`, draw the Mermaid by hand with `references/drawing-by-hand.md`, and say it was drawn by hand.

## 5. Say what the drawing reveals

Drawing a workflow shows what a list of its tasks hides. Go through the drawing with the person, asking of it the questions in step 10 of `references/guide.md`, and of the trace too if you made one. For each publishing line, say what crosses, from which scope to which, and who gains sight of it or which terms loosen.

Say too how the work is shared out: how many steps AI, code and a person each decide.

With the person's agreement, mark each gap in the files as a comment, as `"Missing from the design: nothing keeps the screen current"`, tied to the part it is about, then check and draw again. A red line, `breaks: true`, is only for something a live system did that its design doesn't allow, found in its records. Never use one for a gap in the design.

The files stay where the person keeps their work; if they want to share the workflow, the awml-publish skill can publish it.

## Common mistakes

- **A draft step leading to an outside action**, even through a review. `awml check` says `only a do step leads to an outside action`. Put a `do` step between the draft and the review, such as code that asks the person to send it, or make the person's own step `op: do` when they edit and send it themselves.
- **Publishing drawn as routing.** A `data` line into a box that isn't the source's or one inside it moves content to more people or under looser terms. `awml check` says `draw it as publish, or as refused if the design refuses the read`. Make it a `publish` line, labelled with the standing rule or the yes that releases it, or a `refused` read. If the move really narrows, the target's box belongs inside the source's.
- **An Ask action with nobody in front of it.** `awml check` says `no person in between`. Put a review or a standing approval between the `do` step and the action, or make the person's own step take it. A standing approval only named in the action's label or a comment doesn't count.
- **A review used as a decision.** A review has one way out, labelled `"[yes]"`. Draw no `[no]` line and no second branch, since a review isn't a decision node. If a person changes the content, that is their own step, not a review.
- **An `answer` or `check` step writing to a context.** They read contexts and speak on surfaces. A step that files what it found is a different step, such as `learn` or `sort`.
- **Unquoted values YAML reads as something else.** A date or a number as a label gives `YAML read its label as …, not text: put it in quotes`. An unquoted label with `: ` in it isn't YAML at all, one with a comma is split in two and shows up as an unknown field, and an unquoted `[yes]` is read as a list. Quote every label.
- **A line to an unknown part.** Usually a typo, or a line drawn to a group's id. Lines join parts only.
- **A connector in the wrong place.** In the security view it sits in the scope its inputs take, not wherever the service is. In the platform view it sits in the block of the service it reaches. Its label says whose access it is, as `"Owen's supermarket account"`.
- **Kinds and fields from other notations.** `agent`, `human`, `decision` and `note` aren't kinds, and `description` and `colour` aren't fields. Use only the kinds and fields the spec lists.
- **Hyphens in ids.** `meal-plan` isn't an id, but `meal_plan` is. An `op` such as `bring-in` isn't an id, so its hyphen is right.
- **An `op` on a part that isn't a step.** Only `ai`, `code` and `person` parts take one. A review needs none.
- **A comment joined by a `data` line.** A comment takes `tie` lines only.
- **Posting to a chat drawn as an outside action.** Delivery to a surface is a `data` line.
- **Boxes nested when the scopes only overlap.** Two scopes that each have someone the other lacks sit side by side.
