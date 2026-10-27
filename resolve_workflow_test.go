package awml

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type resolveStep struct {
	Name            string            `yaml:"name"`
	ID              string            `yaml:"id"`
	If              string            `yaml:"if"`
	Uses            string            `yaml:"uses"`
	Run             string            `yaml:"run"`
	Env             map[string]string `yaml:"env"`
	ContinueOnError bool              `yaml:"continue-on-error"`
}

type resolveJob struct {
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Strategy    struct {
		Matrix map[string]any `yaml:"matrix"`
	} `yaml:"strategy"`
	Steps []resolveStep `yaml:"steps"`
}

const resolveMarker = "<!-- factory-resolve-status -->"

func readResolveWorkflow(t *testing.T) map[string]resolveJob {
	t.Helper()
	var workflow struct {
		Jobs map[string]resolveJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readText(t, ".github/workflows/resolve.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs
}

func (j resolveJob) needs() []string {
	switch n := j.Needs.(type) {
	case string:
		return []string{n}
	case []any:
		var all []string
		for _, each := range n {
			all = append(all, fmt.Sprint(each))
		}
		return all
	}
	return nil
}

func (j resolveJob) runs() string {
	var all []string
	for _, step := range j.Steps {
		all = append(all, step.Run)
	}
	return strings.Join(all, "\n")
}

func (j resolveJob) step(t *testing.T, name string) (resolveStep, int) {
	t.Helper()
	for i, step := range j.Steps {
		if step.Name == name {
			return step, i
		}
	}
	t.Fatalf("no step named %q", name)
	return resolveStep{}, -1
}

// As soon as the run knows which pull requests clash with main, each one gets a new comment saying
// so, from a job that may write to pull requests but checks out and runs nothing.
func TestResolvePostsAStatusCommentOnEachPullRequest(t *testing.T) {
	start, ok := readResolveWorkflow(t)["start"]
	if !ok {
		t.Fatal("resolve.yml has no start")
	}
	if got := start.needs(); len(got) != 1 || got[0] != "find" {
		t.Errorf("start needs %v, want [find]", got)
	}
	if !strings.Contains(fmt.Sprint(start.Strategy.Matrix["pull"]), "needs.find.outputs.pulls") {
		t.Error("start doesn't run once per pull request found")
	}
	for name, level := range start.Permissions {
		if level == "write" && name != "pull-requests" {
			t.Errorf("start may write %s, but needs to write only pull-requests", name)
		}
	}
	if start.Permissions["pull-requests"] != "write" {
		t.Errorf("start's pull-requests permission is %q, want write", start.Permissions["pull-requests"])
	}
	for _, step := range start.Steps {
		if step.Uses != "" {
			t.Errorf("start uses %s, but should run only gh", step.Uses)
		}
		if !step.ContinueOnError {
			t.Errorf("start's step %q stops the job when it fails", step.Name)
		}
	}
	script := start.runs()
	for _, want := range []string{resolveMarker, "actions/runs/$GITHUB_RUN_ID", "gh pr comment"} {
		if !strings.Contains(script, want) {
			t.Errorf("start doesn't contain %q", want)
		}
	}
	if strings.Contains(script, "<!-- factory-status -->") {
		t.Error("start uses the build's marker, <!-- factory-status -->")
	}
	if strings.Contains(script, "PATCH") {
		t.Error("start should post a new comment, never edit an old one")
	}
}

// Posting the status must never stop the rebase.
func TestResolveDoesNotWaitForTheStatusComment(t *testing.T) {
	for name, job := range readResolveWorkflow(t) {
		for _, need := range job.needs() {
			if need == "start" {
				t.Errorf("%s waits for start", name)
			}
		}
	}
}

// When the push job ends, it edits this run's status comment to say how it went, and still posts
// the comments that notify, since GitHub sends no notification for an edit.
func TestResolveEditsItsStatusCommentWhenItEnds(t *testing.T) {
	push := readResolveWorkflow(t)["push"]
	if push.Permissions["pull-requests"] != "write" {
		t.Errorf("push's pull-requests permission is %q, want write", push.Permissions["pull-requests"])
	}
	pushStep, _ := push.step(t, "Push the rebased branch")
	if pushStep.ID != "push" {
		t.Errorf("the push step has id %q, want push", pushStep.ID)
	}
	edit, editAt := push.step(t, "Edit the status comment")
	if !edit.ContinueOnError {
		t.Error("a failed edit stops the job")
	}
	if !strings.Contains(edit.If, "!cancelled()") {
		t.Errorf("the edit's condition is %q, want it to run unless cancelled", edit.If)
	}
	if !strings.Contains(edit.Env["PUSHED"], "steps.push.outcome") {
		t.Errorf("the edit takes PUSHED from %q, want the push step's outcome", edit.Env["PUSHED"])
	}
	for _, want := range []string{resolveMarker, "PATCH", "actions/runs/$GITHUB_RUN_ID"} {
		if !strings.Contains(edit.Run, want) {
			t.Errorf("the edit doesn't contain %q", want)
		}
	}
	for _, name := range []string{"Comment on the pull request", "Comment when resolution failed"} {
		step, at := push.step(t, name)
		if !strings.Contains(step.Run, "gh pr comment") {
			t.Errorf("%q no longer posts a comment", name)
		}
		if at > editAt {
			t.Errorf("%q comes after the edit", name)
		}
	}
}
