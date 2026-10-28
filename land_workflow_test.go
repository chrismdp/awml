package awml

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// land.yml runs main's code when CI finishes, never the branch's: its find job reads with a token
// that can't write and checks out main's commit, and its land job checks out nothing.
func TestLandRunsOnlyMainsCode(t *testing.T) {
	jobs := readFactoryWorkflow(t, "land.yml").Jobs
	onlyReads(t, "land.yml", "find", jobs["find"])
	for name, job := range jobs {
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout") && (name != "find" || step.With["ref"] != "${{ github.sha }}") {
				t.Errorf("land.yml's %s checks out %q, not main's commit", name, step.With["ref"])
			}
		}
	}
	if got := jobs["land"].needs(); len(got) != 1 || got[0] != "find" {
		t.Errorf("the land job needs %v, want find", got)
	}
	for _, output := range []string{"outcome", "reasons", "pr", "head_sha", "labeller", "classify"} {
		if jobs["find"].Outputs[output] == "" {
			t.Errorf("the find job doesn't pass on %s", output)
		}
	}
	if len(jobs["find"].Outputs) != 6 {
		t.Errorf("the find job passes on %v, want only the sha, the outcome, the login, the reasons and the links", jobs["find"].Outputs)
	}
}

func landRun(event, branch string) string {
	return `
  "api repos/owner/repo/actions/runs/77"*) answer <<< '{"path": ".github/workflows/ci.yml", "event": "` + event + `", "status": "completed",
    "head_sha": "` + strings.Repeat("ab", 20) + `", "head_branch": "` + branch + `", "html_url": "https://github.com/owner/repo/actions/runs/77",
    "head_repository": {"full_name": "owner/repo"}}' ;;`
}

func landJobs(jobs ...string) string {
	return `
  "api --paginate repos/owner/repo/actions/runs/77/jobs"*) answer <<< '{"jobs": [` + strings.Join(jobs, ", ") + `]}' ;;`
}

func landJob(name, status, conclusion string) string {
	return `{"name": "` + name + `", "status": "` + status + `", "conclusion": "` + conclusion + `", "html_url": "https://github.com/owner/repo/actions/runs/77/job/1"}`
}

func landPulls(user, sha string) string {
	pulls := "[]"
	if user != "" {
		pulls = `[{"number": 8, "base": {"ref": "main"}, "head": {"ref": "build/issue-120", "sha": "` + sha +
			`", "repo": {"full_name": "owner/repo"}}, "user": {"login": "` + user + `", "type": "Bot"}}]`
	}
	return `
  "api repos/owner/repo/pulls?head=owner:build/issue-120&base=main&state=open"*) answer <<< '` + pulls + `' ;;
  "api --paginate repos/owner/repo/issues/120/events"*) answer <<< '[
    {"event": "labeled", "label": {"name": "build"}, "actor": {"login": "the-maintainer", "type": "User"}},
    {"event": "labeled", "label": {"name": "build"}, "actor": {"login": "awml-factory[bot]", "type": "Bot"}}]' ;;`
}

// land.yml exits quietly unless CI's run is a pull_request run on a factory branch of this
// repository, the pull request is the app's and open, its head is CI's commit, and every check has
// finished and passed: a failed check is retry-a-build's or the fixer's. It finds who last added
// the issue's build label, a person, to tell or ask.
func TestLandExitsQuietlyUnlessEveryCheckPassedOnTheHead(t *testing.T) {
	ready, _ := readFactoryWorkflow(t, "land.yml").Jobs["find"].step(t, "ready")
	head := strings.Repeat("ab", 20)
	passed := landJobs(landJob("test (1.22)", "completed", "success"), landJob("test (stable)", "completed", "success"),
		landJob("Bug review", "completed", "success"), landJob("Security review", "completed", "success"),
		landJob("Classify", "completed", "success"), landJob("report", "completed", "skipped"))

	r := newRehearsal(t)
	r.run(ready, map[string]string{"CI_RUN": "77"}, landRun("pull_request", "build/issue-120")+passed+landPulls("awml-factory[bot]", head))
	r.mustPass()
	if r.outputs["ready"] != "true" || r.outputs["pr"] != "8" || r.outputs["head_sha"] != head ||
		r.outputs["labeller"] != "the-maintainer" || r.outputs["classify"] != "https://github.com/owner/repo/actions/runs/77/job/1" {
		t.Errorf("land didn't find the pull request ready to judge: %v", r.outputs)
	}

	quiet := map[string]string{
		"a push's run":            landRun("push", "build/issue-120") + passed + landPulls("awml-factory[bot]", head),
		"a person's branch":       landRun("pull_request", "scones") + passed + landPulls("awml-factory[bot]", head),
		"a failed review":         landRun("pull_request", "build/issue-120") + landJobs(landJob("test (1.22)", "completed", "success"), landJob("test (stable)", "completed", "success"), landJob("Bug review", "completed", "failure"), landJob("Security review", "completed", "success"), landJob("Classify", "completed", "skipped")) + landPulls("awml-factory[bot]", head),
		"a failed test":           landRun("pull_request", "build/issue-120") + landJobs(landJob("test (1.22)", "completed", "failure"), landJob("test (stable)", "completed", "success"), landJob("Bug review", "completed", "success"), landJob("Security review", "completed", "success")) + landPulls("awml-factory[bot]", head),
		"an unfinished Classify":  landRun("pull_request", "build/issue-120") + landJobs(landJob("test (1.22)", "completed", "success"), landJob("test (stable)", "completed", "success"), landJob("Bug review", "completed", "success"), landJob("Security review", "completed", "success"), landJob("Classify", "in_progress", "")) + landPulls("awml-factory[bot]", head),
		"reviews that never ran":  landRun("pull_request", "build/issue-120") + landJobs(landJob("test (1.22)", "completed", "success"), landJob("test (stable)", "completed", "success"), landJob("Bug review", "completed", "skipped"), landJob("Security review", "completed", "skipped")) + landPulls("awml-factory[bot]", head),
		"a moved head":            landRun("pull_request", "build/issue-120") + passed + landPulls("awml-factory[bot]", strings.Repeat("cd", 20)),
		"a person's pull request": landRun("pull_request", "build/issue-120") + passed + landPulls("someone", head),
		"a person named as the app": landRun("pull_request", "build/issue-120") + passed +
			strings.Replace(landPulls("awml-factory[bot]", head), `"type": "Bot"}}]`, `"type": "User"}}]`, 1),
		"no open pull request": landRun("pull_request", "build/issue-120") + passed + landPulls("", head),
		"a fork's run":         fromAFork(landRun("pull_request", "build/issue-120")) + passed + landPulls("awml-factory[bot]", head),
	}
	for name, answers := range quiet {
		q := newRehearsal(t)
		q.run(ready, map[string]string{"CI_RUN": "77"}, answers)
		q.mustPass()
		if q.outputs["ready"] != "false" {
			t.Errorf("with %s, land went on: %v", name, q.outputs)
		}
		if strings.Contains(q.calls, "comment") {
			t.Errorf("with %s, land commented", name)
		}
	}
}

// rehearseJudge rehearses land's judge step on a branch of the given commits, made on a bakery's main in an
// origin of its own, with the reviews' checks and Classify's verdict as the stub answers them, and
// main's rule built from this repository.
func rehearseJudge(t *testing.T, commits []struct{ author, message string }, annotations, verdict string, classifyPassed ...string) *rehearsal {
	t.Helper()
	return rehearseJudgeWith(t, nil, nil, nil, commits, annotations, verdict, classifyPassed...)
}

// rehearseJudgeWith rehearses the judge step as rehearseJudge does, with main starting from the
// files given, the branch's last commit also making branchChanges, if it's given, and main moved on
// by mainMoves, if it's given, once the branch has been made.
func rehearseJudgeWith(t *testing.T, files map[string]string, branchChanges, mainMoves func(origin string), commits []struct{ author, message string },
	annotations, verdict string, classifyPassed ...string) *rehearsal {
	t.Helper()
	find := readFactoryWorkflow(t, "land.yml").Jobs["find"]
	step, _ := find.step(t, "judge")
	r := newRehearsal(t)
	build := exec.Command("go", "build", "-o", filepath.Join(r.temp, "build-rule"), "./internal/buildrule")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the rule: %v\n%s", err, out)
	}
	r.write("build-rules.yml", readText(t, ".github/build-rules.yml"))
	r.write("read-findings.sh", readText(t, ".github/read-findings.sh"))
	main := map[string]string{"recipes.md": "Bread\n"}
	for file, text := range files {
		main[file] = text
	}
	origin, _ := r.newRepo("origin", main)
	gitIn(t, origin, "switch", "-q", "-c", "build/issue-120")
	for at, c := range commits {
		r.write(filepath.Join(origin, "recipes.md"), "Bread\n"+strings.Repeat("Scones\n", at+1))
		if at == len(commits)-1 && branchChanges != nil {
			branchChanges(origin)
		}
		gitIn(t, origin, "add", "-A")
		gitIn(t, origin, "-c", "user.name=x", "-c", "user.email="+c.author, "commit", "-q", "--author", "Someone <"+c.author+">", "-m", c.message)
	}
	head := gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "main")
	if mainMoves != nil {
		mainMoves(origin)
	}
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	passed := "true"
	if len(classifyPassed) > 0 {
		passed = classifyPassed[0]
	}
	r.run(step, map[string]string{"CI_RUN": "77", "HEAD_SHA": head, "CLASSIFY_PASSED": passed}, `
  "api --paginate repos/owner/repo/actions/runs/77/jobs"*) answer <<< '{"jobs": [
    {"name": "Bug review", "check_run_url": "https://api.github.com/repos/owner/repo/check-runs/2"},
    {"name": "Security review", "check_run_url": "https://api.github.com/repos/owner/repo/check-runs/3"},
    {"name": "Classify", "check_run_url": "https://api.github.com/repos/owner/repo/check-runs/4"}]}' ;;
  "api --paginate repos/owner/repo/check-runs/2/annotations"*) answer <<< '`+annotations+`' ;;
  "api --paginate repos/owner/repo/check-runs/3/annotations"*) answer <<< '[]' ;;
  "api --paginate repos/owner/repo/check-runs/4/annotations"*) answer <<< '[`+verdict+`]' ;;`)
	r.mustPass()
	return r
}

// The rule judges the pushed commit with git, reading the branch's commits since main, the reviews'
// findings and Classify's verdict, and can only move the verdict towards ask: a fix commit, or a
// Nit left unfixed, means at least show; a commit that isn't the factory's, or no verdict, means ask.
func TestLandJudgesThePushedCommit(t *testing.T) {
	const app = "41+awml-factory[bot]@users.noreply.github.com"
	type made = struct{ author, message string }
	ship := `{"annotation_level": "notice", "title": "Verdict", "message": "ship"}`
	nit := `[{"annotation_level": "warning", "title": "Nit: A clearer name", "path": "recipes.md", "start_line": 1, "message": "Say it."}]`
	tests := []struct {
		name        string
		commits     []made
		annotations string
		verdict     string
		outcome     string
		reason      string
	}{
		{name: "a build commit Classify ships", commits: []made{{app, "Add scones"}}, annotations: "[]", verdict: ship,
			outcome: "ship", reason: "No rule moved the verdict towards ask."},
		{name: "a fix commit on top", commits: []made{{app, "Add scones"}, {app, "Fix\n\nFixed-by: fix-review-findings"}}, annotations: "[]", verdict: ship,
			outcome: "show", reason: "The branch carries 1 fix commit."},
		{name: "a Nit left unfixed", commits: []made{{app, "Add scones"}}, annotations: nit, verdict: ship,
			outcome: "show", reason: "The reviews left 1 Nit unfixed."},
		{name: "a person's commit", commits: []made{{app, "Add scones"}, {"someone@example.com", "Tweak"}}, annotations: "[]", verdict: ship,
			outcome: "ask", reason: "A commit since main isn't the factory's."},
		{name: "no verdict", commits: []made{{app, "Add scones"}}, annotations: "[]", verdict: "",
			outcome: "ask", reason: "Classify gave no verdict, so the change waits for review."},
	}
	for _, tt := range tests {
		r := rehearseJudge(t, tt.commits, tt.annotations, tt.verdict)
		var reasons []string
		if err := json.Unmarshal([]byte(r.outputs["reasons"]), &reasons); err != nil {
			t.Fatalf("%s: the reasons %q aren't a list: %v", tt.name, r.outputs["reasons"], err)
		}
		if r.outputs["outcome"] != tt.outcome || !contains(reasons, tt.reason) {
			t.Errorf("%s: land judged %s for %q, want %s for %q", tt.name, r.outputs["outcome"], reasons, tt.outcome, tt.reason)
		}
		if got := gitIn(t, r.dir, "status", "--porcelain"); got != "" {
			t.Errorf("%s: judging changed main's checkout: %s", tt.name, got)
		}
	}
}

// fromAFork makes a CI run's answer a fork's, whose branch carries the same name as one of this
// repository's.
func fromAFork(answers string) string {
	return strings.Replace(answers, `"head_repository": {"full_name": "owner/repo"}`, `"head_repository": {"full_name": "a-stranger/repo"}`, 1)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// Ship merges, pinned with --match-head-commit to the commit judged, with the app's token. Show
// merges the same way, then tells the person who added the label what to look at, in a new
// comment. Ask requests their review in a new comment and doesn't merge. The rule's reasons are
// cleaned, and Classify's are linked, never quoted.
func TestLandShipsShowsOrAsks(t *testing.T) {
	workflow := readFactoryWorkflow(t, "land.yml")
	if strings.Contains(readText(t, ".github/workflows/land.yml"), "cancel-in-progress: true\n\njobs:") {
		t.Error("land.yml cancels a whole run, which could stop it between the merge and its comment")
	}
	land := workflow.Jobs["land"]
	if !strings.Contains(readText(t, ".github/workflows/land.yml"), "group: land-pull-${{ needs.find.outputs.pr }}\n      cancel-in-progress: false") {
		t.Error("a land job that has started merging can be cancelled")
	}
	write, _ := land.step(t, "write")
	merge, _ := land.step(t, "merge")
	show, _ := land.step(t, "show")
	ask, _ := land.step(t, "ask")
	if merge.If != "env.OUTCOME == 'ship' || env.OUTCOME == 'show'" || merge.Env["GH_TOKEN"] != "${{ steps.app.outputs.token }}" {
		t.Errorf("the merge runs if %q with %q, want ship or show, with the app's token", merge.If, merge.Env["GH_TOKEN"])
	}
	if show.If != "env.OUTCOME == 'show'" || ask.If != "env.OUTCOME == 'ask'" {
		t.Errorf("show runs if %q and ask if %q", show.If, ask.If)
	}
	if strings.Contains(ask.Run, "pr merge") {
		t.Error("ask merges")
	}
	head := strings.Repeat("ab", 20)
	env := map[string]string{"OUTCOME": "show", "PR_NUMBER": "8", "HEAD_SHA": head, "LABELLER": "the-maintainer",
		"CLASSIFY": "https://github.com/owner/repo/actions/runs/77/job/1",
		"REASONS":  `["The branch carries 1 fix commit.", "` + "`docs/@someone <b>.md`" + ` is a path that always needs review."]`}

	r := newRehearsal(t)
	r.run(write, env, "")
	r.mustPass()
	shown := r.read("show.md")
	for _, want := range []string{"@the-maintainer,", "- The branch carries 1 fix commit.", "@\u200bsomeone &lt;b>", "[Classify's reasons](https://github.com/owner/repo/actions/runs/77/job/1)"} {
		if !strings.Contains(shown, want) {
			t.Errorf("the show comment lacks %q:\n%s", want, shown)
		}
	}
	if !strings.Contains(r.read("merge.md"), "Shown by land.yml") || !strings.Contains(r.read("ask.md"), "needs your review") {
		t.Errorf("the merge message or the ask comment don't say what land did:\n%s\n%s", r.read("merge.md"), r.read("ask.md"))
	}

	r.run(merge, env, `  "pr merge"*) ;;`)
	r.mustPass()
	if !strings.Contains(r.calls, "pr merge 8 --repo owner/repo --squash --match-head-commit "+head+" --body-file") {
		t.Errorf("the merge isn't pinned to the commit judged:\n%s", r.calls)
	}

	asked := newRehearsal(t)
	env["OUTCOME"] = "ask"
	asked.run(write, env, "")
	asked.run(ask, env, `
  "pr edit 8 --repo owner/repo --add-reviewer the-maintainer"*) ;;
  "pr comment 8"*) ;;`)
	asked.mustPass()
	if bodies := asked.bodies(); len(bodies) != 1 || !strings.Contains(bodies[0], "@the-maintainer, CI passed") {
		t.Errorf("ask said %q, want a new comment asking the labeller", bodies)
	}

	for _, bad := range []map[string]string{{"OUTCOME": "merge"}, {"LABELLER": "a b"}, {"HEAD_SHA": "main"}, {"CLASSIFY": "javascript:x"}} {
		b := newRehearsal(t)
		broken := map[string]string{}
		for k, v := range env {
			broken[k] = v
		}
		for k, v := range bad {
			broken[k] = v
		}
		b.run(write, broken, "")
		if !b.failed {
			t.Errorf("the land job took %v from the find job", bad)
		}
	}
}

// npm's and yarn's settings and a committed node_modules change what npm runs, so the build rule
// asks a person about a change to any of them, wherever it sits in the tree.
func TestTheBuildRuleAsksAboutNpmsOwnFiles(t *testing.T) {
	var rules struct {
		ReviewPaths []string `yaml:"review_paths"`
	}
	if err := yaml.Unmarshal([]byte(readText(t, ".github/build-rules.yml")), &rules); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"**/.npmrc", "**/.yarnrc", "**/.yarnrc.yml", "**/node_modules/"} {
		if !contains(rules.ReviewPaths, path) {
			t.Errorf("the build rule doesn't ask about %s", path)
		}
	}
}

// A Classify job that failed or was cancelled as a whole, by a timeout or a lost runner, gave no
// verdict anyone can trust, and nothing else would act on the pull request: Classify's failure wakes
// neither the fixer nor retry. So land goes on once every job has finished and the four required
// checks have passed, and reads such a Classify as giving no verdict, which asks.
func TestLandAsksWhenClassifyEndedBadly(t *testing.T) {
	ready, _ := readFactoryWorkflow(t, "land.yml").Jobs["find"].step(t, "ready")
	head := strings.Repeat("ab", 20)
	required := []string{landJob("test (1.22)", "completed", "success"), landJob("test (stable)", "completed", "success"),
		landJob("Bug review", "completed", "success"), landJob("Security review", "completed", "success")}
	for conclusion, passed := range map[string]string{"success": "true", "failure": "false", "cancelled": "false", "timed_out": "false"} {
		r := newRehearsal(t)
		r.run(ready, map[string]string{"CI_RUN": "77"}, landRun("pull_request", "build/issue-120")+
			landJobs(append(append([]string{}, required...), landJob("Classify", "completed", conclusion))...)+landPulls("awml-factory[bot]", head))
		r.mustPass()
		if r.outputs["ready"] != "true" || r.outputs["classify_passed"] != passed {
			t.Errorf("with Classify %s, land read %v, want ready with classify_passed=%s", conclusion, r.outputs, passed)
		}
	}
	r := newRehearsal(t)
	r.run(ready, map[string]string{"CI_RUN": "77"}, landRun("pull_request", "build/issue-120")+
		landJobs(append(append([]string{}, required...), landJob("Classify", "completed", "skipped"))...)+landPulls("awml-factory[bot]", head))
	r.mustPass()
	if r.outputs["classify_passed"] != "false" {
		t.Errorf("with Classify skipped on a factory branch, land read %v, want no verdict", r.outputs)
	}

	const app = "41+awml-factory[bot]@users.noreply.github.com"
	judged := rehearseJudge(t, []struct{ author, message string }{{app, "Add scones"}}, "[]",
		`{"annotation_level": "notice", "title": "Verdict", "message": "ship"}`, "false")
	if judged.outputs["outcome"] != "ask" || !strings.Contains(judged.outputs["reasons"], "Classify gave no verdict") {
		t.Errorf("a Classify that didn't pass, with a ship notice, judged %s for %s, want ask", judged.outputs["outcome"], judged.outputs["reasons"])
	}
}

// On pull_request GitHub runs ci.yml from the merge with main, so a branch that hasn't touched
// ci.yml is checked by main's, however main's has changed since. Only a branch that changes ci.yml
// itself had checks other than main's, and the build rule asks about it, as it asks about any
// change under .github/. Land adds no rule of its own for it.
func TestLandAsksWhenTheBranchChangesItsCI(t *testing.T) {
	const app = "41+awml-factory[bot]@users.noreply.github.com"
	ship := `{"annotation_level": "notice", "title": "Verdict", "message": "ship"}`
	commits := []struct{ author, message string }{{app, "Add scones"}}
	ci := map[string]string{".github/workflows/ci.yml": "name: CI\n"}
	write := func(origin string) {
		if err := os.WriteFile(filepath.Join(origin, ".github/workflows/ci.yml"), []byte("name: CI\n# Another check\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	same := rehearseJudgeWith(t, ci, nil, nil, commits, "[]", ship)
	if same.outputs["outcome"] != "ship" {
		t.Errorf("a branch with main's ci.yml judged %s for %s, want ship", same.outputs["outcome"], same.outputs["reasons"])
	}
	moved := rehearseJudgeWith(t, ci, nil, func(origin string) {
		write(origin)
		gitIn(t, origin, "commit", "-qam", "Main's CI changes")
	}, commits, "[]", ship)
	if moved.outputs["outcome"] != "ship" {
		t.Errorf("a branch older than main's ci.yml, which CI runs from the merge, judged %s for %s, want ship", moved.outputs["outcome"], moved.outputs["reasons"])
	}
	changed := rehearseJudgeWith(t, ci, write, nil, commits, "[]", ship)
	if changed.outputs["outcome"] != "ask" || !strings.Contains(changed.outputs["reasons"], "`.github/workflows/ci.yml` is a path that always needs review.") {
		t.Errorf("a branch that changes ci.yml judged %s for %s, want ask", changed.outputs["outcome"], changed.outputs["reasons"])
	}
}

// strangersFirst puts a stranger's newer pull request from the same branch, at the same head, first
// in an answer's list of the branch's open pull requests, as anyone may open one from a branch here.
func strangersFirst(answers, branch, sha string) string {
	stranger := `{"number": 99, "base": {"ref": "main"}, "head": {"ref": "` + branch + `", "sha": "` + sha +
		`", "repo": {"full_name": "owner/repo"}}, "user": {"login": "a-stranger", "type": "User"}, "author_association": "NONE"}`
	at := strings.Index(answers, "&state=open")
	open := strings.Index(answers[at:], "<<< '[") + at + len("<<< '[")
	if strings.HasPrefix(answers[open:], "]") {
		return answers[:open] + stranger + answers[open:]
	}
	return answers[:open] + stranger + ", " + answers[open:]
}

// A stranger may open a pull request of their own from a factory branch. Land takes only the app's,
// so a stranger's newer one on the branch changes nothing.
func TestLandIgnoresAStrangersPullRequestOnTheBranch(t *testing.T) {
	ready, _ := readFactoryWorkflow(t, "land.yml").Jobs["find"].step(t, "ready")
	head := strings.Repeat("ab", 20)
	passed := landJobs(landJob("test (1.22)", "completed", "success"), landJob("test (stable)", "completed", "success"),
		landJob("Bug review", "completed", "success"), landJob("Security review", "completed", "success"),
		landJob("Classify", "completed", "success"))
	r := newRehearsal(t)
	r.run(ready, map[string]string{"CI_RUN": "77"}, landRun("pull_request", "build/issue-120")+passed+
		strangersFirst(landPulls("awml-factory[bot]", head), "build/issue-120", head))
	r.mustPass()
	if r.outputs["ready"] != "true" || r.outputs["pr"] != "8" {
		t.Errorf("with a stranger's newer pull request on the branch, land read %v, want the app's #8", r.outputs)
	}
}

func TestLandAddsNoRuleOfItsOwnForCI(t *testing.T) {
	if strings.Contains(readText(t, ".github/workflows/land.yml"), "-- .github/workflows/ci.yml") {
		t.Error("land.yml keeps a ci.yml check of its own beside the build rule's review_paths")
	}
}
