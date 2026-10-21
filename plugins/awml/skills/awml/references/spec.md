# AWML: the Agentic Workflow Modelling Language

Draft 0.2, 20 October 2026.

AWML describes agentic work as diagrams, written as YAML files and drawn with Mermaid.

**Contents:** [What it describes](#what-it-describes) · [The parts](#the-parts) · [The lines](#the-lines) · [The security view](#the-security-view-who-may-see-each-part) · [Connectors](#connectors) · [The platform view](#the-platform-view-where-each-part-runs) · [Tiers and yeses](#tiers-and-yeses) · [Standing approvals](#standing-approvals) · [Operations](#operations) · [Patterns](#patterns) · [Records](#records) · [The red line](#the-red-line) · [What AWML is not](#what-awml-is-not) · [The file format](#the-file-format)

## What it describes

A **workflow** is the chain from a trigger to the surfaces and outside actions it ends in, through the steps and approvals that pass work along. A **design** is the written description of a system that does such work: who takes part, what each part may read and write, and what needs a person's yes. AWML draws one workflow of a design as one picture, so a person can see who decides each step, who may see each part and what leaves the system.

Each workflow can be drawn two ways, one file for each: the **security view** (`view: security`), by who may see each part, and the **platform view** (`view: platform`), by where each part runs. AWML builds on the UML activity diagram, so each part means what its UML element means.

## The parts

| Part | `kind` | Drawn as | Colour | UML element |
|---|---|---|---|---|
| Trigger | `trigger` | A pill, with its time if it has one | Pink `#fbe1ee` | Accept event, or accept time event |
| Step decided by AI | `ai` | A rounded box, with 🤖 and its operation above its label | Blue `#cdd3fb` | Action |
| Step decided by code | `code` | A rounded box, with ⚙️ and its operation above its label | Green `#c4e3d1` | Action |
| Step decided by a person | `person` | A rounded box, with 👤 and its operation above its label | Amber `#f7d79a` | Action |
| Review | `review` | A rounded box, with 👤 ✅ review above its label, left by `[yes]` | Amber `#f7d79a` | Action, whose one way out is guarded by `[yes]`, so not a decision node |
| Fork | `fork` | A black bar | Black `#16171b` | Fork node |
| Context | `context` | A cylinder | Pale blue `#e2eef9` | Data store node |
| Connector | `connector` | A hexagon | Grey `#eceef2` | None |
| Surface | `surface` | A window box | White | None |
| Outside action | `outside` | A flag, with its tier | Coral `#fde4dc` | Send signal action |
| Comment | `comment` | A dashed box with a cut corner | White | Comment |

A **trigger** starts a workflow: a time, or an event such as someone asking in a chat. A **step** is one piece of work, drawn with the colour and emoji of who decides it: AI, code (which does the same thing every time) or a person. A **review** is a person's yes to exact content before it goes ahead. A **fork** is where work splits to run side by side. A **context** is the notes a step may read or write, kept as one folder. A **connector** is someone's access to an outside service, such as a calendar or a bank account. A **surface** is where people meet the system's work: a chat, a list on their phone, a screen on a wall. An **outside action** changes something outside the system, and its label ends with its tier. A **comment** marks a gap, or gives an example of what a part could carry.

The emoji say what the colours say, for a reader who can't tell the colours apart and on a page printed without them: 🤖 is AI, ⚙️ code and 👤 a person, on every step and review. Code is a gear rather than a computer, because a computer reads as a machine, and machines such as a laptop or a server are what the platform view shows.

UML has nothing for a connector or a surface, so those two shapes are AWML's own.

In either view, a trigger or comment given no group is drawn in the group of the part at the other end of its first line that isn't hidden. A view may leave out parts that don't help it, as a platform view often leaves out triggers.

## The lines

| `kind` | Drawn as | Means |
|---|---|---|
| `start` | A dotted arrow | Starts something |
| `data` | A solid arrow | Moves data |
| `publish` | A thick arrow | Moves content into a wider scope, which is publishing |
| `refused` | A line ending in a cross | A read the design refuses |
| `tie` | A dotted line | Links a comment to what it explains |
| `hidden` | Nothing | Steers the layout only |

A line may carry a label. Words in square brackets are a guard, the condition the line is taken on, as in UML: `[yes]` leaves a review. A comment moves nothing, so only `tie` lines link it.

## The security view: who may see each part

A **scope** is a set of people. Everything belongs to one scope, which says who may see it and which companies may process it: the makers of the models the system uses, and services such as transcription or the host of a folder. Its terms say whether a company may keep the content, and a company that may train on it is never used. A scope may also allow no processing, as for something told in confidence. Scopes aren't a ladder: two may overlap, each with someone the other lacks, or one may hold only people the other has, under looser terms.

In the security view, each scope is a dashed box, labelled with its name and members, such as `family: parents, kids`, or with `+` and the people it adds to the scope inside it, such as `carers: + June, Beth`.

**Where a part sits.** A context sits in the box of its scope. A step sits in the box of the scope it writes into, with the reviews and outside actions it leads to. A connector sits in the box of the scope its inputs take. A surface sits in the box of the scope its viewers make up.

**Boxes inside boxes.** One box sits inside another only where every member of the inner scope is in the outer one, and its terms are at least as strict. Otherwise they sit side by side. A family's scopes often nest: the parents' box sits inside the family's, which sits inside the box of the family and its carers. A consultant's client scope and their side venture's share only the consultant, so they sit side by side.

**Reading.** A step reads from its own scope or a wider one, since everyone in a scope may see what a wider scope holds. A read from a narrower scope is one the design refuses, drawn as a line ending in a cross.

**Moving content.** A move **narrows** only when every member of the target scope is in the source and the target's terms are at least as strict. Narrowing is routing, drawn as an ordinary line. Any other move adds a viewer or loosens the terms, which is **publishing**, drawn as a thick line. On the drawing, a move into the box it starts in, or a box inside that one, narrows. A part drawn outside every box counts as wider than any of them. `awml check` reports a `data` line that moves anywhere else, which is publishing drawn as routing. Publishing waits for a person's yes, unless a **standing rule** names that target for that kind of item, and the line's label then names the rule. Only the item crosses, never what lies behind it. Nothing reaches the public without a person's yes to that item, under any rule or standing approval.

## Connectors

A **connector** is access to one outside service through one credential: a person's, one an outside party lends, or the system's own. The design names whose access each one is, which service it reaches and what it may do, such as read, post or carry an outside action. Its label says whose it is, such as `Owen's supermarket account`.

A person's connector reaches only what that person can, and what it sends speaks in their name, so they, or a role they choose, approve what it sends. It stops when they leave, so work that must outlive them shouldn't rest on one. The system's own connector, such as a service account, often reaches more than any one person, so each item it brings in takes the scope of who could see it at its source, never everyone's.

## The platform view: where each part runs

In the platform view, each platform is a tinted block, and each part sits in the block of the platform it runs on or lives in. A **platform** is a machine or service that brings its own surfaces, its own **harness** (the program that runs each step and calls a model) and its own models, and often its own triggers: Claude Cowork on a Mac mini, a mail service or a phone. A trigger sits in the block of the platform that fires it. A model on its own isn't a platform: it's a choice a platform makes. A **machine** is anywhere the system's tools run that shares one set of access: a server, a laptop, or a sandbox on either.

A block is labelled with what an **implementation** chose there, such as the model, as in `Server: claude-sonnet-4-5`, and with when the machine is on, if that isn't always, as in `Noor's laptop: on some evenings`. Work for a machine that is off waits for it. An implementation is one running system built from the design, and each draws its own platform view, with the same parts in different blocks.

Anything running on a machine can reach everything that machine holds, and send it to any host the machine can reach. So a machine holds only contexts and connectors whose scopes allow every company that processes content there, and reaches only the hosts its connectors need.

An implementation's choices change how a step is done, never what it may do: the contexts a step reads, the tier of each action it takes and its review stay the design's call. A different model may mean a different company processing the content, which every context feeding the step must allow.

## Tiers and yeses

Each outside action has a **tier**, which ends its label, as in `Send the mail: Ask`.

| Tier | When it applies | Drawn as |
|---|---|---|
| **Allow** | The action can be undone when it is taken, is seen only within its scope, and stays within the design's spending limits: a note, a draft, a label, an entry in a person's own calendar | A flag a step leads to directly: it just happens |
| **Ask** | Any other action that isn't blocked, and any the design marks Ask because the person who answers for it would be surprised | A flag with a review on the way to it, or one a person's own `do` step leads straight to: it waits for a yes, unless a standing approval covers it |
| **Block** | Never, whatever anyone says | Never a line. A flag with nothing leading to it may name it, and a line into one is red |


A **yes** is an approver's yes to one action and its exact content, shown with what it is for: the work it serves, why now, what started it and what it drew on. Any change to the content after the yes, a fix included, needs a new yes. An **approver** is a role, such as the approver of sends, held by one or more people. A rare, high-stakes kind of action may need a yes from two approvers, two people in two roles, to the same content, drawn as two reviews in a row, and kept for few kinds.

When a person does a `do` step themselves, as when the consultant edits the reply and sends it, that is their yes to the exact content. So an Ask action reached straight from a person's `do` step needs no review in front of it.

## Standing approvals

A **standing approval** lets the runs of one scheduled task take named kinds of action that can be undone, within limits such as how many and how often, without a yes to each one. The person who answers for the task gives it once. Each action under it is recorded and listed for that person with how to undo it. One that acts in someone else's name needs their yes too, and ends when their access does. Nothing that can't be undone goes this way, such as a send to a person or a payment, and nothing that reaches the public.

A standing approval is drawn on the outside action it covers: the label names it, as in `Renew the book: Ask, under Sam's standing approval`, and a comment tied to the flag gives its kinds and limits. The file format needs no field for one, since a drawing can show no more of it than that.

## Operations

A step's colour and its decider's emoji say who decides it. A step may also name its **operation**, with `op`, to say what kind of work it is. The two are separate: AI can sort or draft, and so can a person. The drawing shows both on a line of their own above the step's label, in smaller, softer type: who decides first, then the operation's emoji and name, as in `🤖 ✏️ draft`. `op` is optional, and a step without one has who decides it alone on that line, as in `🤖`. For work that fits none of the nine, `op: other` draws `❔ other` and holds the step to no operation's limits.

| Operation | `op` | It asks | It produces | It must not |
|---|---|---|---|---|
| 📥 Bring in | `bring-in` | What's new? | The input as it really is, such as an email, a transcript or a calendar, made readable | Decide where it goes |
| 🗂️ Sort | `sort` | Where does this belong? | A place for it, or none with the reason | Change anything outside the system |
| 🧠 Learn | `learn` | What does this teach us? | New notes, or more in old ones, and rules taken from corrections | Act in the world |
| 📋 Prepare | `prepare` | What's needed before this? | A brief for a named moment, such as a meeting | Decide for a person |
| 🔍 Check | `check` | Is anything wrong or missing? | A verdict | Change anything |
| 🪞 Reflect | `reflect` | Is time going where it should? | Evidence and questions for the people who set the aims | Make up an aim |
| ✏️ Draft | `draft` | What should be said? | A draft | Send it or publish it |
| 🛠️ Do | `do` | What should change, in the world or in a piece of work? | The change, within its tier | Act without the yes its tier needs, or replace something without keeping what was there |
| 💬 Answer | `answer` | What do we know about this? | An answer from the notes, naming them | Act, or change a note |

A drawing can show some of these limits, and `awml check` holds each step to those:

- **Only a `do` step leads to an outside action**, by a line straight to it or through a review or a fork. A draft goes to a `do` step, which asks to send it, and a person's yes releases the send, or to a person's own `do` step, which is their yes. A draft step never leads to an outside action, even through a review.
- **An `answer` step and a `check` step never write to a context.** They read contexts, and speak on surfaces.

A line leads somewhere when it starts something, moves data or publishes. A hidden line, a refused read and a tie lead nowhere. The other limits, such as deciding for a person or making up an aim, are about what happens inside a step, which no drawing shows, so its readers judge those.

## Patterns

These shapes come up again and again. The first two are the commonest.

### Draft for review

<img src="examples/tour/01-draft-and-send.svg" width="190" alt="Draft for review">

A `draft` step writes the content, a `do` step asks to send it, a person says yes to that exact content, and the send goes out. When a person edits the draft and sends it in their own `do` step, that step is their yes, so the send needs no review in front of it. Use it for anything that goes out in someone's name or can't be taken back. Any change to the content after the yes needs a new yes. A draft step never leads to an outside action, even through a review.

### Do

<img src="examples/tour/11-do.svg" width="360" alt="Do">

A `do` step whose outside action either just happens, because it is Allow and can be undone, or waits for a yes, because it is Ask. A person's own `do` step is their yes, so an Ask action straight after one needs no review. Use it for any change in the world, or in the state of a piece of work.

### Standing approval

<img src="examples/tour/12-standing-approval.svg" width="410" alt="Standing approval">

A person approves a scheduled task once to take named kinds of action that can be undone, within limits, so each item doesn't wait for a yes. Use it for small, frequent actions that a yes each time would only slow down. Nothing that can't be undone and nothing public goes this way.

### Answer from the notes

<img src="examples/tour/03-refused-read.svg" width="400" alt="Answer from the notes">

An `answer` step that reads only what everyone present may see, and says which notes it used. Use it wherever someone asks a question on a surface others can see.

### Check by code

<img src="examples/tour/13-check-by-code.svg" width="410" alt="Check by code">

A `check` step, decided by code, that reads and reports, and changes nothing. Use it for any question code can settle, such as whether each bill expected was paid.

### Publish

<img src="examples/tour/04-publishing.svg" width="316" alt="Publish">

Content moving into a wider scope, drawn as a thick line, which waits for a person's yes unless a standing rule names that move. Use it whenever something leaves its scope for one with more people or looser terms.

## Records

Every step leaves one **record** each time it runs: what started it, what decided it (code, a model with its instructions and exact version, or a person), what it drew on, what it made and what it cost. Records are only ever added to, and only the system writes them.

A drawing shows what each record should say, never what any record does say. A step's lines show what it may draw on, its colour and emoji who decides it, and its operation what kind of work it does, so a record naming other sources, another kind of decider or other work is a finding. A drawing can't show whether a step ran, which model version decided it, what it cost or what it really read. Those are in the records.

## The red line

Where a live system does something its design doesn't allow, the line for it is drawn red, labelled with what is wrong. A red line is a finding about the live system, not part of the design, so `awml check` doesn't hold it to the operations' limits or to the boxes it crosses.

## What AWML is not

AWML runs nothing and grants nothing. In a workflow tool such as n8n, the diagram is the program: drawing a line makes data flow. An AWML drawing is the description a system is checked against. Every part and line is something the design names, so a drawing can be made from the design itself, and then it can't drift from it. Drawing a workflow shows what a list of scheduled tasks hides, such as a kitchen screen the design says shows the week's dinners, though nothing in the design keeps it current.

## The file format

One YAML file holds one view of one workflow.

```yaml
awml: "0.2"
title: The family's dinners
view: security
groups:
  - { id: carers, label: "carers: + June, Beth" }
  - { id: family, label: "family: parents, kids", in: carers }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
  - { id: mealplan, kind: context, label: "Meal plan", in: family }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: mealplan, kind: data, label: posted }
```

The file needs `awml`, the version `"0.2"`; `title`, the workflow's name; `view`, `security` or `platform`; and `parts`. It may have `groups` and `lines`.

- A **group** has an `id` and a `label`, and may have `in`, the id of the group it sits inside.
- A **part** has an `id`, a `kind` and a `label`, which a fork doesn't need and never shows. It may have `in`, the id of its group. A step, `ai`, `code` or `person`, may have `op`, its operation.
- A **line** has `from` and `to`, the ids of two parts, and a `kind`. It may have a `label`, and `breaks: true` to draw it red.

An id is letters, digits and underscores, starting with a letter, and no two are the same. Every id, label, kind and reference to an id is text. `awml` reads YAML 1.2, which reads `true`, `false`, numbers and dates such as `2026-10-20` as something other than text, so quote them. YAML 1.1, which many other tools read, also reads `yes`, `no`, `on` and `off` as true or false, and `19:00` as a number, so quoting those too keeps a file the same in every tool. `breaks` is `true` or `false`, unquoted. The drawing keeps the parts in the order the file lists them, and draws each group after the parts beside it. List the parts in the order the work flows, with its triggers first.

`awml check` reports a missing or unknown field, an unknown kind or view, an id used twice or badly written, a value YAML didn't read as text, a `breaks` that isn't true or false, a part or group in an unknown group, a group nested in itself, a line to or from an unknown part, a comment linked by anything but `tie`, a `tie` with no comment at either end, an unknown `op` or one on a part that isn't a step, a step that does what its operation must not, as far as the drawing shows it, and a `data` line in the security view that moves content anywhere but its own box or a box inside it.
