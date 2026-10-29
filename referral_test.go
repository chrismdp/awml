package awml

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// issueAnswers answers the issue's GraphQL with comments by the owner, a writer, a member who may
// only read, a stranger, the app's own bot and a user named like it, each with its link. The build
// label was added at 10:32, after the app's first comment, and before its second, and before its
// third was edited.
const issueAnswers = `
  "api graphql --paginate"*) answer <<< '{"data": {"repository": {"issue": {"comments": {"nodes": [
    {"author": {"login": "the-owner", "__typename": "User"}, "createdAt": "2026-10-27T10:00:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-1",
     "bodyHTML": "<p>Keep the scones plain.</p>"},
    {"author": {"login": "a-member", "__typename": "User"}, "createdAt": "2026-10-27T10:10:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-2",
     "bodyHTML": "<p>A reader says jam.</p>"},
    {"author": {"login": "a-stranger", "__typename": "User"}, "createdAt": "2026-10-27T10:20:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-3",
     "bodyHTML": "<p>A stranger says cream.</p>"},
    {"author": {"login": "awml-factory", "__typename": "Bot"}, "createdAt": "2026-10-27T10:30:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-4",
     "bodyHTML": "<p>The fixer referred a finding back.</p>"},
    {"author": {"login": "awml-factory", "__typename": "User"}, "createdAt": "2026-10-27T10:35:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-5",
     "bodyHTML": "<p>A user named like the app.</p>"},
    {"author": {"login": "awml-factory", "__typename": "Bot"}, "createdAt": "2026-10-27T10:20:00Z", "lastEditedAt": "2026-10-27T10:50:00Z",
     "url": "https://github.com/owner/repo/issues/120#issuecomment-7", "bodyHTML": "<p>The app edited this after the label.</p>"},
    {"author": {"login": "a-writer", "__typename": "User"}, "createdAt": "2026-10-27T10:40:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-6",
     "bodyHTML": "<p>Answer: option B.</p>"},
    {"author": {"login": "awml-factory", "__typename": "Bot"}, "createdAt": "2026-10-27T10:45:00Z", "url": "https://github.com/owner/repo/issues/120#issuecomment-8",
     "bodyHTML": "<p>The app posted this after the label.</p>"}]}}}}}' ;;
  "api --paginate repos/owner/repo/issues/120/events"*) answer <<< '[
    {"event": "labeled", "label": {"name": "build"}, "actor": {"login": "the-owner", "type": "User"}, "created_at": "2026-10-27T09:00:00Z"},
    {"event": "labeled", "label": {"name": "tool"}, "actor": {"login": "the-owner", "type": "User"}, "created_at": "2026-10-27T10:31:00Z"},
    {"event": "labeled", "label": {"name": "build"}, "actor": {"login": "the-owner", "type": "User"}, "created_at": "2026-10-27T10:32:00Z"}]' ;;
  "api graphql"*) answer <<< '{"data": {"repository": {"issue": {"number": 120, "title": "Bake scones",
    "bodyHTML": "<p>Add a scone recipe.</p>", "labels": {"nodes": [{"name": "build"}]}}}}}' ;;` + permissions

// The decisions on an issue are its text and the comments of people who can write to the
// repository, by their permission, and the app's own, by its exact bot, if the build label, last
// added, approved them as they read: the build reads them, and so does the fixer, from the one
// script, each comment with its link.
func TestTheIssuesTextKeepsWritersAndTheApp(t *testing.T) {
	r := newRehearsal(t)
	r.write("issue-text.sh", readText(t, ".github/issue-text.sh"))
	r.run(workflowStep{Run: `bash "$RUNNER_TEMP/issue-text.sh" 120 "$RUNNER_TEMP/issue.md"`}, nil, issueAnswers)
	r.mustPass()
	text := r.read("issue.md")
	for _, want := range []string{"# #120: Bake scones", "<p>Add a scone recipe.</p>", "Keep the scones plain.", "The fixer referred a finding back.",
		"Answer: option B.", "https://github.com/owner/repo/issues/120#issuecomment-6"} {
		if !strings.Contains(text, want) {
			t.Errorf("the issue's text lacks %q:\n%s", want, text)
		}
	}
	for _, never := range []string{"A reader says jam", "A stranger says cream", "A user named like the app",
		"The app posted this after the label", "The app edited this after the label"} {
		if strings.Contains(text, never) {
			t.Errorf("the issue's text holds %q, which nobody who can write wrote:\n%s", never, text)
		}
	}
}

// The fixer reads the decisions on the pull request's issue: the build's branch names it, and
// otherwise the pull request's closing reference does. A pull request with no issue gets no
// decisions file, and the fixer works as before.
func TestTheFixerReadsTheIssuesDecisions(t *testing.T) {
	bring, _ := readFactoryWorkflow(t, "fix.yml").Jobs["fix"].step(t, "bring")
	for _, c := range []struct{ branch, closing, issue string }{
		{"build/issue-120", "[]", "120"}, {"scones", `[{"number": 120}]`, "120"}, {"scones", "[]", ""},
	} {
		r := newRehearsal(t)
		r.write("issue-text.sh", readText(t, ".github/issue-text.sh"))
		r.write("read-findings.sh", "#!/bin/bash\nmkdir -p \"$2\"\necho '[]' > \"$2/findings.json\"\n")
		r.write("fix/in/.keep", "")
		t.Cleanup(func() { exec.Command("chmod", "-R", "u+w", filepath.Join(r.temp, "fix")).Run() })
		r.run(bring, map[string]string{"PR_NUMBER": "8", "HEAD_SHA": strings.Repeat("ab", 20), "CI_RUN": "77", "BRANCH": c.branch}, `
  "api graphql -F owner=owner -F name=repo -F number=8"*) answer <<< '{"data": {"repository": {"pullRequest": {"closingIssuesReferences": {"nodes": `+c.closing+`}}}}}' ;;`+
			issueAnswers+`
  "api repos/owner/repo/compare/"*) answer <<< 'diff' ;;
  "api --paginate repos/owner/repo/pulls/8/commits"*) answer <<< '[{"sha": "abc", "commit": {"message": "A commit"}}]' ;;`)
		r.mustPass()
		decisions := r.read("fix/in/decisions.md")
		if c.issue == "" {
			if _, err := os.Stat(filepath.Join(r.temp, "fix", "in", "decisions.md")); err == nil || r.outputs["issue"] != "" {
				t.Errorf("a pull request with no issue got decisions:\n%s", decisions)
			}
			continue
		}
		if r.outputs["issue"] != c.issue || !strings.Contains(decisions, "Keep the scones plain.") || strings.Contains(decisions, "A stranger says cream") {
			t.Errorf("on %s, the fixer read issue %q's decisions as:\n%s", c.branch, r.outputs["issue"], decisions)
		}
	}
	prompt := strings.Join(strings.Fields(readText(t, ".github/fix-prompt.md")), " ")
	for _, want := range []string{"FIX_DIR/decisions.md", "OUT_DIR/referred.md", "would change a decision",
		"If you refer any finding back, change nothing"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the fix prompt doesn't say %q", want)
		}
	}
}

// fixPatchStep rehearses the fix job's patch step on a tree the session left: changed or not, with
// a referral in its out folder or not.
func fixPatchStep(t *testing.T, changed, referred bool) *rehearsal {
	t.Helper()
	patch, _ := readFactoryWorkflow(t, "fix.yml").Jobs["fix"].step(t, "patch")
	r := newRehearsal(t)
	repo, _ := r.newRepo("repo", map[string]string{"recipes.md": "Bread\n"})
	r.write("host/scrub-placeholders.sh", readText(t, ".github/scrub-placeholders.sh"))
	r.write("host/baseline/absent", "")
	r.write("host/baseline/git-config", readText(t, filepath.Join(repo, ".git/config")))
	folders, err := exec.Command("stat", "-c", "%d:%i", repo, filepath.Join(repo, ".git")).Output()
	if err != nil {
		t.Fatal(err)
	}
	r.write("host/baseline/folders", string(folders))
	r.write("fix/out/message.md", "Fix the slip\n")
	if changed {
		r.write(filepath.Join(repo, "recipes.md"), "Bread\nScones\n")
	}
	if referred {
		r.write("fix/out/referred.md", "- The Security review's finding would undo [the decision](https://github.com/owner/repo/issues/120#issuecomment-1).\n")
	}
	r.dir = repo
	r.run(patch, map[string]string{"GITHUB_WORKSPACE": repo, "HEAD_SHA": gitIn(t, repo, "rev-parse", "HEAD")}, "")
	return r
}

// When the session referred findings back and changed nothing, the run ends there, referred:
// nothing is pushed, so no fix commit counts as a try and no CI run starts a retry. A session that
// referred findings back must change nothing, since a pushed fix would start the reviews again, which
// would find the same finding to refer back: one that changed something too is refused, and nothing
// is pushed.
func TestAFixThatReferredPushesNothing(t *testing.T) {
	all := fixPatchStep(t, false, true)
	all.mustPass()
	if all.outputs["decision"] != "referred" || !strings.Contains(all.read("host/fix/referred.md"), "would undo") {
		t.Errorf("a session that only referred findings back decided %q:\n%s", all.outputs["decision"], all.printed)
	}
	none := fixPatchStep(t, false, false)
	none.mustPass()
	if none.outputs["decision"] != "giveup" || none.outputs["reason"] != "nothing" {
		t.Errorf("a session that changed and referred nothing decided %q, %q", none.outputs["decision"], none.outputs["reason"])
	}
	mixed := fixPatchStep(t, true, true)
	mixed.mustPass()
	if mixed.outputs["decision"] != "giveup" || mixed.outputs["reason"] != "mixed" {
		t.Errorf("a session that referred findings back and changed something decided %q, %q:\n%s",
			mixed.outputs["decision"], mixed.outputs["reason"], mixed.printed)
	}
	unchanged := fixPatchStep(t, true, false)
	unchanged.mustPass()
	if unchanged.outputs["decision"] != "" {
		t.Errorf("a session that fixed everything decided %q", unchanged.outputs["decision"])
	}

	referred := fixGuard(t, map[string]string{"DECISION": "referred", "ISSUE": "120"}, func(r *rehearsal, repo string) {
		r.write("fix/referred.md", "- A finding referred back.\n")
	}, false)
	referred.mustPass()
	if referred.outputs["outcome"] != "referred" {
		t.Errorf("a run that only referred findings back ended %v", referred.outputs)
	}
	nowhere := fixGuard(t, map[string]string{"DECISION": "referred", "ISSUE": ""}, func(r *rehearsal, repo string) {
		r.write("fix/referred.md", "- A finding referred back.\n")
	}, false)
	nowhere.mustPass()
	if nowhere.outputs["outcome"] != "giveup" || nowhere.outputs["reason"] != "nothing" {
		t.Errorf("a referral with no issue to post it on ended %v", nowhere.outputs)
	}
	refused := fixGuard(t, map[string]string{"DECISION": "giveup", "STOPPED": "mixed"}, func(r *rehearsal, repo string) {}, false)
	refused.mustPass()
	if refused.outputs["outcome"] != "giveup" || refused.outputs["reason"] != "mixed" {
		t.Errorf("a fix refused for referring and changing ended %v", refused.outputs)
	}
	write := readFactoryWorkflow(t, "fix.yml").Jobs["write"]
	giveup, _ := write.step(t, "giveup")
	if !strings.Contains(giveup.Run, `mixed) why="`) {
		t.Error("giving up doesn't say why a fix that referred findings back and changed something isn't pushed")
	}
	ask, _ := write.step(t, "ask")
	if ask.If != "steps.guard.outputs.outcome == 'referred'" {
		t.Errorf("the referral is posted if %q", ask.If)
	}
	for _, step := range write.Steps {
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") && !strings.Contains(step.If, "env.DECISION == 'referred'") {
			t.Error("the write job doesn't bring in a run's referral when nothing was fixed")
		}
	}
}

// The referral is one new comment on the issue, with the app's token: it says what it is, carries
// the session's text with every mention broken, and says how to go on. The log shows it inside
// stop-commands, so no line of it can be a workflow command.
func TestTheReferralIsPostedEscaped(t *testing.T) {
	ask, _ := readFactoryWorkflow(t, "fix.yml").Jobs["write"].step(t, "ask")
	if ask.Env["GH_TOKEN"] != "${{ steps.asker.outputs.token }}" {
		t.Errorf("the referral is posted with %q, not the app's token for the issue", ask.Env["GH_TOKEN"])
	}
	// A build's pull request goes on with a fresh build. A person's goes on with its author, since
	// the label would start a build of its own beside it.
	for branch, end := range map[string]string{
		"build/issue-120": "To go on, answer here, then remove the build label and add it again.",
		"scones":          "To go on, answer here; the pull request's author takes it from there.",
	} {
		r := newRehearsal(t)
		r.write("fix/referred.md", "- A finding.\n")
		r.run(ask, map[string]string{"ISSUE": "120", "PR_NUMBER": "8", "BRANCH": branch}, `  "issue comment 120"*) ;;`)
		r.mustPass()
		if bodies := r.bodies(); len(bodies) != 1 || !strings.HasSuffix(strings.TrimSpace(bodies[0]), end) {
			t.Errorf("on %s, the referral doesn't end %q:\n%s", branch, end, bodies)
		}
	}
	r := newRehearsal(t)
	r.write("fix/referred.md", "- The finding would undo what @the-owner decided.\n::error::forged\n")
	r.run(ask, map[string]string{"ISSUE": "120", "PR_NUMBER": "8", "BRANCH": "build/issue-120"}, `  "issue comment 120"*) ;;`)
	r.mustPass()
	bodies := r.bodies()
	if len(bodies) != 1 {
		t.Fatalf("the referral made %d comments, want 1", len(bodies))
	}
	body := bodies[0]
	if !strings.HasPrefix(body, "**The fixer referred findings back") {
		t.Errorf("the referral doesn't say what it is:\n%s", body)
	}
	if strings.Contains(body, "@the-owner") || !strings.Contains(body, "@\u200bthe-owner") {
		t.Errorf("the referral's mention isn't broken:\n%s", body)
	}
	stop := strings.Index(r.printed, "::stop-commands::")
	if forged := strings.Index(r.printed, "\n::error::forged"); stop < 0 || (forged >= 0 && forged < stop) {
		t.Errorf("the log shows the referral outside stop-commands:\n%s", r.printed)
	}
}

// After a referral the maintainer answers on the issue and adds the build label again. The build
// then drafts afresh from main, reading the issue with the app's referral and the answer, and pushes
// over the open pull request's branch with a lease, giving it the new try's title and description.
func TestARebuildAfterAReferralReadsTheAnswer(t *testing.T) {
	keep, _ := readBuildWorkflow(t)["draft"].step(t, "keep")
	r := newRehearsal(t)
	repo, _ := r.newRepo("repo", map[string]string{"AGENTS.md": "How to work.\n", "REVIEW.md": "What fails.\n", ".github/build-prompt.md": "Build it.\n",
		".github/sandbox-settings.sh": "echo settings\n", ".github/scrub-placeholders.sh": "echo placeholders\n",
		".github/issue-text.sh": readText(t, ".github/issue-text.sh")})
	r.dir = repo
	r.run(keep, map[string]string{"ISSUE": "120"}, issueAnswers)
	r.mustPass()
	issue := r.read("build/issue.md")
	for _, want := range []string{"The fixer referred a finding back.", "Answer: option B."} {
		if !strings.Contains(issue, want) {
			t.Errorf("the rebuild's issue lacks %q:\n%s", want, issue)
		}
	}
	build := readText(t, ".github/workflows/build.yml")
	for _, want := range []string{`push --force-with-lease="refs/heads/$BRANCH:$BRANCH_HEAD"`, `gh pr edit "$number"`} {
		if !strings.Contains(build, want) {
			t.Errorf("the rebuild doesn't push over the open pull request and describe it afresh: no %s", want)
		}
	}
	if !strings.Contains(strings.Join(strings.Fields(readText(t, ".github/build-prompt.md")), " "), "answer to a finding the fixer referred back") {
		t.Error("the build prompt doesn't say an answer to a referral is a decision")
	}
}
