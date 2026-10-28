package awml

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		`", "repo": {"full_name": "owner/repo"}}, "author_association": "OWNER", "user": {"type": "User"}}' ;;
  "pr view ` + number + ` --repo owner/repo --json mergeable --jq .mergeable") echo CONFLICTING ;;`
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
