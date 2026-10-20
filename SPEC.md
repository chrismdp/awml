# AWML: the Agentic Workflow Modelling Language

Draft 0.1, 20 October 2026.

AWML describes agentic work as diagrams, written as YAML files and drawn with Mermaid.

## What it describes

A **workflow** is the chain from a trigger to the surfaces and outside actions it ends in, through the steps and approvals that pass work along. A **design** is the written description of a system that does such work: who takes part, what each part may read and write, and what needs a person's yes. AWML draws one workflow of a design as one picture, so a person can see who decides each step, who may see each part and what leaves the system.

AWML builds on the UML activity diagram, so each part means what its UML element means.

## The parts

| Part | `kind` | Drawn as | Colour | UML element |
|---|---|---|---|---|
| Trigger | `trigger` | A pill, with its time if it has one | Pink `#fbe1ee` | Accept event, or accept time event |
| Step decided by a model | `ai` | A rounded box, with its operation above its label | Blue `#cdd3fb` | Action |
| Step decided by a rule | `rule` | A rounded box, with its operation above its label | Green `#c4e3d1` | Action |
| Step decided by a person | `human` | A rounded box, with its operation above its label | Amber `#f7d79a` | Action |
| Review | `review` | A diamond, left by `[yes]` | Amber `#f7d79a` | Decision node with guards |
| Fork | `fork` | A black bar | Black `#16171b` | Fork node |
| Context | `context` | A cylinder | Pale blue `#e2eef9` | Data store node |
| Connector | `connector` | A hexagon | Grey `#eceef2` | None |
| Surface | `surface` | A window box | White | None |
| Outside action | `outside` | A flag, with its tier | Coral `#fde4dc` | Send signal action |
| Comment | `comment` | A dashed box with a cut corner | White | Comment |

A **trigger** starts a workflow: a time, or an event such as someone asking in a chat. A **step** is one piece of work, coloured by what decides it: a model, a rule (code that does the same thing every time) or a person. A **review** is a person's yes to exact content before it goes ahead. A **fork** is where work splits to run side by side. A **context** is the notes a step may read or write, kept as one folder. A **connector** is someone's access to an outside service, such as a calendar or a bank account. A **surface** is where people meet the system's work: a chat, a list on their phone, a screen on a wall. An **outside action** changes something outside the system, and its label carries its **tier**: Allow, which just happens because it can be undone; Ask, which waits for a review; or Block, which never happens. A **comment** points out a gap, or gives an example of what a part could carry.

UML has nothing for a connector or a surface, so those two shapes are AWML's own.

## What kind of work a step is

A step's colour says who decides it. A step may also name its **operation**, with `op`, to say what kind of work it is. The two are separate: a model can sort or draft, and so can a person. The drawing shows the operation above the step's label as a UML stereotype, smaller and quieter than the label, such as `«draft»`. `op` is optional, and a step without one is drawn and checked as before.

| Operation | `op` | It asks | It produces | It must not |
|---|---|---|---|---|
| Bring in | `bring-in` | What's new? | The input as it really is, such as an email, a transcript or a calendar, made readable | Decide where it goes |
| Sort | `sort` | Where does this belong? | A place for it, or none with the reason | Change anything outside the system |
| Learn | `learn` | What does this teach us? | New notes, or more in old ones, and rules taken from corrections | Act in the world |
| Prepare | `prepare` | What's needed before this? | A brief for a named moment, such as a meeting | Decide for a person |
| Check | `check` | Is anything wrong or missing? | A verdict | Change anything |
| Reflect | `reflect` | Is time going where it should? | Evidence and questions for the people who set the aims | Make up an aim |
| Draft | `draft` | What should be said? | A draft | Send it or publish it |
| Do | `do` | What should change, in the world or in a piece of work? | The change, within its tier | Act without the yes its tier needs, or replace something without keeping what was there |
| Answer | `answer` | What do we know about this? | An answer from the notes, naming them | Act, or change a note |

A drawing can show some of these limits, and `awml check` holds each step to those:

- **Only a `do` step leads to an outside action**, by a line straight to it or through a review or a fork. A draft goes to a `do` step, which asks for the yes and then sends.
- **An `answer` step and a `check` step never write to a context.** They read contexts, and speak on surfaces.

A line leads somewhere when it starts something, moves data or publishes. A hidden line, a refused read and a tie lead nowhere. A red line draws what a live system did, not what the design allows, so these rules don't hold it. The other limits, such as deciding for a person or making up an aim, are about what happens inside a step, which no drawing shows, so the people who read it judge those.

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

## The two views

Each workflow can be drawn two ways, one file for each.

**Who may see each part** (`view: scopes`). A **scope** is a set of people. Everything belongs to one scope, which says who may see it and what may process it, such as which companies' models. Each scope is a dashed box, labelled with its name and members, such as `family: parents, kids`, or with `+` and the people it adds to the scope inside it, such as `carers: + June, Beth`.

**Where each part runs** (`view: where`). Each machine and outside service is a tinted block, labelled with what an **implementation** chose there, such as the model: `Server: claude-sonnet-4-5`. An implementation is one running system built from the design, and each draws its own view, with the same parts in different blocks.

An implementation's choices change how a step is done, never what it may do: the contexts a step reads, the tier of each action it takes and its review stay the design's call. A different model may mean a different company processing the content, which every context feeding the step must allow.

## Where a part sits

In a scopes view, a context sits in the box of its scope, and a step in the box of the scope it writes into, with the reviews and outside actions it leads to. A connector sits in the box of the scope its inputs take, so a connector only the parents may use sits in the parents' box. A surface sits in the box of the scope its viewers make up.

In a where view, each part sits in the block of the machine or service it runs on or lives in.

In both views, a trigger or comment given no group is drawn in the group of the part at the other end of its first line that isn't hidden. A view may leave out parts that don't help it, as a where view often leaves out triggers.

## When one scope box sits inside another

One box sits inside another only where every member of the inner scope is in the outer one. Otherwise they sit side by side, and any line from one to the other is publishing. A family's scopes often nest: the parents' box sits inside the family's, which sits inside the box of the family and its carers. A consultant's client scope and their venture scope share only the consultant, so they sit side by side.

## The red line

Where a live system does something its design doesn't allow, the line for it is drawn red, labelled with what is wrong. A red line is a finding about the live system, not part of the design.

## What AWML is not

AWML runs nothing and grants nothing. In a workflow tool such as n8n, the diagram is the program: drawing a line makes data flow. An AWML drawing is the description a system is checked against. Every part and line is something the design names, so a drawing can be made from the design itself, and then it can't drift from it. Drawing a workflow shows what a list of scheduled tasks hides, such as a kitchen screen the design says shows the week's dinners, though nothing in the design keeps it current.

## The file format

One YAML file holds one view of one workflow.

```yaml
awml: "0.1"
title: The family's dinners
view: scopes
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

The file needs `awml`, the version `"0.1"`; `title`, the workflow's name; `view`, `scopes` or `where`; and `parts`. It may have `groups` and `lines`.

- A **group** has an `id` and a `label`, and may have `in`, the id of the group it sits inside.
- A **part** has an `id`, a `kind` and a `label`, which a fork doesn't need and never shows. It may have `in`, the id of its group. A step, `ai`, `rule` or `human`, may have `op`, its operation.
- A **line** has `from` and `to`, the ids of two parts, and a `kind`. It may have a `label`, and `breaks: true` to draw it red.

An id is letters, digits and underscores, starting with a letter, and no two are the same. Every id, label, kind and reference to an id is text. `awml` reads YAML 1.2, which reads `true`, `false`, numbers and dates such as `2026-10-20` as something other than text, so quote them. YAML 1.1, which many other tools read, also reads `yes`, `no`, `on` and `off` as true or false, and `19:00` as a number, so quoting those too keeps a file the same in every tool. `breaks` is `true` or `false`, unquoted. Parts are drawn in the order listed, and each group after the parts beside it.

`awml check` reports a missing or unknown field, an unknown kind or view, an id used twice or badly written, a value YAML didn't read as text, a `breaks` that isn't true or false, a part or group in an unknown group, a group nested in itself, a line to or from an unknown part, a comment linked by anything but `tie`, a `tie` with no comment at either end, an unknown `op` or one on a part that isn't a step, and a step that does what its operation must not, as far as the drawing shows it.
