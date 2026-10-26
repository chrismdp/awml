package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrismdp/awml"
)

// Two workflows that hand work on through a shopping list and a trigger, as in the package's tests.
const planWorkflow = `awml: "0.6"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan next week's dinners", in: family }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: family }
  - { id: ready, kind: trigger, label: The shopping list is ready, ref: shop.yaml#ready }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: list, kind: data }
  - { from: plan, to: ready, kind: start }
`

const shopWorkflow = `awml: "0.6"
title: Shopping
scopes:
  - { id: home, label: "home: parents, kids", processing: any }
parts:
  - { id: ready, kind: trigger, label: The shopping list is ready, wait: PT1H }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: home, ref: plan.yaml#list }
  - { id: fill, kind: code, op: do, label: Fill the basket, in: home }
  - { id: basket, kind: outside, label: "Fill the basket: Allow", in: home }
lines:
  - { from: ready, to: fill, kind: start }
  - { from: list, to: fill, kind: data }
  - { from: fill, to: basket, kind: data }
`

const agentLaptop = `awml: "0.6"
platform: { id: laptop, label: A laptop }
off_for_up_to: PT0S
steps: agent
agents: [anthropic]
calls: []
holds: []
`

// folder writes files into one new folder and gives its path.
func folder(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < len(files); i += 2 {
		if err := os.WriteFile(filepath.Join(dir, files[i]), []byte(files[i+1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckFollowsEachRefBetweenTheFilesGiven(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shop)
	plan, shopping := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
	want := plan + ": ok\n" + shopping + `: part "list" refers to plan.yaml#lists, but plan.yaml has no part "lists"` + "\n"
	if got := awmlRun("check", plan, shopping); got != (result{1, want, ""}) {
		t.Errorf("check gave %+v\nwant %q", got, want)
	}
	// Given alone, each is checked as it stands.
	if got := awmlRun("check", shopping); got != (result{0, shopping + ": ok\n", ""}) {
		t.Errorf("check of one gave %+v", got)
	}
}

func TestCheckPrintsTheWarningsOnlyTheFilesTogetherShow(t *testing.T) {
	shop := strings.Replace(shopWorkflow, "  - { from: fill, to: basket", "  - { from: fill, to: list, kind: data }\n  - { from: fill, to: basket", 1)
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shop)
	plan, shopping := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
	want := plan + `: warning: context "list" is written by ` + plan + " and " + shopping + ": each may change what another relies on\n" +
		plan + ": ok\n" + shopping + `: warning: do step "fill" leads to 2 changes ("list", "basket"): each do step should make one` + "\n" + shopping + ": ok\n"
	if got := awmlRun("check", plan, shopping); got != (result{0, want, ""}) {
		t.Errorf("check gave %+v\nwant %q", got, want)
	}
}

func TestAFolderStandsForEveryYamlFileDirectlyInsideIt(t *testing.T) {
	dir := folder(t, "shop.yaml", strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1), "plan.yaml", planWorkflow, "notes.txt", "not a workflow")
	if err := os.Mkdir(filepath.Join(dir, "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deeper", "bad.yaml"), []byte("nonsense: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	got, want := awmlRun("check", dir), awmlRun("check", filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml"))
	if got != want || got.code != 1 {
		t.Errorf("check of a folder gave %+v\nwant %+v", got, want)
	}
}

func TestDrawOfSeveralWorkflowsDrawsTheirMap(t *testing.T) {
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	plan, shopping := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
	want := awml.DrawMap([]awml.Workflow{{File: plan, Doc: doc(t, planWorkflow)}, {File: shopping, Doc: doc(t, shopWorkflow)}}) + "\n"
	if got := awmlRun("draw", plan, shopping); got != (result{0, want, ""}) {
		t.Errorf("draw gave %+v", got)
	}
	if got := awmlRun("draw", dir); got != (result{0, want, ""}) {
		t.Errorf("draw of a folder gave %+v", got)
	}
	for _, option := range []string{"--untrusted", "--implementation=home"} {
		if got := awmlRun("draw", option, plan, shopping); got.code != 2 || !strings.Contains(got.stderr, "draws one workflow") {
			t.Errorf("draw %s of two gave %+v", option, got)
		}
	}
}

func TestSummaryOfSeveralWorkflowsNamesWhatTheyShare(t *testing.T) {
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	plan, shopping := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
	summary := awml.SummariseTogether([]awml.Workflow{{File: plan, Doc: doc(t, planWorkflow)}, {File: shopping, Doc: doc(t, shopWorkflow)}})
	want, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if got := awmlRun("summary", "--json", dir); got != (result{0, string(want) + "\n", ""}) {
		t.Errorf("summary --json gave %+v", got)
	}
	got := awmlRun("summary", plan, shopping)
	if got.code != 0 || !strings.HasSuffix(got.stdout, "shared parts: 2\n  Shopping list (context): Dinners, Shopping\n  The shopping list is ready (trigger): Dinners, Shopping\n") {
		t.Errorf("summary gave %+v", got)
	}
}

func TestFitTellsDescriptionsFromWorkflowsByWhatTheyHold(t *testing.T) {
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	laptop := write(t, "laptop.yaml", agentLaptop)
	want := "laptop  no: plan#list is kept at notes/list, which the platform doesn't hold\n" +
		"        no: shop#fill is a code step, which anthropic's agent would carry out\n"
	for _, args := range [][]string{
		{"fit", laptop, filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")},
		{"fit", dir, laptop},
	} {
		if got := awmlRun(args...); got != (result{0, want, ""}) {
			t.Errorf("awml %q gave %+v", args, got)
		}
	}
	if got := awmlRun("fit", "--implementation", "home", dir, laptop); got.code != 2 || !strings.Contains(got.stderr, "judges one workflow") {
		t.Errorf("fit --implementation of two gave %+v", got)
	}
}

// chdir moves the test into a folder until it ends.
func chdir(t *testing.T, dir string) {
	t.Helper()
	was, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(was) })
}

// A ref is followed to where a file is, whether it was given by an absolute path, or by a relative
// one from the folder awml runs in.
func TestARefIsFollowedToWhereAFileIsHoweverItIsNamed(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"family", "work"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	plan := strings.Replace(planWorkflow, "shop.yaml#ready", "../work/shop.yaml#readied", 1)
	shop := strings.Replace(shopWorkflow, "plan.yaml#list", "family/plan#lists", 1)
	for name, text := range map[string]string{"family/plan.yaml": plan, "work/shop.yaml": shop} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	chdir(t, filepath.Join(root, "family"))
	for _, other := range []string{"../work/shop.yaml", filepath.Join(root, "work", "shop.yaml")} {
		want := "plan.yaml: part \"ready\" refers to ../work/shop.yaml#readied, but ../work/shop.yaml has no part \"readied\"\n" +
			other + ": part \"list\" refers to family/plan#lists, but family/plan has no part \"lists\"\n"
		if got := awmlRun("check", "plan.yaml", other); got != (result{1, want, ""}) {
			t.Errorf("check plan.yaml %s gave %+v\nwant %q", other, got, want)
		}
		if got := awmlRun("check", "--json", "plan.yaml", other); got.code != 1 || !strings.Contains(got.stdout, `family/plan has no part \"lists\"`) {
			t.Errorf("check --json plan.yaml %s gave %+v", other, got)
		}
	}
}

// A file named twice, as by its folder and by itself, is read once, where it's first named.
func TestAFileGivenTwiceIsReadOnce(t *testing.T) {
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	plan := filepath.Join(dir, "plan.yaml")
	for _, args := range [][]string{
		{dir, plan},
		{dir, filepath.Join(dir, ".", "shop.yaml"), plan},
	} {
		for _, command := range []string{"check", "summary"} {
			if got, want := awmlRun(append([]string{command}, args...)...), awmlRun(command, dir); got != want {
				t.Errorf("%s %q gave %+v\nwant %+v", command, args, got, want)
			}
		}
	}
	chdir(t, dir)
	if got, want := awmlRun("check", "plan.yaml", ".", "./plan.yaml"), awmlRun("check", "plan.yaml", "shop.yaml"); got != want {
		t.Errorf("check plan.yaml . ./plan.yaml gave %+v\nwant %+v", got, want)
	}
}
