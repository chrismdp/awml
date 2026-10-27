# Reviewing a change to AWML

This says what failure looks like in this repository. The reviews on every pull request, and the build workflow's own before it opens one, hold every change to it, and a person reviewing by hand can use it too. `SPEC.md` is the authority on what AWML means, and `AGENTS.md` says how the repository is worked on.

A finding is **Important** when the change breaks one of these, or does something else that would be wrong on main. It's a **Nit** when it works but could be clearer or simpler. It's **Pre-existing** when the fault was there before the change and the change only passes near it.

## What failure looks like

- **The code drifts from the spec.** The CLI or the package accepts a file the spec forbids, rejects one it allows, or works something out differently from how the spec says. A change to what AWML means changes `SPEC.md`, the guide and the README with it, and the skill's copies match their sources.
- **WebAssembly answers differently from the CLI.** `check`, `summary`, `draw`, `fit` and `convert` give the same answer through `cmd/awml-wasm` and `wasm/awml.js` as through the CLI, and the registry relies on that. A change to one that leaves the other behind, or a parity test that no longer covers what changed, is a failure.
- **A single file behaves differently from main.** A workflow on its own, with no other files beside it, gives the same answers it gave before, unless the change means it to and says so. A change to references between workflows, or to shared parts, mustn't alter how a lone file is read.
- **Output whose order isn't fixed.** Everything the tool writes, such as drawings, summaries, messages and JSON, comes out in the same order every time. Ranging over a Go map, or sorting without a tie-break, gives a drawing or a test that changes from run to run.
- **Untrusted text reaches an outside action.** Text from an issue, a comment, a pull request, a log or a model never decides what a workflow pushes, merges, labels or comments, except as data that a person or a code rule reads. A path, a number or a name taken from such text is checked before it's used.
- **Event, issue or Claude text goes into a `${{ }}` expression in a workflow.** Inside `run:`, such text reaches the shell only through `env:`, quoted. A `${{ }}` that expands it in the script lets the text run as code.
