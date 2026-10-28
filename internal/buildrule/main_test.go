package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yml")
	writeFile(t, path, `review_paths: [SPEC.md, .github/]
max_changed_lines: 400
`)

	got, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.ReviewPaths, ",") != "SPEC.md,.github/" ||
		got.MaxChangedLines != 400 {
		t.Fatalf("loadConfig() = %#v", got)
	}
}

func TestEvaluateStraightforwardPatch(t *testing.T) {
	repo := newRepo(t)
	stage(t, repo, "main.go", "package main\n")

	got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(t, got, "ship", "No rule moved the verdict towards ask.")
}

// The reviews' findings can only move the verdict towards ask: an Important one asks, and a Nit
// left unfixed means at least show. A pre-existing one is about code the change didn't write.
func TestEvaluateFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings string
		outcome  string
		reason   string
	}{
		{
			name:     "an Important finding",
			findings: `[{"severity": "Important"}]`,
			outcome:  "ask",
			reason:   "The reviews left 1 Important finding.",
		},
		{
			name:     "a Nit and an Important finding",
			findings: `[{"severity": "Nit"}, {"severity": "Important"}]`,
			outcome:  "ask",
			reason:   "The reviews left 1 Important finding.",
		},
		{
			name:     "two Nits",
			findings: `[{"severity": "Nit"}, {"severity": "Nit"}]`,
			outcome:  "show",
			reason:   "The reviews left 2 Nits unfixed.",
		},
		{
			name:     "only a pre-existing finding",
			findings: `[{"severity": "Pre-existing"}]`,
			outcome:  "ship",
			reason:   "No rule moved the verdict towards ask.",
		},
		{
			name:     "no findings",
			findings: `[]`,
			outcome:  "ship",
			reason:   "No rule moved the verdict towards ask.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, "main.go", "package main\n")
			findings := filepath.Join(t.TempDir(), "findings.json")
			writeFile(t, findings, tt.findings)

			got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{Findings: findings})
			if err != nil {
				t.Fatal(err)
			}
			assertVerdict(t, got, tt.outcome, tt.reason)
		})
	}
}

// Classify's verdict is where the rule starts, and the rule can only move it towards ask. A
// missing verdict asks.
func TestEvaluateClassify(t *testing.T) {
	tests := []struct {
		name     string
		classify string
		path     string
		outcome  string
		reason   string
	}{
		{name: "ship stays ship", classify: "ship", path: "main.go", outcome: "ship", reason: "No rule moved the verdict towards ask."},
		{name: "show stays show", classify: "show", path: "main.go", outcome: "show", reason: "Classify said to show it."},
		{name: "ask stays ask", classify: "ask", path: "main.go", outcome: "ask", reason: "Classify asked for a review."},
		{name: "no verdict asks", classify: "missing", path: "main.go", outcome: "ask", reason: "Classify gave no verdict, so the change waits for review."},
		{name: "the rule's paths move ship to ask", classify: "ship", path: "SPEC.md", outcome: "ask", reason: "`SPEC.md` is a path that always needs review."},
		{name: "the rule's paths move show to ask", classify: "show", path: "SPEC.md", outcome: "ask", reason: "`SPEC.md` is a path that always needs review."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, tt.path, "changed\n")

			got, err := evaluate(repo, config{ReviewPaths: []string{"SPEC.md"}, MaxChangedLines: 400},
				inputs{Classify: tt.classify})
			if err != nil {
				t.Fatal(err)
			}
			assertVerdict(t, got, tt.outcome, tt.reason)
		})
	}
}

func TestEvaluateRejectsUnreadableRecords(t *testing.T) {
	tests := []struct {
		name     string
		findings string
		classify string
	}{
		{name: "unknown severity", findings: `[{"severity": "Minor"}]`},
		{name: "findings not a list", findings: `{"severity": "Nit"}`},
		{name: "missing findings file", findings: "missing"},
		{name: "an unknown verdict", classify: "merge"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, "main.go", "package main\n")
			in := inputs{Classify: tt.classify}
			if tt.findings != "" {
				in.Findings = filepath.Join(t.TempDir(), "findings.json")
				if tt.findings != "missing" {
					writeFile(t, in.Findings, tt.findings)
				}
			}

			if got, err := evaluate(repo, config{MaxChangedLines: 400}, in); err == nil {
				t.Fatalf("evaluate() = %#v, want an error", got)
			}
		})
	}
}

// The rule reads the branch's commits since main from git. A fix commit, known by the line its
// message carries, means at least show. A commit the factory's app didn't both write and commit,
// or a merge, asks, since only a person can say what it brings.
func TestEvaluateTheBranchsCommits(t *testing.T) {
	const factory = "awml-factory[bot]"
	const app = "Factory <41+awml-factory[bot]@users.noreply.github.com>"
	const person = "A Person <person@example.invalid>"
	type made struct {
		author, committer, message string
	}
	tests := []struct {
		name    string
		commits []made
		outcome string
		reason  string
	}{
		{
			name:    "one build commit of the app's",
			commits: []made{{app, app, "Build the thing\n\nCloses #1\n"}},
			outcome: "ship",
			reason:  "No rule moved the verdict towards ask.",
		},
		{
			name:    "a fix commit on top",
			commits: []made{{app, app, "Build the thing\n"}, {app, app, "Fix the findings\n\n" + fixLine + "\n"}},
			outcome: "show",
			reason:  "The branch carries 1 fix commit.",
		},
		{
			name:    "the fix line only inside a longer line counts for nothing",
			commits: []made{{app, app, "Build the thing\n\nSays " + fixLine + " in passing\n"}},
			outcome: "ship",
			reason:  "No rule moved the verdict towards ask.",
		},
		{
			name:    "a person's commit",
			commits: []made{{app, app, "Build the thing\n"}, {person, person, "Tweak\n"}},
			outcome: "ask",
			reason:  "A commit since main isn't the factory's.",
		},
		{
			name:    "a person's commit that the app rebased",
			commits: []made{{person, app, "Tweak\n"}},
			outcome: "ask",
			reason:  "A commit since main isn't the factory's.",
		},
		{
			name:    "a commit that only claims the app's name",
			commits: []made{{"Factory <awml-factory[bot]@example.invalid>", app, "Build\n"}},
			outcome: "ask",
			reason:  "A commit since main isn't the factory's.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, "main.go", "package main\n")
			commit(t, repo)
			base := gitOutput(t, repo, "rev-parse", "HEAD")
			for at, c := range tt.commits {
				stage(t, repo, "main.go", fmt.Sprintf("package main\n\n// %d\n", at))
				commitAs(t, repo, c.author, c.committer, c.message)
			}
			head := gitOutput(t, repo, "rev-parse", "HEAD")
			judgeAt(t, repo, base, head)

			got, err := evaluate(repo, config{MaxChangedLines: 400},
				inputs{Classify: "ship", Head: head, Factory: factory})
			if err != nil {
				t.Fatal(err)
			}
			assertVerdict(t, got, tt.outcome, tt.reason)
		})
	}

	t.Run("a merge commit", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "main.go", "package main\n")
		commit(t, repo)
		base := gitOutput(t, repo, "rev-parse", "HEAD")
		runGit(t, repo, "checkout", "-q", "-b", "side")
		stage(t, repo, "side.go", "package main\n")
		commitAs(t, repo, app, app, "Side\n")
		runGit(t, repo, "checkout", "-q", "-")
		stage(t, repo, "other.go", "package main\n")
		commitAs(t, repo, app, app, "Other\n")
		runGit(t, repo, "-c", "user.name=Factory", "-c", "user.email=41+awml-factory[bot]@users.noreply.github.com",
			"merge", "-q", "--no-edit", "side")
		head := gitOutput(t, repo, "rev-parse", "HEAD")
		judgeAt(t, repo, base, head)

		got, err := evaluate(repo, config{MaxChangedLines: 400},
			inputs{Classify: "ship", Head: head, Factory: factory})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "The branch since main holds a merge commit.")
	})
}

func TestEvaluateReviewPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "exact path", path: "SPEC.md"},
		{name: "directory prefix", path: ".github/workflows/build.yml"},
		{name: "similar prefix is not enough", path: ".github-old/build.yml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, tt.path, "changed\n")

			got, err := evaluate(repo, config{
				ReviewPaths:     []string{"SPEC.md", ".github/"},
				MaxChangedLines: 400,
			}, inputs{})
			if err != nil {
				t.Fatal(err)
			}
			want := "ask"
			if tt.name == "similar prefix is not enough" {
				want = "ship"
			}
			if got.Outcome != want {
				t.Fatalf("evaluate().Outcome = %q, want %q; reasons: %q", got.Outcome, want, got.Reasons)
			}
		})
	}
}

func TestEvaluateDeletedTests(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		before string
		after  *string
		reason string
	}{
		{
			name:   "removed Go test function",
			path:   "thing_test.go",
			before: "package thing\n\nfunc TestKept(t *testing.T) {}\nfunc TestRemoved(t *testing.T) {}\n",
			after:  stringPointer("package thing\n\nfunc TestKept(t *testing.T) {}\n"),
			reason: "`thing_test.go` removes a test function.",
		},
		{
			name:   "deleted Go test file",
			path:   "thing_test.go",
			before: "package thing\n",
			reason: "`thing_test.go` deletes a test file.",
		},
		{
			name:   "deleted JavaScript test file",
			path:   "wasm/thing.test.mjs",
			before: "export const answer = 42;\n",
			reason: "`wasm/thing.test.mjs` deletes a test file.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, tt.path, tt.before)
			commit(t, repo)
			if tt.after == nil {
				if err := os.Remove(filepath.Join(repo, tt.path)); err != nil {
					t.Fatal(err)
				}
				runGit(t, repo, "add", "-A")
			} else {
				stage(t, repo, tt.path, *tt.after)
			}

			got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
			if err != nil {
				t.Fatal(err)
			}
			assertVerdict(t, got, "ask", tt.reason)
		})
	}
}

func TestEvaluateChangedLineLimit(t *testing.T) {
	t.Run("counts additions and removals", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "large.txt", numberedLines("old", 201))
		commit(t, repo)
		stage(t, repo, "large.txt", numberedLines("new", 201))

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "The patch changes 402 lines, more than the limit of 400.")
	})

	t.Run("ignores generated reference copies", func(t *testing.T) {
		repo := newRepo(t)
		path := "plugins/awml/skills/awml/references/spec.md"
		stage(t, repo, path, numberedLines("old", 201))
		commit(t, repo)
		stage(t, repo, path, numberedLines("new", 201))

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ship", "No rule moved the verdict towards ask.")
	})
}

func TestEvaluateBinaryFiles(t *testing.T) {
	t.Run("a new .gitattributes can't hide the patch's lines", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, ".gitattributes", "* -diff\n")
		stage(t, repo, "large.txt", numberedLines("new", 500))

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`.gitattributes` changes how git reads files.")
		assertVerdict(t, got, "ask", "The patch changes 501 lines, more than the limit of 400.")
	})

	t.Run("a .gitattributes in a subdirectory", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "docs/.gitattributes", "*.md text\n")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`docs/.gitattributes` changes how git reads files.")
	})

	t.Run("a .gitattributes can't hide a removed test function", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "thing_test.go", "package thing\n\nfunc TestKept(t *testing.T) {}\nfunc TestRemoved(t *testing.T) {}\n")
		commit(t, repo)
		stage(t, repo, ".gitattributes", "*_test.go binary\n")
		stage(t, repo, "thing_test.go", "package thing\n\nfunc TestKept(t *testing.T) {}\n")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`thing_test.go` removes a test function.")
		if contains(got.Reasons, "`thing_test.go` is a binary file.") {
			t.Fatalf("reasons = %q, want thing_test.go read as text", got.Reasons)
		}
	})

	t.Run("a NUL byte in a test that also loses a test function", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "thing_test.go", "package thing\n\nfunc TestKept(t *testing.T) {}\nfunc TestRemoved(t *testing.T) {}\n")
		commit(t, repo)
		stage(t, repo, "thing_test.go", "package thing\n\x00\nfunc TestKept(t *testing.T) {}\n")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`thing_test.go` is a binary file.")
		assertVerdict(t, got, "ask", "`thing_test.go` removes a test function.")
	})

	t.Run("a new image or font that starts as its kind does", func(t *testing.T) {
		for path, contents := range map[string]string{
			"examples/picture.png":            "\x89PNG\r\n\x1a\n\x00pixels",
			"examples/moving.gif":             "GIF89a\x00frames",
			"examples/photo.jpg":              "\xff\xd8\xff\x00photo",
			"examples/photo.jpeg":             "\xff\xd8\xff\x00photo",
			"examples/picture.webp":           "RIFF\x10\x00\x00\x00WEBP\x00pixels",
			"examples/favicon.ico":            "\x00\x00\x01\x00\x01",
			"examples/fonts/Bakery.ttf":       "\x00\x01\x00\x00\x00glyphs",
			"examples/fonts/Bakery.otf":       "OTTO\x00glyphs",
			"examples/fonts/Bakery.woff":      "wOFF\x00glyphs",
			"examples/fonts/Bakery.woff2":     "wOF2\x00glyphs",
			"examples/fonts/Bakery-Mac.ttf":   "true\x00glyphs",
			"examples/older-moving-image.gif": "GIF87a\x00frames",
		} {
			repo := newRepo(t)
			stage(t, repo, path, contents)

			got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != "ship" {
				t.Errorf("a new %s that starts as its kind does needs review: %q", path, got.Reasons)
			}
		}
	})

	t.Run("an image or font that doesn't start as its name says", func(t *testing.T) {
		for path, contents := range map[string]string{
			"examples/picture.png":        "\x89PNG\x00pixels",
			"examples/moving.gif":         "GIF90a\x00frames",
			"examples/picture.webp":       "RIFF\x10\x00\x00\x00WAVE\x00sound",
			"examples/fonts/Bakery.woff2": "wOFF\x00glyphs",
			"examples/fonts/Bakery.ttf":   "#!/bin/sh\x00",
		} {
			repo := newRepo(t)
			stage(t, repo, path, contents)

			got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
			if err != nil {
				t.Fatal(err)
			}
			assertVerdict(t, got, "ask", "`"+path+"` is binary, and doesn't start as its name says, so the rule can't read it.")
		}
	})

	t.Run("an image that replaces one on main", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "examples/picture.png", "\x89PNG\r\n\x1a\n\x00old")
		commit(t, repo)
		stage(t, repo, "examples/picture.png", "\x89PNG\r\n\x1a\n\x00new")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`examples/picture.png` replaces a binary file on main, which only a person can compare.")
	})

	t.Run("a deleted image leaves nothing to read", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "examples/picture.png", "\x89PNG\r\n\x1a\n\x00old")
		commit(t, repo)
		runGit(t, repo, "rm", "-q", "examples/picture.png")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ship", "No rule moved the verdict towards ask.")
	})

	t.Run("an image larger than the limit", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "examples/small.png", "\x89PNG\r\n\x1a\n\x00"+strings.Repeat("\x00", maxBinaryBytes-9))
		stage(t, repo, "examples/large.png", "\x89PNG\r\n\x1a\n\x00"+strings.Repeat("\x00", maxBinaryBytes-8))

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`examples/large.png` is binary and larger than 1 MiB, the most an image or font may be without review.")
		for _, reason := range got.Reasons {
			if strings.Contains(reason, "small.png") {
				t.Errorf("an image at the limit needs review: %q", reason)
			}
		}
	})

	t.Run("a font with a capital extension, as the review workflow reads it, case and all", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "examples/fonts/Inter.WOFF2", "wOF2\x00")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != "ask" {
			t.Errorf("a binary Inter.WOFF2 merged by itself, though bring-patch.sh's list, which is case-sensitive, refuses it: %+v", got)
		}
	})

	t.Run("an image under .github", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, ".github/picture.png", "\x89PNG\x00")

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, "ask", "`.github/picture.png` is a binary file.")
	})
}

// A reason reaches the pull request's description, so the path in it sits in a code span, escaped
// as JSON escapes it, so that no name can end the span or the line, or add a bullet of its own.
func TestEvaluateShowsEachPathInACodeSpan(t *testing.T) {
	repo := newRepo(t)
	stage(t, repo, "docs/a`b.md", "changed\n")
	stage(t, repo, "docs/c\n- Merged by the maintainer.md", "changed\n")
	stage(t, repo, "docs/<!-- d.md", "changed\n")

	got, err := evaluate(repo, config{ReviewPaths: []string{"docs/"}, MaxChangedLines: 400}, inputs{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"`docs/a\\u0060b.md` is a path that always needs review.",
		"`docs/c\\n- Merged by the maintainer.md` is a path that always needs review.",
		"`docs/\\u003c!-- d.md` is a path that always needs review.",
	} {
		assertVerdict(t, got, "ask", want)
	}
	for _, reason := range got.Reasons {
		if strings.ContainsAny(reason, "\n<") || strings.Count(reason, "`") != 2 {
			t.Errorf("a reason can break out of its code span: %q", reason)
		}
	}
}

// A symbolic link's target is read wherever the link is followed, so a new or changed one always
// needs review.
func TestEvaluateSymbolicLinks(t *testing.T) {
	repo := newRepo(t)
	if err := os.Symlink("/proc/self/environ", filepath.Join(repo, "notes.md")); err != nil {
		t.Skip("can't make a symbolic link here:", err)
	}
	runGit(t, repo, "add", "notes.md")

	got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{})
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(t, got, "ask", "`notes.md` is a symbolic link, so it always needs review.")
}

func assertVerdict(t *testing.T, got verdict, outcome string, reason string) {
	t.Helper()
	if got.Outcome != outcome {
		t.Fatalf("Outcome = %q, want %q; reasons: %q", got.Outcome, outcome, got.Reasons)
	}
	if !contains(got.Reasons, reason) {
		t.Fatalf("reasons = %q, want %q", got.Reasons, reason)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.invalid")
	return dir
}

func stage(t *testing.T, repo, path, contents string) {
	t.Helper()
	fullPath := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fullPath, contents)
	runGit(t, repo, "add", "--", path)
}

// commitAs commits what's staged with the author, committer and message given.
func commitAs(t *testing.T, repo, author, committer, message string) {
	t.Helper()
	name, email, _ := strings.Cut(strings.TrimSuffix(committer, ">"), " <")
	command := exec.Command("git", "-C", repo, "-c", "user.name="+name, "-c", "user.email="+email,
		"commit", "-q", "--author", author, "-F", "-")
	command.Stdin = strings.NewReader(message)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}
}

// judgeAt leaves the repository as land.yml judges a commit: the files and HEAD at the base on
// main, and the index holding the head's tree.
func judgeAt(t *testing.T, repo, base, head string) {
	t.Helper()
	runGit(t, repo, "checkout", "-q", "--detach", base)
	runGit(t, repo, "read-tree", head)
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func commit(t *testing.T, repo string) {
	t.Helper()
	runGit(t, repo, "commit", "-qm", "fixture")
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stringPointer(value string) *string { return &value }

func numberedLines(prefix string, count int) string {
	var result strings.Builder
	for number := 0; number < count; number++ {
		fmt.Fprintf(&result, "%s %03d\n", prefix, number)
	}
	return result.String()
}

// A gitlink pins another repository's commit in the tree, whose contents no diff here shows, so a
// new or changed one always needs review.
func TestEvaluateGitlinks(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("ab", 20)+",vendor/lib")

	got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{Classify: "ship"})
	if err != nil {
		t.Fatal(err)
	}
	assertVerdict(t, got, "ask", "`vendor/lib` pins another repository's commit, so it always needs review.")
}
