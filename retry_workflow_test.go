package awml

import (
	"fmt"
	"strings"
	"testing"
)

func retryAnswers(test, review, branchSha, edited string, labels []string, builds int, renamed ...string) string {
	return retryAnswersOn(1, "", test, review, branchSha, edited, labels, builds, renamed...)
}

// factoryCommit and personsCommit are commits as GitHub's compare lists them.
const factoryCommit = `{"commit": {"author": {"email": "41+awml-factory[bot]@users.noreply.github.com"}, "committer": {"email": "41+awml-factory[bot]@users.noreply.github.com"}}}`
const personsCommit = `{"commit": {"author": {"email": "someone@example.com"}, "committer": {"email": "someone@example.com"}}}`

// onTheBranch answers the compare of the branch with main with the commits given, oldest first,
// the first commit's sha shaOf(1), the next shaOf(2), and so on.
func onTheBranch(answers string, commits ...string) string {
	var listed []string
	for at, c := range commits {
		listed = append(listed, `{"sha": "`+shaOf(at+1)+`", `+strings.TrimPrefix(c, "{"))
	}
	return strings.Replace(answers, `"commits": [`+factoryCommit+`]`, `"commits": [`+strings.Join(listed, ", ")+`]`, 1)
}

func shaOf(n int) string { return fmt.Sprintf("%040x", n) }

// earlierFailure is an earlier CI run on the branch, on the commit given, whose tests failed.
func earlierFailure(id int, sha string) string {
	return fmt.Sprintf(`[,
    {"id": %d, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T13:10:00Z", "head_sha": "%s", "head_repository": {"full_name": "owner/repo"}}]}' ;;
  "api --paginate repos/owner/repo/actions/runs/%d/jobs"*) answer <<< '{"jobs": [{"name": "test (1.22)", "conclusion": "failure"}]}' ;;`, id, sha, id)
}

// retryAnswersOn answers as retryAnswers does, for the attempt of CI's run given, with CI's earlier
// runs on the branch given as a list of runs and the case arms for their jobs.
func retryAnswersOn(attempt int, earlier, test, review, branchSha, edited string, labels []string, builds int, renamed ...string) string {
	if earlier == "" {
		earlier = `[]' ;;`
	}
	head := strings.Repeat("ab", 20)
	runs := []string{
		// A build of another issue, and one from before the label was last added.
		`{"display_title": "Build of issue 121", "head_branch": "main", "event": "issues", "created_at": "2026-10-27T11:00:00Z"}`,
		`{"display_title": "Build of issue 120", "head_branch": "main", "event": "issues", "created_at": "2026-10-27T09:00:00Z"}`,
	}
	for i := 0; i < builds; i++ {
		runs = append(runs, `{"display_title": "Build of issue 120", "head_branch": "main", "event": "workflow_dispatch", "created_at": "2026-10-27T1`+string(rune('1'+i))+`:00:00Z"}`)
	}
	return `
  "api repos/owner/repo/actions/runs/77"*) answer <<< '{"path": ".github/workflows/ci.yml", "event": "pull_request", "status": "completed",
    "head_sha": "` + head + `", "head_branch": "build/issue-120", "head_repository": {"full_name": "owner/repo"},
    "id": 77, "run_attempt": ` + fmt.Sprint(attempt) + `, "created_at": "2026-10-27T13:30:00Z", "pull_requests": [{"number": 8}],
    "triggering_actor": {"login": "awml-factory[bot]", "type": "Bot"}}' ;;
  "api --paginate repos/owner/repo/compare/main...` + head + `"*) answer <<< '{"commits": [` + factoryCommit + `]}' ;;
  "api repos/owner/repo/pulls?head=owner:build/issue-120&base=main&state=open"*) answer <<< '[
    {"number": 99, "head": {"repo": {"full_name": "owner/repo"}}, "user": {"login": "a-stranger", "type": "User"}},
    {"number": 8, "head": {"repo": {"full_name": "owner/repo"}}, "user": {"login": "awml-factory[bot]", "type": "Bot"}}]' ;;
  "api --paginate repos/owner/repo/actions/workflows/ci.yml/runs"*) answer <<< '{"workflow_runs": [
    {"id": 77, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T13:30:00Z", "head_repository": {"full_name": "owner/repo"}}` +
		strings.TrimPrefix(earlier, "[") + `
  "api --paginate repos/owner/repo/actions/runs/77/jobs"*) answer <<< '{"jobs": [
    {"name": "test (1.22)", "conclusion": "` + test + `"}, {"name": "test (stable)", "conclusion": "success"},
    {"name": "Bug review", "conclusion": "` + review + `"}]}' ;;
  "api repos/owner/repo/branches/build/issue-120"*) answer <<< '{"commit": {"sha": "` + branchSha + `"}}' ;;
  "api --paginate repos/owner/repo/actions/workflows/build.yml/runs"*) answer <<< '{"workflow_runs": [` + strings.Join(runs, ", ") + `]}' ;;` +
		issueAnswer(edited, labels...) + labelEventsRenamed(strings.Join(renamed, "")) + permissions
}

// retry.yml wakes when CI's tests fail on a factory branch's pull request, whatever else failed. It
// starts the build again, with only the issue's number, while fewer than three builds of the issue
// have run since the label was last added, counted by their runs' names. After three, or if the
// issue was edited since the label, it says so in a new comment. A run that isn't at the branch's
// head, a failure of the reviews alone, or a label taken off, starts nothing.
func TestRetryActsWhenTheTestsFailAndCountsTheBuilds(t *testing.T) {
	job := readFactoryWorkflow(t, "retry.yml").Jobs["retry"]
	tries, _ := job.step(t, "tries")
	head := strings.Repeat("ab", 20)
	tests := []struct {
		name, answers, decision string
	}{
		{"a failed test", retryAnswers("failure", "success", head, "", []string{"build"}, 1), "again"},
		{"a failed test and review", retryAnswers("failure", "failure", head, "", []string{"build"}, 2), "again"},
		{"three builds", retryAnswers("failure", "success", head, "", []string{"build"}, 3), "report"},
		{"an issue edited since", retryAnswers("failure", "success", head, "2026-10-27T10:30:00Z", []string{"build"}, 1), "edited"},
		{"an issue renamed since", retryAnswers("failure", "success", head, "", []string{"build"}, 1, "2026-10-27T10:30:00Z"), "edited"},
		{"only a failed review", retryAnswers("success", "failure", head, "", []string{"build"}, 1), "none"},
		{"a moved branch", retryAnswers("failure", "success", strings.Repeat("cd", 20), "", []string{"build"}, 1), "none"},
		{"the label taken off", retryAnswers("failure", "success", head, "", []string{"tool"}, 1), "none"},
		{"a fork's run", fromAFork(retryAnswers("failure", "success", head, "", []string{"build"}, 1)), "none"},
		{"a run a stranger started", strings.Replace(retryAnswers("failure", "success", head, "", []string{"build"}, 1),
			`"triggering_actor": {"login": "awml-factory[bot]", "type": "Bot"}`, `"triggering_actor": {"login": "a-stranger", "type": "User"}`, 1), "none"},
		{"a run a writer started", strings.Replace(retryAnswers("failure", "success", head, "", []string{"build"}, 1),
			`"triggering_actor": {"login": "awml-factory[bot]", "type": "Bot"}`, `"triggering_actor": {"login": "a-writer", "type": "User"}`, 1), "again"},
		// A person's commit on the branch means they've taken it over: the first time, a comment says
		// so, and the build is never started over it.
		{"a person's commit at the head", onTheBranch(retryAnswers("failure", "success", head, "", []string{"build"}, 1), factoryCommit, personsCommit), "takenover"},
		// Said once: unless an earlier run on a commit from the takeover on already failed its tests,
		// and so was told, however many commits the person pushed at once.
		{"a person's commit before the head", onTheBranch(retryAnswers("failure", "success", head, "", []string{"build"}, 1), factoryCommit, personsCommit, factoryCommit), "takenover"},
		{"two commits of the person's pushed at once", onTheBranch(retryAnswers("failure", "success", head, "", []string{"build"}, 1), factoryCommit, personsCommit, personsCommit), "takenover"},
		{"a second commit of the person's after a failure on the first", onTheBranch(retryAnswersOn(1, earlierFailure(73, shaOf(2)), "failure", "success", head, "", []string{"build"}, 1),
			factoryCommit, personsCommit, personsCommit), "none"},
		{"a person's commit after a failure on the factory's", onTheBranch(retryAnswersOn(1, earlierFailure(74, shaOf(1)), "failure", "success", head, "", []string{"build"}, 1),
			factoryCommit, personsCommit), "takenover"},
		{"a person's commit the app rebased", onTheBranch(retryAnswers("failure", "success", head, "", []string{"build"}, 1), factoryCommit,
			`{"commit": {"author": {"email": "someone@example.com"}, "committer": {"email": "41+awml-factory[bot]@users.noreply.github.com"}}}`), "takenover"},
		{"the app's commit a person amended", onTheBranch(retryAnswers("failure", "success", head, "", []string{"build"}, 1), factoryCommit,
			`{"commit": {"author": {"email": "41+awml-factory[bot]@users.noreply.github.com"}, "committer": {"email": "someone@example.com"}}}`), "takenover"},
		{"a run for a stranger's pull request from the branch", strings.Replace(retryAnswers("failure", "success", head, "", []string{"build"}, 1),
			`"pull_requests": [{"number": 8}]`, `"pull_requests": [{"number": 99}]`, 1), "none"},
		// After three builds, a person is told once: on the first run since the last build started
		// whose tests failed, whatever commit it's on, a fix's or a rebase's, and never on a re-run.
		{"an earlier run since the last build whose tests failed", retryAnswersOn(1, `[,
    {"id": 70, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T13:10:00Z", "head_repository": {"full_name": "owner/repo"}}]}' ;;
  "api --paginate repos/owner/repo/actions/runs/70/jobs"*) answer <<< '{"jobs": [{"name": "test (1.22)", "conclusion": "failure"}]}' ;;`,
			"failure", "success", head, "", []string{"build"}, 3), "none"},
		{"an earlier run since the last build that failed by a review", retryAnswersOn(1, `[,
    {"id": 71, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T13:10:00Z", "head_repository": {"full_name": "owner/repo"}}]}' ;;
  "api --paginate repos/owner/repo/actions/runs/71/jobs"*) answer <<< '{"jobs": [{"name": "test (1.22)", "conclusion": "success"}, {"name": "Bug review", "conclusion": "failure"}]}' ;;`,
			"failure", "success", head, "", []string{"build"}, 3), "report"},
		{"a failed run from before the last build", retryAnswersOn(1, `[,
    {"id": 60, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T12:30:00Z", "head_repository": {"full_name": "owner/repo"}}]}' ;;
  "api --paginate repos/owner/repo/actions/runs/60/jobs"*) answer <<< '{"jobs": [{"name": "test (1.22)", "conclusion": "failure"}]}' ;;`,
			"failure", "success", head, "", []string{"build"}, 3), "report"},
		{"a fork's failed run since the last build", retryAnswersOn(1, `[,
    {"id": 72, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T13:10:00Z", "head_repository": {"full_name": "a-stranger/repo"}}]}' ;;
  "api --paginate repos/owner/repo/actions/runs/72/jobs"*) answer <<< '{"jobs": [{"name": "test (1.22)", "conclusion": "failure"}]}' ;;`,
			"failure", "success", head, "", []string{"build"}, 3), "report"},
		{"a re-run of the third build's failure", retryAnswersOn(2, "", "failure", "success", head, "", []string{"build"}, 3), "none"},
		// A re-run of a run that started a build started it already.
		{"a re-run of a failure that started a build", retryAnswersOn(2, "", "failure", "success", head, "", []string{"build"}, 1), "none"},
		// An edit since the label is said once too: unless an earlier run since the edit failed its
		// tests, or this is a re-run. A takeover is checked first, since no edit brings the build back.
		{"an issue edited since, after a failure since the edit", retryAnswersOn(1, earlierFailure(75, shaOf(9)), "failure", "success", head, "2026-10-27T10:30:00Z", []string{"build"}, 1), "none"},
		{"an issue edited since, after a failure before the edit", retryAnswersOn(1, `[,
    {"id": 76, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T10:15:00Z", "head_repository": {"full_name": "owner/repo"}}]}' ;;
  "api --paginate repos/owner/repo/actions/runs/76/jobs"*) answer <<< '{"jobs": [{"name": "test (1.22)", "conclusion": "failure"}]}' ;;`,
			"failure", "success", head, "2026-10-27T10:30:00Z", []string{"build"}, 1), "edited"},
		{"a re-run of an edited issue's failure", retryAnswersOn(2, "", "failure", "success", head, "2026-10-27T10:30:00Z", []string{"build"}, 1), "none"},
		{"an issue edited since, on a branch a person took over", onTheBranch(retryAnswers("failure", "success", head, "2026-10-27T10:30:00Z", []string{"build"}, 1),
			factoryCommit, personsCommit), "takenover"},
		{"a re-run of a takeover's failure", onTheBranch(retryAnswersOn(2, "", "failure", "success", head, "", []string{"build"}, 1), factoryCommit, personsCommit), "none"},
	}
	for _, tt := range tests {
		r := newRehearsal(t)
		r.run(tries, map[string]string{"CI_RUN": "77"}, tt.answers)
		r.mustPass()
		if r.outputs["decision"] != tt.decision {
			t.Errorf("%s: retry decided %q, want %q\n%s", tt.name, r.outputs["decision"], tt.decision, r.printed)
		}
		if tt.decision != "none" && r.outputs["issue"] != "120" {
			t.Errorf("%s: retry passed on issue %q", tt.name, r.outputs["issue"])
		}
	}

	again, _ := job.step(t, "again")
	r := newRehearsal(t)
	r.run(again, map[string]string{"ISSUE": "120"}, `  "workflow run build.yml"*) ;;`)
	r.mustPass()
	if strings.TrimSpace(r.calls) != "workflow run build.yml --repo owner/repo --ref main -f issue=120" {
		t.Errorf("retry started the build with %q, want only the issue's number", r.calls)
	}

	report, _ := job.step(t, "report")
	for decision, want := range map[string]string{"report": "failed three times", "edited": "edited or renamed since the build label",
		"takenover": "A person has taken this build's branch over"} {
		c := newRehearsal(t)
		c.run(report, map[string]string{"ISSUE": "120", "DECISION": decision, "CI_URL": "https://github.com/owner/repo/actions/runs/77"},
			`  "issue comment 120"*) ;;`)
		c.mustPass()
		if bodies := c.bodies(); len(bodies) != 1 || !strings.Contains(bodies[0], want) || !strings.Contains(bodies[0], "remove the build label and add it again") {
			t.Errorf("retry's %s comment is %q", decision, bodies)
		}
	}
	if len(job.Permissions) != 4 || job.Permissions["actions"] != "write" || job.Permissions["issues"] != "write" {
		t.Errorf("retry has %v, want to write only actions and issues", job.Permissions)
	}
	for _, step := range job.Steps {
		if step.Uses != "" {
			t.Errorf("retry uses %s, but checks out nothing and runs only gh", step.Uses)
		}
	}
}

// The build runs are fetched once, for both their count and when the last one started.
func TestRetryFetchesTheBuildsOnce(t *testing.T) {
	tries, _ := readFactoryWorkflow(t, "retry.yml").Jobs["retry"].step(t, "tries")
	if got := strings.Count(tries.Run, "actions/workflows/build.yml/runs"); got != 1 {
		t.Errorf("retry fetches the build runs %d times, want once", got)
	}
}

// The drawing shows the takeover as retry's other outcomes are shown: a code step that cleans,
// started from the tries step, and a comment on the issue, which the header mentions too.
func TestRetrysDrawingShowsTheTakeover(t *testing.T) {
	drawing := readText(t, ".github/drawings/retry-a-build.yaml")
	for _, want := range []string{"id: takenover, kind: code, op: do, cleans: true,", "from: tries, to: takenover, kind: start, label: \"[a commit since main isn't the factory's]\"",
		"from: takenover, to: takennote, kind: data", "id: takennote, kind: outside, label: \"Comment on the issue that a person took the branch over: Allow\"",
		"to: takenover, kind: tie"} {
		if !strings.Contains(drawing, want) {
			t.Errorf("retry's drawing lacks %s", want)
		}
	}
	if !strings.Contains(readText(t, ".github/workflows/retry.yml"), "taken the branch over") {
		t.Error("retry.yml's header doesn't say what it does when a person has taken the branch over")
	}
}
