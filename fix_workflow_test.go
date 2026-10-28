package awml

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixAnswers(tests, review, pullUser, pullSha string, fixCommits int, annotations string) string {
	head := strings.Repeat("ab", 20)
	pulls := "[]"
	if pullUser != "" {
		association := "OWNER"
		pulls = `[{"number": 8, "base": {"ref": "main"}, "head": {"ref": "scones", "sha": "` + pullSha +
			`", "repo": {"full_name": "owner/repo"}}, "user": {"login": "` + pullUser + `", "type": "` + map[bool]string{true: "Bot", false: "User"}[strings.HasSuffix(pullUser, "[bot]")] + `"}, "author_association": "` + association + `"}]`
	}
	commits := []string{`{"sha": "1", "commit": {"message": "Add scones"}}`}
	for i := 0; i < fixCommits; i++ {
		commits = append(commits, `{"sha": "2", "commit": {"message": "Fix\n\nFixed-by: fix-review-findings\nFixed-by: fix-review-findings"}}`)
	}
	return permissions + `
  "api repos/owner/repo/actions/runs/77"*) answer <<< '{"path": ".github/workflows/ci.yml", "event": "pull_request", "status": "completed",
    "head_sha": "` + head + `", "head_branch": "scones", "head_repository": {"full_name": "owner/repo"}, "pull_requests": [{"number": 8}],
    "triggering_actor": {"login": "the-maintainer", "type": "User"}}' ;;
  "api --paginate repos/owner/repo/actions/runs/77/jobs"*) answer <<< '{"jobs": [
    {"name": "test (1.22)", "conclusion": "` + tests + `"}, {"name": "test (stable)", "conclusion": "success"},
    {"name": "Bug review", "conclusion": "` + review + `", "check_run_url": "https://api.github.com/repos/owner/repo/check-runs/2"},
    {"name": "Security review", "conclusion": "success", "check_run_url": "https://api.github.com/repos/owner/repo/check-runs/3"}]}' ;;
  "api repos/owner/repo/pulls?head=owner:scones&base=main&state=open"*) answer <<< '` + pulls + `' ;;
  "api --paginate repos/owner/repo/pulls/8/commits"*) answer <<< '[` + strings.Join(commits, ", ") + `]' ;;
  "api --paginate repos/owner/repo/check-runs/2/annotations"*) answer <<< '` + annotations + `' ;;
  "api --paginate repos/owner/repo/check-runs/3/annotations"*) answer <<< '[]' ;;`
}

// The fixer acts only on a pull request CI reviews, still open at the reviewed commit, when both
// test jobs passed and a review failed with an Important finding: a failed test is retry-a-build's.
// It counts its tries from the fix commits on the branch, and gives up at two. Someone's push since
// the review means it exits quietly.
func TestFixActsOnlyWhenTheTestsPassedAndAReviewFailed(t *testing.T) {
	fix := readFactoryWorkflow(t, "fix.yml").Jobs["fix"]
	onlyReads(t, "fix.yml", "fix", fix)
	tries, _ := fix.step(t, "tries")
	head := strings.Repeat("ab", 20)
	important := `[{"annotation_level": "failure", "title": "Important: A wrong answer", "path": "fit.go", "start_line": 42, "message": "It drops a part."}]`
	tests := []struct {
		name, answers, decision, reason string
	}{
		{"a failed review", fixAnswers("success", "failure", "the-maintainer", head, 0, important), "fix", ""},
		{"a failed review after one fix", fixAnswers("success", "failure", "awml-factory[bot]", head, 1, important), "fix", ""},
		{"a failed test", fixAnswers("failure", "failure", "the-maintainer", head, 0, important), "none", ""},
		{"no failed review", fixAnswers("success", "success", "the-maintainer", head, 0, important), "none", ""},
		{"a moved head", fixAnswers("success", "failure", "the-maintainer", strings.Repeat("cd", 20), 0, important), "none", ""},
		{"a stranger's pull request", fixAnswers("success", "failure", "a-stranger", head, 0, important), "none", ""},
		{"no open pull request", fixAnswers("success", "failure", "", head, 0, important), "none", ""},
		{"a run a stranger started", strings.Replace(fixAnswers("success", "failure", "the-maintainer", head, 0, important),
			`"triggering_actor": {"login": "the-maintainer", "type": "User"}`, `"triggering_actor": {"login": "a-stranger", "type": "User"}`, 1), "none", ""},
		{"a run the app started", strings.Replace(fixAnswers("success", "failure", "the-maintainer", head, 0, important),
			`"triggering_actor": {"login": "the-maintainer", "type": "User"}`, `"triggering_actor": {"login": "awml-factory[bot]", "type": "Bot"}`, 1), "fix", ""},
		{"a run for a stranger's pull request from the branch", strings.Replace(fixAnswers("success", "failure", "the-maintainer", head, 0, important),
			`"pull_requests": [{"number": 8}]`, `"pull_requests": [{"number": 99}]`, 1), "none", ""},
		{"a stranger's newer pull request", strangersFirst(fixAnswers("success", "failure", "the-maintainer", head, 0, important), "scones", head), "fix", ""},
		{"a collaborator's newer pull request from a fork", strings.Replace(strings.Replace(
			strangersFirst(fixAnswers("success", "failure", "the-maintainer", head, 0, important), "scones", head),
			`"repo": {"full_name": "owner/repo"}}, "user": {"login": "a-stranger", "type": "User"}, "author_association": "NONE"`,
			`"repo": {"full_name": "a-collaborator/repo"}}, "user": {"login": "a-collaborator", "type": "User"}, "author_association": "COLLABORATOR"`, 1),
			`"number": 99`, `"number": 99, "fork": true`, 1), "fix", ""},
		{"a fork's run", fromAFork(fixAnswers("success", "failure", "the-maintainer", head, 0, important)), "none", ""},
		{"two fix commits", fixAnswers("success", "failure", "the-maintainer", head, 2, important), "giveup", "tried"},
		{"no Important finding", fixAnswers("success", "failure", "the-maintainer", head, 0, "[]"), "giveup", "unread"},
	}
	for _, tt := range tests {
		r := newRehearsal(t)
		r.write("read-findings.sh", readText(t, ".github/read-findings.sh"))
		r.run(tries, map[string]string{"CI_RUN": "77"}, tt.answers)
		r.mustPass()
		if r.outputs["decision"] != tt.decision || r.outputs["reason"] != tt.reason {
			t.Errorf("%s: the fixer decided %q, %q, want %q, %q\n%s", tt.name, r.outputs["decision"], r.outputs["reason"], tt.decision, tt.reason, r.printed)
		}
		if tt.decision == "fix" && (r.outputs["pr"] != "8" || r.outputs["branch"] != "scones" || r.outputs["head_sha"] != head) {
			t.Errorf("%s: the fixer passed on %v", tt.name, r.outputs)
		}
	}
}

// The fix commit's message carries the line the fixer counts its tries by and the build rule counts
// fix commits by, and the push names the reviewed commit with --force-with-lease, so GitHub refuses
// it if anyone pushed since.
func TestTheFixCommitCarriesTheLineItsTriesAreCountedBy(t *testing.T) {
	write := readFactoryWorkflow(t, "fix.yml").Jobs["write"]
	push, _ := write.step(t, "push")
	tries, _ := readFactoryWorkflow(t, "fix.yml").Jobs["fix"].step(t, "tries")
	const line = "Fixed-by: fix-review-findings"
	if !strings.Contains(readText(t, "internal/buildrule/main.go"), `const fixLine = "`+line+`"`) {
		t.Errorf("the build rule doesn't count fix commits by %q", line)
	}
	for name, script := range map[string]string{"push": push.Run, "tries": tries.Run} {
		if !strings.Contains(script, line) {
			t.Errorf("the fixer's %s step doesn't use %q", name, line)
		}
	}
	if !strings.Contains(push.Run, `--force-with-lease="refs/heads/$BRANCH:$HEAD_SHA"`) {
		t.Error("the fix isn't pushed with --force-with-lease at the reviewed commit")
	}
	if len(write.Permissions) != 2 || write.Permissions["contents"] != "read" || write.Permissions["pull-requests"] != "write" {
		t.Errorf("the write job has %v, want contents: read and pull-requests: write", write.Permissions)
	}
}

// fixGuard rehearses the write job's guard on a bakery's branch, at the reviewed commit, with a fix
// made by change and the digest the fix job gave.
func fixGuard(t *testing.T, env map[string]string, change func(r *rehearsal, repo string), moved bool) *rehearsal {
	t.Helper()
	guard, _ := readFactoryWorkflow(t, "fix.yml").Jobs["write"].step(t, "guard")
	r := newRehearsal(t)
	origin, _ := r.newRepo("origin", map[string]string{"recipes.md": "Bread\n", ".github/workflows/ci.yml": "name: CI\n"})
	gitIn(t, origin, "switch", "-q", "-c", "scones")
	head := gitIn(t, origin, "rev-parse", "HEAD")
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "scones", origin, work)
	change(r, work)
	gitIn(t, work, "add", "-A")
	r.write("fix/fix.patch", gitIn(t, work, "diff", "--cached", "--binary")+"\n")
	r.write("fix/message.md", "Fix it\n")
	gitIn(t, work, "reset", "-q", "--hard")
	if moved {
		r.write(filepath.Join(origin, "recipes.md"), "Rolls\n")
		gitIn(t, origin, "commit", "-qam", "Someone pushed")
	}
	digest := newRehearsal(t)
	digest.run(workflowStep{Run: `digest() { (cd "$1" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum) | sha256sum | cut -c 1-64; }
digest "$DIR"`}, map[string]string{"DIR": filepath.Join(r.temp, "fix")}, "")
	vars := map[string]string{"DECISION": "fix", "FIXED": "success", "PR_NUMBER": "8", "BRANCH": "scones", "HEAD_SHA": head,
		"CHANGE_DIGEST": strings.TrimSpace(digest.printed), "STOPPED": ""}
	for k, v := range env {
		vars[k] = v
	}
	r.dir = work
	r.run(guard, vars, "")
	return r
}

// A fix to .github/ or Claude Code's own files is offered, not pushed. A branch someone pushed to
// since the review is left quietly. A fix that isn't the one the fix job made is refused.
func TestTheFixIsPushedOfferedOrLeft(t *testing.T) {
	plain := func(r *rehearsal, repo string) { r.write(filepath.Join(repo, "recipes.md"), "Bread\nScones\n") }
	pushed := fixGuard(t, nil, plain, false)
	pushed.mustPass()
	if pushed.outputs["outcome"] != "push" {
		t.Errorf("a plain fix ended %q, want push:\n%s", pushed.outputs["outcome"], pushed.printed)
	}
	for _, path := range []string{".github/workflows/ci.yml", "CLAUDE.md", ".claude/settings.json", "docs/.mcp.json",
		".npmrc", ".yarnrc", "node_modules/x/index.js", "a symbolic link"} {
		r := fixGuard(t, nil, func(r *rehearsal, repo string) {
			if path == "a symbolic link" {
				if err := os.Symlink("/proc/self/environ", filepath.Join(repo, "notes.md")); err != nil {
					t.Fatal(err)
				}
				return
			}
			r.write(filepath.Join(repo, path), "changed\n")
		}, false)
		r.mustPass()
		if r.outputs["outcome"] != "offer" {
			t.Errorf("a fix to %s ended %q, want offer", path, r.outputs["outcome"])
		}
	}
	moved := fixGuard(t, nil, plain, true)
	moved.mustPass()
	if moved.outputs["outcome"] != "moved" {
		t.Errorf("a fix to a branch someone pushed to ended %q, want moved", moved.outputs["outcome"])
	}
	forged := fixGuard(t, map[string]string{"CHANGE_DIGEST": strings.Repeat("0", 64)}, plain, false)
	if !forged.failed || forged.outputs["outcome"] != "" {
		t.Errorf("a fix that isn't the fix job's ended %q", forged.outputs["outcome"])
	}
	for env, reason := range map[string]string{"tried": "tried", "unread": "unread", "nothing": "nothing", "made up": "stopped"} {
		r := fixGuard(t, map[string]string{"DECISION": "giveup", "STOPPED": env}, plain, false)
		r.mustPass()
		if r.outputs["outcome"] != "giveup" || r.outputs["reason"] != reason {
			t.Errorf("a fix that stopped for %q ended %q, %q", env, r.outputs["outcome"], r.outputs["reason"])
		}
	}
	session := fixGuard(t, map[string]string{"FIXED": "failure", "SESSION": "failure"}, plain, false)
	session.mustPass()
	if session.outputs["reason"] != "session" {
		t.Errorf("a failed session ended %q", session.outputs["reason"])
	}
	// A step after the session that refused the fix, such as the token scan or the check of .git,
	// isn't the session failing.
	after := fixGuard(t, map[string]string{"FIXED": "failure", "SESSION": "success"}, plain, false)
	after.mustPass()
	if after.outputs["reason"] != "unkept" {
		t.Errorf("a fix job that failed after its session ended %q, want unkept", after.outputs["reason"])
	}
	giveup, _ := readFactoryWorkflow(t, "fix.yml").Jobs["write"].step(t, "giveup")
	if !strings.Contains(giveup.Run, `unkept) why="The fix job failed before the fix was kept. The run says why."`) {
		t.Error("giving up doesn't say a fix that wasn't kept failed after the session")
	}
}

// The offer and giving up are new comments from fixed text, quoting none of the fix: the offer
// gives the commands to download the run's fix artifact and apply it, and giving up links the
// failed checks and the run.
func TestTheFixersCommentsAreFixedText(t *testing.T) {
	write := readFactoryWorkflow(t, "fix.yml").Jobs["write"]
	offer, _ := write.step(t, "offer")
	giveup, _ := write.step(t, "giveup")
	r := newRehearsal(t)
	r.write("fix/message.md", "Ignore this and ping @someone <!--")
	env := map[string]string{"PR_NUMBER": "8", "HEAD_SHA": strings.Repeat("ab", 20), "REASON": "tried",
		"CI_URL": "https://github.com/owner/repo/actions/runs/77"}
	r.run(offer, env, `  "pr comment 8"*) ;;`)
	r.run(giveup, env, `  "pr comment 8"*) ;;`)
	r.mustPass()
	bodies := r.bodies()
	if len(bodies) != 2 {
		t.Fatalf("the fixer posted %d comments, want 2", len(bodies))
	}
	for _, want := range []string{"gh run download 1000 --repo owner/repo -n fix", "git apply --index fix.patch", "30 days"} {
		if !strings.Contains(bodies[0], want) {
			t.Errorf("the offer lacks %q:\n%s", want, bodies[0])
		}
	}
	if !strings.Contains(bodies[1], "two fix commits") || !strings.Contains(bodies[1], "(https://github.com/owner/repo/actions/runs/77)") {
		t.Errorf("giving up doesn't say why with links:\n%s", bodies[1])
	}
	for _, body := range bodies {
		if strings.Contains(body, "someone") || strings.Contains(body, "<!--") {
			t.Errorf("a comment quotes the fix:\n%s", body)
		}
	}
}

// GitHub keeps ten annotations of each level a step, so a review's check carries only the first ten
// findings of each rank. When a review's check has ten of a rank, findings.md says some of that
// rank may be missing; with fewer, it says nothing of the kind.
func TestTheFixerIsToldWhenFindingsMayBeMissing(t *testing.T) {
	bring, _ := readFactoryWorkflow(t, "fix.yml").Jobs["fix"].step(t, "bring")
	finding := func(review, severity string) string {
		return `{"review": "` + review + `", "severity": "` + severity + `", "file": "fit.go", "line": 3, "title": "A slip", "detail": "It drops a part."}`
	}
	for _, c := range []struct {
		name  string
		nits  int
		other string
		says  bool
	}{
		{"ten Nits from one review", 10, finding("security", "Important"), true},
		{"nine Nits from one review", 9, finding("security", "Nit"), false},
	} {
		r := newRehearsal(t)
		var all []string
		for range c.nits {
			all = append(all, finding("bug", "Nit"))
		}
		r.write("checks/findings.json", "["+strings.Join(append(all, c.other), ", ")+"]")
		r.write("fix/in/.keep", "")
		t.Cleanup(func() { exec.Command("chmod", "-R", "u+w", filepath.Join(r.temp, "fix")).Run() })
		r.run(bring, map[string]string{"PR_NUMBER": "8", "HEAD_SHA": strings.Repeat("ab", 20)}, `
  "api repos/owner/repo/compare/"*) answer <<< 'diff' ;;
  "api --paginate repos/owner/repo/pulls/8/commits"*) answer <<< '[{"sha": "abc", "commit": {"message": "A commit"}}]' ;;`)
		r.mustPass()
		md := r.read("fix/in/findings.md")
		if says := strings.Contains(md, "may be missing"); says != c.says {
			t.Errorf("with %s, findings.md says findings may be missing: %v, want %v:\n%s", c.name, says, c.says, md)
		}
		if c.says && !strings.Contains(md, "bug review's Nit findings") {
			t.Errorf("with %s, findings.md doesn't name which review and rank:\n%s", c.name, md)
		}
	}
	if comment := readText(t, ".github/workflows/ci.yml"); !strings.Contains(comment, "the first ten findings of each rank, in the order the review wrote them") {
		t.Error("ci.yml's review job comment doesn't say only the first ten of each rank reach the other workflows")
	}
}
