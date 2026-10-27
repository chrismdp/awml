package awml

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowStep struct {
	ID   string            `yaml:"id"`
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
}

type workflowJob struct {
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []workflowStep    `yaml:"steps"`
}

type factoryWorkflow struct {
	RunName string                 `yaml:"run-name"`
	On      map[string]any         `yaml:"on"`
	Jobs    map[string]workflowJob `yaml:"jobs"`
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

// Once Claude's patch is applied in the open job, the workspace holds the change's code, and later
// steps hold the app's token. So no step after the patch runs anything from the workspace: a
// script it needs is copied from main before the patch.
func TestOpenRunsNothingFromTheChange(t *testing.T) {
	open := readBuildWorkflow(t)["open"]
	applied := false
	for _, step := range open.Steps {
		if step.Name == "Apply the change to main as it is now" {
			applied = true
			continue
		}
		if !applied {
			continue
		}
		if step.Uses != "" && !strings.HasPrefix(step.Uses, "actions/") {
			t.Errorf("open's step %q uses %s after the patch is applied", step.Name, step.Uses)
		}
		for _, from := range []string{".github/", "examples/", "go run", "go build", "go test", "make ", "npm ", "npx ", "node ", "bash ", "source "} {
			if strings.Contains(step.Run, from) {
				t.Errorf("open's step %q runs %q from the workspace after the patch is applied", step.Name, from)
			}
		}
	}
	if !applied {
		t.Fatal("open has no step that applies the change to main as it is now")
	}
}

// The change's code runs in the draft and check jobs, so they don't save Go's module cache, which
// land's find job would otherwise restore before it builds the rule. The find job saves none
// either.
func TestTheGoCacheIsNeverShared(t *testing.T) {
	for file, jobs := range map[string]map[string]workflowJob{
		"build.yml": readBuildWorkflow(t),
		"land.yml":  readFactoryWorkflow(t, "land.yml").Jobs,
	} {
		for name, job := range jobs {
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "actions/setup-go") && step.With["cache"] != "false" {
					t.Errorf("%s's %s sets up Go with its cache on", file, name)
				}
			}
		}
	}
}

// Every step that runs Claude Code holds the maintainer's login token, so Claude Code strips it,
// and any other credential, from the commands it starts, which text in an issue could steer.
func TestClaudesCommandsNeverSeeItsToken(t *testing.T) {
	found := 0
	for name, job := range readBuildWorkflow(t) {
		for _, step := range job.Steps {
			if _, holds := step.Env["CLAUDE_CODE_OAUTH_TOKEN"]; !holds {
				continue
			}
			found++
			if step.Env["CLAUDE_CODE_SUBPROCESS_ENV_SCRUB"] != "1" {
				t.Errorf("build.yml's %s step %q gives Claude its token without scrubbing it from Claude's commands", name, step.Name)
			}
		}
	}
	if found == 0 {
		t.Error("no step in build.yml runs Claude Code with its token")
	}
}

// Each build and retry run is named for its issue and try by GitHub, from its own inputs, so a run
// that reads another can check that the run is the one it means before it trusts its artifacts.
func TestRunsAreCheckedBeforeTheirRecordsAreTrusted(t *testing.T) {
	for _, file := range []string{"build.yml", "retry.yml"} {
		name := readFactoryWorkflow(t, file).RunName
		if !strings.Contains(name, "issue") || !strings.Contains(name, "try") {
			t.Errorf("%s's run-name is %q, which doesn't name the issue and the try", file, name)
		}
	}
	build := readBuildWorkflow(t)
	for _, name := range []string{"start", "draft"} {
		runs := build[name].runs()
		if !strings.Contains(runs, "display_title") || !strings.Contains(runs, "retry.yml") {
			t.Errorf("build.yml's %s doesn't check that the previous run is retry.yml's for this issue", name)
		}
	}
	if !strings.Contains(readFactoryWorkflow(t, "retry.yml").Jobs["retry"].runs(), "display_title") {
		t.Error("retry.yml doesn't check that the build it reads is this issue's")
	}
	find := readFactoryWorkflow(t, "land.yml").Jobs["find"].runs()
	for _, want := range []string{"display_title", "head_branch", ".bot"} {
		if !strings.Contains(find, want) {
			t.Errorf("land.yml's find doesn't check the build run's %s", want)
		}
	}
}
