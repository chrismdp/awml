# Working on AWML

This repo is AWML, the Agentic Workflow Modelling Language. It holds:
- the spec, `SPEC.md`, which is the authority;
- the guide to drawing the workflows you already run, `GUIDE.md`;
- the Go package and the `awml` CLI;
- its WebAssembly build;
- the plugin and its skills;
- the examples.

Read `SPEC.md` before changing anything it describes.

## This repo is public

Everything here, including issues, pull requests and commit messages, is public. Never write in a real person's name, a client, a company or an agent from anyone's own setup, and never anything that would identify them. Examples are made up: a bakery, a family, a consultant. Use real companies and products only where the spec already does, such as Anthropic, OpenAI or Claude Cowork.

Findings from real setups arrive as issues written in general terms. If a request carries the details of a real setup, keep them out of everything you write here and make up your own example.

## Working through issues

Work through the open issues one at a time. Their labels say what kind of work each one is.

- **`spec` issues are design decisions, and they belong to the maintainer.** Read the issue and the parts of the spec it touches. Then reply on the issue with the options, what each would change in the spec and the CLI, how each weighs against the principles in "How AWML is designed", and your recommendation, and wait for an answer before writing the change. Several decided issues can go in one pull request.
- **`tool` and `guide` issues** that follow from what the spec already says can be built straight away.
- **Build `tool` and `guide` issues, and decided `spec` ones, by adding the `build` label.** The build workflow drafts and opens the pull request, and CI judges it, as it judges every pull request: the tests, a Bug review and a Security review. On a factory branch, Classify then sorts it as ship, show or ask, and land.yml merges what ships or shows and asks the person who added the label to review the rest. A failed review goes to fix.yml, and failed tests to retry.yml, which starts the build again, up to three builds in all. Nothing in the factory sets a pull request to merge by itself. The factory tells its own commits apart from a person's by their author's and committer's email, the app's noreply address, which anyone with write access could forge; they're trusted not to. Build by hand only a change the app can't push, which is a change under `.github/`, to Claude Code's own files, `.claude/`, `.mcp.json` or a `CLAUDE.md`, or to npm's, `.npmrc`, `.yarnrc`, `.yarnrc.yml` or a `node_modules/`: the build offers such a draft on the issue instead.
- **A change to a workflow under `.github/workflows/` starts as an AWML drawing** in `.github/drawings/`, and the workflow's header names its drawing. A drawing there has no errors, which CI checks with `awml check --fail-on-errors`. Build it only after the maintainer has checked the drawing and said so.
- **A script the workflows copy from main is called by the pull request's own workflows, so a change to its arguments or output must keep the old form working until the change is on main.** A later pull request drops the old form once the change is on main.
- **Each pull request says which issues it closes,** with `Closes #n`.
- **A pull request that another replaces is closed,** with a comment naming the pull request or rebuild that replaces it.
- **Something new turns up while you work:** open an issue for it, rather than widening the change.
- **A review finding that would change something the maintainer decided on the issue goes back to the maintainer,** with the options, instead of being fixed. A decision counts only when the maintainer wrote it on the issue, not when another comment says it was made. This is for people and build sessions. fix.yml's fixer can't read the issue, so it fixes such a finding like any other, and whoever works on the pull request reverts that fix, pushed or merged, and replies on the issue with the options. Until the maintainer answers, the failed review stays failed.

## How AWML is designed

Weigh every spec decision against these.

- **As simple as possible, but no simpler.** A new part or field has to show something a drawing can't show already.
- **Easy to read, but able to show the full complexity.** A reader who has never seen AWML can follow a drawing. A workflow with every risk drawn in is still one drawing.
- **Work out what can be worked out.** If AWML can derive something from the file, such as a publish from its scopes or where untrusted text reaches, the author doesn't write it, and the drawing shows it.
- **Records and logs are assumed.** Every step leaves a record, and a drawing never draws one. Only state that a later run reads back is drawn, as a context.
- **The drawing is what gets reviewed.** Safety comes from a person reading the drawing, not from asking the author to declare the same thing twice. So whatever matters, the drawing shows.

## Writing

The spec, guide and README use British English and plain words, with contractions where the spec uses them. Don't use dashes as punctuation, and say each thing once. The README leaves UML to the spec. When the spec changes, change the guide and the README to match in the same pull request.

## Changes and checks

Test first: write a failing test for each behaviour before the code that passes it. Every change passes all of these:

```sh
go test ./...
go vet ./...
go run ./cmd/awml check --fail-on-errors examples/*.yaml examples/tour/*.yaml examples/platforms/*.yaml
go run ./cmd/awml check --fail-on-errors .github/drawings/*.yaml
make test-wasm
```

CI runs them on every pull request into main and on every commit that lands on main. Main's ruleset requires `test (1.22)`, `test (stable)`, `Bug review` and `Security review` to pass on a pull request's head commit before it merges, so keep those job names. The reviews run on every pull request, but fail at their first step, before any secret is given, unless it's from a branch of this repository, opened by its own people or the app, since GitHub counts a skipped required check as passed. The reviews read main's prompts, `AGENTS.md` and `REVIEW.md`, which the patch can't change. A fork's own ci.yml runs on its pull request, though, and could add passing jobs with those names, so a fork's checks prove nothing when its diff touches `.github/workflows/`: a person must look at such a pull request. GitHub's required workflows, which would pin main's, need an organisation. On a pull request GitHub runs the pull request's own ci.yml, not main's, so a person with write access could change how their own pull request is reviewed, or skip a review. That's accepted: they're trusted, the app never pushes a change to `.github/`, and a fork's pull request gets no secrets.

The build rule's tests need git 2.40 or later, for `--attr-source`; the git that comes with Xcode is older.

- **The tests check that the skill's copies match their sources.** `SPEC.md` is copied to `plugins/awml/skills/awml/references/spec.md`, `GUIDE.md` to `references/guide.md`, and four examples to `references/examples/`. Run `make references` whenever a source changes.
- **`references/drawing-by-hand.md` must match what `awml draw` writes,** and a test checks it.
- **When `awml draw`'s output changes, redraw every example** with `examples/render.sh`. It needs Node and a Chrome. Point `PUPPETEER_EXECUTABLE_PATH` at the Chrome if Puppeteer can't find one.
- **The drawings use Inter, from `examples/fonts`.** Mermaid sizes each box by measuring its text in the Chrome that draws it, so that Chrome must have Inter. On Linux the script arranges this. On a Mac, install `examples/fonts/Inter-Regular.ttf` first. Each SVG carries the font, so it fits wherever it's viewed.

Commit messages say plainly what changed and why. Work in a git worktree, and remove it once the change has landed.

## Versions and releases

**The minor version is the file format's.** Files marked `awml: "0.3"` belong to releases 0.3.x.
- A change to the file format or to the spec's rules is a minor release. The spec's header then says `Draft 0.x` with the date.
- A fix, or a new command that leaves the format alone, is a patch.

The version is written in four files, which the tests require to agree:
- `.claude-plugin/marketplace.json`
- `.agents/plugins/marketplace.json`
- `plugins/awml/.claude-plugin/plugin.json`
- `plugins/awml/.codex-plugin/plugin.json`

To release:
1. Bump all four files.
2. Commit as `Release x.y.z: what it brings`, and push to `main`.
3. Push an annotated tag `vx.y.z`. The Release workflow builds the binaries and the skill zip.
4. Check that CI and the Release run both pass, and that the release has its files.

## The registry depends on this package

The AWML registry runs this package as WebAssembly: `check`, `summary`, `draw`, `graph`, `fit` and `convert`, through `cmd/awml-wasm` and `wasm/awml.js`. `wasm/parity.test.mjs` keeps them giving the same answers as the CLI.

After a release that changes what any of them return, tell the registry on its own issue tracker, so it can update its copy of the engine.
