package awml

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hostileClaude stands in for a session whose commands do all the sandbox lets them: it reads the
// folders its settings deny writes to, then swaps every file it can reach under $RUNNER_TEMP for a
// link to the secret, and leaves links where its outputs, PLANT, should be.
const hostileClaude = `#!/usr/bin/env python3
import json, os
home, temp, secret = os.environ["HOME"], os.environ["RUNNER_TEMP"], os.environ["SECRET"]
deny = json.load(open(os.path.join(home, ".claude", "settings.json")))["sandbox"]["filesystem"]["denyWrite"]
def denied(path):
    return any(path == d or path.startswith(d.rstrip("/") + "/") for d in deny)
def plant(path):
    if denied(path):
        return
    try:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        if os.path.lexists(path):
            os.remove(path)
        os.symlink(secret, path)
    except OSError:
        pass
for dirpath, dirs, files in os.walk(temp):
    for name in files:
        plant(os.path.join(dirpath, name))
for path in os.environ.get("PLANT", "").split(":"):
    if path:
        plant(path)
print("The session ran.")
`

const hostSecret = "SECRET-0123456789-not-a-real-token"

// hostileRehearsal is a rehearsal with the hostile claude on its path and a secret beside it.
func hostileRehearsal(t *testing.T) (*rehearsal, map[string]string) {
	t.Helper()
	r := newRehearsal(t)
	t.Cleanup(func() {
		_ = filepath.Walk(r.temp, func(path string, _ os.FileInfo, _ error) error { _ = os.Chmod(path, 0o755); return nil })
	})
	secret := r.write(filepath.Join(r.root, "secret.txt"), hostSecret+"\n")
	r.write(filepath.Join(r.root, "bin", "claude"), hostileClaude)
	if err := os.Chmod(filepath.Join(r.root, "bin", "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.write("sandbox-settings.sh", readText(t, ".github/sandbox-settings.sh"))
	// The runner keeps its file commands in folders of their own, which the sandbox denies.
	output := r.write(filepath.Join(r.root, "commands", "output"), "")
	summary := r.write(filepath.Join(r.root, "summaries", "summary"), "")
	return r, map[string]string{"SECRET": secret, "GITHUB_OUTPUT": output, "GITHUB_STEP_SUMMARY": summary}
}

// noSecretReached fails if the secret is in what a step printed or wrote, or if any folder the host
// made after the session holds a link.
func noSecretReached(t *testing.T, what string, r *rehearsal, made ...string) {
	t.Helper()
	if strings.Contains(r.printed, hostSecret) {
		t.Errorf("%s printed the secret through a link a session left:\n%s", what, r.printed)
	}
	for _, file := range []string{filepath.Join(r.root, "commands", "output"), filepath.Join(r.root, "summaries", "summary")} {
		if data, _ := os.ReadFile(file); strings.Contains(string(data), hostSecret) {
			t.Errorf("%s wrote the secret to %s", what, filepath.Base(file))
		}
	}
	for _, folder := range made {
		_ = filepath.Walk(filepath.Join(r.temp, folder), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				t.Errorf("%s left a link in %s: %s", what, folder, path)
			} else if data, _ := os.ReadFile(path); info.Mode().IsRegular() && strings.Contains(string(data), hostSecret) {
				t.Errorf("%s copied the secret into %s", what, path)
			}
			return nil
		})
	}
}

func with(env map[string]string, more map[string]string) map[string]string {
	all := map[string]string{}
	for name, value := range env {
		all[name] = value
	}
	for name, value := range more {
		all[name] = value
	}
	return all
}

// A session's commands can swap any file they may write for a link to a file of secrets, such as
// /proc/self/environ. So the host reads, after a session, only from folders the session's sandbox
// denies writes to, and from the session's own output folder only once it, and all in it, is a
// plain folder or file where it should be. Each session step, and each step after it that reads what
// the session left, is rehearsed beside the hostile session: none prints or keeps the secret.
func TestNoHostReadFollowsALinkASessionLeft(t *testing.T) {
	ci := readFactoryWorkflow(t, "ci.yml")
	for _, c := range []struct{ job, prompt, out string }{
		{"review", "review-bugs-prompt.md", "bugs.json"}, {"classify", "classify-prompt.md", "classification.json"},
	} {
		session, _ := ci.Jobs[c.job].step(t, "session")
		post, _ := ci.Jobs[c.job].step(t, "post")
		r, env := hostileRehearsal(t)
		r.write("prompts/"+c.prompt, "Read it.\n")
		r.write("review/change.patch", "")
		env = with(env, map[string]string{"KIND": "bugs", "PROMPT": c.prompt, "REVIEW_DIR": filepath.Join(r.temp, "review"),
			"GITHUB_WORKSPACE": r.root, "UNREAD": "0", "PLANT": filepath.Join(r.temp, "session", "out", c.out)})
		r.run(session, env, "")
		noSecretReached(t, "ci.yml's "+c.job+" session", r)
		r.run(post, env, "")
		noSecretReached(t, "ci.yml's "+c.job+" post", r)
	}

	for _, c := range []struct{ file, job, session, prompt string }{
		{"build.yml", "draft", "draft", "build-prompt.md"}, {"fix.yml", "fix", "session", "fix-prompt.md"},
	} {
		job := readFactoryWorkflow(t, c.file).Jobs[c.job]
		session, _ := job.step(t, c.session)
		patch, _ := job.step(t, "patch")
		r, env := hostileRehearsal(t)
		repo, _ := r.newRepo("repo", map[string]string{"recipes.md": "Bread\n"})
		r.dir = repo
		r.write(c.prompt, "Do it.\n")
		r.write("build/issue.md", "# Bake scones\n")
		r.write("fix/in/findings.md", "- A finding\n")
		r.write("fix/out/.keep", "")
		plant := filepath.Join(r.temp, "build", "message.md") + ":" + filepath.Join(r.temp, "build", "pictures", "after.png") + ":" +
			filepath.Join(r.temp, "fix", "out", "message.md")
		env = with(env, map[string]string{"GITHUB_WORKSPACE": repo, "BUILD_DIR": filepath.Join(r.temp, "build"),
			"FIX_DIR": filepath.Join(r.temp, "fix", "in"), "OUT_DIR": filepath.Join(r.temp, "fix", "out"),
			"HEAD_SHA": gitIn(t, repo, "rev-parse", "HEAD"), "PLANT": plant})
		r.run(session, env, "")
		noSecretReached(t, c.file+"'s session", r)
		r.write(filepath.Join(repo, "recipes.md"), "Bread\nScones\n")
		r.run(patch, env, "")
		noSecretReached(t, c.file+"'s patch step", r, "host/change", "host/fix")
	}

	job := readFactoryWorkflow(t, "resolve.yml").Jobs["draft"]
	session, _ := job.step(t, "resolve")
	check, _ := job.step(t, "check")
	r, env := hostileRehearsal(t)
	head := strings.Repeat("ab", 20)
	r.write("host/pulls.tsv", "5\t"+head+"\n")
	r.write("host/main/resolve-prompt.md", "Rebase it.\n")
	r.write("host/sandbox-settings.sh", readText(t, ".github/sandbox-settings.sh"))
	for _, folder := range []string{"home", "work", "in", "out"} {
		r.write(filepath.Join("host", "pr-5", folder, ".keep"), "")
	}
	for _, folder := range []string{"check", "resolved", "logs"} {
		r.write(filepath.Join("host", folder, ".keep"), "")
	}
	workspace := keptWorkspace(t, r)
	out := filepath.Join(r.temp, "host", "pr-5", "out")
	env = with(env, map[string]string{"MAIN_SHA": head, "GITHUB_WORKSPACE": workspace,
		"PLANT": filepath.Join(out, "rebased.bundle") + ":" + filepath.Join(out, "message.md")})
	r.run(session, env, "")
	noSecretReached(t, "resolve.yml's session", r)
	r.run(check, env, "")
	noSecretReached(t, "resolve.yml's check step", r, "host/resolved")
}

// Every path a step reads after a session, in the session's own step once Claude has finished, or
// throughout it when it runs one session after another, and in every later step of its job, is in
// the host's own folder, or in the session's own output folder, which that step first checks holds
// nothing but plain files and folders where it should be, or in the checked-out workspace. The
// host's folder is root's /var/lib/awml-host, made by root before anything is put there, so no
// command a session runs, as the runner's user, can rename it or a folder above it away, and every
// session's sandbox denies it, all but a rebase session's own pull request's folder.
func TestEveryHostReadAfterASessionIsInTheHostsFolder(t *testing.T) {
	outputs := map[string]string{
		"ci.yml review": "$RUNNER_TEMP/session/out", "ci.yml classify": "$RUNNER_TEMP/session/out",
		"build.yml draft": "$RUNNER_TEMP/build", "fix.yml fix": "$RUNNER_TEMP/fix/out", "resolve.yml draft": "$HOST_DIR/pr-$number/out",
	}
	assigned := regexp.MustCompile(`(?m)^\s*([a-z_]+)="(\$(?:RUNNER_TEMP|HOST_DIR)[^"]*|\$[a-z_]+/[^"]*)"\s*$`)
	read := regexp.MustCompile(`\$(RUNNER_TEMP|HOST_DIR)(/[^"\s)]*)?`)
	made := regexp.MustCompile(`sudo install -d -o "\$\(id -u\)" -g "\$\(id -g\)" -m 700 "\$HOST_DIR"`)
	expand := func(script string) string {
		for _, m := range assigned.FindAllStringSubmatch(script, -1) {
			value := m[2]
			for _, n := range assigned.FindAllStringSubmatch(script, -1) {
				value = strings.ReplaceAll(value, "$"+n[1]+"/", n[2]+"/")
			}
			script = regexp.MustCompile(`\$`+m[1]+`\b`).ReplaceAllLiteralString(script, value)
		}
		return script
	}
	eachJob(t, func(file, name string, job workflowJob) {
		at := job.session()
		if at < 0 {
			return
		}
		out, ok := outputs[file+" "+name]
		if !ok {
			t.Errorf("%s's %s runs a session with no output folder named here", file, name)
			return
		}
		if job.Env["HOST_DIR"] != "/var/lib/awml-host" {
			t.Errorf("%s's %s keeps the host's folder at %q, not under root's /var/lib", file, name, job.Env["HOST_DIR"])
		}
		// Root makes the host's folder before any step puts anything there.
		first := -1
		for i, step := range job.Steps {
			if strings.Contains(step.Run, "$HOST_DIR") || strings.Contains(fmt.Sprint(step.With), "HOST_DIR") {
				first = i
				break
			}
		}
		if first < 0 || !made.MatchString(job.Steps[first].Run) ||
			strings.Index(job.Steps[first].Run, "$HOST_DIR") < made.FindStringIndex(job.Steps[first].Run)[0] {
			t.Errorf("%s's %s uses the host's folder before root makes it", file, name)
		}
		session := expand(job.Steps[at].Run)
		// The settings' call, with its continued lines; its first path is the session's home, and
		// every other folder it denies is the host's.
		call := regexp.MustCompile(`bash "\$(?:RUNNER_TEMP|HOST_DIR)/sandbox-settings\.sh"(?:[^\n]*\\\n)*[^\n]*\n`).FindString(session)
		var denied []string
		for _, arg := range regexp.MustCompile(`"(\$HOST_DIR[^"]*)"`).FindAllStringSubmatch(call, -1) {
			if !strings.HasSuffix(arg[1], "/home") {
				denied = append(denied, arg[1])
			}
		}
		loop := strings.Contains(session[strings.Index(session, "claude -p"):], "done <")
		if loop {
			// One session after another: each is denied every other pull request's folder, and all the
			// host's own files and folders.
			if !strings.Contains(call, `"${others[@]}"`) {
				t.Errorf("%s's %s lets a session write another pull request's folder", file, name)
			}
		} else if !contains(denied, "$HOST_DIR") {
			t.Errorf("%s's %s doesn't deny its session the host's folder: %s", file, name, call)
		}
		// What the host reads once Claude has finished: from the end of Claude's command on, or the
		// whole step when it runs sessions one after another.
		later := []workflowStep{{Run: session}}
		if !loop {
			claude := strings.Index(session, "claude -p")
			later[0].Run = session[claude+strings.Index(session[claude:], "2>&1"):]
		}
		later = append(later, job.Steps[at+1:]...)
		for i, step := range later {
			script := assigned.ReplaceAllString(expand(step.Run), "")
			// Within the step that runs one session after another, the next session's own folder
			// is the host's until it starts, and every earlier session was denied it.
			own := loop && i == 0
			for _, value := range step.With {
				script += "\n" + strings.NewReplacer("${{ runner.temp }}", "$RUNNER_TEMP", "${{ env.HOST_DIR }}", "$HOST_DIR").Replace(value)
			}
			for _, path := range read.FindAllString(script, -1) {
				if path == "$RUNNER_TEMP" || path == "$HOST_DIR" {
					continue
				}
				if own && (path == "$HOST_DIR/pr-$number" || strings.HasPrefix(path, "$HOST_DIR/pr-$number/")) {
					continue
				}
				safe := false
				for _, folder := range denied {
					if path == folder || strings.HasPrefix(path, folder+"/") {
						safe = true
					}
				}
				if path == out || strings.HasPrefix(path, out+"/") {
					safe = strings.Contains(script, `find "`+out+`" ! -type f ! -type d`) && strings.Contains(script, `realpath -e "`+out+`"`)
				}
				if !safe {
					t.Errorf("%s's %s reads %s after a session, which isn't in the host's own folder", file, name, path)
				}
			}
		}
	})
}
