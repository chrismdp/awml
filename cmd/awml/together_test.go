package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrismdp/awml"
)

// Two workflows that hand work on through a shopping list, and an outside action that starts the
// shopping, as in the package's tests.
const planWorkflow = `awml: "0.9"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan next week's dinners", in: family }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: family }
  - { id: tell, kind: code, op: do, label: Tell the shopping the list is ready, in: family }
  - { id: ready, kind: outside, label: "Start the shopping: Allow", in: family, ref: shop.yaml#ready }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: list, kind: data }
  - { from: plan, to: tell, kind: start }
  - { from: tell, to: ready, kind: start }
`

const shopWorkflow = `awml: "0.9"
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

const agentLaptop = `awml: "0.9"
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
	if got := awmlRun("draw", "--implementation=home", plan, shopping); got.code != 2 || !strings.Contains(got.stderr, "draws one workflow") {
		t.Errorf("draw --implementation=home of two gave %+v", got)
	}
}

// Untrusted text that reaches an outside action unstopped in a workflow another starts is a line
// that breaks a rule, an error: check passes, saying it, and every command takes the files.
func TestUntrustedTextAcrossWorkflowsIsAnError(t *testing.T) {
	plan := strings.Replace(planWorkflow, "  - { id: tell,", "  - { id: form, kind: connector, label: The order form, untrusted: true, in: family }\n  - { id: tell,", 1)
	plan = strings.Replace(plan, "lines:\n", "lines:\n  - { from: form, to: tell, kind: data }\n", 1)
	dir := folder(t, "plan.yaml", plan, "shop.yaml", shopWorkflow)
	planPath, shopPath := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
	got := awmlRun("check", planPath, shopPath)
	if got.code != 0 || !strings.Contains(got.stdout, shopPath+`: error: untrusted text from part "form" in `+planPath+` reaches outside action "basket" with nothing to stop it or accept the risk: `) {
		t.Errorf("check gave %+v", got)
	}
	laptop := write(t, "laptop.yaml", agentLaptop)
	for _, args := range [][]string{{"draw", planPath, shopPath}, {"summary", planPath, shopPath}, {"fit", planPath, shopPath, laptop}, {"fit", "--parts", planPath, shopPath, laptop}} {
		if got := awmlRun(args...); got.code != 0 || got.stderr != "" {
			t.Errorf("awml %q gave %+v", args, got)
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
	if got.code != 0 || !strings.HasSuffix(got.stdout, "shared parts: 1\n  Shopping list (context): Dinners, Shopping\n") {
		t.Errorf("summary gave %+v", got)
	}
}

func TestFitTellsDescriptionsFromWorkflowsByWhatTheyHold(t *testing.T) {
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	laptop := write(t, "laptop.yaml", agentLaptop)
	want := "laptop  no: plan#list is kept at notes/list, which the platform doesn't hold\n" +
		"        no: plan#tell and shop#fill are code steps, which anthropic's agent would carry out\n"
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
	// Each part they share is judged once, named after its file.
	if got := awmlRun("fit", "--parts", dir, laptop); got.code != 0 || !strings.HasPrefix(got.stdout, "plan#thu    held by laptop\n") || strings.Count(got.stdout, "\nplan#list   held by none\n") != 1 || !strings.HasSuffix(got.stdout, "no placement: plan#list, plan#tell and shop#fill are held by no platform given\n") {
		t.Errorf("fit --parts of two gave %+v", got)
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

// One file named twice is one workflow, so it's drawn, judged and summarised as one.
func TestOneFileGivenTwiceIsOneWorkflow(t *testing.T) {
	placed := planWorkflow + "implementations:\n  home:\n    label: At home\n    blocks:\n      - { id: box, label: A laptop, parts: [plan, list, tell, ready] }\n"
	dir := folder(t, "plan.yaml", placed, "laptop.yaml", agentLaptop)
	plan, laptop := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "laptop.yaml")
	for _, args := range [][]string{
		{"summary", plan},
		{"draw", plan},
		{"graph", plan},
		{"draw", "--implementation", "home", plan},
		{"fit", "--implementation", "home", plan, laptop},
	} {
		twice := append(append([]string{}, args...), dir+"/./plan.yaml")
		if got, want := awmlRun(twice...), awmlRun(args...); got != want {
			t.Errorf("%q gave %+v\nwant %+v", twice, got, want)
		}
	}
}

// Mistakes found by following refs between files stop the commands that draw, count and fit them,
// as one file's own mistakes do: each is printed, and nothing else.
func TestCommandsRefuseFilesWhoseRefsHaveMistakes(t *testing.T) {
	laptop := write(t, "laptop.yaml", agentLaptop)
	for _, tc := range []struct{ name, plan, shop string }{
		{"a missing part", planWorkflow, strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#lists", 1)},
		{"a copy that disagrees", planWorkflow, strings.Replace(shopWorkflow, "label: Shopping list", "label: The list", 1)},
		{"a copy of another kind", planWorkflow, strings.Replace(shopWorkflow, "kind: context, label: Shopping list, at: notes/list", "kind: connector, label: Shopping list", 1)},
		{"a copy of a step", planWorkflow, strings.Replace(shopWorkflow, "plan.yaml#list", "plan.yaml#plan", 1)},
		{"a ref to its own workflow", strings.Replace(planWorkflow, "shop.yaml#ready", "plan.yaml#thu", 1), shopWorkflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := folder(t, "plan.yaml", tc.plan, "shop.yaml", tc.shop)
			plan, shop := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
			var want string
			for _, line := range strings.Split(awmlRun("check", plan, shop).stdout, "\n") {
				if line != "" && !strings.Contains(line, ": warning: ") && !strings.HasSuffix(line, ": ok") {
					want += line + "\n"
				}
			}
			if want == "" {
				t.Fatal("check finds no mistake")
			}
			out := filepath.Join(dir, "page.html")
			for _, args := range [][]string{
				{"draw", plan, shop}, {"summary", plan, shop}, {"summary", "--json", plan, shop},
				{"fit", plan, shop, laptop}, {"fit", "--json", plan, shop, laptop}, {"page", "-o", out, plan, shop},
			} {
				if got := awmlRun(args...); got != (result{1, "", want}) {
					t.Errorf("awml %q gave %+v\nwant stderr %q", args[:len(args)-2], got, want)
				}
			}
			if _, err := os.Stat(out); err == nil {
				t.Error("page wrote its file")
			}
		})
	}
	// One file is held to its refs too.
	path := write(t, "plan.yaml", strings.Replace(planWorkflow, "shop.yaml#ready", "plan.yaml#thu", 1))
	want := path + `: part "ready" refers to plan.yaml#thu, which is in its own workflow: a ref names a part of another workflow` + "\n"
	for _, args := range [][]string{{"draw", path}, {"summary", path}, {"fit", path, laptop}} {
		if got := awmlRun(args...); got != (result{1, "", want}) {
			t.Errorf("awml %q gave %+v", args, got)
		}
	}
}

// fit names two files of the same name by the folders they're in.
func TestFitNamesFilesOfTheSameNameByTheirFolders(t *testing.T) {
	root := t.TempDir()
	plan := strings.Replace(planWorkflow, "  - { id: tell, kind: code, op: do, label: Tell the shopping the list is ready, in: family }\n"+
		"  - { id: ready, kind: outside, label: \"Start the shopping: Allow\", in: family, ref: shop.yaml#ready }\n", "", 1)
	plan = strings.Replace(plan, "  - { from: plan, to: tell, kind: start }\n  - { from: tell, to: ready, kind: start }\n", "", 1)
	for _, dir := range []string{"x", "y"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "plan.yaml"), []byte(plan), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	laptop := write(t, "laptop.yaml", strings.Replace(agentLaptop, "holds: []", "holds: [notes]", 1))
	chdir(t, root)
	want := "laptop  fits: x/plan#plan and y/plan#plan may use anthropic\n"
	if got := awmlRun("fit", "x/plan.yaml", "y/plan.yaml", laptop); got != (result{0, want, ""}) {
		t.Errorf("fit gave %+v\nwant %q", got, want)
	}
}

// A folder with no .yaml file in it is a mistake in the command, which says so.
func TestAFolderWithNoYamlFilesIsAMistakeInTheCommand(t *testing.T) {
	empty := folder(t, "notes.txt", "not a workflow")
	plan := write(t, "plan.yaml", planWorkflow)
	for _, command := range []string{"check", "draw", "summary", "fit"} {
		for _, args := range [][]string{{empty}, {plan, empty}} {
			got := awmlRun(append([]string{command}, args...)...)
			want := commands[command].usage + "awml " + command + ": error: " + empty + " holds no .yaml files\n"
			if got != (result{2, "", want}) {
				t.Errorf("%s %q gave %+v\nwant stderr %q", command, args, got, want)
			}
		}
	}
}
