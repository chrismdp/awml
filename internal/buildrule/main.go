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

// What land.yml read from GitHub about the commit it judges: the reviews' findings, as a file of
// their severities, Classify's verdict, and the commit with the factory app's login, whose own
// commits since main the rule reads from git. Each may be left out, as the tests do; land.yml gives
// them all, and "missing" for a Classify check that gave no verdict.
type inputs struct {
	Findings string
	Classify string
	Head     string
	Factory  string
}

// The outcomes, from the one that merges with no person to the one that waits for a person. The
// rule can only move a verdict towards ask.
var outcomes = []string{"ship", "show", "ask"}

type verdict struct {
	Outcome string
	Reasons []string
}

// The line each fix commit's message carries, which fix.yml writes and counts its tries by.
const fixLine = "Fixed-by: fix-review-findings"

func main() {
	repo := flag.String("repo", ".", "repository with the judged commit's tree in its index, at its base on main")
	rules := flag.String("rules", ".github/build-rules.yml", "build rules file")
	findings := flag.String("findings", "", "the reviews' findings, as JSON")
	classify := flag.String("classify", "", "Classify's verdict: ship, show, ask or missing")
	head := flag.String("head", "", "the commit judged, whose commits since the repository's HEAD are read")
	factory := flag.String("factory", "", "the login of the factory's app, whose commits are the factory's")
	flag.Parse()

	cfg, err := loadConfig(*rules)
	if err != nil {
		fatal(err)
	}
	result, err := evaluate(*repo, cfg, inputs{Findings: *findings, Classify: *classify, Head: *head, Factory: *factory})
	if err != nil {
		fatal(err)
	}
	fmt.Println(result.Outcome)
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
	outcome := "ship"
	var reasons []string
	// towards moves the outcome to at least the one given, with the reason why.
	towards := func(at, reason string) {
		if rank(at) > rank(outcome) {
			outcome = at
		}
		reasons = append(reasons, reason)
	}
	ask := func(reason string) { towards("ask", reason) }

	switch in.Classify {
	case "", "ship":
	case "show":
		towards("show", "Classify said to show it.")
	case "ask":
		ask("Classify asked for a review.")
	case "missing":
		ask("Classify gave no verdict, so the change waits for review.")
	default:
		return verdict{}, fmt.Errorf("Classify's verdict %q is none of ship, show, ask or missing", in.Classify)
	}

	if in.Findings != "" {
		important, nits, err := countFindings(in.Findings)
		if err != nil {
			return verdict{}, err
		}
		if important > 0 {
			ask(fmt.Sprintf("The reviews left %s.", count(important, "Important finding", "Important findings")))
		}
		if nits > 0 {
			towards("show", fmt.Sprintf("The reviews left %s unfixed.", count(nits, "Nit", "Nits")))
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
				ask(fmt.Sprintf("%s is a path that always needs review.", shown(path)))
				break
			}
		}
	}
	for _, path := range paths {
		if path == ".gitattributes" || strings.HasSuffix(path, "/.gitattributes") {
			ask(fmt.Sprintf("%s changes how git reads files.", shown(path)))
		}
	}

	deleted, err := deletedPaths(repo)
	if err != nil {
		return verdict{}, err
	}
	for _, path := range deleted {
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.mjs") {
			ask(fmt.Sprintf("%s deletes a test file.", shown(path)))
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
			ask(fmt.Sprintf("%s removes a test function.", shown(path)))
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
			ask(fmt.Sprintf("%s is a symbolic link, so it always needs review.", shown(path)))
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
			ask(fmt.Sprintf("%s is a binary file.", shown(path)))
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
			ask(fmt.Sprintf("%s is binary, and doesn't start as its name says, so the rule can't read it.", shown(path)))
		}
		if mode.before != noFile {
			ask(fmt.Sprintf("%s replaces a binary file on main, which only a person can compare.", shown(path)))
		}
		if size > maxBinaryBytes {
			ask(fmt.Sprintf("%s is binary and larger than 1 MiB, the most an image or font may be without review.", shown(path)))
		}
	}
	if changedLines > cfg.MaxChangedLines {
		ask(fmt.Sprintf("The patch changes %d lines, more than the limit of %d.", changedLines, cfg.MaxChangedLines))
	}

	if in.Head != "" {
		branch, err := readBranch(repo, in.Head, in.Factory)
		if err != nil {
			return verdict{}, err
		}
		if branch.merges > 0 {
			ask("The branch since main holds a merge commit.")
		}
		if branch.others > 0 {
			ask("A commit since main isn't the factory's.")
		}
		if branch.fixes > 0 {
			towards("show", fmt.Sprintf("The branch carries %s.", count(branch.fixes, "fix commit", "fix commits")))
		}
	}

	if len(reasons) == 0 {
		reasons = []string{"No rule moved the verdict towards ask."}
	}
	return verdict{Outcome: outcome, Reasons: reasons}, nil
}

func rank(outcome string) int {
	for at, known := range outcomes {
		if known == outcome {
			return at
		}
	}
	return len(outcomes)
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

type branch struct {
	fixes, others, merges int
}

// readBranch reads each commit between the repository's HEAD, the commit of main the branch sits
// on, and the head, from git's own record of it, so no message can pass for another commit's
// header. A commit is the factory's only if the app's login is in both its author's and its
// committer's noreply address: a person's commit that the app rebased is still the person's. Anyone
// with write access could forge the address, and they're trusted to, so the email is enough here.
func readBranch(repo repository, head, factory string) (branch, error) {
	var result branch
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(head) {
		return result, fmt.Errorf("the head %q isn't a commit id", head)
	}
	output, err := repo.git("rev-list", "HEAD.."+head)
	if err != nil {
		return result, err
	}
	for _, id := range strings.Fields(string(output)) {
		raw, err := repo.git("cat-file", "commit", id)
		if err != nil {
			return result, err
		}
		header, message, _ := strings.Cut(string(raw), "\n\n")
		parents, author, committer := 0, "", ""
		for _, line := range strings.Split(header, "\n") {
			field, value, _ := strings.Cut(line, " ")
			switch field {
			case "parent":
				parents++
			case "author":
				author = value
			case "committer":
				committer = value
			}
		}
		if parents != 1 {
			result.merges++
		}
		if factory == "" || !byApp(author, factory) || !byApp(committer, factory) {
			result.others++
		}
		for _, line := range strings.Split(message, "\n") {
			if strings.TrimRight(line, "\r") == fixLine {
				result.fixes++
				break
			}
		}
	}
	return result, nil
}

// byApp says whether an author or committer line names the app's noreply address, with or
// without the bot's user id before it.
func byApp(line, factory string) bool {
	start := strings.LastIndex(line, "<")
	end := strings.LastIndex(line, ">")
	if start < 0 || end < start {
		return false
	}
	email := line[start+1 : end]
	address := factory + "@users.noreply.github.com"
	if email == address {
		return true
	}
	id, rest, ok := strings.Cut(email, "+")
	return ok && rest == address && id != "" && strings.Trim(id, "0123456789") == ""
}

// countFindings counts the Important findings and the Nits. A pre-existing one is about code the
// change didn't write, so it's shown but moves nothing.
func countFindings(path string) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("read the reviews' findings: %w", err)
	}
	var findings []struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(data, &findings); err != nil {
		return 0, 0, fmt.Errorf("read the reviews' findings: %w", err)
	}
	important, nits := 0, 0
	for _, finding := range findings {
		switch finding.Severity {
		case "Important":
			important++
		case "Nit":
			nits++
		case "Pre-existing":
		default:
			return 0, 0, fmt.Errorf("a finding has an unknown severity %q", finding.Severity)
		}
	}
	return important, nits, nil
}

// allowedBinary says whether a binary file is an image or a font, which a diff can't show anyway.
// Under .github/ even those need review. It matches the extension's case exactly, as the list in
// .github/bring-patch.sh does, so the two agree.
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
