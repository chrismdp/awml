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

// The most an image or font may be and still merge by itself, 1 MiB. The drawings' own font,
// examples/fonts/Inter-Regular.ttf, is a fifteenth of that.
const maxBinaryBytes = 1 << 20

// A symbolic link's mode in git, and the mode git gives a path the change deletes.
const (
	symbolicLink = "120000"
	noFile       = "000000"
)

// The signatures the images and fonts the rule lets through must start with, by extension, matched
// case and all as allowedBinary matches them. The PNG one is the one the build's open job checks
// its pictures by.
var signatures = map[string][]string{
	".png":   {"\x89PNG\r\n\x1a\n"},
	".gif":   {"GIF87a", "GIF89a"},
	".jpg":   {"\xff\xd8\xff"},
	".jpeg":  {"\xff\xd8\xff"},
	".ico":   {"\x00\x00\x01\x00"},
	".ttf":   {"\x00\x01\x00\x00", "true"},
	".otf":   {"OTTO"},
	".woff":  {"wOFF"},
	".woff2": {"wOF2"},
}

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

func evaluate(repoDir string, cfg config, in inputs) (verdict, error) {
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

	repo, err := openRepository(repoDir)
	if err != nil {
		return verdict{}, err
	}

	paths, err := changedPaths(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range paths {
		for _, protected := range cfg.ReviewPaths {
			if path == protected || (strings.HasSuffix(protected, "/") && strings.HasPrefix(path, protected)) {
				reasons = append(reasons, fmt.Sprintf("%s is a path that always needs review.", shown(path)))
				break
			}
		}
	}
	for _, path := range paths {
		if path == ".gitattributes" || strings.HasSuffix(path, "/.gitattributes") {
			reasons = append(reasons, fmt.Sprintf("%s changes how git reads files.", shown(path)))
		}
	}

	deleted, err := deletedPaths(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range deleted {
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.mjs") {
			reasons = append(reasons, fmt.Sprintf("%s deletes a test file.", shown(path)))
		}
	}

	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") || containsPath(deleted, path) {
			continue
		}
		// --text reads a file with a NUL byte as text too, so it can't hide a removed test.
		diff, err := repo.git("diff", "--cached", "--text", "--unified=0", "--no-color", "--no-ext-diff", "--", path)
		if err != nil {
			return verdict{}, err
		}
		if removedGoTest.Match(diff) {
			reasons = append(reasons, fmt.Sprintf("%s removes a test function.", shown(path)))
		}
	}

	// A symbolic link's target is read wherever the link is followed, so a new or changed one
	// always needs review.
	modes, err := changedModes(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range paths {
		if modes[path].after == symbolicLink {
			reasons = append(reasons, fmt.Sprintf("%s is a symbolic link, so it always needs review.", shown(path)))
		}
	}

	// A binary file can't be counted or read, so it needs review, unless it's a new image or font,
	// no larger than the limit, that starts as its kind does. One that replaces a file on main could
	// change what a page shows, and no diff shows how. A deleted one leaves nothing to read.
	changedLines, binary, err := countChangedLines(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range binary {
		if !allowedBinary(path) {
			reasons = append(reasons, fmt.Sprintf("%s is a binary file.", shown(path)))
			continue
		}
		mode := modes[path]
		if mode.after == noFile {
			continue
		}
		start, size, err := staged(repo, path)
		if err != nil {
			return verdict{}, err
		}
		if !startsAsNamed(path, start) {
			reasons = append(reasons, fmt.Sprintf("%s is binary, and doesn't start as its name says, so the rule can't read it.", shown(path)))
		}
		if mode.before != noFile {
			reasons = append(reasons, fmt.Sprintf("%s replaces a binary file on main, which only a person can compare.", shown(path)))
		}
		if size > maxBinaryBytes {
			reasons = append(reasons, fmt.Sprintf("%s is binary and larger than 1 MiB, the most an image or font may be without review.", shown(path)))
		}
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

// allowedBinary says whether a binary file is an image or a font, which a diff can't show anyway.
// Under .github/ even those need review. It matches the extension's case exactly, as the list in
// .github/workflows/review.yml does, so the two agree.
func allowedBinary(path string) bool {
	if strings.HasPrefix(path, ".github/") {
		return false
	}
	switch filepath.Ext(path) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".ttf", ".otf", ".woff", ".woff2":
		return true
	}
	return false
}

// startsAsNamed says whether a file starts with a signature its extension implies. A WebP image is
// a RIFF file whose form, at its ninth byte, is WEBP.
func startsAsNamed(path string, start []byte) bool {
	extension := filepath.Ext(path)
	if extension == ".webp" {
		return len(start) >= 12 && string(start[:4]) == "RIFF" && string(start[8:12]) == "WEBP"
	}
	for _, signature := range signatures[extension] {
		if bytes.HasPrefix(start, []byte(signature)) {
			return true
		}
	}
	return false
}

// staged reads the first bytes of a path as the change stages it, and its size.
func staged(repo repository, path string) ([]byte, int, error) {
	output, err := repo.git("cat-file", "-s", ":0:"+path)
	if err != nil {
		return nil, 0, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return nil, 0, fmt.Errorf("unexpected size %q of %s", output, path)
	}
	contents, err := repo.git("cat-file", "blob", ":0:"+path)
	if err != nil {
		return nil, 0, err
	}
	if len(contents) > 16 {
		contents = contents[:16]
	}
	return contents, size, nil
}

type modeChange struct{ before, after string }

// changedModes gives each changed path's mode on main and once the change is applied, "000000"
// where there's no file, from git's raw records: ":<old mode> <new mode> <old id> <new id>
// <status>", then the path.
func changedModes(repo repository) (map[string]modeChange, error) {
	output, err := repo.git("diff", "--cached", "--raw", "--no-renames", "-z", "--")
	if err != nil {
		return nil, err
	}
	fields := nulFields(output)
	modes := map[string]modeChange{}
	for at := 0; at+1 < len(fields); at += 2 {
		parts := strings.Fields(strings.TrimPrefix(fields[at], ":"))
		if len(parts) < 2 || len(parts[0]) != 6 || len(parts[1]) != 6 {
			return nil, fmt.Errorf("unexpected raw record %q", fields[at])
		}
		modes[fields[at+1]] = modeChange{before: parts[0], after: parts[1]}
	}
	return modes, nil
}

// shown puts a path in a code span for a reason, escaped as JSON escapes it, with a backtick
// escaped too, since the reasons reach the pull request's description: no name can end the span
// or the line there.
func shown(path string) string {
	quoted, _ := json.Marshal(path)
	inner := strings.ReplaceAll(string(quoted[1:len(quoted)-1]), "`", `\u0060`)
	return "`" + inner + "`"
}

func changedPaths(repo repository) ([]string, error) {
	output, err := repo.git("diff", "--cached", "--name-only", "--no-renames", "-z", "--")
	if err != nil {
		return nil, err
	}
	return nulFields(output), nil
}

func deletedPaths(repo repository) ([]string, error) {
	output, err := repo.git("diff", "--cached", "--diff-filter=D", "--name-only", "--no-renames", "-z", "--")
	if err != nil {
		return nil, err
	}
	return nulFields(output), nil
}

// countChangedLines counts the lines the patch adds and removes, and lists the files git reads
// as binary, whose lines it can't count.
func countChangedLines(repo repository) (int, []string, error) {
	output, err := repo.git("diff", "--cached", "--numstat", "--no-renames", "-z", "--")
	if err != nil {
		return 0, nil, err
	}
	total := 0
	var binary []string
	for _, record := range nulFields(output) {
		fields := strings.SplitN(record, "\t", 3)
		if len(fields) != 3 {
			return 0, nil, fmt.Errorf("unexpected numstat record %q", record)
		}
		if fields[0] == "-" && fields[1] == "-" {
			binary = append(binary, fields[2])
			continue
		}
		if strings.HasPrefix(fields[2], generatedReferences) {
			continue
		}
		for _, value := range fields[:2] {
			number, err := strconv.Atoi(value)
			if err != nil {
				return 0, nil, fmt.Errorf("unexpected numstat count %q", value)
			}
			total += number
		}
	}
	return total, binary, nil
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

// repository runs git with the empty tree as the source of attributes, so a .gitattributes in the
// patch can't decide how its own diff is read.
type repository struct {
	dir       string
	emptyTree string
}

func openRepository(dir string) (repository, error) {
	output, err := git(dir, "hash-object", "-t", "tree", os.DevNull)
	if err != nil {
		return repository{}, err
	}
	return repository{dir: dir, emptyTree: strings.TrimSpace(string(output))}, nil
}

func (repo repository) git(args ...string) ([]byte, error) {
	return git(repo.dir, append([]string{"--attr-source=" + repo.emptyTree}, args...)...)
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
