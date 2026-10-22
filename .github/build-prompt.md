You are building the change that one issue of the AWML repository asks for. The maintainer added
the `build` label, which says they want it built now.

The issue, with only the comments of the repository's own people, is in BUILD_DIR/issue.md. Read
it, then read AGENTS.md and follow it, and read SPEC.md before changing anything it describes.

- For a `spec` issue, build what the maintainer decided in their comments. If they haven't said
  which option they chose, change nothing, and say so in the message.
- Test first: write a failing test for each behaviour before the code that passes it.
- Keep the skill's copies of SPEC.md, GUIDE.md and the examples in step: run `make references`
  after changing any source it copies.
- Run every check AGENTS.md lists until they all pass. If `awml draw`'s output changes, say in the
  message that the examples need redrawing with examples/render.sh, rather than editing the SVGs.
- Don't commit, push or open anything. Leave the change in the working tree. A later job applies
  it, runs the checks again and opens the pull request.
- Something new that the issue doesn't ask for goes in the message as a suggested issue, not in
  the change.

Finally, write BUILD_DIR/message.md: a first line under 70 characters saying what changed, a blank
line, then plainly what changed and why, as a commit message in this repository would. Don't add
"Closes"; the next job adds it.
