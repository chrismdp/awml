package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const generatedReferences = "plugins/awml/skills/awml/references/"

var removedGoTest = regexp.MustCompile(`(?m)^-[[:space:]]*func[[:space:]]+Test`)

type config struct {
	ReviewPaths     []string `yaml:"review_paths"`
	MaxChangedLines int      `yaml:"max_changed_lines"`
}

// The reviews' record and the classifier's verdict, as files the review job wrote. Either may be
// left out: the classifier doesn't run when the reviews leave an Important finding open.
type inputs struct {
	Findings       string
	Classification string
}

type verdict struct {
	Review  bool
	Reasons []string
}

func main() {
	repo := flag.String("repo", ".", "repository containing the staged patch")
	rules := flag.String("rules", ".github/build-rules.yml", "build rules file")
	findings := flag.String("findings", "", "the reviews' findings, as JSON")
	classification := flag.String("classification", "", "the classifier's verdict, as JSON")
	flag.Parse()

	cfg, err := loadConfig(*rules)
	if err != nil {
		fatal(err)
	}
	result, err := evaluate(*repo, cfg, inputs{Findings: *findings, Classification: *classification})
	if err != nil {
		fatal(err)
	}
	if result.Review {
		fmt.Println("review")
	} else {
		fmt.Println("straightforward")
	}
	for _, reason := range result.Reasons {
		fmt.Println(reason)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func loadConfig(path string) (config, error) {
	file, err := os.Open(path)
	if err != nil {
		return config{}, err
	}
	defer file.Close()

	var cfg config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("read build rules: %w", err)
	}
	if cfg.MaxChangedLines < 0 {
		return config{}, errors.New("max_changed_lines must not be negative")
	}
	for _, path := range cfg.ReviewPaths {
		if strings.TrimSpace(path) == "" {
			return config{}, errors.New("review_paths must not contain an empty path")
		}
	}
	return cfg, nil
}

func evaluate(repo string, cfg config, in inputs) (verdict, error) {
	var reasons []string

	if in.Findings != "" {
		open, err := openFindings(in.Findings)
		if err != nil {
			return verdict{}, err
		}
		if open == 1 {
			reasons = append(reasons, "The reviews left 1 finding open.")
		} else if open > 1 {
			reasons = append(reasons, fmt.Sprintf("The reviews left %d findings open.", open))
		}
	}

	if in.Classification != "" {
		asked, err := classifierAsked(in.Classification)
		if err != nil {
			return verdict{}, err
		}
		if asked {
			reasons = append(reasons, "The classifier asked for a review.")
		}
	}

	paths, err := changedPaths(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range paths {
		for _, protected := range cfg.ReviewPaths {
			if path == protected || (strings.HasSuffix(protected, "/") && strings.HasPrefix(path, protected)) {
				reasons = append(reasons, fmt.Sprintf("%s is a path that always needs review.", path))
				break
			}
		}
	}

	deleted, err := deletedPaths(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range deleted {
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.mjs") {
			reasons = append(reasons, fmt.Sprintf("%s deletes a test file.", path))
		}
	}

	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") || containsPath(deleted, path) {
			continue
		}
		diff, err := git(repo, "diff", "--cached", "--unified=0", "--no-color", "--no-ext-diff", "--", path)
		if err != nil {
			return verdict{}, err
		}
		if removedGoTest.Match(diff) {
			reasons = append(reasons, fmt.Sprintf("%s removes a test function.", path))
		}
	}

	changedLines, err := countChangedLines(repo)
	if err != nil {
		return verdict{}, err
	}
	if changedLines > cfg.MaxChangedLines {
		reasons = append(reasons, fmt.Sprintf(
			"The patch changes %d lines, more than the limit of %d.",
			changedLines, cfg.MaxChangedLines,
		))
	}

	if len(reasons) == 0 {
		return verdict{Reasons: []string{"No review rule matched this change."}}, nil
	}
	return verdict{Review: true, Reasons: reasons}, nil
}

// openFindings counts the findings about the change itself. A pre-existing one is about code the
// change didn't write, so it's shown but leaves the change straightforward.
func openFindings(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read the reviews' findings: %w", err)
	}
	var findings []struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(data, &findings); err != nil {
		return 0, fmt.Errorf("read the reviews' findings: %w", err)
	}
	open := 0
	for _, finding := range findings {
		switch finding.Severity {
		case "Important", "Nit":
			open++
		case "Pre-existing":
		default:
			return 0, fmt.Errorf("a finding has an unknown severity %q", finding.Severity)
		}
	}
	return open, nil
}

// classifierAsked reads whether the classifier asked for a review. It can only add a review, so
// its answer of no changes nothing.
func classifierAsked(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read the classifier's verdict: %w", err)
	}
	var classification struct {
		NeedsReview *bool `json:"needs_review"`
	}
	if err := json.Unmarshal(data, &classification); err != nil {
		return false, fmt.Errorf("read the classifier's verdict: %w", err)
	}
	if classification.NeedsReview == nil {
		return false, errors.New("the classifier's verdict has no needs_review")
	}
	return *classification.NeedsReview, nil
}

func changedPaths(repo string) ([]string, error) {
	output, err := git(repo, "diff", "--cached", "--name-only", "--no-renames", "-z", "--")
	if err != nil {
		return nil, err
	}
	return nulFields(output), nil
}

func deletedPaths(repo string) ([]string, error) {
	output, err := git(repo, "diff", "--cached", "--diff-filter=D", "--name-only", "--no-renames", "-z", "--")
	if err != nil {
		return nil, err
	}
	return nulFields(output), nil
}

func countChangedLines(repo string) (int, error) {
	output, err := git(repo, "diff", "--cached", "--numstat", "--no-renames", "-z", "--")
	if err != nil {
		return 0, err
	}
	total := 0
	for _, record := range nulFields(output) {
		fields := strings.SplitN(record, "\t", 3)
		if len(fields) != 3 {
			return 0, fmt.Errorf("unexpected numstat record %q", record)
		}
		if strings.HasPrefix(fields[2], generatedReferences) {
			continue
		}
		for _, value := range fields[:2] {
			if value == "-" {
				continue
			}
			number, err := strconv.Atoi(value)
			if err != nil {
				return 0, fmt.Errorf("unexpected numstat count %q", value)
			}
			total += number
		}
	}
	return total, nil
}

func nulFields(output []byte) []string {
	output = bytes.TrimSuffix(output, []byte{0})
	if len(output) == 0 {
		return nil
	}
	parts := bytes.Split(output, []byte{0})
	result := make([]string, len(parts))
	for index, part := range parts {
		result[index] = string(part)
	}
	return result
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func git(repo string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", filepath.Clean(repo)}, args...)...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitError.Stderr)))
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}
