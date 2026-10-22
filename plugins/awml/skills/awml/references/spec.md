# AWML: the Agentic Workflow Modelling Language

Draft 0.5, 22 October 2026.

AWML describes agentic work as diagrams, written as YAML files and drawn with Mermaid.

**Contents:** [What it describes](#what-it-describes) · [The parts](#the-parts) · [The lines](#the-lines) · [The security view](#the-security-view-who-may-see-each-part) · [Connectors](#connectors) · [The platform view](#the-platform-view-where-each-part-runs) · [Placing a workflow](#placing-a-workflow-where-it-can-run) · [Tiers and yeses](#tiers-and-yeses) · [Standing approvals](#standing-approvals) · [Operations](#operations) · [Patterns](#patterns) · [Records](#records) · [The red line](#the-red-line) · [What AWML is not](#what-awml-is-not) · [The file format](#the-file-format)

## What it describes

A **workflow** is the chain from a trigger to the surfaces and outside actions it ends in, through the steps and approvals that pass work along. A **design** is the written description of a system that does such work: who takes part, what each part may read and write, and what needs a person's yes. AWML draws one workflow of a design as one picture, so a person can see who decides each step, who may see each part and what leaves the system.

One canonical file holds the workflow's design and any **implementations**, the choices of where its parts run. Its **security view** groups the parts by who may see them. Each implementation gives a **platform view**, grouping those same parts by where they run. A separate **platform description** says what one platform offers, so a workflow can be matched to the platforms that could run it. AWML builds on the UML activity diagram, so each part means what its UML element means.

## The parts

| Part | `kind` | Drawn as | Colour | UML element |
|---|---|---|---|---|
| Trigger | `trigger` | A pill, with its time if it has one | Pink `#fbe1ee` | Accept event, or accept time event |
| Step decided by AI | `ai` | A rounded box, with 🤖 and its operation above its label | Blue `#cdd3fb` | Action |
| Step decided by code | `code` | A rounded box, with ⚙️ and its operation above its label | Green `#c4e3d1` | Action |
| Step decided by a person | `person` | A rounded box, with 👤 and its operation above its label | Amber `#f7d79a` | Action |
| Review | `review` | A rounded box, with 👤 ✅ review above its label, left by `[yes]` | Amber `#f7d79a` | Action, whose one way out is guarded by `[yes]`, so not a decision node |
| Standing approval | `approval` | A rounded box, with 👤 📜 standing approval above its label | Amber `#f7d79a` | Action, like a review |
| Fork | `fork` | A black bar | Black `#16171b` | Fork node |
| Context | `context` | A cylinder | Pale blue `#e2eef9` | Data store node |
| Connector | `connector` | A hexagon | Grey `#eceef2` | None |
| Surface | `surface` | A window box | White | None |
| Outside action | `outside` | A flag, with its tier | Coral `#fde4dc` | Send signal action |
| Comment | `comment` | A dashed box with a cut corner | White | Comment |

A **trigger** starts a workflow: a time, or an event such as someone asking in a chat. A **step** is one piece of work, drawn with the colour and emoji of who decides it: AI, code (which does the same thing every time) or a person. A **review** is a person's yes to exact content before it goes ahead. A **standing approval** is a person's yes given once to named kinds of action, within limits. A **fork** is where work splits to run side by side. A **context** is the notes a step may read or write, kept as one folder. A **connector** is someone's access to an outside service, such as a calendar or a bank account. A **surface** is where people meet the system's work: a chat, a list on their phone, a screen on a wall. An **outside action** changes something outside the system, and its label ends with its tier. A **comment** marks a gap, or gives an example of what a part could carry.

The emoji say what the colours say, for a reader who can't tell the colours apart and on a page printed without them: 🤖 is AI, ⚙️ code and 👤 a person, on every step, review and standing approval. Code is a gear rather than a computer, because a computer reads as a machine, and machines such as a laptop or a server are what the platform view shows.

UML has nothing for a connector or a surface, so those two shapes are AWML's own.

In either view, a trigger or design comment given no scope or block is drawn beside the part at the other end of its first line that isn't hidden. An implementation may add comments that appear only in its platform view.

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

A **scope** is a set of people. Everything belongs to one scope, which says who may see it and which companies may process it: the makers of the models the system uses, and services such as transcription or the host of a folder. Its terms say whether a company may keep the content, and a company that may train on it is never used. A scope may also allow no processing, as for something told in confidence. Its `processing` field names the companies, so `awml fit` can check them (see [Placing a workflow](#placing-a-workflow-where-it-can-run)). Scopes aren't a ladder: two may overlap, each with someone the other lacks, or one may hold only people the other has, under looser terms.

In the security view, each scope is a dashed box, labelled with its name and members, such as `family: parents, kids`, or with `+` and the people it adds to the scope inside it, such as `carers: + June, Beth`.

**Where a part sits.** A context sits in the box of its scope. A step sits in the box of the scope it writes into, with the reviews and outside actions it leads to. A standing approval sits in the box of the scope of the person who gives it. A connector sits in the box of the scope its inputs take. A surface sits in the box of the scope its viewers make up.

**Boxes inside boxes.** One box sits inside another only where every member of the inner scope is in the outer one, and its terms are at least as strict. Otherwise they sit side by side. A family's scopes often nest: the parents' box sits inside the family's, which sits inside the box of the family and its carers. A consultant's client scope and their side venture's share only the consultant, so they sit side by side.

**Reading.** A step reads from its own scope or a wider one, since everyone in a scope may see what a wider scope holds. A read from a narrower scope is one the design refuses, drawn as a line ending in a cross.

**Moving content.** A move **narrows** only when every member of the target scope is in the source and the target's terms are at least as strict. Narrowing is routing, drawn as an ordinary line. Any other move adds a viewer or loosens the terms, which is **publishing**, drawn as a thick line. On the drawing, a move into the box it starts in, or a box inside that one, narrows. A part drawn outside every box counts as wider than any of them. `awml check` reports a `data` line that moves anywhere else, which is publishing drawn as routing. Publishing waits for a person's yes, unless a **standing rule** names that target for that kind of item. A standing rule moves only the words of people who agreed to that target, and the line's label names the rule and who they are, as in `co-op notes, under the bakery's rule, for customers who ticked the box`. Only the item crosses, never what lies behind it. Nothing reaches the public without a person's yes to that item, under any rule or standing approval.

## Connectors

A **connector** is access to one outside service through one credential: a person's, one an outside party lends, or the system's own. The design names whose access each one is, which service it reaches and what it may do, such as read, post or carry an outside action. Its label says whose it is, such as `Owen's supermarket account`.

A person's connector reaches only what that person can, and what it sends speaks in their name, so they, or a role they choose, approve what it sends. It stops when they leave, so work that must outlive them shouldn't rest on one. The system's own connector, such as a service account, often reaches more than any one person, so each item it brings in takes the scope of who could see it at its source, never everyone's.

## The platform view: where each part runs

Each implementation lists its platforms as blocks and names the parts placed in each. Its platform view draws each block tinted, with those parts inside it. A **platform** is a machine or service that brings its own surfaces, its own **harness** (the program that runs each step and calls a model) and its own models, and often its own triggers: Claude Cowork on a Mac mini, a mail service or a phone. A trigger sits in the block of the platform that fires it. A model on its own isn't a platform: it's a choice a platform makes. A **machine** is anywhere the system's tools run that shares one set of access: a server, a laptop, or a sandbox on either.

A block is labelled with what an **implementation** chose there, such as the model, as in `Server: claude-sonnet-4-5`, and with when the machine is on, if that isn't always, as in `Noor's laptop: on some evenings`. Work for a machine that is off waits for it. An implementation is one running system built from the design. Parts and lines are written once, so all implementations share the design's labels, operations and rules.

Anything running on a machine can reach everything that machine holds, and send it to any host the machine can reach. So a machine holds only contexts and connectors whose scopes allow every company that processes content there, and reaches only the hosts its connectors need.

A block sits inside another when what runs in it shares that one's access, as a script an agent's session starts can reach everything the session can. A block with its own access, such as a separate server or a model's own service, sits beside it, and a line into it crosses between platforms through a connector. Drawing a block beside another when it shares that one's access would show an isolation that isn't there.

An implementation's choices change how a step is done, never what it may do: the contexts a step reads, the tier of each action it takes and its review stay the design's call. A different model may mean a different company processing the content, which every context feeding the step must allow.

## Placing a workflow: where it can run

Which platforms could run a workflow is a separate question from which one it runs on. The design says what the workflow needs, a **platform description** says what one platform offers, and `awml fit` matches the two, giving the same answer every time. A person, or an agent working with them, writes the needs and describes their own machines. The matching is code. Choosing platforms that fit, adding an implementation and installing the workflow there is the person's call, and their agent's work.

A security drawing on its own is easy to leave untested. Fitting a workflow to real platforms forces concrete choices. Each choice rules out more platforms and leads to the next:

- A scope that allows only certain companies rules out a hosted runtime if its company would hold the data.
- Local files rule out platforms that hold none.
- If the builder must not reach the data, an ordinary container is no boundary when the builder's account can drive the container service, since that access is usually as good as full control of the machine. The runtime then needs its own operating system user, and the builder loses that access.
- The runtime's credentials need their own OAuth project or app. Providers often carry earlier grants across clients in the same project.
- The owner must deploy it with their privileges because the builder can no longer reach the runtime.

This path is hardest when a builder can't see the data that the workflow handles. Examples include a consultant building for a client, a central team building for a department, and an AI agent building for a person. The fit makes the trade-off explicit so the owner can decide it. An owner can relax the boundary for data they already share with the builder, perhaps accepting one held only by instructions. That choice is a valid result of fitting the workflow, not a failure.

**What a workflow needs.** Most of it is already in the drawing: the contexts, connectors and AI steps the workflow reaches. Three fields add the rest:

- **Who may process it.** A scope's `processing` names the companies that may process its content, and for which operations. `{ anthropic: any, openai: [sort] }` lets Anthropic's models do anything in the scope and OpenAI's only sort. `any` lets every company in, and `none` keeps them all out, as for something told in confidence. A company is a short id, which only has to match the id the platform descriptions use. Where one company offers different terms, name the agreement instead, as in `openai-zero-retention`.
- **How long work may wait.** A trigger's `wait` is the longest the work may wait once the trigger fires, as an ISO 8601 duration: `PT5M` for urgent mail, `P1D` for a weekly review. Durations may use weeks, days, hours, minutes and seconds; years and months aren't accepted because they have no fixed length. This is what "always on" means for a workflow.
- **Where a context is kept.** A context's `at` names the folder or store it already is, as a path such as `notes/projects`. A context without `at` isn't looked for: it's state the workflow makes wherever it's installed, or kept by a service a connector reaches.

**What a platform offers.** A platform description is a file of its own. It describes one kind of platform, such as a container on a server, or one real machine, and names no workflow:

```yaml
awml: "0.3"
platform: { id: cowork-mac-mini, label: "Claude Cowork on a Mac mini" }
off_for_up_to: PT0S
steps: agent
agents: [anthropic]
calls: any
holds: [notes]
```

- `off_for_up_to` is the longest the platform may be off, as an ISO 8601 duration: `PT0S` for one that's always on, `P14D` for a laptop that may be shut for a fortnight's holiday.
- `steps` says what carries out the workflow's steps there: `code`, when the platform runs the workflow's own code, as a scheduler on a server does, or `agent`, when an agent carries out each step by following instructions, as in a scheduled session in an agent's app.
- `agents` lists the companies whose agents run there. An agent is a model with tools, so it can read everything the platform holds. An empty list means only the workflow's own code runs there, as in a container that holds nothing else.
- `calls` lists the companies whose models the workflow's own code can call from there, or says `any`.
- `holds` lists the folders and stores the platform holds or can be given, as paths. A platform that holds `notes` holds `notes/projects`.

A description of a kind of platform is a starting point. A real machine's description adds what else is on it, such as the coding agents its owner uses there. One machine may offer several platforms: the scheduled scripts and the agent's scheduled sessions on one Mac mini are two descriptions, which differ in `steps` and share what the machine holds.

**How `awml fit` decides.** Given a workflow and some platform descriptions, `awml fit` says of each platform that the workflow fits, or gives every reason it doesn't:

1. **It's on often enough.** Every trigger's `wait` is at least the platform's `off_for_up_to`.
2. **Every agent there may see what it holds.** Each company in `agents` is allowed `any` by every scope that has a step, context or connector of the workflow on the platform. This is the platform view's rule for a machine. A person's own steps and reviews happen on their own devices, so they don't count.
3. **Every AI step has a model it may use.** Each AI step's scope allows at least one company, for the step's operation, that the platform reaches through `agents` or `calls`. The fit names those companies for each AI step. Which model to use is the implementation's choice.
4. **Every context it relies on is there.** Each context with `at` is in something the platform `holds`.
5. **Code steps run as code.** The platform's `steps` is `code` wherever the workflow has a code step. Where an agent carries out a code step by following instructions, an AI decides it, and every rule the drawing holds by its shape, such as only a `do` step leading to an outside action, is only an instruction to that agent.

With `--implementation ID`, `awml fit` judges that implementation's placement. Each block names its description with `platform`, and each part is judged against the platform of the block it sits in. For rule 2, it's judged against every block around that one too, since what runs inside a block shares its access. A block that names no description isn't judged, and `awml fit` says so.

A block names a description of its owner's own by a bare id, such as `scripts-mac-mini`. A description published to the registry has the id `owner/platforms/slug`, such as `acme/platforms/ops-runner`, which is also its page there, and a block names it by that id. `awml pull acme/platforms/ops-runner` fetches it. `awml fit` matches either kind by id among the descriptions it's given, so it never reaches the network.

An implementation that names a description not given to the command is a mistake, and for a published one `awml fit` says to pull it. Two given descriptions with the same id are a mistake too.

A scope without `processing`, or a trigger without `wait`, is a gap, and `awml fit` reports it rather than guess. Take the client email in the examples, with the private scope allowing any company, the business scope allowing only Anthropic, the engagements and their projects kept under `notes`, and the trigger allowed to wait two hours. Given four platforms, `awml fit` might say:

```text
$ awml fit client-email.yaml cowork-mac-mini.yaml scripts-mac-mini.yaml codex-laptop.yaml worker.yaml
cowork-mac-mini   no: bring and rules are code steps, which anthropic's agent would carry out
scripts-mac-mini  fits: choose, learn and draft may use anthropic
codex-laptop      no: weekdays may wait PT2H, but the platform may be off for P14D
                  no: openai's agents run here, and business doesn't allow openai
worker            no: engagements is kept at notes/engagements, which the platform doesn't hold
                  no: project is kept at notes/projects, which the platform doesn't hold
```

## Tiers and yeses

Each outside action has a **tier**, which follows the last colon in its label, as in `Send the mail: Ask`.

| Tier | When it applies | Drawn as |
|---|---|---|
| **Allow** | The action can be undone when it is taken, is seen only within its scope, and stays within the design's spending limits: a note, a draft, a label, an entry in a person's own calendar | A flag steps lead to with no review or standing approval in front of it: it just happens |
| **Ask** | Any other action that isn't blocked, and any the design marks Ask because the person who answers for it would be surprised | A flag with a person on every way to it: a review, a standing approval, or a person's own `do` step leading straight to it. It waits for a yes, or goes under a standing approval |
| **Block** | Never, whatever anyone says | Never a line. A flag with nothing leading to it may name it, and a line into one is red |

The tier follows from the drawing's shape: wherever a tier says a person decides, the drawing shows them. A way to an action is a line that leads to it, followed back through any fork to the part it comes from. `awml check` reports an Ask action that any part but a review, a standing approval or a person's own step leads to, an Allow action with a review or standing approval in front of it, and any line but a red one into a Block action.

A **yes** is an approver's yes to one action and its exact content, shown with what it is for: the work it serves, why now, what started it and what it drew on. Any change to the content after the yes, a fix included, needs a new yes. An **approver** is a role, such as the approver of sends, held by one or more people. A rare, high-stakes kind of action may need a yes from two approvers, two people in two roles, to the same content, drawn as two reviews in a row, and kept for few kinds.

When a person does a `do` step themselves, as when the consultant edits the reply and sends it, that is their yes to the exact content. So an Ask action reached straight from a person's `do` step needs no review in front of it.

## Standing approvals

A **standing approval** lets the runs of one scheduled task take named kinds of action that can be undone, within limits such as how many and how often, without a yes to each one. The person who answers for the task gives it once. Each action under it is recorded and listed for that person with how to undo it. One that acts in someone else's name needs their yes too, and ends when their access does. Nothing that can't be undone goes this way, such as a send to a person or a payment, and nothing that reaches the public.

A standing approval is a part of its own, drawn in the box of the person who gives it. Its label says whose it is and gives its kinds and limits, as in `Sam's standing approval: renewals only, up to six a week`. The `do` step leads to it, and it leads to each outside action it covers, which is Ask. A line from it goes to the surface where each action is listed with how to undo it, and `awml check` reports a standing approval with no such line. Whether an action can be undone is something no drawing shows, so the readers judge that.

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

A `do` step should be code acting on what an earlier step decided. An AI or a rule turns the input into a score or a choice, and the `do` step acts on it with thresholds, lookups and templates. Each step does one job. A `do` step changes one thing. Filing an item, labelling it and telling a person are three `do` steps after one `sort`.

A drawing can show some of these limits, and `awml check` holds each step to those:

- **Only a `do` step leads to an outside action**, by a line straight to it or through a review, a standing approval or a fork. A draft goes to a `do` step, which asks to send it, and a person's yes releases the send, or to a person's own `do` step, which is their yes. A draft step never leads to an outside action, even through a review.
- **An `answer` step and a `check` step never write to a context.** They read contexts, and speak on surfaces.

A line leads somewhere when it starts something, moves data or publishes. A hidden line, a refused read and a tie lead nowhere. The other limits, such as deciding for a person or making up an aim, are about what happens inside a step, which no drawing shows, so its readers judge those.

`awml check` warns when a `do` step is decided by AI or leads to more than one change. Each outside action and surface it leads to is one change. All the contexts it leads to together are one change, since writing the item, a seen list and other state into the system's own notes is one job. These warnings use the same meaning of leads and ignore red lines. They don't make the check fail.

## Patterns

These shapes come up again and again. The first two are the commonest.

### Draft for review

<img src="examples/tour/01-draft-and-send.svg" width="190" alt="Draft for review">

A `draft` step writes the content, a `do` step asks to send it, a person says yes to that exact content, and the send goes out. When a person edits the draft and sends it in their own `do` step, that step is their yes, so the send needs no review in front of it. Use it for anything that goes out in someone's name or can't be taken back. Any change to the content after the yes needs a new yes. A draft step never leads to an outside action, even through a review.

### Do

<img src="examples/tour/11-do.svg" width="360" alt="Do">

Two `do` steps, each making one change. The Allow action can be undone, so it just happens. The Ask action waits for a yes. A person's own `do` step is their yes, so an Ask action straight after one needs no review. Use a `do` step for any change in the world, or in the state of a piece of work.

### Standing approval

<img src="examples/tour/12-standing-approval.svg" width="410" alt="Standing approval">

A person approves a scheduled task once to take named kinds of action that can be undone, within limits, so each item doesn't wait for a yes. The standing approval stands between the `do` step and the action, and lists each action on the person's surface. Use it for small, frequent actions that a yes each time would only slow down. Nothing that can't be undone and nothing public goes this way.

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

A step's own record of its run is its record, written by the system, so it isn't drawn as a context. A log or state that a later step reads back is a context.

A drawing shows what each record should say, never what any record does say. A step's lines show what it may draw on, its colour and emoji who decides it, and its operation what kind of work it does, so a record naming other sources, another kind of decider or other work is a finding. A drawing can't show whether a step ran, which model version decided it, what it cost or what it really read. Those are in the records.

## The red line

Where a live system does something its design doesn't allow, the line for it is drawn red, labelled with what is wrong. A red line is a finding about the live system, not part of the design, so `awml check` doesn't hold it to the operations' limits or to the boxes it crosses.

## What AWML is not

AWML runs nothing and grants nothing. In a workflow tool such as n8n, the diagram is the program: drawing a line makes data flow. An AWML drawing is the description a system is checked against. Every part and line is something the design names, so a drawing can be made from the design itself, and then it can't drift from it. Drawing a workflow shows what a list of scheduled tasks hides, such as a kitchen screen the design says shows the week's dinners, though nothing in the design keeps it current.

## The file format

One YAML file holds one workflow's design and any implementations. It has no `view`.

```yaml
awml: "0.5"
title: The family's dinners
scopes:
  - { id: carers, label: "carers: + June, Beth", processing: any }
  - { id: family, label: "family: parents, kids", in: carers }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
  - { id: mealplan, kind: context, label: "Meal plan", in: family }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: mealplan, kind: data, label: posted }
implementations:
  home:
    label: At home
    blocks:
      - { id: server, label: "Server: claude-sonnet-4-5", platform: scripts-server, parts: [plan, mealplan] }
    comments:
      - { id: keys, label: "The server also holds the rota app's key", in: server, ties: [plan] }
```

The file needs `awml`, the version `"0.5"`; `title`, the workflow's name; and `parts`. It may have `scopes`, `lines` and `implementations`.

- A **scope** has an `id` and a `label`, and may have `in`, the id of the scope it sits inside, and `processing`: `any`, `none`, or a map from company ids to `any` or a list of operations.
- A **part** has an `id`, a `kind` and a `label`, which a fork doesn't need and never shows. It may have `in`, the id of its scope. A step, `ai`, `code` or `person`, may have `op`, its operation. A trigger may have `wait`, an ISO 8601 duration such as `PT5M`. A context may have `at`, a path such as `notes/projects`.
- A **line** has `from` and `to`, the ids of two parts, and a `kind`. It may have a `label`, and `breaks: true` to draw it red.
- **Implementations** are a map from an implementation id to a `label`, a list of `blocks` and an optional list of `comments`. The map's order is the order the platform views appear on a page.
- A **block** has an `id`, a `label` and `parts`, a list of part ids. It may have `platform`, the id of a platform description: a bare id written like a company id, or `owner/platforms/slug` for a published one. It may have `in`, another block in that implementation. Every part except a trigger or comment must appear in exactly one block of each implementation. Triggers and design comments may be unplaced, but may never be placed twice.
- An implementation **comment** has an `id`, a `label`, `in`, the id of its block, and `ties`, a list of design part ids. It becomes a comment part after the design's parts, with a `tie` line to each target after the design's lines.

An id is letters, digits and underscores, starting with a letter. Scope, part, block and comment ids are unique across the file. Implementation ids have their own namespace. Every id, label, kind and reference to an id is text. `awml` reads YAML 1.2, which reads `true`, `false`, numbers and dates such as `2026-10-20` as something other than text, so quote them. YAML 1.1, which many other tools read, also reads `yes`, `no`, `on` and `off` as true or false, and `19:00` as a number, so quoting those too keeps a file the same in every tool. `breaks` is `true` or `false`, unquoted. The drawing keeps the parts in the order the file lists them, and draws each scope or block after the parts beside it. List the parts in the order the work flows, with its triggers first. A block's `parts` list chooses placement, not drawing order.

`awml check` applies the security rules to the design and the platform rules to each implementation. It reports a missing or unknown field, an unknown kind, an id used twice or badly written, a value YAML didn't read as text, a `breaks` that isn't true or false, an unknown scope, part or block reference, a scope or block nested in itself, a comment linked by anything but `tie`, a `tie` with no comment at either end, an unknown `op` or one on a part that isn't a step, a step that does what its operation must not, as far as the drawing shows it, an outside action whose label gives no tier after its last colon, a tier that disagrees with what stands in front of the action, a standing approval with no line to a surface, a `data` line that moves content anywhere but its own scope or a scope inside it, a `processing` that isn't `any`, `none` or a map of company ids to `any` or known operations, an inner scope's `processing` that allows a company or operation its outer scope doesn't, `processing` on a block, `platform` on a scope, a block's `platform` badly written, a `wait` that isn't a duration or is on a part that isn't a trigger, and an `at` that isn't a path or is on a part that isn't a context. It also reports a part placed twice in an implementation or left unplaced when it needs a block. It warns when AI decides a `do` step or one `do` step leads to more than one change. Warnings leave the exit status alone. Messages for rules shared with older files keep their wording, including the word group for a scope or block.

A **platform description** is a file of its own. It needs `awml`, the version `"0.3"`, `"0.4"` or `"0.5"`; `platform`, with an `id`, written like a company id, or in a 0.5 description as `owner/platforms/slug`, and a `label`; `off_for_up_to`, an ISO 8601 duration; `steps`, `code` or `agent`; `agents`, a list of company ids, which may be empty; `calls`, a list of company ids or `any`; and `holds`, a list of paths, which may be empty. It has no workflow fields. `awml check` reports a missing or unknown field, a value of the wrong kind and a company id or path badly written. A company id is lower-case letters, digits and hyphens, starting with a letter. In `owner/platforms/slug`, the owner and slug are each written like a company id. A path is names of letters, digits, dots, hyphens and underscores, joined by `/`, with no leading `/` and no `.` or `..` names.

`awml draw file.yaml` draws the security view. `awml draw --implementation ID file.yaml` draws one platform view. `awml page` draws the security view followed by every implementation, each with a heading. `awml summary` counts the workflow once and includes the platforms from all its implementations.

`awml fit file.yaml DESC...` judges the design against platform descriptions. With `--implementation ID` it judges the chosen placement. It prints one line for each platform that fits, naming the companies each AI step may use there, and one line for each reason a platform doesn't fit, as in [Placing a workflow](#placing-a-workflow-where-it-can-run). It exits 0 when it could judge every platform, whether or not any fits, and 1 when a file has mistakes or the design has a gap. With `--json` it prints an object whose `results` hold one entry for each platform, or each block of an implementation: its `platform`, whether it was `judged`, whether the workflow `fits`, the `reasons` it doesn't, and in `models` the companies each AI step there may use. Its `gaps` list what kept the workflow from being judged.

Files marked `"0.4"` are read as before: they have no standing approval, their tiers are only words in a label, which `awml check` doesn't read, and a block's `platform` may only be a bare id. Files marked `"0.3"` or `"0.2"` keep their separate security and platform views, with `groups` and `view`, and are read as before. A 0.2 file may use none of the fields 0.3 added. Given a legacy pair with the same title, `awml check` compares parts by id and lines by their ends and kind, apart from comments and any trigger the platform view leaves out. Legacy `awml fit` still takes a security view followed by an optional platform view and the descriptions.

`awml convert [--implementation ID] SECURITY PLATFORM` prints one 0.4 file from a matching legacy pair. It refuses the same part and line differences as the comparison. The security view supplies the title, parts, lines and their order; platform groups become blocks in the named implementation, which is `default` when the option is absent, and platform-only comments become its comments. Clashing block and implementation comment ids gain a `block_` or `comment_` prefix until unique. YAML comments from both files are kept at the top. Conversion keeps the canonical design, so platform labels, layout and omitted triggers may change its drawing.
