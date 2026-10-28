package awml

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The rebase's jobs: find rebases with git alone, draft starts Claude only on a real conflict, and
// push guards, pushes and comments once. No job runs checks of its own, or posts a status comment.
func TestResolveHasNoChecksOfItsOwn(t *testing.T) {
	jobs := readFactoryWorkflow(t, "resolve.yml").Jobs
	var names []string
	for name := range jobs {
		names = append(names, name)
	}
	if strings.Join(sortedStrings(names), " ") != "draft find push" {
		t.Errorf("resolve.yml's jobs are %v, want find, draft and push", names)
	}
	if holdsClaudesToken(jobs["find"]) || holdsClaudesToken(jobs["push"]) {
		t.Error("a clean rebase or the push runs beside Claude's token")
	}
	if !strings.Contains(jobs["draft"].If, "needs.find.outputs.conflicts != '[]'") {
		t.Errorf("Claude starts if %q, want only on a real conflict", jobs["draft"].If)
	}
	onlyReads(t, "resolve.yml", "find", jobs["find"])
	onlyReads(t, "resolve.yml", "draft", jobs["draft"])
	text := readText(t, ".github/workflows/resolve.yml")
	for _, never := range []string{"go test", "make test-wasm", "awml check"} {
		if strings.Contains(text, never) {
			t.Errorf("resolve.yml runs %q, but CI judges the pushed commit", never)
		}
	}
}

// resolveRepo makes a bakery's origin whose main has moved on from where two branches started: one
// rebases cleanly, and one conflicts with main.
func resolveRepo(t *testing.T, r *rehearsal) (origin, main, clean, conflicting string) {
	t.Helper()
	origin, _ = r.newRepo("origin", map[string]string{"recipes.md": "Bread\n", "notes.md": "Notes\n"})
	gitIn(t, origin, "switch", "-q", "-c", "clean")
	r.write(filepath.Join(origin, "notes.md"), "Notes\nMore\n")
	gitIn(t, origin, "commit", "-qam", "More notes")
	clean = gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "-c", "clashing", "main")
	r.write(filepath.Join(origin, "recipes.md"), "Scones\n")
	gitIn(t, origin, "commit", "-qam", "Scones")
	conflicting = gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "main")
	r.write(filepath.Join(origin, "recipes.md"), "Rolls\n")
	gitIn(t, origin, "commit", "-qam", "Rolls")
	main = gitIn(t, origin, "rev-parse", "HEAD")
	return origin, main, clean, conflicting
}

func resolvePull(number, branch, sha string) string {
	return `
  "api repos/owner/repo/pulls/` + number + `") answer <<< '{"base": {"ref": "main"}, "head": {"ref": "` + branch + `", "sha": "` + sha +
		`", "repo": {"full_name": "owner/repo"}}, "author_association": "OWNER", "user": {"login": "the-owner", "type": "User"}}' ;;
  "pr view ` + number + ` --repo owner/repo --json mergeable --jq .mergeable") echo CONFLICTING ;;` + permissions
}

// The find job rebases each conflicting pull request with git alone. A clean rebase is bundled, its
// digest the job's output, committed as the app's bot with each commit's author kept; a real
// conflict is left for Claude.
func TestResolveRebasesWithGitAloneFirst(t *testing.T) {
	find, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["find"].step(t, "find")
	r := newRehearsal(t)
	origin, main, clean, conflicting := resolveRepo(t, r)
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	r.run(find, map[string]string{"GITHUB_SHA": main}, `
  "api users/awml-factory%5Bbot%5D --jq .id") echo 41 ;;
  "api --paginate repos/owner/repo/pulls?state=open&per_page=100 --jq .[].number") printf '5\n6\n' ;;`+
		resolvePull("5", "clean", clean)+resolvePull("6", "clashing", conflicting))
	r.mustPass()
	var pulls []map[string]any
	if err := json.Unmarshal([]byte(r.outputs["pulls"]), &pulls); err != nil {
		t.Fatal(err)
	}
	if len(pulls) != 2 || pulls[0]["clean"] != true || pulls[1]["clean"] != false {
		t.Fatalf("the find job found %v, want #5 clean and #6 conflicting", pulls)
	}
	if !strings.Contains(r.outputs["conflicts"], `"number":6`) || strings.Contains(r.outputs["conflicts"], `"number":5`) {
		t.Errorf("the conflicts for Claude are %s, want only #6", r.outputs["conflicts"])
	}
	var digests map[string]string
	if err := json.Unmarshal([]byte(r.outputs["digests"]), &digests); err != nil || len(digests) != 1 || len(digests["5"]) != 64 {
		t.Fatalf("the find job's digests are %s, want #5's alone", r.outputs["digests"])
	}
	check := filepath.Join(r.root, "check")
	gitIn(t, r.root, "clone", "-q", origin, check)
	gitIn(t, check, "fetch", "-q", filepath.Join(r.temp, "rebased", "5.bundle"), "HEAD:refs/rebased")
	if got := gitIn(t, check, "log", "-1", "--format=%an <%ae> %cn <%ce>", "refs/rebased"); got !=
		"A baker <baker@example.com> awml-factory[bot] <41+awml-factory[bot]@users.noreply.github.com>" {
		t.Errorf("the rebased commit is %q, want the person's, committed by the app's bot", got)
	}
	if gitIn(t, check, "rev-parse", "refs/rebased^") != main {
		t.Error("the clean rebase doesn't sit on main")
	}
}

// The push job checks each bundle against the digest the job that made it gave, and pushes only a
// rebase that sits on main, with no conflict markers and no change to .github/ or Claude Code's own
// files, with --force-with-lease at the head the find job saw. Each pull request gets one new
// comment when it ends, with Claude's note escaped.
func TestResolvePushesOnlyWhatPassesItsGuards(t *testing.T) {
	push, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["push"].step(t, "push")
	r := newRehearsal(t)
	origin, main, clean, conflicting := resolveRepo(t, r)
	gitIn(t, origin, "switch", "-q", "-c", "workflow", "main")
	r.write(filepath.Join(origin, ".github/workflows/ci.yml"), "name: CI\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "A workflow")
	workflow := gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "main")
	gitIn(t, origin, "config", "receive.denyCurrentBranch", "ignore")

	bundle := func(dir, name, branch string, resolve bool) string {
		work := filepath.Join(r.root, "bundle-"+name)
		gitIn(t, r.root, "clone", "-q", "--branch", branch, origin, work)
		if resolve {
			if err := gitRebase(work, main); err == nil {
				t.Fatal("the clashing branch rebased cleanly")
			}
			r.write(filepath.Join(work, "recipes.md"), "Rolls\nScones\n")
			gitIn(t, work, "add", "-A")
			gitIn(t, work, "-c", "core.editor=true", "rebase", "--continue")
		} else {
			gitIn(t, work, "rebase", "-q", main)
		}
		path := filepath.Join(r.temp, dir, name+".bundle")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		gitIn(t, work, "bundle", "create", path, main+"..HEAD")
		sum := newRehearsal(t)
		sum.run(workflowStep{Run: `sha256sum < "$FILE" | cut -c 1-64`}, map[string]string{"FILE": path}, "")
		return strings.TrimSpace(sum.printed)
	}
	found := map[string]string{"5": bundle("rebased", "5", "clean", false), "9": bundle("rebased", "9", "workflow", false), "7": strings.Repeat("0", 64)}
	bundle("rebased", "7", "clean", false)
	resolved := map[string]string{"6.bundle": bundle("resolved", "6", "clashing", true)}
	note := r.write("resolved/6.md", "Kept both, as @someone asked. <!-- hide")
	sum := newRehearsal(t)
	sum.run(workflowStep{Run: `sha256sum < "$FILE" | cut -c 1-64`}, map[string]string{"FILE": note}, "")
	resolved["6.md"] = strings.TrimSpace(sum.printed)
	asJSON := func(v any) string { data, _ := json.Marshal(v); return string(data) }
	pulls := []map[string]any{
		{"number": 5, "branch": "clean", "head_sha": clean, "clean": true},
		{"number": 6, "branch": "clashing", "head_sha": conflicting, "clean": false},
		{"number": 7, "branch": "clean", "head_sha": clean, "clean": true},
		{"number": 9, "branch": "workflow", "head_sha": workflow, "clean": true},
		{"number": 11, "branch": "clashing", "head_sha": conflicting, "clean": false},
	}
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	r.run(push, map[string]string{"APP_TOKEN": "not-a-token", "PULLS": asJSON(pulls), "FOUND": asJSON(found),
		"RESOLVED": asJSON(resolved), "MAIN_SHA": main}, `  "pr comment "*) ;;`)
	if !r.failed {
		t.Error("the push job passed though some pull requests couldn't be pushed")
	}
	bodies := r.bodies()
	if len(bodies) != 5 {
		t.Fatalf("the push job posted %d comments, want one for each pull request:\n%s", len(bodies), r.printed)
	}
	for at, want := range []string{"pushed it", "pushed it", "isn't the one the job that rebased it made", "never pushes", "couldn't resolve"} {
		if !strings.Contains(bodies[at], want) {
			t.Errorf("comment %d doesn't say %q:\n%s", at, want, bodies[at])
		}
	}
	if !strings.Contains(bodies[0], "/commit/"+clean+")") || !strings.Contains(bodies[0], "git push --force origin "+clean+":clean") {
		t.Errorf("the comment doesn't link the old head or say how to put it back:\n%s", bodies[0])
	}
	if !strings.Contains(bodies[1], "@\u200bsomeone") || strings.Contains(bodies[1], "<!--") {
		t.Errorf("Claude's note isn't escaped:\n%s", bodies[1])
	}
	if gitIn(t, origin, "rev-parse", "clean^") != main || gitIn(t, origin, "rev-parse", "workflow") != workflow {
		t.Error("the push job didn't push the clean rebase, or pushed a change to a workflow")
	}
}

func gitRebase(dir, onto string) error {
	command := gitCommandIn(dir, "rebase", "-q", onto)
	return command.Run()
}

// A branch whose commits change .github/ or Claude Code's own files is never pushed by the app, so
// the find job holds it back before any rebase, Claude's included, and the push job says so in the
// pull request's one comment.
func TestResolveHoldsBackABranchThatTouchesGithub(t *testing.T) {
	for _, path := range []string{".npmrc", "node_modules/x/index.js"} {
		holdsBack(t, path)
	}
	find, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["find"].step(t, "find")
	r := newRehearsal(t)
	origin, main, _, _ := resolveRepo(t, r)
	gitIn(t, origin, "switch", "-q", "-c", "workflow", main+"^")
	r.write(filepath.Join(origin, ".github/workflows/ci.yml"), "name: CI\n")
	r.write(filepath.Join(origin, "recipes.md"), "Muffins\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "A workflow and muffins")
	workflow := gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "main")
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	r.run(find, map[string]string{"GITHUB_SHA": main}, `
  "api users/awml-factory%5Bbot%5D --jq .id") echo 41 ;;
  "api --paginate repos/owner/repo/pulls?state=open&per_page=100 --jq .[].number") printf '9\n' ;;`+
		resolvePull("9", "workflow", workflow))
	r.mustPass()
	if !strings.Contains(r.outputs["pulls"], `"held":true`) || r.outputs["conflicts"] != "[]" || r.outputs["digests"] != "{}" {
		t.Errorf("the find job didn't hold the branch back: %v", r.outputs)
	}

	push, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["push"].step(t, "push")
	p := newRehearsal(t)
	p.dir = work
	p.run(push, map[string]string{"APP_TOKEN": "not-a-token", "MAIN_SHA": main, "FOUND": "{}", "RESOLVED": "{}",
		"PULLS": `[{"number": 9, "branch": "workflow", "head_sha": "` + workflow + `", "clean": false, "held": true}]`},
		`  "pr comment "*) ;;`)
	if bodies := p.bodies(); len(bodies) != 1 || !strings.Contains(bodies[0], "never pushes") {
		t.Errorf("the push job said %q, want that the app never pushes .github/", bodies)
	}
}

// holdsBack checks that the find job holds back a branch that changes the path given, which the app
// never pushes.
func holdsBack(t *testing.T, path string) {
	t.Helper()
	find, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["find"].step(t, "find")
	r := newRehearsal(t)
	origin, main, _, _ := resolveRepo(t, r)
	gitIn(t, origin, "switch", "-q", "-c", "npm", main+"^")
	r.write(filepath.Join(origin, path), "registry=https://example.com/\n")
	r.write(filepath.Join(origin, "recipes.md"), "Muffins\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "npm's settings and muffins")
	head := gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "main")
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	r.run(find, map[string]string{"GITHUB_SHA": main}, `
  "api users/awml-factory%5Bbot%5D --jq .id") echo 41 ;;
  "api --paginate repos/owner/repo/pulls?state=open&per_page=100 --jq .[].number") printf '9\n' ;;`+
		resolvePull("9", "npm", head))
	r.mustPass()
	if !strings.Contains(r.outputs["pulls"], `"held":true`) {
		t.Errorf("the find job didn't hold back a branch that changes %s: %v", path, r.outputs)
	}
}

// A session's bundle and note are read only from its own output folder: if that folder, or its pull
// request's folder, has become a symbolic link, or it resolves anywhere but inside the pull
// request's own folder in the host's, nothing of the session's is kept.
func TestResolveReadsOnlyTheSessionsOwnFolder(t *testing.T) {
	check, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["draft"].step(t, "check")
	head := strings.Repeat("ab", 20)
	for _, how := range []string{"out", "root"} {
		r := newRehearsal(t)
		elsewhere := filepath.Join(r.root, "elsewhere")
		r.write(filepath.Join(elsewhere, "out", "message.md"), "Forged.\n")
		r.write(filepath.Join(elsewhere, "out", "rebased.bundle"), "not a bundle\n")
		root := filepath.Join(r.temp, "host", "pr-9")
		switch how {
		case "out":
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(elsewhere, "out"), filepath.Join(root, "out")); err != nil {
				t.Fatal(err)
			}
		case "root":
			if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, root); err != nil {
				t.Fatal(err)
			}
		}
		r.write("host/pulls.tsv", "9\t"+head+"\n")
		workspace := keptWorkspace(t, r)
		r.run(check, map[string]string{"MAIN_SHA": strings.Repeat("cd", 20), "GITHUB_WORKSPACE": workspace}, "")
		r.mustPass()
		if entries, _ := os.ReadDir(filepath.Join(r.temp, "host", "resolved")); len(entries) != 0 {
			t.Errorf("with %s a symbolic link, the check kept %d of the session's files", how, len(entries))
		}
	}
}

// keptWorkspace makes a workspace with a .git, and the baseline of both the prepare step keeps.
func keptWorkspace(t *testing.T, r *rehearsal) string {
	t.Helper()
	workspace, _ := r.newRepo("workspace", map[string]string{"recipes.md": "Bread\n"})
	folders, err := exec.Command("stat", "-c", "%d:%i", workspace, filepath.Join(workspace, ".git")).Output()
	if err != nil {
		t.Fatal(err)
	}
	r.write("host/baseline/folders", string(folders))
	r.write("host/scrub-placeholders.sh", readText(t, ".github/scrub-placeholders.sh"))
	r.write("host/baseline/absent", "")
	return workspace
}

// The check step fetches main and each branch from the workspace, so it does so only if the
// workspace and its .git are the very folders they were before the sessions, since a session could
// move a folder above them and leave another in their place.
func TestResolveFetchesOnlyFromTheWorkspaceItKept(t *testing.T) {
	check, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["draft"].step(t, "check")
	r := newRehearsal(t)
	workspace := keptWorkspace(t, r)
	r.write("host/pulls.tsv", "9\t"+strings.Repeat("ab", 20)+"\n")
	if err := os.Rename(workspace, workspace+".moved"); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-a", workspace+".moved", workspace).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	r.run(check, map[string]string{"MAIN_SHA": strings.Repeat("cd", 20), "GITHUB_WORKSPACE": workspace}, "")
	if !r.failed || !strings.Contains(r.printed, "isn't the folder it was before the sessions") {
		t.Errorf("the check fetched from a workspace that isn't the one kept:\n%s", r.printed)
	}
}

// resolve.yml touches only branches of this repository whose pull request the app's own bot opened
// from a build/issue- branch, or a person who may write opened: not any bot, nor a member who may
// only read, whatever their association.
func TestResolveTouchesOnlyItsOwnPeoplesBranches(t *testing.T) {
	find, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["find"].step(t, "find")
	for name, tt := range map[string]struct {
		user, kind, branch string
		touched            bool
	}{
		"the app's build":               {"awml-factory[bot]", "Bot", "build/issue-5", true},
		"another bot on a build branch": {"dependabot[bot]", "Bot", "build/issue-5", false},
		"the app off a build branch":    {"awml-factory[bot]", "Bot", "scones", false},
		"a writer":                      {"a-writer", "User", "scones", true},
		"a member who may only read":    {"a-member", "User", "scones", false},
	} {
		r := newRehearsal(t)
		origin, main, clean, _ := resolveRepo(t, r)
		work := filepath.Join(r.root, "work")
		gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
		r.dir = work
		pull := strings.Replace(strings.Replace(resolvePull("5", tt.branch, clean), `"login": "the-owner", "type": "User"`,
			`"login": "`+tt.user+`", "type": "`+tt.kind+`"`, 1), `"author_association": "OWNER"`, `"author_association": "MEMBER"`, 1)
		r.run(find, map[string]string{"GITHUB_SHA": main}, `
  "api users/awml-factory%5Bbot%5D --jq .id") echo 41 ;;
  "api --paginate repos/owner/repo/pulls?state=open&per_page=100 --jq .[].number") printf '5\n' ;;`+pull)
		if tt.touched {
			r.mustPass()
		}
		if got := strings.Contains(r.outputs["pulls"], `"number":5`); got != tt.touched {
			t.Errorf("%s: resolve touched the branch: %v, want %v\n%s", name, got, tt.touched, r.printed)
		}
	}
}

// A branch whose commits change .github/ or Claude Code's files is held back even if a later commit
// takes the change out again, since each of its commits would be pushed.
func TestResolveHoldsBackABranchByEachOfItsCommits(t *testing.T) {
	find, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["find"].step(t, "find")
	r := newRehearsal(t)
	origin, main, _, _ := resolveRepo(t, r)
	gitIn(t, origin, "switch", "-q", "-c", "sneaky", main+"^")
	r.write(filepath.Join(origin, ".github/workflows/x.yml"), "name: X\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "A workflow")
	gitIn(t, origin, "rm", "-q", ".github/workflows/x.yml")
	r.write(filepath.Join(origin, "recipes.md"), "Muffins\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "Taken out again, and muffins")
	head := gitIn(t, origin, "rev-parse", "HEAD")
	gitIn(t, origin, "switch", "-q", "main")
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	r.run(find, map[string]string{"GITHUB_SHA": main}, `
  "api users/awml-factory%5Bbot%5D --jq .id") echo 41 ;;
  "api --paginate repos/owner/repo/pulls?state=open&per_page=100 --jq .[].number") printf '9\n' ;;`+resolvePull("9", "sneaky", head))
	r.mustPass()
	if !strings.Contains(r.outputs["pulls"], `"held":true`) {
		t.Errorf("a branch that added a workflow in one commit and took it out in the next wasn't held back: %v", r.outputs)
	}
}

// The push job checks each rebased commit's files, not the branch's net change, so a session that
// added a change to .github/ in one commit and took it out in the next still can't have it pushed.
func TestResolvePushesNoCommitThatTouchesGithub(t *testing.T) {
	push, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["push"].step(t, "push")
	r := newRehearsal(t)
	origin, main, _, _ := resolveRepo(t, r)
	gitIn(t, origin, "config", "receive.denyCurrentBranch", "ignore")
	gitIn(t, origin, "switch", "-q", "-c", "sneaky", main)
	head := gitIn(t, origin, "rev-parse", "HEAD")
	r.write(filepath.Join(origin, ".github/workflows/x.yml"), "name: X\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "A workflow")
	gitIn(t, origin, "rm", "-q", ".github/workflows/x.yml")
	r.write(filepath.Join(origin, "recipes.md"), "Muffins\n")
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-qm", "Taken out again, and muffins")
	bundle := filepath.Join(r.temp, "resolved", "6.bundle")
	if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, origin, "bundle", "create", bundle, main+"..HEAD")
	gitIn(t, origin, "reset", "-q", "--hard", head)
	gitIn(t, origin, "switch", "-q", "main")
	sum := newRehearsal(t)
	sum.run(workflowStep{Run: `sha256sum < "$FILE" | cut -c 1-64`}, map[string]string{"FILE": bundle}, "")
	work := filepath.Join(r.root, "work")
	gitIn(t, r.root, "clone", "-q", "--branch", "main", origin, work)
	r.dir = work
	r.run(push, map[string]string{"APP_TOKEN": "not-a-token", "MAIN_SHA": main, "FOUND": "{}",
		"RESOLVED": `{"6.bundle": "` + strings.TrimSpace(sum.printed) + `"}`,
		"PULLS":    `[{"number": 6, "branch": "sneaky", "head_sha": "` + head + `", "clean": false}]`}, `  "pr comment "*) ;;`)
	if bodies := r.bodies(); len(bodies) != 1 || !strings.Contains(bodies[0], "never pushes") {
		t.Errorf("the push job said %q, want that the app never pushes .github/", bodies)
	}
	if gitIn(t, origin, "rev-parse", "sneaky") != head {
		t.Error("the push job pushed a branch one of whose commits touches .github/")
	}
}

// A rebase's session may write the workspace's .git, its refs among them, so the check step reads
// each branch from the workspace by the commit the find job saw, never by a ref a session could
// have moved.
func TestTheCheckReadsEachBranchByItsCommit(t *testing.T) {
	check, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["draft"].step(t, "check")
	if strings.Contains(check.Run, "refs/heads/factory-rebase-") || !strings.Contains(check.Run, `"$GITHUB_WORKSPACE" "$MAIN_SHA" "$head_sha"`) {
		t.Error("the check step reads a branch from the workspace by a ref a session could have moved")
	}
}

// A rebase's session makes the commits in its bundle, so the check step keeps a bundle only if its
// commits since main form one straight line from main, each with the one before it as its only
// parent, as many as the branch's own since it left main. A merge with a parent of the session's
// making, or a commit the branch never had, is refused.
func TestTheCheckKeepsOnlyAStraightRebase(t *testing.T) {
	check, _ := readFactoryWorkflow(t, "resolve.yml").Jobs["draft"].step(t, "check")
	commit := func(repo, file, text, message string) string {
		if err := os.WriteFile(filepath.Join(repo, file), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, repo, "add", "-A")
		gitIn(t, repo, "-c", "user.name=x", "-c", "user.email=x@example.invalid", "commit", "-qm", message)
		return gitIn(t, repo, "rev-parse", "HEAD")
	}
	for _, shape := range []string{"straight", "merged main in", "a merge", "a merge onto main's history", "an extra commit"} {
		r := newRehearsal(t)
		workspace := keptWorkspace(t, r)
		base := gitIn(t, workspace, "rev-parse", "HEAD")
		gitIn(t, workspace, "switch", "-q", "-c", "scones")
		commit(workspace, "scones.md", "Scones\n", "Scones")
		if shape == "merged main in" {
			// main merged into the branch, as GitHub's Update branch does: a plain rebase drops
			// the merge, so the branch's own commits are counted without it.
			gitIn(t, workspace, "switch", "-q", "main")
			commit(workspace, "flour.md", "Flour\n", "Flour")
			gitIn(t, workspace, "switch", "-q", "scones")
			gitIn(t, workspace, "-c", "user.name=x", "-c", "user.email=x@example.invalid", "merge", "-q", "--no-edit", "main")
		}
		head := commit(workspace, "scones.md", "Scones\nJam\n", "Jam")
		gitIn(t, workspace, "switch", "-q", "-")
		main := commit(workspace, "bread.md", "Rye\n", "Rye")

		clone := filepath.Join(r.root, "clone")
		gitIn(t, r.root, "clone", "-q", workspace, clone)
		gitIn(t, clone, "switch", "-q", "--detach", head)
		gitIn(t, clone, "-c", "user.name=x", "-c", "user.email=x@example.invalid", "rebase", "-q", main)
		switch shape {
		case "a merge":
			// The tip given an extra parent, a commit the session wrote.
			side := commit(clone, "other.md", "Other\n", "Other")
			gitIn(t, clone, "reset", "-q", "--hard", "HEAD~1")
			tree := gitIn(t, clone, "rev-parse", "HEAD^{tree}")
			merged := gitIn(t, clone, "-c", "user.name=x", "-c", "user.email=x@example.invalid", "commit-tree", tree,
				"-p", "HEAD~1", "-p", side, "-m", "Jam")
			gitIn(t, clone, "reset", "-q", "--hard", merged)
		case "a merge onto main's history":
			// An extra parent already in main's history: as many commits as the branch's, so only
			// the one-parent rule refuses it.
			tree := gitIn(t, clone, "rev-parse", "HEAD^{tree}")
			merged := gitIn(t, clone, "-c", "user.name=x", "-c", "user.email=x@example.invalid", "commit-tree", tree,
				"-p", "HEAD~1", "-p", base, "-m", "Jam")
			gitIn(t, clone, "reset", "-q", "--hard", merged)
		case "an extra commit":
			commit(clone, "cream.md", "Cream\n", "Cream")
		}
		out := filepath.Join(r.temp, "host", "pr-5", "out")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		gitIn(t, clone, "bundle", "create", "-q", filepath.Join(out, "rebased.bundle"), main+"..HEAD")
		r.write("host/pulls.tsv", "5\t"+head+"\n")
		r.dir = workspace
		r.run(check, map[string]string{"MAIN_SHA": main, "GITHUB_WORKSPACE": workspace}, "")
		r.mustPass()
		_, kept := os.Stat(filepath.Join(r.temp, "host", "resolved", "5.bundle"))
		if shape == "straight" || shape == "merged main in" {
			if kept != nil {
				t.Errorf("with %s, a straight rebase wasn't kept:\n%s", shape, r.printed)
			}
			continue
		}
		if kept == nil || !strings.Contains(r.printed, "isn't one straight line") {
			t.Errorf("with %s, the check kept the rebase or didn't say why:\n%s", shape, r.printed)
		}
	}
}

// The rebase's session runs on Sonnet; every other session, the build's, the fix's and CI's
// reviews and Classify, runs on Opus. Each workflow's drawing names the model its session runs on.
func TestEachSessionRunsOnItsModel(t *testing.T) {
	model := regexp.MustCompile(`claude -p "\$prompt" --model (\S+)`)
	for _, at := range []struct{ file, drawing, want string }{
		{"resolve.yml", "rebase-conflicting-pull-requests.yaml", "sonnet"},
		{"build.yml", "issue-to-pull-request.yaml", "opus"},
		{"fix.yml", "fix-review-findings.yaml", "opus"},
		{"ci.yml", "ci-a-commit.yaml", "opus"},
	} {
		found := model.FindAllStringSubmatch(readText(t, ".github/workflows/"+at.file), -1)
		if len(found) == 0 {
			t.Errorf("%s starts no Claude session with a model", at.file)
		}
		for _, m := range found {
			if m[1] != at.want {
				t.Errorf("%s starts a session on %s, want %s", at.file, m[1], at.want)
			}
		}
		if !strings.Contains(readText(t, ".github/drawings/"+at.drawing), "Claude Code, claude-"+at.want+"-5-5,") {
			t.Errorf("%s doesn't name claude-%s-5-5 for the session", at.drawing, at.want)
		}
	}
}
