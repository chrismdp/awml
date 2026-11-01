You are building the change that one issue of the AWML repository asks for. The `build` label was
added by someone with write access or the repository's app, which says it is wanted now.

The issue, with only the comments of people who can write to the repository and of the
repository's app, is in BUILD_DIR/issue.md, each body as GitHub renders it, in HTML, so you read
just what the maintainer saw. The app's comments may include findings the fixer referred back,
because fixing them would have changed a decision on the issue. The maintainer's answer to a finding
the fixer referred back is a decision like any other: build to it. Read it, then read
BUILD_DIR/AGENTS.md, main's copy, and follow it, and read SPEC.md before changing anything it
describes. Read BUILD_DIR/REVIEW.md too: once the pull request opens, CI's two fresh
reviews hold the change to it.

- For a `spec` issue, build what the maintainer decided in their comments. If they haven't said
  which option they chose, change nothing, and say so in the message.
- If you're not sure, ask: it is always fine to escalate. Where the issue leaves open a choice that
  changes what a reader sees, such as cutting or rewriting an example, changing the story it tells,
  or dropping something a page shows, don't choose. Change nothing, and write the message as the
  question for the maintainer: the choice, the options, and which you'd recommend. It is posted on
  the issue, and the build waits for their answer there.
- Test first: write a failing test for each behaviour before the code that passes it.
- The session runs offline: Go's modules for the tree are fetched before it starts, and nothing can
  be fetched while it runs. So add no new dependency. If the change seems to need one, change
  nothing and say so in the message, as a question for the maintainer.
- Keep the skill's copies of SPEC.md, GUIDE.md and the examples in step: run `make references`
  after changing any source it copies.
- Run every check AGENTS.md lists until they all pass. If `awml draw`'s output changes, regenerate
  the .mmd files, but don't run examples/render.sh yourself.
- If your change alters how something is drawn and no example shows it, make one show it: the pull
  request's pictures come from the examples, and a reviewer should see the change, not only read the
  test that asks for it. A drawing that breaks a rule goes in a small new example in examples/live/,
  added to TestThereAreLiveExamples; anything else in a small new example in examples/tour/,
  numbered one more than the highest there, checking with no warnings, added to the list in
  TestThereAreExamples, and given its own short section in the README's gallery. Its SVG comes from
  `.github/picture-drawings.sh BUILD_DIR`, so run that as soon as its .mmd is written, then the
  checks again. Change an existing example's story only by asking, as above.
- Once the checks pass, if your change alters an example's .mmd or the renderer under
  examples/render*, run `.github/picture-drawings.sh BUILD_DIR`. It redraws the drawings your
  change alters into the working tree, puts every other SVG back, and pictures, before and after,
  those that look different, into BUILD_DIR/pictures. Leave the SVGs and the pictures as it wrote
  them: they go into the pull request.
- Change nothing under `.github/`, none of Claude Code's own files, `.claude/`, `.mcp.json`, a
  `CLAUDE.md` or a `CLAUDE.local.md`, and none of npm's, `.npmrc`, `.yarnrc`, `.yarnrc.yml` or a
  `node_modules/`, unless the issue asks for it: the app never pushes those, so
  such a change is offered on the issue for a person to apply, and the build stops there.
- Don't commit, push or open anything. Leave the change in the working tree. A later job pushes it
  and opens the pull request, and CI checks and reviews it there.
- Every untracked file left in the tree goes into the pull request, so delete any scratch file you
  made before you finish.
- Something new that the issue doesn't ask for goes in the message as a suggested issue, not in
  the change.

Write BUILD_DIR/message.md: a first line under 70 characters saying what changed, a blank line,
then plainly what changed and why, as a commit message in this repository would. Don't add
"Closes"; a later job adds it.

Finally, write BUILD_DIR/pull-request.md, the pull request's description, in GitHub Markdown with
no hard wrapping: each paragraph and each bullet on one line. Open with a short paragraph saying
what the change does and why. Then `## What changed`, as bullets. Then `## Decisions`, as bullets,
only if you made a decision the issue and its comments didn't settle, chose between options,
changed behaviour beyond what the issue asks, or aren't sure, saying for each what you'd like the
reviewer to look at. Leave out a title, review reasons, pictures and "Closes": a later job adds
them. You don't decide whether the change needs review: Classify and the build rule do, after
CI's reviews.
