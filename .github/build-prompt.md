You are building the change that one issue of the AWML repository asks for. The maintainer added
the `build` label, or one of the repository's own workflows started the build, which says it is
wanted now.

The issue, with only the comments of the repository's own people, is in BUILD_DIR/issue.md. Read
it, then read AGENTS.md and follow it, and read SPEC.md before changing anything it describes.
If issue.md ends with a section saying the last try failed, your job is to make a change that
passes where that try failed, not to start over without a reason.

- For a `spec` issue, build what the maintainer decided in their comments. If they haven't said
  which option they chose, change nothing, and say so in the message.
- Test first: write a failing test for each behaviour before the code that passes it.
- Keep the skill's copies of SPEC.md, GUIDE.md and the examples in step: run `make references`
  after changing any source it copies.
- Run every check AGENTS.md lists until they all pass. If `awml draw`'s output changes, regenerate
  the .mmd files but leave the SVGs alone: a later job redraws them with examples/render.sh and
  pictures what changed.
- Don't commit, push or open anything. Leave the change in the working tree. A later job applies
  it, runs the checks again and opens the pull request.
- Something new that the issue doesn't ask for goes in the message as a suggested issue, not in
  the change.

If the change needs the maintainer's eyes, write BUILD_DIR/review.md and say why. It needs their
eyes if you made a decision the issue and its comments didn't settle, chose between options,
changed behaviour beyond what the issue asks, or aren't sure. Otherwise don't write that file.
Your note can send a change to review, but it can never skip a review required by the build rules.
Write it in GitHub Markdown for the reviewer, with each decision as its own bullet and what you'd
like them to look at, and don't wrap lines by hand. It goes into the pull request as you wrote it.

Write BUILD_DIR/message.md: a first line under 70 characters saying what changed, a blank line,
then plainly what changed and why, as a commit message in this repository would. Don't add
"Closes"; the next job adds it.

Finally, write BUILD_DIR/pull-request.md, the pull request's description, in GitHub Markdown with
no hard wrapping: each paragraph and each bullet on one line. Open with a short paragraph saying
what the change does and why. Then `## What changed`, as bullets. Then `## Decisions`, as bullets,
only if you made decisions the issue and its comments didn't settle. Leave out a title, review
reasons, pictures and "Closes": the next job adds them.
