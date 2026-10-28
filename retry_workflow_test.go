package awml

import (
	"fmt"
	"strings"
	"testing"
)

func retryAnswers(test, review, branchSha, edited string, labels []string, builds int, renamed ...string) string {
	return retryAnswersOn(1, "", test, review, branchSha, edited, labels, builds, renamed...)
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
    "id": 77, "run_attempt": ` + fmt.Sprint(attempt) + `, "created_at": "2026-10-27T13:30:00Z"}' ;;
  "api --paginate repos/owner/repo/actions/workflows/ci.yml/runs"*) answer <<< '{"workflow_runs": [
    {"id": 77, "event": "pull_request", "conclusion": "failure", "created_at": "2026-10-27T13:30:00Z", "head_repository": {"full_name": "owner/repo"}}` +
		strings.TrimPrefix(earlier, "[") + `
  "api --paginate repos/owner/repo/actions/runs/77/jobs"*) answer <<< '{"jobs": [
    {"name": "test (1.22)", "conclusion": "` + test + `"}, {"name": "test (stable)", "conclusion": "success"},
    {"name": "Bug review", "conclusion": "` + review + `"}]}' ;;
  "api repos/owner/repo/branches/build/issue-120"*) answer <<< '{"commit": {"sha": "` + branchSha + `"}}' ;;
  "api --paginate repos/owner/repo/actions/workflows/build.yml/runs"*) answer <<< '{"workflow_runs": [` + strings.Join(runs, ", ") + `]}' ;;` +
		issueAnswer(edited, labels...) + labelEventsRenamed(strings.Join(renamed, ""))
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
	for decision, want := range map[string]string{"report": "failed three times", "edited": "edited or renamed since the build label"} {
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
