package awml

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The factory's workflows, which #160 rebuilt. release.yml publishes a tag and isn't one of them.
var factoryFiles = []string{"build.yml", "ci.yml", "fix.yml", "land.yml", "resolve.yml", "retry.yml"}

type workflowStep struct {
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	If              string            `yaml:"if"`
	Uses            string            `yaml:"uses"`
	Run             string            `yaml:"run"`
	With            map[string]string `yaml:"with"`
	Env             map[string]string `yaml:"env"`
	ContinueOnError bool              `yaml:"continue-on-error"`
}

type workflowJob struct {
	Name        string            `yaml:"name"`
	If          string            `yaml:"if"`
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Strategy    struct {
		Matrix map[string]any `yaml:"matrix"`
	} `yaml:"strategy"`
	Outputs map[string]string `yaml:"outputs"`
	Env     map[string]string `yaml:"env"`
	Steps   []workflowStep    `yaml:"steps"`
}

// step finds a job's step by its id, which stays put when the step's name changes.
func (j workflowJob) step(t *testing.T, id string) (workflowStep, int) {
	t.Helper()
	for i, step := range j.Steps {
		if step.ID == id {
			return step, i
		}
	}
	t.Fatalf("the job has no step with id %q", id)
	return workflowStep{}, -1
}

func (j workflowJob) needs() []string {
	switch needs := j.Needs.(type) {
	case string:
		return []string{needs}
	case []any:
		var all []string
		for _, need := range needs {
			all = append(all, fmt.Sprint(need))
		}
		return all
	}
	return nil
}

func (j workflowJob) runs() string {
	var all []string
	for _, step := range j.Steps {
		all = append(all, step.Run)
	}
	return strings.Join(all, "\n")
}

// session is the index of the job's step that starts Claude, or -1.
func (j workflowJob) session() int {
	for i, step := range j.Steps {
		if strings.Contains(step.Run, "claude -p") {
			return i
		}
	}
	return -1
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

// eachJob calls visit with every job of every factory workflow, in a fixed order.
func eachJob(t *testing.T, visit func(file, name string, job workflowJob)) {
	t.Helper()
	for _, file := range factoryFiles {
		jobs := readFactoryWorkflow(t, file).Jobs
		var names []string
		for name := range jobs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			visit(file, name, jobs[name])
		}
	}
}

func sortedStrings(values []string) []string {
	sorted := append([]string{}, values...)
	sort.Strings(sorted)
	return sorted
}

func holdsClaudesToken(job workflowJob) bool {
	for _, step := range job.Steps {
		if _, ok := step.Env["CLAUDE_CODE_OAUTH_TOKEN"]; ok {
			return true
		}
	}
	return false
}

func onlyReads(t *testing.T, workflow, name string, job workflowJob) {
	t.Helper()
	for permission, level := range job.Permissions {
		if level != "read" {
			t.Errorf("%s's %s has %s: %s, but should only read", workflow, name, permission, level)
		}
	}
}

// The factory is the six workflows #160 describes, and review.yml's jobs live in ci.yml now.
func TestTheFactoryIsTheWorkflowsOf160(t *testing.T) {
	files, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, path := range files {
		got = append(got, filepath.Base(path))
	}
	want := append(append([]string{}, factoryFiles...), "release.yml")
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the workflows are %v, want %v", got, want)
	}
}

// Each workflow starts from a GitHub event and reads what it needs from GitHub then. None runs on
// pull_request_target, which GitHub turns off on public repositories, and which runs main's copy with
// main's secrets for a stranger's pull request.
func TestEachWorkflowStartsFromItsEvent(t *testing.T) {
	want := map[string][]string{
		"build.yml":   {"issues", "workflow_dispatch"},
		"ci.yml":      {"pull_request", "push", "workflow_dispatch"},
		"fix.yml":     {"workflow_run"},
		"land.yml":    {"workflow_run"},
		"resolve.yml": {"push", "workflow_dispatch"},
		"retry.yml":   {"workflow_run"},
	}
	for _, file := range append(append([]string{}, factoryFiles...), "release.yml") {
		on := readFactoryWorkflow(t, file).On
		var events []string
		for event := range on {
			events = append(events, event)
		}
		sort.Strings(events)
		if _, ok := on["pull_request_target"]; ok {
			t.Errorf("%s runs on pull_request_target", file)
		}
		if wanted, ok := want[file]; ok && strings.Join(events, " ") != strings.Join(wanted, " ") {
			t.Errorf("%s starts on %v, want %v", file, events, wanted)
		}
	}

	ci := readFactoryWorkflow(t, "ci.yml").On
	pull, _ := ci["pull_request"].(map[string]any)
	push, _ := ci["push"].(map[string]any)
	if got := fmt.Sprint(pull["branches"]); got != "[main]" {
		t.Errorf("CI runs on pull requests into %s, want [main]", got)
	}
	if got := fmt.Sprint(push["branches"]); got != "[main]" {
		t.Errorf("CI runs on pushes to %s, want only [main], so each commit has one run", got)
	}
	for _, file := range []string{"fix.yml", "land.yml", "retry.yml"} {
		run, _ := readFactoryWorkflow(t, file).On["workflow_run"].(map[string]any)
		if fmt.Sprint(run["workflows"]) != "[CI]" || fmt.Sprint(run["types"]) != "[completed]" {
			t.Errorf("%s wakes on %v, want CI completed", file, run)
		}
		branches := fmt.Sprint(run["branches"])
		if file == "fix.yml" && branches != "<nil>" {
			t.Errorf("fix.yml wakes only on %s, but fixes any of its own people's pull requests", branches)
		}
		if file != "fix.yml" && branches != "[build/issue-*]" {
			t.Errorf("%s wakes on %s, want only factory branches", file, branches)
		}
	}
	dispatch, _ := readFactoryWorkflow(t, "build.yml").On["workflow_dispatch"].(map[string]any)
	inputs, _ := dispatch["inputs"].(map[string]any)
	if len(inputs) != 1 || inputs["issue"] == nil {
		t.Errorf("build.yml takes %v by dispatch, want only the issue's number", inputs)
	}
	resolve, _ := readFactoryWorkflow(t, "resolve.yml").On["push"].(map[string]any)
	if fmt.Sprint(resolve["branches"]) != "[main]" {
		t.Errorf("resolve.yml starts on pushes to %v, want main", resolve["branches"])
	}
}

// Nothing in the factory sets a pull request to merge by itself, so nothing set for an older commit
// can carry over to a newer one.
func TestNothingInTheFactoryUsesAutoMerge(t *testing.T) {
	for _, file := range factoryFiles {
		text := readText(t, ".github/workflows/"+file)
		for _, auto := range []string{"--auto", "disable-auto", "auto_merge", "AutoMerge", "Allow auto-merge"} {
			if strings.Contains(text, auto) {
				t.Errorf("%s uses auto-merge: %q", file, auto)
			}
		}
	}
}

// A command a script runs, at the start of a line or after ;, &&, || or a pipe.
var commandWord = regexp.MustCompile(`(?m)(?:^|[;&|(]\s*|\$\(\s*)(go|make|node|npx|npm)\s`)

// No job that holds Claude's login token runs anything outside Claude's sandbox: the runner has
// sudo, and its memory holds the job's secrets, so the change's code, its tests among them, runs
// only in Claude's sandbox, as Claude's own commands. Installing Claude Code is the one npm, and it
// runs nothing from the change.
func TestNoJobWithClaudesTokenRunsCodeOutsideClaude(t *testing.T) {
	jobs := 0
	eachJob(t, func(file, name string, job workflowJob) {
		if !holdsClaudesToken(job) {
			return
		}
		jobs++
		for _, step := range job.Steps {
			script := strings.ReplaceAll(step.Run, "npm install -g @anthropic-ai/claude-code", "")
			for _, found := range commandWord.FindAllStringSubmatch(script, -1) {
				t.Errorf("%s's %s step %q runs %s outside Claude's sandbox", file, name, step.Name, found[1])
			}
			if step.Uses != "" && !regexp.MustCompile(`^actions/(checkout|setup-go|setup-node|upload-artifact|download-artifact)@`).MatchString(step.Uses) {
				t.Errorf("%s's %s step %q uses %s beside the login token", file, name, step.Name, step.Uses)
			}
		}
	})
	if jobs != 5 {
		t.Errorf("found %d jobs that hold Claude's token, want the reviews, Classify, the draft, the fix and the rebase", jobs)
	}
}

var quotedRule = regexp.MustCompile(`"[A-Za-z]+\([^"]*\)"`)
var gitCommand = regexp.MustCompile(`(?:^|[\s;&|(])git\s`)

// Git after a session runs with no hooks and no fsmonitor, so nothing a session left in the tree
// runs: each git command carries both, or goes through a git function that does.
func TestGitAfterASessionRunsNoHooks(t *testing.T) {
	eachJob(t, func(file, name string, job workflowJob) {
		at := job.session()
		if at < 0 {
			return
		}
		for _, step := range job.Steps[at:] {
			script := quotedRule.ReplaceAllString(step.Run, "")
			wrapped := strings.Contains(script, "git() {")
			for _, line := range strings.Split(script, "\n") {
				if !gitCommand.MatchString(line) || wrapped {
					continue
				}
				if !strings.Contains(line, "-c core.hooksPath=/dev/null -c core.fsmonitor=false") {
					t.Errorf("%s's %s step %q runs git after the session with hooks on: %s", file, name, step.Name, strings.TrimSpace(line))
				}
			}
			if wrapped && !strings.Contains(script, "command git -c core.hooksPath=/dev/null -c core.fsmonitor=false") {
				t.Errorf("%s's %s step %q wraps git without turning hooks and fsmonitor off", file, name, step.Name)
			}
		}
	})
}

// Every Claude session holds the maintainer's login token, so Claude Code strips it from its
// commands, loads only the user's settings and no MCP servers from the tree, reads only the
// workspace and its own folders, never /proc, can't change .git, and follows main's prompt, kept
// out of the tree before it starts. Its log is shown with workflow commands stopped, so nothing it
// says can make an annotation or set an output.
func TestEverySessionIsSandboxedAndBounded(t *testing.T) {
	sessions := 0
	eachJob(t, func(file, name string, job workflowJob) {
		for _, step := range job.Steps {
			if !strings.Contains(step.Run, "claude -p") {
				continue
			}
			sessions++
			where := fmt.Sprintf("%s's %s step %q", file, name, step.Name)
			if step.Env["CLAUDE_CODE_SUBPROCESS_ENV_SCRUB"] != "1" {
				t.Errorf("%s gives Claude its token without scrubbing it from Claude's commands", where)
			}
			for _, want := range []string{"--setting-sources user ", "--strict-mcp-config", `"Read(//proc/**)"`,
				`"Edit(./.git/**)"`, `"Write(./.git/**)"`, `"Read(./**)"`, "::stop-commands::", `"$RUNNER_TEMP/`} {
				if !strings.Contains(step.Run, want) {
					t.Errorf("%s doesn't have %s", where, want)
				}
			}
			allowed := step.Run[strings.Index(step.Run, "--allowedTools"):]
			for _, bare := range []string{`"Read"`, `"Edit"`, `"Write"`} {
				if strings.Contains(allowed, bare) {
					t.Errorf("%s allows %s anywhere", where, bare)
				}
			}
			if strings.Contains(step.Run, "sed \"s|") && strings.Contains(step.Run, " .github/") {
				t.Errorf("%s reads its prompt from the tree, not main's copy", where)
			}
		}
	})
	if sessions != 5 {
		t.Errorf("found %d Claude sessions, want the reviews', Classify's, the draft's, the fix's and the rebase's", sessions)
	}
}

// Claude Code runs its commands in a sandbox, bubblewrap with socat for its network, so that it can
// scrub its login token from them, and they can't start without both. So every job that installs
// Claude Code installs both with it and checks that bubblewrap works.
func TestEveryJobThatRunsClaudeHasBubblewrap(t *testing.T) {
	eachJob(t, func(file, name string, job workflowJob) {
		if job.session() < 0 {
			return
		}
		installs := false
		for _, step := range job.Steps {
			if !strings.Contains(step.Run, "npm install -g @anthropic-ai/claude-code") {
				continue
			}
			installs = true
			if !strings.Contains(step.Run, "apt-get install -y bubblewrap socat") || !strings.Contains(step.Run, "bwrap --") {
				t.Errorf("%s's %s installs Claude Code without installing bubblewrap and socat and checking bubblewrap", file, name)
			}
		}
		if !installs {
			t.Errorf("%s's %s runs Claude without installing it beside bubblewrap", file, name)
		}
	})
}

// The token check every workflow uses on what a session wrote.
const tokenCheck = `looks_like_token() {
            LC_ALL=C sed -E 's#base64,[A-Za-z0-9+/]+=*##g; s#data-points="[A-Za-z0-9+/]+=*"##g' |
              LC_ALL=C grep -aoiE 'sk-ant-|[A-Za-z0-9_-]{80,}' | grep -iE 'sk-ant-|[0-9]' > /dev/null
          }`

// Everything a session wrote is scanned for anything like the login token before it's uploaded,
// pushed or posted: each job that runs Claude has the one check, after the session and before any
// upload.
func TestEverythingASessionWroteIsScannedForAToken(t *testing.T) {
	eachJob(t, func(file, name string, job workflowJob) {
		at := job.session()
		if at < 0 {
			return
		}
		scanned := -1
		for i, step := range job.Steps {
			if strings.Contains(step.Run, "looks_like_token") && i >= at {
				scanned = i
				if !strings.Contains(strings.Join(strings.Fields(step.Run), " "), strings.Join(strings.Fields(tokenCheck), " ")) {
					t.Errorf("%s's %s step %q checks for a token its own way", file, name, step.Name)
				}
				break
			}
		}
		if scanned < 0 {
			t.Errorf("%s's %s doesn't scan what its session wrote for a token", file, name)
		}
		for i, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/upload-artifact") && i < scanned {
				t.Errorf("%s's %s uploads before it scans for a token", file, name)
			}
		}
	})
}

// The check skips the drawings' base64 and their data-points, and a long run counts only if it holds
// a digit, since a long name with none, such as a test's, is no token.
func TestTheTokenCheckSkipsDrawingsAndNames(t *testing.T) {
	script := tokenCheck + "\nif looks_like_token < \"$FILE\"; then echo found; else echo clean; fi\n"
	for text, want := range map[string]string{
		"no token here":                                  "clean",
		"sk-ant-oat01-abc":                               "found",
		strings.Repeat("aB3_", 25):                       "found",
		strings.Repeat("abcd", 25):                       "clean",
		"base64," + strings.Repeat("A1", 60):             "clean",
		`data-points="` + strings.Repeat("A1", 60) + `"`: "clean",
	} {
		r := newRehearsal(t)
		r.run(workflowStep{Run: script}, map[string]string{"FILE": r.write("text", text)}, "")
		r.mustPass()
		if strings.TrimSpace(r.printed) != want {
			t.Errorf("the token check says %q for %.40q, want %s", strings.TrimSpace(r.printed), text, want)
		}
	}
}

// Artifacts stay within a run: none is read from another run, and a job reads one only after checking
// it has the sha256 the job that made it gave as an output. A draft or fix that may be offered for a
// person is kept 30 days.
func TestArtifactsStayWithinARunAndAreChecked(t *testing.T) {
	eachJob(t, func(file, name string, job workflowJob) {
		for at, step := range job.Steps {
			for _, line := range strings.Split(step.Run, "\n") {
				if strings.Contains(line, "gh run download") && !strings.Contains(line, "printf") {
					t.Errorf("%s's %s step %q downloads another run's artifact: %s", file, name, step.Name, strings.TrimSpace(line))
				}
			}
			if strings.HasPrefix(step.Uses, "actions/upload-artifact") {
				offered := step.With["name"] == "change" || step.With["name"] == "fix"
				if offered && step.With["retention-days"] != "30" {
					t.Errorf("%s's %s keeps the %s artifact for %q days, want 30", file, name, step.With["name"], step.With["retention-days"])
				}
			}
			if !strings.HasPrefix(step.Uses, "actions/download-artifact") {
				continue
			}
			if step.With["run-id"] != "" {
				t.Errorf("%s's %s downloads an artifact from another run", file, name)
			}
			checked := false
			for _, later := range job.Steps[at+1:] {
				for _, value := range later.Env {
					if strings.Contains(value, "needs.") && strings.Contains(value, ".outputs.digest") && strings.Contains(later.Run, "sha256sum") {
						checked = true
					}
				}
			}
			if !checked {
				t.Errorf("%s's %s uses the %s artifact without checking the digest its maker gave", file, name, step.With["name"])
			}
		}
	})
}

// Each job that makes an app token makes it for this repository alone.
func TestAppTokensAreScopedToThisRepository(t *testing.T) {
	tokens := 0
	eachJob(t, func(file, name string, job workflowJob) {
		for _, step := range job.Steps {
			if !strings.HasPrefix(step.Uses, "actions/create-github-app-token") {
				continue
			}
			tokens++
			if step.With["owner"] != "${{ github.repository_owner }}" || step.With["repositories"] != "${{ github.event.repository.name }}" {
				t.Errorf("%s's %s makes an app token for more than this repository", file, name)
			}
		}
	})
	if tokens != 5 {
		t.Errorf("found %d app tokens, want CI's report, the build's, land's, the fixer's and the rebase's", tokens)
	}
}

// Wherever a pull request's code runs, Go's module cache is neither saved nor restored, so it can't
// poison a later run on main.
func TestTheGoCacheIsNeverShared(t *testing.T) {
	eachJob(t, func(file, name string, job workflowJob) {
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/setup-go") && step.With["cache"] != "false" {
				t.Errorf("%s's %s sets up Go with its cache on", file, name)
			}
		}
	})
}

// No step expands an expression in its script. Event, issue, log, Claude and artifact text reaches a
// script only through env:, quoted, so it can't run as code.
func TestNoScriptExpandsAnExpression(t *testing.T) {
	eachJob(t, func(file, name string, job workflowJob) {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "${{") {
				t.Errorf("%s's %s step %q expands an expression in its script", file, name, step.Name)
			}
		}
	})
}

// Comments are for people: each is a new one, never edited, and carries no marker for a workflow to
// parse.
func TestNoCommentIsEditedOrMarked(t *testing.T) {
	for _, file := range append(append([]string{}, factoryFiles...), "../pull-request-body.sh") {
		text := readText(t, ".github/workflows/"+file)
		for _, never := range []string{"<!--", "-X PATCH", "issues/comments/", "--edit-last"} {
			if strings.Contains(text, never) {
				t.Errorf("%s has %q, but comments are new ones with no markers", file, never)
			}
		}
	}
}

// Main's prompt, AGENTS.md and REVIEW.md are copied out of the tree before any session starts, so a
// change can't rewrite what it's held to.
func TestSessionsFollowMainsPromptAndAgents(t *testing.T) {
	for _, prompt := range []string{"build-prompt.md", "fix-prompt.md", "resolve-prompt.md", "review-bugs-prompt.md",
		"review-security-prompt.md", "classify-prompt.md"} {
		text := readText(t, ".github/"+prompt)
		if !regexp.MustCompile(`[A-Z]+_DIR/AGENTS\.md`).MatchString(text) {
			t.Errorf("%s doesn't point the session at main's copy of AGENTS.md", prompt)
		}
	}
	eachJob(t, func(file, name string, job workflowJob) {
		at := job.session()
		if at < 0 {
			return
		}
		copied := false
		for _, step := range job.Steps[:at] {
			if regexp.MustCompile(`cp [^\n]*AGENTS\.md`).MatchString(step.Run) || strings.Contains(step.Run, "bring-patch.sh") {
				copied = true
			}
		}
		if !copied {
			t.Errorf("%s's %s doesn't copy main's AGENTS.md out before its session", file, name)
		}
	})
	if !strings.Contains(readText(t, ".github/bring-patch.sh"), "cp REVIEW.md AGENTS.md") {
		t.Error("bring-patch.sh doesn't keep main's REVIEW.md and AGENTS.md")
	}
}

// A rehearsal runs one step's script as GitHub runs it, with bash -eo pipefail, in a scratch
// RUNNER_TEMP, with gh replaced by a stub that logs each call, keeps what each call sends, and
// answers from the case arms a test gives it. Nothing reaches GitHub.
type rehearsal struct {
	t       *testing.T
	root    string
	temp    string
	dir     string
	outputs map[string]string
	calls   string
	inputs  []map[string]any
	printed string
	failed  bool
}

const ghStub = `#!/bin/bash
# Stands in for gh in a rehearsal.
call="$*"
printf '%%s\n' "$call" >> "$GH_CALLS"
query=""
while [ $# -gt 0 ]; do
  case "$1" in
    --jq) query="$2"; shift ;;
    --body-file) sed 's/^/> /' "$2" >> "$GH_CALLS"; cp "$2" "$GH_BODIES/$(ls "$GH_BODIES" | wc -l)"; shift ;;
    --input) [ "$2" != - ] || jq -c . >> "$GH_INPUTS"; shift ;;
  esac
  shift
done
answer() { if [ -n "$query" ]; then jq -r "$query"; else cat; fi; }
case "$call" in
%s
  *) echo "the stub gh has no answer for: $call" >&2; exit 1 ;;
esac
`

func newRehearsal(t *testing.T) *rehearsal {
	t.Helper()
	for _, tool := range []string{"bash", "jq", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s isn't installed", tool)
		}
	}
	root := t.TempDir()
	r := &rehearsal{t: t, root: root, temp: filepath.Join(root, "runner")}
	for _, dir := range []string{r.temp, filepath.Join(root, "bodies")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// write puts a file under the rehearsal's RUNNER_TEMP, or at an absolute path.
func (r *rehearsal) write(path, text string) string {
	r.t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.temp, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		r.t.Fatal(err)
	}
	return path
}

func (r *rehearsal) read(path string) string {
	r.t.Helper()
	data, err := os.ReadFile(filepath.Join(r.temp, path))
	if err != nil {
		return ""
	}
	return string(data)
}

// bodies are the files given to gh's --body-file, in the order of the calls.
func (r *rehearsal) bodies() []string {
	r.t.Helper()
	entries, _ := os.ReadDir(filepath.Join(r.root, "bodies"))
	bodies := make([]string, len(entries))
	for _, entry := range entries {
		var at int
		fmt.Sscan(entry.Name(), &at)
		data, _ := os.ReadFile(filepath.Join(r.root, "bodies", entry.Name()))
		if at < len(bodies) {
			bodies[at] = string(data)
		}
	}
	return bodies
}

func (r *rehearsal) run(step workflowStep, env map[string]string, answers string) {
	r.t.Helper()
	bin := filepath.Join(r.root, "bin")
	r.write(filepath.Join(bin, "gh"), fmt.Sprintf(ghStub, answers))
	if err := os.Chmod(filepath.Join(bin, "gh"), 0o755); err != nil {
		r.t.Fatal(err)
	}
	script := r.write(filepath.Join(r.root, "step.sh"), step.Run)
	vars := map[string]string{
		"PATH":                    bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME":                    r.root,
		"RUNNER_TEMP":             r.temp,
		"GITHUB_OUTPUT":           filepath.Join(r.root, "output"),
		"GITHUB_STEP_SUMMARY":     filepath.Join(r.root, "summary"),
		"GITHUB_REPOSITORY":       "owner/repo",
		"GITHUB_REPOSITORY_OWNER": "owner",
		"GITHUB_SERVER_URL":       "https://github.com",
		"GITHUB_RUN_ID":           "1000",
		"GITHUB_SHA":              strings.Repeat("0", 40),
		"GH_TOKEN":                "not-a-token",
		"GH_CALLS":                filepath.Join(r.root, "calls"),
		"GH_INPUTS":               filepath.Join(r.root, "inputs"),
		"GH_BODIES":               filepath.Join(r.root, "bodies"),
		"GIT_CONFIG_GLOBAL":       "/dev/null",
		"GIT_CONFIG_NOSYSTEM":     "1",
	}
	for name, value := range step.Env {
		if !strings.Contains(value, "${{") {
			vars[name] = value
		}
	}
	for name, value := range env {
		vars[name] = value
	}
	command := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
	command.Dir = r.root
	if r.dir != "" {
		command.Dir = r.dir
	}
	for name, value := range vars {
		command.Env = append(command.Env, name+"="+value)
	}
	printed, err := command.CombinedOutput()
	r.printed = string(printed)
	r.failed = err != nil
	r.outputs = map[string]string{}
	output, _ := os.ReadFile(vars["GITHUB_OUTPUT"])
	for _, line := range strings.Split(string(output), "\n") {
		if name, value, ok := strings.Cut(line, "="); ok {
			r.outputs[name] = value
		}
	}
	calls, _ := os.ReadFile(vars["GH_CALLS"])
	r.calls = string(calls)
	inputs, _ := os.ReadFile(vars["GH_INPUTS"])
	r.inputs = nil
	for _, line := range strings.Split(strings.TrimSpace(string(inputs)), "\n") {
		if line == "" {
			continue
		}
		var input map[string]any
		if err := json.Unmarshal([]byte(line), &input); err != nil {
			r.t.Fatal(err)
		}
		r.inputs = append(r.inputs, input)
	}
}

func (r *rehearsal) mustPass() {
	r.t.Helper()
	if r.failed {
		r.t.Fatalf("the step failed:\n%s\ngh was called with:\n%s", r.printed, r.calls)
	}
}

func (r *rehearsal) summary() string {
	data, _ := os.ReadFile(filepath.Join(r.root, "summary"))
	return string(data)
}

// gitCommandIn is git in a directory, as the bakery's baker, with no configuration of the machine's.
func gitCommandIn(dir string, args ...string) *exec.Cmd {
	command := exec.Command("git", append([]string{"-c", "user.name=A baker", "-c", "user.email=baker@example.com",
		"-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false"}, args...)...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	return command
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitCommandIn(dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo makes a git repository of the bakery's under the rehearsal, with the files given as its
// first commit on main, and returns its path and that commit.
func (r *rehearsal) newRepo(name string, files map[string]string) (string, string) {
	r.t.Helper()
	repo := filepath.Join(r.root, name)
	for file, text := range files {
		r.write(filepath.Join(repo, file), text)
	}
	gitIn(r.t, repo, "init", "-q")
	gitIn(r.t, repo, "add", "-A")
	gitIn(r.t, repo, "commit", "-q", "-m", "The bakery's main")
	return repo, gitIn(r.t, repo, "rev-parse", "HEAD")
}

var sessionHome = regexp.MustCompile(`HOME="([^"]+)" claude -p`)
var sandboxSettings = regexp.MustCompile(`bash "\$RUNNER_TEMP/sandbox-settings\.sh" "([^"]+)" "([^"]+)" (builds|reads)\n`)

// Every Claude session runs its commands in Claude Code's own sandbox, set in the user settings of
// a home of its own, the only settings --setting-sources user loads (#131). The drafting sessions,
// the draft, the fix and the rebase, may reach only the Go module proxy, its checksum database and
// npm's registry; the review and Classify sessions run no commands and reach nothing. Main's script
// writes the settings, copied out of the tree before the session starts. Under
// CLAUDE_CODE_SUBPROCESS_ENV_SCRUB the permission mode is always default, so each session says so.
func TestEverySessionRunsInItsOwnSandbox(t *testing.T) {
	kinds := map[string]string{"ci.yml review": "reads", "ci.yml classify": "reads", "build.yml draft": "builds",
		"fix.yml fix": "builds", "resolve.yml draft": "builds"}
	sessions := 0
	eachJob(t, func(file, name string, job workflowJob) {
		at := job.session()
		if at < 0 {
			return
		}
		sessions++
		step := job.Steps[at]
		where := fmt.Sprintf("%s's %s step %q", file, name, step.Name)
		home := sessionHome.FindStringSubmatch(step.Run)
		settings := sandboxSettings.FindStringSubmatch(step.Run)
		if home == nil || settings == nil {
			t.Errorf("%s doesn't start Claude in a home of its own with main's sandbox settings", where)
			return
		}
		if settings[1] != home[1] {
			t.Errorf("%s writes the sandbox settings into %s but starts Claude in %s", where, settings[1], home[1])
		}
		if strings.Index(step.Run, settings[0]) > strings.Index(step.Run, home[0]) {
			t.Errorf("%s writes the sandbox settings after Claude starts", where)
		}
		if want := kinds[file+" "+name]; settings[3] != want {
			t.Errorf("%s gives its session the %s sandbox, want %s", where, settings[3], want)
		}
		if !strings.Contains(step.Run, "--permission-mode default") || strings.Contains(step.Run, "acceptEdits") {
			t.Errorf("%s doesn't say its permission mode is default, which the scrub forces", where)
		}
		copied := false
		for _, before := range job.Steps[:at+1] {
			if regexp.MustCompile(`cp [^\n]*\.github/sandbox-settings\.sh[^\n]* "\$RUNNER_TEMP/"\n`).MatchString(before.Run) {
				copied = true
			}
		}
		if !copied {
			t.Errorf("%s's %s doesn't copy main's sandbox-settings.sh out before its session", file, name)
		}
	})
	if sessions != len(kinds) {
		t.Errorf("found %d sessions, want %d", sessions, len(kinds))
	}
}

// The settings turn the sandbox on, fail rather than run a command without it, refuse every
// unsandboxed retry, deny writes to .git and to the runner's own files, and allow only the hosts a
// session's commands need, refusing every other without a prompt.
func TestTheSandboxSettingsAreExplicit(t *testing.T) {
	for kind, domains := range map[string]string{"builds": `["proxy.golang.org","sum.golang.org","registry.npmjs.org"]`, "reads": `[]`} {
		r := newRehearsal(t)
		r.write("sandbox-settings.sh", readText(t, ".github/sandbox-settings.sh"))
		tree := filepath.Join(r.root, "tree")
		workspace := filepath.Join(r.root, "workspace")
		for _, dir := range []string{tree + "/.git", workspace + "/.git", r.root + "/runners/2.0/bin"} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		home := filepath.Join(r.temp, "session", "home")
		r.run(workflowStep{Run: `bash "$RUNNER_TEMP/sandbox-settings.sh" "$SESSION" "$TREE" ` + kind + "\n"},
			map[string]string{"SESSION": home, "TREE": tree, "GITHUB_WORKSPACE": workspace}, "")
		r.mustPass()
		var settings struct {
			Sandbox struct {
				Enabled                  *bool `json:"enabled"`
				FailIfUnavailable        *bool `json:"failIfUnavailable"`
				AllowUnsandboxedCommands *bool `json:"allowUnsandboxedCommands"`
				Filesystem               struct {
					DenyWrite []string `json:"denyWrite"`
					DenyRead  []string `json:"denyRead"`
				} `json:"filesystem"`
				Network struct {
					AllowedDomains  []string `json:"allowedDomains"`
					StrictAllowlist *bool    `json:"strictAllowlist"`
				} `json:"network"`
			} `json:"sandbox"`
		}
		data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
		if err != nil {
			t.Fatalf("%s: no settings file in the session's home: %v", kind, err)
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}
		box := settings.Sandbox
		if box.Enabled == nil || !*box.Enabled || box.FailIfUnavailable == nil || !*box.FailIfUnavailable ||
			box.AllowUnsandboxedCommands == nil || *box.AllowUnsandboxedCommands || box.Network.StrictAllowlist == nil || !*box.Network.StrictAllowlist {
			t.Errorf("%s: the sandbox isn't on, strict and required:\n%s", kind, data)
		}
		if got, _ := json.Marshal(box.Network.AllowedDomains); string(got) != domains {
			t.Errorf("%s: the sandbox allows %s, want %s", kind, got, domains)
		}
		for _, denied := range []string{tree + "/.git", workspace + "/.git", r.root + "/runners"} {
			if !contains(box.Filesystem.DenyWrite, denied) {
				t.Errorf("%s: the sandbox lets commands write %s: %v", kind, denied, box.Filesystem.DenyWrite)
			}
		}
		if !contains(box.Filesystem.DenyRead, r.root+"/runners") {
			t.Errorf("%s: the sandbox lets commands read the runner's own files: %v", kind, box.Filesystem.DenyRead)
		}
	}
}

// GitHub runs any line a step prints that starts with ::, so a step that prints text from a patch, a
// branch or a session, a file's name included, prints it inside ::stop-commands::. Such a step is one
// that brings in a patch, rebases a branch, starts Claude, or lists a change's files; and a list of
// names is printed JSON-escaped, so even the end of the stop can't be forged.
func TestUntrustedTextIsPrintedWithWorkflowCommandsStopped(t *testing.T) {
	prints := regexp.MustCompile(`bring-patch\.sh"|\brebase\b|claude -p|--name-only`)
	steps := 0
	eachJob(t, func(file, name string, job workflowJob) {
		for _, step := range job.Steps {
			script := quotedRule.ReplaceAllString(step.Run, "")
			for _, line := range strings.Split(script, "\n") {
				if strings.Contains(line, `tr '\0' '\n'`) && !strings.Contains(line, "jq -R") && !strings.Contains(line, "> ") {
					t.Errorf("%s's %s step %q prints a list of names raw: %s", file, name, step.Name, strings.TrimSpace(line))
				}
			}
			at := prints.FindStringIndex(script)
			if at == nil {
				continue
			}
			steps++
			stop := strings.Index(script, `echo "::stop-commands::$stop"`)
			if stop < 0 || stop > at[0] {
				t.Errorf("%s's %s step %q prints untrusted text before it stops workflow commands", file, name, step.Name)
			}
		}
	})
	if steps < 8 {
		t.Errorf("found %d steps that print untrusted text, fewer than the factory has", steps)
	}
}
