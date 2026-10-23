package awml

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowJob struct {
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []struct {
		Uses string `yaml:"uses"`
		Run  string `yaml:"run"`
	} `yaml:"steps"`
}

func readBuildWorkflow(t *testing.T) map[string]workflowJob {
	t.Helper()
	var workflow struct {
		Jobs map[string]workflowJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readText(t, ".github/workflows/build.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs
}

func (j workflowJob) runs() string {
	var all []string
	for _, step := range j.Steps {
		all = append(all, step.Run)
	}
	return strings.Join(all, "\n")
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
	for _, name := range []string{"draft", "check"} {
		for permission, level := range jobs[name].Permissions {
			if level != "read" {
				t.Errorf("%s has %s: %s, but should only read", name, permission, level)
			}
		}
	}
}

// The jobs that end a build edit the same status comment, to say how it ended.
func TestBuildEditsItsStatusWhenItEnds(t *testing.T) {
	jobs := readBuildWorkflow(t)
	for _, name := range []string{"open", "retry"} {
		if !strings.Contains(jobs[name].runs(), "<!-- factory-status -->") {
			t.Errorf("%s doesn't edit the status comment", name)
		}
		if jobs[name].Permissions["issues"] != "write" {
			t.Errorf("%s can't write to issues, so can't edit the status comment", name)
		}
	}
}
