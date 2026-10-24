# AWML

Agentic work today is tangled up in the tools that run it, so it's hard to say what a workflow does, who may see each part of it, or how to change one piece without rebuilding the rest. AWML, the Agentic Workflow Modelling Language, describes a workflow in a small YAML file, apart from those tools, and draws it as a diagram through two lenses: who may see each part, and where each part runs. That lets you audit where content crosses to a wider audience, improve one step at a time, move a part to a better platform, and optimise the patterns that recur.

The file lists the workflow's triggers, the steps that pass work along, the notes they read, and the surfaces and outside actions it ends in. AWML draws it with Mermaid and gives each step a colour and an emoji so you can see at a glance whether AI, code or a person decides it. A drawing runs nothing and grants nothing: it's the description a live system is checked against.

![A family's dinners, by who may see each part](examples/dinners.svg)

A fictional family's dinners. On Thursday evening AI plans next week's dinners and posts the plan in the family chat. On Saturday code fills the supermarket basket, and checking out waits for a parent's yes to the exact basket. Every morning code checks the joint account against the bills expected, which only the parents see. The thick line is the meal plan moving out to a wider scope, so the carers can see it on the kitchen screen. The comment is a gap that drawing it found: nothing keeps that screen current.

[`SPEC.md`](SPEC.md) is the short, exact definition, and wherever it and this README differ, the spec is right.

## Contents

- [Install](#install)
- [Use it with Claude or ChatGPT](#use-it-with-claude-or-chatgpt)
- [Why AWML](#why-awml)
- [Drawing your own workflows](#drawing-your-own-workflows)
- [Three layers, for one consultant](#three-layers-for-one-consultant)
- [Reading a drawing](#reading-a-drawing)
- [Operations](#operations)
- [Two lenses on one workflow](#two-lenses-on-one-workflow)
- [The security view](#the-security-view-who-may-see-each-part)
- [Connectors](#connectors)
- [The platform view](#the-platform-view-where-each-part-runs)
- [Tiers and yeses](#tiers-and-yeses)
- [Standing approvals](#standing-approvals)
- [Untrusted text](#untrusted-text)
- [Records](#records)
- [The red line](#the-red-line)
- [A tour of the gallery](#a-tour-of-the-gallery)
- [Improving one step at a time](#improving-one-step-at-a-time)
- [What AWML is not](#what-awml-is-not)
- [Using awml](#using-awml)
- [What comes next](#what-comes-next)
- [Status and licence](#status-and-licence)

## Install

```sh
go install github.com/chrismdp/awml/cmd/awml@latest
```

This needs Go 1.22 or later, and puts `awml` in Go's own folder for commands, usually `~/go/bin`. Or download a binary for Linux, macOS or Windows from the [Releases page](https://github.com/chrismdp/awml/releases), unpack it and put `awml` on your path. [Using awml](#using-awml) covers the commands and the file format.

## Use it with Claude or ChatGPT

This repository is also a plugin with two skills. `awml` helps an AI assistant work out a workflow with you, write its design and implementations in one file, check it and draw its views, all on your machine. `awml-publish` is optional: see [Publishing to a registry](#publishing-to-a-registry).

- **Claude Code:** run `/plugin marketplace add chrismdp/awml`, then `/plugin install awml@awml`. Then ask Claude to draw a workflow, or type `/awml:awml`.
- **claude.ai and the Claude desktop app:** download `awml-skill.zip` from the [Releases page](https://github.com/chrismdp/awml/releases). Go to **Customize > Skills**, click **+**, then **Create skill**, then **Upload a skill**, and choose the zip. Skills need a Pro, Max, Team or Enterprise plan, with **Code execution and file creation** turned on in **Settings > Capabilities**. You can instead add the plugin in **Customize > Plugins**, under **Add > Add marketplace**, with `chrismdp/awml`.
- **Codex:** run `codex plugin marketplace add chrismdp/awml`, then `codex plugin add awml@awml`, or install it from `/plugins` in the Codex CLI. Start a new session to use it.
- **ChatGPT:** a workspace owner or admin can add the plugin for their workspace. In the Admin console, open **Plugins**, choose **Add**, then **Import marketplace**, and enter `https://github.com/chrismdp/awml`. OpenAI's help pages describe no way to add it to a single account.

### Publishing to a registry

A registry stores and draws AWML workflows, numbering each version. [awml.io](https://awml.io) is the public one, and a company can run its own. Installing the plugin connects to no registry. The `awml-publish` skill comes into play only when you ask your AI to publish or share a workflow. It comes with the plugin; `awml-skill.zip` on the Releases page holds only the `awml` skill.

When you ask, the skill uses the registry's tools if your AI already has them. If not, it gives you one line to connect `https://awml.io/mcp` for your client, such as `claude mcp add --transport http awml https://awml.io/mcp` for Claude Code, or it publishes with `awml login` and `awml push`. Every call to a registry's MCP server needs you signed in, reading included. Public workflows stay readable without an account on the website and with `awml pull`.

To use a registry of your own, set `AWML_REGISTRY_URL` to its address, such as `https://registry.example.com`. The skill and the `awml` command then use it in place of `https://awml.io`, and its MCP server is at `https://registry.example.com/mcp`. `--registry` names a registry for one command.

### For admins

To point everyone in a company at your own registry without changing the plugin, connect the registry for them and set `AWML_REGISTRY_URL`. When the registry's tools are already connected, the skill uses them and never asks which registry. Each person still signs in to the registry once, as themselves.

**Claude Code.** In managed settings, `managedMcpServers` gives every user a remote MCP server alongside their own, and `env` sets environment variables for every session and the commands it runs. Put them in `managed-settings.json`, in `/Library/Application Support/ClaudeCode/` on macOS, `/etc/claude-code/` on Linux and WSL, or `C:\Program Files\ClaudeCode\` on Windows, or deliver them as server-managed settings from the claude.ai admin console:

```json
{
  "managedMcpServers": {
    "awml": { "type": "http", "url": "https://registry.example.com/mcp" }
  },
  "env": { "AWML_REGISTRY_URL": "https://registry.example.com" }
}
```

`managedMcpServers` needs Claude Code v2.1.259 or later and an `https://` address. People sign in with `/mcp` the first time. To allow only the servers you list, deploy `managed-mcp.json` in the same folder instead, written like a project's `.mcp.json`. See [Control MCP server access for your organization](https://code.claude.com/docs/en/managed-mcp), [Deploy managed settings](https://code.claude.com/docs/en/managed-settings) and [Settings](https://code.claude.com/docs/en/settings).

**Claude.ai and Claude Desktop, on a Team or Enterprise plan.** An owner adds the registry in **Organization settings > Connectors**: **Add**, then **Custom**, then **Web**, with the URL `https://registry.example.com/mcp`. Each member then connects it in **Customize > Connectors** and signs in. See [Get started with custom connectors using remote MCP](https://support.claude.com/en/articles/11175166-getting-started-with-custom-connectors-using-remote-mcp).

**Codex.** Managed defaults go in `/etc/codex/managed_config.toml` on Linux and macOS, or in the `config_toml_base64` key of the `com.openai.codex` domain through MDM on macOS, and apply when Codex starts. These lines connect the registry, with each person's token in `AWML_TOKEN`, and set `AWML_REGISTRY_URL` for the commands Codex runs:

```toml
[mcp_servers.awml]
url = "https://registry.example.com/mcp"
bearer_token_env_var = "AWML_TOKEN"

[shell_environment_policy.set]
AWML_REGISTRY_URL = "https://registry.example.com"
```

See [Managed configuration](https://learn.chatgpt.com/docs/enterprise/managed-configuration) and the [configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference.md).

## Why AWML

Any AI system that does work has three layers:

- **The operating model:** what the work is and how it's organised.
- **The workflow:** the steps the work goes through, driven by the operating model.
- **The platforms:** the AI systems, harnesses, models and programs that carry out the work.

AWML describes the middle layer.

AWML has no notion of an agent: an AI might act as one, depending on the platform, but the word isn't precise enough to describe a workflow. Each step says who decides it instead: AI, code or a person.

Drawing a workflow gives you:

- **Auditing:** where content moves to a wider scope or across tools, and how much of the work is automated.
- **Hill climbing:** each step can be scored and improved on its own, as [Improving one step at a time](#improving-one-step-at-a-time) shows.
- **Portability:** the workflow stays the same when a platform changes, so each part can move to the best platform.
- **Efficiency:** the same patterns of steps turn up again and again, so each can be optimised once.

The thinking behind AWML is in [You Must Draw Your AI Workflow to Control It](https://www.chrismdp.com/awml/).

## Drawing your own workflows

If you already run AI routines, such as scheduled tasks, bots and automations, [`GUIDE.md`](GUIDE.md) turns them into AWML files, one workflow at a time. You trace real runs from their records, draw what the design allows, and mark in red what the runs did beyond it. The `awml` skill follows the same guide to find and trace a workflow you already run.

## Three layers, for one consultant

Take a fictional consultant who answers clients' email:

- **Operating model:** every client engagement is a project, and anything that goes out in the consultant's name waits for the consultant's yes.
- **Workflow:** each weekday morning the inbox is brought in, and a client's email updates the engagement's project. A reply is drafted and left in the consultant's drafts folder, and the consultant edits it and sends it.
- **Platforms:** Claude Cowork on a Mac mini, Markdown notes synced to the laptop and the Mac mini, Gmail, and the consultant's phone. Any of these can change.

The operating model has little to do with AI: it's how work is organised, whoever or whatever does it. The platforms each work with AI in their own way, slowly converging but with no standard yet, and much of what they run isn't AI at all: code that sorts receipts needs no model. A platform's incentive is lock-in, so the description of the work tends to end up in the platform's own format, where only it can read it. AWML keeps the workflow in a plain file of its own, so the platforms can change around it.

## Reading a drawing

### The parts

![Every kind of part, laid out as a key](examples/legend.svg)

- A **trigger** is a pink pill. It starts the workflow: a time, such as `Weekdays 07:00`, or an event, such as someone asking in a chat or an email arriving.
- A **step** is a rounded box: one piece of work, with the colour and emoji of who decides it.
- A **review** is an amber rounded box, like a step, with `👤 ✅ review` above its label: a person's yes to exact content before it goes ahead. Its one way out is a line with the guard `[yes]`.
- A **standing approval** is an amber rounded box too, with `👤 📜 standing approval` above its label: a person's yes given once to named kinds of action, within limits.
- A **fork** is a black bar, where work splits to run side by side.
- A **context** is a pale blue cylinder: the notes a step may read or write, kept as one folder.
- A **connector** is a grey hexagon: someone's access to an outside service, such as a calendar, a mailbox or a bank account.
- A **surface** is a white window box: where people meet the system's work, such as a chat, a list on their phone or a screen on a wall.
- An **outside action** is a coral flag: something that changes the world outside the system, such as sending an email or paying a bill. Its label gives its [tier](#tiers-and-yeses) after its last colon, such as `Send the reply: Ask`.
- A **comment** is a dashed box with a cut corner. It points out a gap, or gives an example of what a part could carry, and is tied to what it explains.

Posting on a shared surface to the people it serves is delivery, not an outside action, so a family chat needs no flag and no review.

### Who decides each step

Read a step's colour and emoji first, because they tell you what kind of thing you're trusting. The emoji says what the colour says, for anyone who can't tell the colours apart and on a page printed without them.

| Colour | Emoji | Who decides | What that means |
|---|---|---|---|
| Green | ⚙️ | Code | It does the same thing every time. It costs almost nothing to run, and you can test it once and rely on it. |
| Blue | 🤖 | AI | A judgement call that can be wrong in new ways, so it needs inspecting and improving, and its output is worth scoring. |
| Amber | 👤 | A person | The slowest step of all, and the one that needs someone to be there. Keep it for where it matters, such as a yes to something that can't be undone. |

A workflow whose every step is green ⚙️ has no AI in it, even if AI helped build it.

### The lines

| Line | Drawn as | Means |
|---|---|---|
| `start` | An arrow | Starts something, as a trigger starts a step |
| `data` | An arrow | Moves data from one part to the next |
| `publish` | A thick arrow | Moves content into a wider scope, which is publishing |
| `refused` | A line ending in a cross | A read the design refuses |
| `tie` | A grey dotted line | Links a comment to what it explains |
| `hidden` | Nothing | Steers the layout only |

The solid lines are the path the work takes, from the trigger through each step to the next. A line into or out of a context, a connector or a surface is dotted, since it's information moving in and out. A `publish` line is thick either way, and a `refused` line keeps its cross.

A line may carry a label. Words in square brackets are a **guard**, the condition the line is taken on. A line drawn red is a finding: see [The red line](#the-red-line).

## Operations

Besides who decides it, a step may name its **operation**, the kind of work it does. The two are separate: AI can sort or draft, and so can a person. The drawing shows both in small grey type above the step's label, who decides first and then the operation, as in `🤖 ✏️ draft` for a draft AI writes. Naming an operation is optional, and a step without one shows only who decides it.

![One step in each of four operations](examples/tour/10-operations.svg)

| | Operation | `op` | It answers | It produces | It must never |
|---|---|---|---|---|---|
| 📥 | Bring in | `bring-in` | What's new? | The input as it really is, such as an email, a transcript or a calendar, made readable | Decide where it goes |
| 🗂️ | Sort | `sort` | Where does this belong? | A place for it, or none with the reason | Change anything outside the system |
| 🧠 | Learn | `learn` | What does this teach us? | New notes, or more in old ones, and rules taken from corrections | Act in the world |
| 📋 | Prepare | `prepare` | What's needed before this? | A brief for a named moment, such as a meeting | Decide for a person |
| 🔍 | Check | `check` | Is anything wrong or missing? | A verdict | Change anything |
| 🪞 | Reflect | `reflect` | Is time going where it should? | Evidence and questions for the people who set the aims | Make up an aim |
| ✏️ | Draft | `draft` | What should be said? | A draft | Send it or publish it |
| 🛠️ | Do | `do` | What should change, in the world or in a piece of work? | The change, within its tier | Act without the yes its tier needs, or replace something without keeping what was there |
| 💬 | Answer | `answer` | What do we know about this? | An answer from the notes, naming them | Act, or change a note |
| ❔ | Other | `other` | Work that fits none of the nine | Whatever it is for | Nothing AWML checks |

A `do` step should be code acting on what an earlier step decided. An AI or a rule turns the input into a score or a choice, and the `do` step acts on it with thresholds, lookups and templates. Each step does one job. A `do` step changes one thing. Filing an item, labelling it and telling a person are three `do` steps after one `sort`.

A drawing can show some of these limits, and `awml check` holds every step to them:

- **Only a `do` step leads to an outside action**, by a line straight to it or through a review, a standing approval or a fork. A draft step never leads to an outside action, even through a review: a draft goes to a `do` step that asks to send it, or to a person's own `do` step.
- **An `answer` step and a `check` step never write to a context.** They read contexts, and speak on surfaces. Within one run, a check's verdict travels on its lines out as the branch taken, such as `[all pass]` or `[a check fails]`. For a later run, a step after the check writes the verdict as ordinary state, which that run reads as a context.
- **An `op` names one of the ten,** and only a step names one.

A line leads somewhere when it starts something, moves data or publishes. A hidden line, a refused read and a tie lead nowhere. A step marked `other`, or with no `op` at all, is held to none of these rules.

`awml check` warns when a `do` step is decided by AI or leads to more than one change. Each outside action and surface is one change, while all the contexts together are one change. Red lines don't count, and warnings don't make the check fail.

The other limits, such as never deciding for a person, are about what happens inside a step, which no drawing shows, so the people who read it judge them.

When a rule is broken, `awml check` says which step and why:

```
part "write" leads to outside action "send", but its op is draft: only a do step leads to an outside action
part "answer" writes to context "notes", but its op is answer: an answer step never writes to a context
part "due" has unknown op "chek"
```

## Two lenses on one workflow

Every workflow raises two questions, and a drawing that answers one hides the other, so each gets its own view:

- **The security view**: who may see each part, and where content crosses from one group of people to a wider one.
- **The platform view**: where each part runs, and how the platforms fit together.

One file holds the design and its implementations. The parts and lines are written once, and each implementation places them in blocks. Draw the security view and each platform view, and read them side by side.

The consultant's email, through both lenses:

<table>
<tr><th>The security view</th><th>The platform view</th></tr>
<tr>
<td valign="top"><img src="examples/client-email.svg" width="416" alt="A client's email, by who may see each part"></td>
<td valign="top"><img src="examples/client-email-office.svg" width="353" alt="A client's email, by where each part runs"></td>
</tr>
</table>

The security view has two scopes side by side: the consultant's private scope, and the business, which the consultant shares with an associate. Two thick lines cross between them, and each is publishing, for a different reason. A standing rule lets a client's email into the business, which adds a viewer, the associate. The consultant takes the reply back into the private scope by editing it and sending it, which loosens the terms: the business keeps client work under the engagement's confidentiality terms, and the consultant's own mailbox doesn't carry them.

The platform view spreads the same twelve parts over four platforms, with the edit on the consultant's phone. That's how most agentic work is put together today: no single tool can yet run a whole workflow, so most of us join several. Wherever the flow crosses from one platform to another, it needs the right connector in place, or someone has to copy and paste. Moving the notes into a notes service, or the scheduled task from Claude Cowork to a managed agent service, moves those parts to other blocks and changes nothing else. Swapping the model moves no part at all.

## The security view: who may see each part

A **scope** is a set of people. Everything in a workflow belongs to one scope, which says who may see it and which companies may process it: the makers of the models the system uses, and services such as transcription or the host of a folder. A scope's terms say whether a company may keep the content, and a company that may train on it is never used at all. A scope may also allow no processing, as for something told in confidence.

In the security view, each scope is a dashed box, labelled with its name and members, such as `family: parents, kids`. A box around another can name only the people it adds, with `+`, as in `carers: + June, Beth` around the family's box.

### Where each part sits

- A **context** sits in the box of its scope.
- A **step** sits in the box of the scope it writes into, with the reviews and outside actions it leads to.
- A **standing approval** sits in the box of the scope of the person who gives it.
- A **connector** sits in the box of the scope its inputs take, so a connector only the parents may use sits in the parents' box, and nothing outside that box can read through it.
- A **surface** sits in the box of the scope its viewers make up.

### Boxes inside boxes, and side by side

One box sits inside another only where every member of the inner scope is in the outer one. A family's scopes often nest: the parents' box sits inside the family's, which sits inside the box of the family and its carers.

Scopes aren't a ladder, though. Two may overlap, each with someone the other lacks, and then their boxes sit side by side. Boxes also sit side by side when one scope's people are all in the other but its terms are looser, as with the consultant's private scope beside the business in [the client email](#two-lenses-on-one-workflow).

![A checklist moving from a client's scope to a side venture's](examples/tour/05-side-by-side.svg)

Here Jo keeps the books for a bakery and runs a pottery shop. The two scopes share only Jo, so neither box goes inside the other, and any line between them, such as this checklist, is publishing.

### Reading across scopes

A step reads from its own scope or a wider one, since everyone in a scope may see what a wider scope holds. A read from a narrower scope is one the design refuses, drawn as a line ending in a cross. In the family's question below, the answer may use the family's notes and the carers' notes, but never one person's own notes, even when that person is the one asking.

![A question in the family chat](examples/question.svg)

### Publishing

Content moves between scopes in only two ways.

- A move **narrows** when every member of the target scope is in the source, and the target's terms are at least as strict. Narrowing is routing: it shows content to fewer people, or to the same people more carefully, so it's drawn as an ordinary line.
- Any other move adds a viewer or loosens the terms. That's **publishing**, drawn as a thick line so you notice it first.

Publishing waits for a person's yes, unless a **standing rule** names that target for that kind of item. A standing rule moves only the words of people who agreed to that target, and the thick line's label names the rule and who they are, as in `co-op notes, under the bakery's rule, for customers who ticked the box`. Only the item crosses, never what lies behind it. Nothing reaches the public without a person's yes to that item, under any rule or approval.

## Connectors

A **connector** is access to one outside service through one credential: a person's, one an outside party lends, or the system's own. The design names whose access each connector is, which service it reaches and what it may do, such as read, post or carry an outside action, and its label says whose it is, as in `Owen's supermarket account`.

Lines to a connector follow the data. A read comes out of the connector, and a write, or an outside action carried out through it, goes in, so an order placed through a supplier account has its line into the account's connector.

Whose access it is matters, because a connector speaks with that access.

- **A person's connector** reaches only what that person can, and speaks in their name, so they, or a role they choose, approve what it sends. It stops when they leave, so work that must outlive them shouldn't rest on one.
- **The system's own connector**, such as a service account, often reaches more than any one person. So each item it brings in takes the scope of who could see it where it came from, never everyone's.

## The platform view: where each part runs

In the platform view, each platform is a tinted block, and each part sits in the block of the platform it runs on or lives in. A **platform** is a machine or service that brings its own surfaces, its own **harness** (the program that runs each step and calls a model) and its own models, and often its own triggers: Claude Cowork on a Mac mini, a mail service or a phone. So a trigger sits in the block of the platform that fires it, and a model on its own isn't a platform: it's a choice a platform makes. A **machine** is anywhere the system's tools run that shares one set of access: a server, a laptop, or a sandbox on either.

![The family's dinners, by where each part runs](examples/dinners-home.svg)

A block is labelled with what an **implementation** chose there, such as the model it runs, and with when the machine is on, if that isn't always, as in `Noor's laptop: on some evenings`. Work for a machine that is off waits for it. An implementation is one running system built from a design, and each draws its own platform view, with the same parts in different blocks.

Two rules make this view worth drawing.

- **Anything running on a machine can reach everything that machine holds, and send it to any host the machine can reach.** So a machine holds only contexts and connectors whose scopes allow every company that processes content there, and reaches only the hosts its connectors need.
- **An implementation's choices change how a step is done, never what it may do.** The contexts a step reads, the tier of each action it takes and its review stay the design's call. A different model may mean a different company processing the content, and every context feeding the step must allow that company.

## Tiers and yeses

Every outside action has a **tier**.

| Tier | When it applies | Drawn as |
|---|---|---|
| **Allow** | The action can be undone when it's taken, is seen only within its scope, and stays within the design's spending limits: a note, a draft, a label, an entry in a person's own calendar | A flag steps lead to with no review or standing approval in front of it: it just happens |
| **Ask** | Any other action that isn't blocked, and any the design marks Ask because the person who answers for it would be surprised | A flag with a person on every way to it: a review, a standing approval, or a person's own `do` step leading straight to it. It waits for a yes, or goes under a standing approval |
| **Block** | Never, whatever anyone says | Never a line. A flag with nothing leading to it may name it, and a line into one is red |

The tier follows from the drawing's shape: wherever a tier says a person decides, the drawing shows them. `awml check` reports an Ask action with no person on one of its ways in, an Allow action behind a review or a standing approval, and a line that isn't red into a Block action.

A draft may be a context or an Allow outside action. A draft the workflow keeps for itself and reads back, such as a file it revises, is a context. A draft a `do` step leaves in an outside service for a person to pick up, such as the drafts folder of their mailbox, is an Allow outside action: the `draft` step writes the text, and the `do` step saves it there.

A **yes** is an approver's yes to one action and its exact content, shown with what it's for: the work it serves, why now, what started it and what it drew on. A yes to "send the reply" in general is no yes at all. Any change to the content after the yes, a fix included, needs a new yes.

A person's own `do` step is their yes: when the consultant edits the reply and sends it, they've said yes to its exact content. So an Ask action reached straight from one needs no review in front of it.

An **approver** is a role, such as the approver of sends, held by one or more people. A rare, high-stakes kind of action may need a yes from two approvers, two people in two roles, to the same content, drawn as two reviews in a row. Every such action waits on both, so it's kept for few kinds.

## Standing approvals

Asking for a yes to every small, reversible action soon trains people to say yes without looking. A **standing approval** is the way out. The person who answers for a scheduled task approves it once to take named kinds of action that can be undone, within limits such as how many and how often, so each item doesn't wait for a yes. Each action under it is recorded and listed for that person, with how to undo it. One that acts in someone else's name needs their yes too, and ends when their access does. Nothing that can't be undone goes this way, such as a send to a person or a payment, and nothing that reaches the public.

A standing approval is a part of its own, in the box of the person who gives it, between the `do` step and the Ask actions it covers. Its label says whose it is and gives its kinds and limits. A line from it goes to the surface where each action is listed with how to undo it, and `awml check` reports one without. No drawing shows whether an action can be undone, so that stays for its readers to judge.

![Library renewals under a standing approval](examples/tour/12-standing-approval.svg)

Here Sam approved the morning renewals once, for renewals only and up to six a week, and each renewal is listed on Sam's list.

## Untrusted text

Text that anyone can write, such as a web form or a public inbox, can try to steer the AI step that reads it. Mark the trigger, connector or context it comes in through with `untrusted: true`, and it's drawn with ⚠️ after its label. The text goes wherever the lines lead on, until a person's review, a person's own step or a code step marked `cleans: true` stops it. A `cleans` step promises to pass on only what's safe, such as an order number and a quantity, and its label says how. Only code can promise that, since strangers' text can steer AI however it's instructed. A standing approval never sees the text, so it doesn't stop it.

`awml check` warns of each outside action the text reaches, and `awml summary` lists what each untrusted part's text reaches. `awml draw --untrusted` draws the untrusted view: the security view with each part the text reaches tinted, 🛡️ on what stops it, and each outside action it reaches in red.

![A cake order from the website, in the untrusted view](examples/tour/14-untrusted-text-untrusted-view.svg)

Here anyone can type into the bakery's order form. A code step keeps only the cake, the quantity and the date, so the bake is booked from those alone, and the customer's note stops at Ana's yes to the exact reply.

## Records

Every step leaves one **record** each time it runs: what started it, what decided it (code, a model with its instructions and exact version, or a person), what it drew on, what it made and what it cost. Records are only ever added to, and only the system writes them.

A step's own record of its run is its record, written by the system, so it isn't drawn as a context. A log or state that a later step reads back is a context.

A drawing shows what each record should say, never what any record does say, so a record that names other sources, another kind of decider or other work than the drawing shows is a finding. Only the records show whether a step ran, which model version decided it, what it cost and what it really read.

## The red line

Where a live system does something its design doesn't allow, the line for it is drawn red, labelled with what is wrong. A red line is a finding about the live system, not part of the design. It usually comes from the records, set against the drawing.

![The club's Friday message, sent without a yes](examples/tour/06-red-line.svg)

Here the design waits for Ella's yes to the exact message, but one week the live system sent the draft straight out. Drawn as it ran, that line breaks the rules for an Ask action and for a draft step, so `awml check` reports it and `awml draw` draws it red with what's wrong. Nothing marks it by hand, and a drawing with a red line never passes the check as a finished design. A breach the check can't find goes in a comment tied to the part it's about.

## A tour of the gallery

Each drawing here shows one idea, with fictional people and everyday cases, and their files are in [`examples/tour/`](examples/tour). Several are patterns, and the commonest are the first two: the draft for review and the do step. The rest of the gallery appears beside the idea it draws: [two scopes side by side](#boxes-inside-boxes-and-side-by-side), [operations](#operations), [a standing approval](#standing-approvals), [untrusted text](#untrusted-text) and [a red line](#the-red-line).

### Draft for review

AI drafts the swimming club's Friday message to the parents. A `do` step asks Ella to send it, and the message goes out only after Ella's yes to that exact message. Use it for anything that goes out in someone's name or can't be taken back.

![The club's Friday message: a draft, Ella's yes and the send](examples/tour/01-draft-and-send.svg)

### Do steps

Two `do` steps settle the window cleaner's visit, one change each. Putting the next visit in the parents' calendar can be undone, so it's Allow and just happens. Paying is Ask, so it waits for Dev's yes to the exact payment. Use a `do` step for any change in the world, or in the state of a piece of work.

![The window cleaner's visit: one Allow action and one Ask action](examples/tour/11-do.svg)

### Answer from the notes

Ana asks in the flat's chat when the boiler is serviced. The answer comes from the flat's notes alone, and the cross marks Tom's own notes, which it may not read. Use it wherever someone asks a question on a surface others can see.

![A question in the flat's chat that may not read Tom's own notes](examples/tour/03-refused-read.svg)

### Check by code

Every morning, code matches the joint account against the bills expected, reports anything missing or different in the parents' chat, and changes nothing. Use it for any question code can settle.

![The morning bills check](examples/tour/13-check-by-code.svg)

### Publish

The week's school pickups are copied into the parents' notes, and the thick line moves them into the wider scope that adds Grandma Rose, under the standing rule it names. Use it whenever something leaves its scope for a wider one.

![The week's pickups published to Grandma Rose's tablet](examples/tour/04-publishing.svg)

### A fork

The school's newsletter arrives, and the bar splits the work so two steps take it side by side: one takes the term dates into the family notes, and one looks for anything that needs a reply.

![The school newsletter, split between two steps](examples/tour/02-fork.svg)

### A step a person does

Code checks what the car needs each month and puts the MOT on Ade's list. Booking it is Ade's own step, so it's amber and marked 👤, and it waits there.

![The car's MOT, booked from Ade's list](examples/tour/07-person-step.svg)

### A gap

Drawing bin day shows what a list of its tasks hides: nothing posts the bin day in the house chat the night before. A comment marks the gap.

![Bin day, with a comment marking a gap](examples/tour/08-gap.svg)

### A platform view

The family's photos run on two machines: a server that never stops, and Noor's laptop, which is on only some evenings. The photos wait on the server until the laptop can sort them.

![The family's photos, on a server and a laptop](examples/tour/09-platform-view-home.svg)

## Improving one step at a time

Once each step is a box with a known input and a known output, you can improve it on its own.

1. Keep a set of past cases for the step: what came in, and what it should have produced.
2. Score the step's output against those cases.
3. Try a change to that step alone: a different prompt, a different model, or code in place of AI.
4. Keep the change only if the score goes up.

Every review is a label for the step before it. Each yes, each no and each edit says how that step is doing, so the workflow gathers its own test cases as it runs. The comments on the family's planning step in the platform view are examples: one says the step is scored against the plans the family kept, and the other that it's tried on a second model some weeks.

This only works if you understand the workflow, and if your platform lets you see and change one step without touching the rest. Most of today's tools don't.

## What AWML is not

AWML isn't a visual workflow tool, where the diagram is the program and drawing a line makes data flow. Every part and line in an AWML drawing is something the design names, so the drawing can be made from the design itself, and then it can't drift from it.

## Using awml

### The commands

```sh
awml --version                            # print the installed release
awml check examples/question.yaml        # report mistakes; exits 1 on any
awml fit examples/client-email.yaml examples/platforms/*.yaml
awml draw examples/question.yaml         # print the Mermaid
awml draw --implementation home examples/dinners.yaml
awml draw --untrusted examples/tour/14-untrusted-text.yaml
awml page examples/dinners.yaml -o dinners.html
awml summary examples/dinners.yaml --json
awml login
awml push family/dinners examples/dinners.yaml
awml pull family/dinners@1 -o dinners
```

- `awml check` checks the design and each implementation. It prints each warning before `ok` or the mistakes, and exits 1 only if it finds a mistake. It still compares legacy view pairs. `--json` lists every check it makes, passing ones included, each with a stable id and its result: `pass`, `fail`, `skipped` or `warn`.
- `awml fit` judges the design against platform descriptions. Add `--implementation ID` to judge that placement, and `--json` for the report's JSON form. An implementation can be given without descriptions, and then each block says it has no platform or that its platform wasn't described.
- `awml draw` prints the security view's Mermaid. Add `--implementation ID` for a platform view, or `--untrusted` for the untrusted view. It refuses a file with mistakes, and prints them instead.
- `awml page` writes one standalone HTML page with the security view, the untrusted view if there is one, then every implementation, each with a heading. Several files appear in the order given. The page takes their shared title, or Workflows when they differ.
- `awml summary` counts the scopes, platforms, parts and lines in one workflow. It includes platforms from every implementation and counts the design once, and lists what each untrusted part's text reaches. `--json` prints the registry's JSON form.
- `awml convert security.yaml platform.yaml` joins a matching 0.3 or 0.2 pair into one 0.4 file, printed to standard output. It keeps the security design and turns platform groups into blocks of the `default` implementation. Add `--implementation ID` to name the implementation. YAML comments move to the top. The canonical labels, part order and triggers may change the old platform drawing.
- `awml login` saves an API token for the registry. `awml push` checks and uploads one 0.4, 0.5 or 0.6 file in the API's workflow field. `awml pull` downloads the latest or a numbered version as `<slug>.yaml`, and `awml pull owner/platforms/slug` does the same for a published platform description. It still downloads an older pair as `security.yaml` and `platform.yaml`. The commands use `https://awml.io` unless `--registry` or `AWML_REGISTRY_URL` says otherwise. Add `--private` to the first push of a private organisation workflow.

For example, the client email workflow can run on the scheduled scripts platform, while the other descriptions show why they don't fit:

```text
$ awml fit examples/client-email.yaml examples/platforms/cowork-mac-mini.yaml examples/platforms/scripts-mac-mini.yaml examples/platforms/codex-laptop.yaml examples/platforms/worker.yaml
cowork-mac-mini   no: bring, rules and save are code steps, which anthropic's agent would carry out
scripts-mac-mini  fits: choose, learn and draft may use anthropic
codex-laptop      no: weekdays may wait PT2H, but the platform may be off for P14D
                  no: openai's agents run here, and business doesn't allow openai
worker            no: engagements is kept at notes/engagements, which the platform doesn't hold
                  no: project is kept at notes/projects, which the platform doesn't hold
```

The Mermaid works anywhere that draws Mermaid 11.4.1. To make an SVG, use the Mermaid CLI, which needs Node:

```sh
awml draw my-workflow.yaml > my-workflow.mmd
npx -y @mermaid-js/mermaid-cli@11.4.1 -b white -i my-workflow.mmd -o my-workflow.svg
```

`examples/render.sh` redraws every example this way.

### Writing a file

One file holds one workflow and its implementations:

```yaml
awml: "0.6"
title: The family's dinners
scopes:
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
implementations:
  home:
    label: At home
    blocks:
      - { id: server, label: Server, parts: [favourites, plan] }
      - { id: phone, label: Family phone, parts: [chat] }
```

- The file needs `awml`, the version `"0.6"`; `title`, the workflow's name; and `parts`. It may have `scopes`, `lines` and `implementations`. Version `"0.5"` still reads as before, with no `untrusted` or `cleans`. Version `"0.4"` reads as before too, with no standing approval, no check of its tiers and only a bare id as a block's `platform`, and versions `"0.3"` and `"0.2"` with `groups` and `view`.
- A **scope** has an `id` and a `label`, and may have `in`, the id of its parent scope, and `processing`, the companies it allows.
- A **part** has an `id`, a `kind` and a `label`, which a fork doesn't need. Its `in` names its scope. A step may have `op`, a trigger `wait`, and a context `at`. A trigger, connector or context may have `untrusted`, and a code step `cleans`.
- An **implementation** has an id in the `implementations` map, a `label` and `blocks`. Each block has an `id`, a `label` and `parts`, the ids it holds. It may have `in`, its parent block, and `platform`, a platform description id: a bare id for your own, or `owner/platforms/slug` for one published to the registry. Every part except a trigger or comment needs exactly one block per implementation.
- An implementation may have **comments**, each with `id`, `label`, `in`, its block, and `ties`, a list of design part ids. These comments appear only in its platform view.
- A **line** has `from` and `to`, the ids of two parts, and a `kind`. It may have a `label`.
- The part kinds are `trigger`, `ai`, `code`, `person`, `review`, `approval`, `fork`, `context`, `connector`, `surface`, `outside` and `comment`. The line kinds are `start`, `data`, `publish`, `refused`, `tie` and `hidden`.
- An id is letters, digits and underscores, starting with a letter, and scope, part, block and comment ids are unique across the file. Implementation ids have their own namespace.
- Parts are drawn in the design's order, and each scope or block after the parts beside it. Unplaced triggers and design comments are drawn beside the other end of their first visible line. A `hidden` line steers the layout when Mermaid puts something in an awkward place.

Every id, label, kind and reference to an id is text. `awml` reads YAML 1.2, which reads `true`, `false`, numbers and dates such as `2026-10-20` as something other than text, so quote those. Tools that read YAML 1.1 also read `yes`, `no`, `on` and `off` as true or false, and `19:00` as a number, so quoting those too keeps a file the same in every tool. `untrusted` and `cleans` are `true` or `false`, unquoted.

### What `awml check` reports

`awml check` reports structural mistakes, invalid operations, tiers that disagree with the drawing and publishing drawn as routing. It also checks each implementation's block references, nesting and placement, and its comments' ties. It warns when AI decides a `do` step, when one `do` step leads to more than one change, and when untrusted text reaches an outside action. Warnings leave the exit status alone. A line that breaks a rule, such as one into a Block action, is still a mistake, but `awml draw` draws it red rather than refusing the file. [`SPEC.md`](SPEC.md) gives the exact rules, including the checks for older view pairs.

### Examples to start from

Besides the gallery, [`examples/`](examples) has three workflows to start from: the family's dinners and the consultant's email, each with an implementation, and the question in the family chat. It also has [`examples/legend.yaml`](examples/legend.yaml), the key at the top of [Reading a drawing](#reading-a-drawing). The tests run with `go test ./...`, and they check that every example has no mistakes and that its drawings are current.

## What comes next

- **Platforms that conform to AWML.** A platform that can read a workflow file, run it as drawn and report each step's record against it would let people move their work between platforms. Such a platform could also give the guarantees these designs badly need, such as refusing an outside action that no yes released, rather than leaving them to each workflow's author.
- **A plan for each platform.** From an AWML file, generate something like a Terraform plan: a repeatable description of how a workflow fits a particular platform, which that platform can apply. Moving a workflow would then mean applying a different plan, rather than rebuilding it by hand.
- **Evaluations for every step.** A platform could capture evaluations for each step as it runs, and rerun them on different models to find the cheapest one that still does the job well.

## Status and licence

Draft 0.6. The notation and the file format may still change.

MIT. See `LICENSE`.
