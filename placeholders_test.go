package awml

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Claude Code's scrub denies writes to paths in the tree a session runs in, and in the workspace,
// that a command could use to change what runs next: npm's and bun's files, the .env files, .claude,
// .github, .gitmodules and .git's hooks and config among them. Where one doesn't exist, the sandbox
// makes a mount point for it in the real tree, an empty file, or an empty folder for one that holds
// another, and leaves it there. scrub-placeholders.sh records, before the session, which are
// missing, and puts a small package.json of its own in place of a missing one, since node refuses an
// empty one; after the session, it removes each that is still only a placeholder, and refuses one
// that holds anything.
func TestTheSandboxsPlaceholdersAreCleanedOrRefused(t *testing.T) {
	script := readText(t, ".github/scrub-placeholders.sh")
	const standin = "{\"private\": true}\n"
	setup := func(t *testing.T) (*rehearsal, string) {
		r := newRehearsal(t)
		r.write("scrub-placeholders.sh", script)
		tree, _ := r.newRepo("tree", map[string]string{"recipes.md": "Bread\n", "scripts/bake.sh": "echo bake\n"})
		r.run(workflowStep{Run: `bash "$RUNNER_TEMP/scrub-placeholders.sh" before "$TREE" "$RUNNER_TEMP/absent" standin`},
			map[string]string{"TREE": tree}, "")
		r.mustPass()
		return r, tree
	}
	after := func(r *rehearsal, tree string) {
		r.run(workflowStep{Run: `bash "$RUNNER_TEMP/scrub-placeholders.sh" after "$TREE" "$RUNNER_TEMP/absent"`},
			map[string]string{"TREE": tree}, "")
	}
	placeholders := func(r *rehearsal, tree string) {
		for _, file := range []string{".git/config.worktree", ".git/config.lock", ".git/modules", ".gitmodules", ".env", ".claude", "yarn.lock"} {
			r.write(filepath.Join(tree, file), "")
		}
		if err := os.MkdirAll(filepath.Join(tree, "node_modules"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	r, tree := setup(t)
	if got := readText(t, filepath.Join(tree, "package.json")); got != standin {
		t.Fatalf("before the session, package.json is %q, want the stand-in", got)
	}
	// git refuses a repository whose commondir is empty, so it gets a stand-in too, and git works.
	if got := readText(t, filepath.Join(tree, ".git/commondir")); got != ".\n" {
		t.Fatalf("before the session, .git/commondir is %q, want one naming the git directory itself", got)
	}
	gitIn(t, tree, "status", "--short")
	placeholders(r, tree)
	after(r, tree)
	r.mustPass()
	for _, gone := range []string{"package.json", ".git/config.worktree", ".git/config.lock", ".git/commondir", ".git/modules", ".gitmodules", ".env", ".claude", "yarn.lock", "node_modules"} {
		if _, err := os.Lstat(filepath.Join(tree, gone)); err == nil {
			t.Errorf("after the session, the placeholder %s is still there", gone)
		}
	}
	for _, kept := range []string{"scripts/bake.sh", ".git/config", ".git/hooks", ".git/info/exclude"} {
		if _, err := os.Lstat(filepath.Join(tree, kept)); err != nil {
			t.Errorf("cleaning removed %s, which was there before the session", kept)
		}
	}

	for name, leave := range map[string]func(tree string){
		"a placeholder with something in it": func(tree string) { r.write(filepath.Join(tree, ".env"), "TOKEN=x\n") },
		"a link": func(tree string) {
			_ = os.Remove(filepath.Join(tree, ".claude"))
			if err := os.Symlink("/etc/hostname", filepath.Join(tree, ".claude")); err != nil {
				t.Fatal(err)
			}
		},
		"a folder with something in it": func(tree string) { r.write(filepath.Join(tree, "node_modules", ".bin", "go"), "#!/bin/sh\n") },
		"a changed stand-in":            func(tree string) { r.write(filepath.Join(tree, "package.json"), "{\"scripts\": {}}\n") },
		"a missing stand-in":            func(tree string) { _ = os.Remove(filepath.Join(tree, "package.json")) },
	} {
		r, tree := setup(t)
		placeholders(r, tree)
		leave(tree)
		after(r, tree)
		if !r.failed || !strings.Contains(r.printed, "isn't run on it") {
			t.Errorf("with %s, the tree was taken:\n%s", name, r.printed)
		}
	}

	// A package.json the tree had is left alone, and no stand-in is made over it.
	r = newRehearsal(t)
	r.write("scrub-placeholders.sh", script)
	tree, _ = r.newRepo("tree", map[string]string{"package.json": "{\"name\": \"bakery\"}\n"})
	r.run(workflowStep{Run: `bash "$RUNNER_TEMP/scrub-placeholders.sh" before "$TREE" "$RUNNER_TEMP/absent" standin
bash "$RUNNER_TEMP/scrub-placeholders.sh" after "$TREE" "$RUNNER_TEMP/absent"`}, map[string]string{"TREE": tree}, "")
	r.mustPass()
	if got := readText(t, filepath.Join(tree, "package.json")); got != "{\"name\": \"bakery\"}\n" {
		t.Errorf("the tree's own package.json became %q", got)
	}
}

// The build and the fix record the placeholders' paths before the session, from main's script in
// the host's folder, and clean them before git runs on the tree. Resolve's clones get the stand-in
// and keep the placeholders out of what a session commits, and its check step cleans the workspace,
// where the scrub protects .git too, before it fetches from it.
func TestEverySessionTreeIsCleanedOfPlaceholdersBeforeGit(t *testing.T) {
	for _, at := range []struct{ file, job string }{{"build.yml", "draft"}, {"fix.yml", "fix"}} {
		job := readFactoryWorkflow(t, at.file).Jobs[at.job]
		session := job.Steps[job.session()].Run
		before := strings.Index(session, `bash "$HOST_DIR/scrub-placeholders.sh" before "$GITHUB_WORKSPACE" "$HOST_DIR/baseline/absent" standin`)
		if before < 0 || before > strings.Index(session, "claude -p") || !strings.Contains(session, `cp "$RUNNER_TEMP/scrub-placeholders.sh" "$HOST_DIR/"`) {
			t.Errorf("%s's %s doesn't record the placeholders' paths with main's script before its session", at.file, at.job)
		}
		patch, _ := job.step(t, "patch")
		clean := strings.Index(patch.Run, `bash "$HOST_DIR/scrub-placeholders.sh" after "$GITHUB_WORKSPACE" "$HOST_DIR/baseline/absent"`)
		if clean < 0 || clean > strings.Index(patch.Run, "git()") {
			t.Errorf("%s's %s doesn't clean the placeholders before git runs", at.file, at.job)
		}
	}
	job := readFactoryWorkflow(t, "resolve.yml").Jobs["draft"]
	prepare, _ := job.step(t, "prepare")
	for _, want := range []string{`bash "$HOST_DIR/scrub-placeholders.sh" before "$root/work" "$root/absent" standin`,
		`bash "$HOST_DIR/scrub-placeholders.sh" exclude "$root/work"`,
		`bash "$HOST_DIR/scrub-placeholders.sh" before "$GITHUB_WORKSPACE" "$HOST_DIR/baseline/absent"`} {
		if !strings.Contains(prepare.Run, want) {
			t.Errorf("resolve's prepare step lacks %s", want)
		}
	}
	check, _ := job.step(t, "check")
	clean := strings.Index(check.Run, `bash "$HOST_DIR/scrub-placeholders.sh" after "$GITHUB_WORKSPACE" "$HOST_DIR/baseline/absent"`)
	if clean < 0 || clean > strings.Index(check.Run, `fetch -q --no-tags "$GITHUB_WORKSPACE"`) {
		t.Error("resolve's check step doesn't clean the workspace's placeholders before it fetches from it")
	}
}

// A rebase's session may run git add in its clone, so the clone's info/exclude names every path the
// sandbox may leave a placeholder at, and none is committed into the rebase.
func TestAResolveSessionCommitsNoPlaceholder(t *testing.T) {
	r := newRehearsal(t)
	r.write("scrub-placeholders.sh", readText(t, ".github/scrub-placeholders.sh"))
	tree, _ := r.newRepo("tree", map[string]string{"recipes.md": "Bread\n"})
	r.run(workflowStep{Run: `bash "$RUNNER_TEMP/scrub-placeholders.sh" exclude "$TREE"
bash "$RUNNER_TEMP/scrub-placeholders.sh" before "$TREE" "$RUNNER_TEMP/absent" standin`}, map[string]string{"TREE": tree}, "")
	r.mustPass()
	for _, file := range []string{".env", ".claude", ".npmrc", "yarn.lock"} {
		r.write(filepath.Join(tree, file), "")
	}
	gitIn(t, tree, "add", "-A")
	if staged := gitIn(t, tree, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("git add -A in a clone staged %s", staged)
	}
}

// A session that leaves the sandbox's placeholders, the stand-in package.json among them, gives a
// patch of its own change alone; one that leaves something in a placeholder stops git.
func TestTheBuildsPatchHoldsNoPlaceholder(t *testing.T) {
	patch, _ := readFactoryWorkflow(t, "build.yml").Jobs["draft"].step(t, "patch")
	for _, leaves := range []string{"empty placeholders", "a placeholder with something in it"} {
		r := newRehearsal(t)
		repo, _ := r.newRepo("repo", map[string]string{"recipes.md": "Bread\n"})
		r.write("host/scrub-placeholders.sh", readText(t, ".github/scrub-placeholders.sh"))
		r.run(workflowStep{Run: `bash "$HOST_DIR/scrub-placeholders.sh" before "$TREE" "$HOST_DIR/baseline/absent" standin`},
			map[string]string{"TREE": repo}, "")
		r.mustPass()
		r.write("host/baseline/git-config", readText(t, filepath.Join(repo, ".git/config")))
		folders, err := exec.Command("stat", "-c", "%d:%i", repo, filepath.Join(repo, ".git")).Output()
		if err != nil {
			t.Fatal(err)
		}
		r.write("host/baseline/folders", string(folders))
		r.write("host/baseline/head", gitIn(t, repo, "rev-parse", "HEAD")+"\n")
		r.write("build/issue.md", "# Bake scones\n")
		r.write(filepath.Join(repo, "recipes.md"), "Bread\nScones\n")
		for _, file := range []string{".git/config.worktree", ".git/config.lock", ".env", ".claude", "package-lock.json"} {
			r.write(filepath.Join(repo, file), "")
		}
		if leaves == "a placeholder with something in it" {
			r.write(filepath.Join(repo, ".env"), "TOKEN=x\n")
		}
		r.dir = repo
		r.run(patch, map[string]string{"GITHUB_WORKSPACE": repo}, "")
		made := r.read("host/change/change.patch")
		if leaves == "empty placeholders" {
			r.mustPass()
			if !strings.Contains(made, "+Scones") || strings.Contains(made, "package.json") || strings.Contains(made, ".env") || strings.Contains(made, ".claude") {
				t.Errorf("the patch isn't the change alone:\n%s", made)
			}
			continue
		}
		if !r.failed || made != "" {
			t.Errorf("with %s, git ran on the tree:\n%s", leaves, r.printed)
		}
	}
}
