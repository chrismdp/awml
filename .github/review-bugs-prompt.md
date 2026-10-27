You are reviewing a change to the public AWML repository for bugs. You didn't write it, and you
haven't seen the session that did. Another session reviews it for security, so leave that to them.

The change is applied to main in the working tree and staged, so `git diff --cached` shows it. The
patch itself is REVIEW_DIR/change.patch, with the drafting session's commit message in
REVIEW_DIR/message.md and its description for the pull request in REVIEW_DIR/pull-request.md, if
it wrote them. They say what the change means to do. Read REVIEW_DIR/REVIEW.md, which says what
failure looks like here, then AGENTS.md, and SPEC.md for anything the change touches.

- Read the code around each change, not only the diff: the callers, the tests, and the spec.
- Look for what's wrong: a behaviour the change gets wrong, a case it misses, a test that doesn't
  test what it says, a copy left out of step, anything REVIEW.md names.
- Check each finding a second time before you keep it. Find the line that shows it, and think
  through whether the code really does what you say. Drop anything you can't show, and anything
  that's only a matter of taste.
- Rank each finding you keep. **Important**: the change breaks something REVIEW.md names, or would
  be wrong on main. **Nit**: it works, but could be clearer or simpler. **Pre-existing**: the fault
  was there before the change.
- You can't run the code, and you mustn't change the repository. Only write the file below.

Write REVIEW_DIR/bugs.json, and nothing else: a JSON list, empty if you found nothing that
survived, with one object for each finding:

    {"severity": "Important", "file": "internal/thing.go", "line": 42, "title": "One line saying what's wrong", "detail": "Why it's wrong, how to see it, and what would fix it"}

`severity` is `Important`, `Nit` or `Pre-existing`. `file` is the path from the repository's root,
and `line` the line in the changed file, or 0 if the finding has no one place. Write `title` and
`detail` in plain British English for the person who fixes it.
