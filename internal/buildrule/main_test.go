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
	assertVerdict(t, got, false, "No review rule matched this change.")
}

func TestEvaluateOpenFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings string
		review   bool
		reason   string
	}{
		{
			name:     "an Important finding",
			findings: `[{"severity": "Important"}]`,
			review:   true,
			reason:   "The reviews left 1 finding open.",
		},
		{
			name:     "a Nit and an Important finding",
			findings: `[{"severity": "Nit"}, {"severity": "Important"}]`,
			review:   true,
			reason:   "The reviews left 2 findings open.",
		},
		{
			name:     "only a pre-existing finding",
			findings: `[{"severity": "Pre-existing"}]`,
			review:   false,
			reason:   "No review rule matched this change.",
		},
		{
			name:     "no findings",
			findings: `[]`,
			review:   false,
			reason:   "No review rule matched this change.",
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
			assertVerdict(t, got, tt.review, tt.reason)
		})
	}
}

func TestEvaluateClassifier(t *testing.T) {
	t.Run("asks for review", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "main.go", "package main\n")
		classification := filepath.Join(t.TempDir(), "classification.json")
		writeFile(t, classification, `{"needs_review": true, "reasons": ["It changes how the parser reads scopes."]}`)

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{Classification: classification})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, true, "The classifier asked for a review.")
	})

	t.Run("can't skip a review the rules require", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "SPEC.md", "changed\n")
		classification := filepath.Join(t.TempDir(), "classification.json")
		writeFile(t, classification, `{"needs_review": false, "reasons": ["It is a small fix."]}`)

		got, err := evaluate(repo, config{ReviewPaths: []string{"SPEC.md"}, MaxChangedLines: 400},
			inputs{Classification: classification})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, true, "SPEC.md is a path that always needs review.")
	})

	t.Run("finding nothing adds nothing", func(t *testing.T) {
		repo := newRepo(t)
		stage(t, repo, "main.go", "package main\n")
		classification := filepath.Join(t.TempDir(), "classification.json")
		writeFile(t, classification, `{"needs_review": false, "reasons": ["It is a small fix."]}`)

		got, err := evaluate(repo, config{MaxChangedLines: 400}, inputs{Classification: classification})
		if err != nil {
			t.Fatal(err)
		}
		assertVerdict(t, got, false, "No review rule matched this change.")
	})
}

func TestEvaluateRejectsUnreadableRecords(t *testing.T) {
	tests := []struct {
		name           string
		findings       string
		classification string
	}{
		{name: "unknown severity", findings: `[{"severity": "Minor"}]`},
		{name: "findings not a list", findings: `{"severity": "Nit"}`},
		{name: "missing findings file", findings: "missing"},
		{name: "no needs_review", classification: `{"reasons": []}`},
		{name: "needs_review not a boolean", classification: `{"needs_review": "yes"}`},
		{name: "missing classification file", classification: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			stage(t, repo, "main.go", "package main\n")
			dir := t.TempDir()
			var in inputs
			if tt.findings != "" {
				in.Findings = filepath.Join(dir, "findings.json")
				if tt.findings != "missing" {
					writeFile(t, in.Findings, tt.findings)
				}
			}
			if tt.classification != "" {
				in.Classification = filepath.Join(dir, "classification.json")
				if tt.classification != "missing" {
					writeFile(t, in.Classification, tt.classification)
				}
			}

			if got, err := evaluate(repo, config{MaxChangedLines: 400}, in); err == nil {
				t.Fatalf("evaluate() = %#v, want an error", got)
			}
		})
	}
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
			wantReview := tt.name != "similar prefix is not enough"
			if got.Review != wantReview {
				t.Fatalf("evaluate().Review = %v, want %v; reasons: %q", got.Review, wantReview, got.Reasons)
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
			reason: "thing_test.go removes a test function.",
		},
		{
			name:   "deleted Go test file",
			path:   "thing_test.go",
			before: "package thing\n",
			reason: "thing_test.go deletes a test file.",
		},
		{
			name:   "deleted JavaScript test file",
			path:   "wasm/thing.test.mjs",
			before: "export const answer = 42;\n",
			reason: "wasm/thing.test.mjs deletes a test file.",
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
			assertVerdict(t, got, true, tt.reason)
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
		assertVerdict(t, got, true, "The patch changes 402 lines, more than the limit of 400.")
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
		assertVerdict(t, got, false, "No review rule matched this change.")
	})
}

func assertVerdict(t *testing.T, got verdict, review bool, reason string) {
	t.Helper()
	if got.Review != review {
		t.Fatalf("Review = %v, want %v; reasons: %q", got.Review, review, got.Reasons)
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
