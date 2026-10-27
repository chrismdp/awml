package awml

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
}

type workflowJob struct {
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []workflowStep    `yaml:"steps"`
}

type factoryWorkflow struct {
	On   map[string]any         `yaml:"on"`
	Jobs map[string]workflowJob `yaml:"jobs"`
}

func readFactoryWorkflow(t *testing.T, name string) factoryWorkflow {
	t.Helper()
	var workflow factoryWorkflow
	if err := yaml.Unmarshal([]byte(readText(t, ".github/workflows/"+name)), &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func readBuildWorkflow(t *testing.T) map[string]workflowJob {
	t.Helper()
	return readFactoryWorkflow(t, "build.yml").Jobs
}

func (j workflowJob) runs() string {
	var all []string
	for _, step := range j.Steps {
		all = append(all, step.Run)
	}
	return strings.Join(all, "\n")
}

func onlyReads(t *testing.T, workflow, name string, job workflowJob) {
	t.Helper()
	for permission, level := range job.Permissions {
		if level != "read" {
			t.Errorf("%s's %s has %s: %s, but should only read", workflow, name, permission, level)
		}
	}
}

// A build says on the issue that it has started before Claude drafts, from a job that may write
// to issues but checks out and runs nothing.
func TestBuildPostsItsStatusBeforeItDrafts(t *testing.T) {
	jobs := readBuildWorkflow(t)
	start, ok := jobs["start"]
	if !ok {
		t.Fatal("build.yml has no start")
	}
	if start.Permissions["issues"] != "write" {
		t.Errorf("start's issues permission is %q, want write", start.Permissions["issues"])
	}
	for name, level := range start.Permissions {
		if level == "write" && name != "issues" {
			t.Errorf("start may write %s, but needs to write only issues", name)
		}
	}
	for _, step := range start.Steps {
		if step.Uses != "" {
			t.Errorf("start uses %s, but should run only gh", step.Uses)
		}
	}
	if !strings.Contains(start.runs(), "<!-- factory-status -->") {
		t.Error("start doesn't mark its comment with <!-- factory-status -->")
	}
	if needs, _ := jobs["draft"].Needs.(string); needs != "start" {
		t.Errorf("draft needs %v, want start", jobs["draft"].Needs)
	}
	for _, name := range []string{"draft", "check", "review"} {
		onlyReads(t, "build.yml", name, jobs[name])
	}
}

// The open job edits the status comment to link the pull request, and retry.yml edits it when the
// build fails.
func TestBuildEditsItsStatusWhenItEnds(t *testing.T) {
	jobs := map[string]workflowJob{
		"build.yml open":  readBuildWorkflow(t)["open"],
		"retry.yml retry": readFactoryWorkflow(t, "retry.yml").Jobs["retry"],
	}
	for name, job := range jobs {
		if !strings.Contains(job.runs(), "<!-- factory-status -->") {
			t.Errorf("%s doesn't edit the status comment", name)
		}
		if job.Permissions["issues"] != "write" {
			t.Errorf("%s can't write to issues, so can't edit the status comment", name)
		}
	}
}

// Claude redraws and pictures the examples its change alters as part of its draft, so the checks
// only check.
func TestTheDraftPicturesWhatItRedraws(t *testing.T) {
	jobs := readBuildWorkflow(t)
	if !strings.Contains(jobs["draft"].runs(), `"Bash(.github/picture-drawings.sh:*)"`) {
		t.Error("the draft step can't run .github/picture-drawings.sh")
	}
	if !strings.Contains(readText(t, ".github/build-prompt.md"), ".github/picture-drawings.sh") {
		t.Error("build-prompt.md doesn't tell Claude to picture what it redraws")
	}
	if strings.Contains(jobs["check"].runs(), "picture-drawings.sh") {
		t.Error("the check job still redraws and pictures")
	}
}

// The bug and security reviews start together, each its own claude run, in a job that only
// reads, and the classifier runs after their findings are weighed.
func TestTheReviewsRunSideBySide(t *testing.T) {
	review := readBuildWorkflow(t)["review"]
	together := -1
	classify := -1
	for i, step := range review.Steps {
		if strings.Contains(step.Run, "review-bugs-prompt.md") && strings.Contains(step.Run, "review-security-prompt.md") {
			together = i
		}
		if strings.Contains(step.Run, "classify-prompt.md") && !strings.Contains(step.Name, "Apply") {
			classify = i
		}
	}
	if together < 0 {
		t.Fatal("no one step of the review job starts both reviews")
	}
	if !strings.Contains(review.Steps[together].Run, "wait") {
		t.Error("the step that starts both reviews doesn't wait for them")
	}
	if classify <= together {
		t.Error("the classifier doesn't run after the reviews")
	}
}

// The open job pushes onto main as it is when it pushes, not the main the build started from, so a
// workflow that changed on main meanwhile isn't pushed back as part of the change (awml #115).
func TestOpenPushesOntoMainAsItIs(t *testing.T) {
	open := readBuildWorkflow(t)["open"]
	for _, step := range open.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout") {
			if step.With["ref"] != "main" {
				t.Errorf("open checks out %q, want main", step.With["ref"])
			}
			return
		}
	}
	t.Error("open checks out nothing")
}

// A failed try, from the build or from CI, only starts retry.yml, and only retry.yml starts the
// build again or says on the issue that it failed.
func TestOnlyRetryStartsTheBuildAgain(t *testing.T) {
	build := readBuildWorkflow(t)["retry"]
	land := readFactoryWorkflow(t, "land.yml").Jobs["land"]
	for name, job := range map[string]workflowJob{"build.yml's retry": build, "land.yml's land": land} {
		if !strings.Contains(job.runs(), "gh workflow run retry.yml") {
			t.Errorf("%s doesn't start retry.yml", name)
		}
	}
	if len(build.Permissions) != 1 || build.Permissions["actions"] != "write" {
		t.Errorf("build.yml's retry has %v, want only actions: write", build.Permissions)
	}
	for _, file := range []string{"build.yml", "land.yml"} {
		text := readText(t, ".github/workflows/"+file)
		for _, only := range []string{"gh workflow run build.yml", "The build failed three times"} {
			if strings.Contains(text, only) {
				t.Errorf("%s has %q, which belongs to retry.yml alone", file, only)
			}
		}
	}
	retry := readFactoryWorkflow(t, "retry.yml")
	runs := retry.Jobs["retry"].runs()
	for _, want := range []string{"gh workflow run build.yml", "The build failed three times"} {
		if !strings.Contains(runs, want) {
			t.Errorf("retry.yml doesn't have %q", want)
		}
	}
	dispatch, _ := retry.On["workflow_dispatch"].(map[string]any)
	inputs, _ := dispatch["inputs"].(map[string]any)
	for _, input := range []string{"issue", "attempt", "requested_by", "run"} {
		if _, ok := inputs[input]; !ok {
			t.Errorf("retry.yml takes no %s", input)
		}
	}
}

// land.yml runs main's code when CI finishes, never the branch's, and its reading job holds no
// token that can write.
func TestLandRunsOnlyMainsCode(t *testing.T) {
	land := readFactoryWorkflow(t, "land.yml")
	if _, ok := land.On["workflow_run"]; !ok {
		t.Error("land.yml doesn't start when CI finishes")
	}
	onlyReads(t, "land.yml", "find", land.Jobs["find"])
	for name, job := range land.Jobs {
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout") && step.With["ref"] != "${{ github.sha }}" {
				t.Errorf("land.yml's %s checks out %q, not main's commit", name, step.With["ref"])
			}
		}
	}
}
