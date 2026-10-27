// The awml command checks, draws, summarises and exchanges workflow files with a registry.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/chrismdp/awml"
)

var version string

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = "usage: awml [-h] [--version] [--registry URL] {draw,page,check,fit,summary,convert,login,push,pull} ...\n"

const help = usage + `
Draw agentic workflows written in AWML.

positional arguments:
  {draw,page,check,fit,summary,convert,login,push,pull}
    draw             print a file's Mermaid
    page             write one HTML page that draws each file
    check            report warnings and structural mistakes
    fit              judge where workflows can run
    summary          count the parts and lines in workflows
    convert          upgrade an older file, or join a legacy pair, into one 0.7 file
    login            save a registry API token
    push             push a new workflow version
    pull             download a workflow or platform version

options:
  -h, --help         show this help message and exit
  --version          show program's version number and exit
  --registry URL     registry URL (default: AWML_REGISTRY_URL or https://awml.io)
`

var commands = map[string]struct{ usage, help string }{
	"draw": {"usage: awml draw [-h] [--implementation ID | --untrusted] files [files ...]\n", `
Draw a workflow's security view, or the map of several workflows drawn together.

positional arguments:
  files       workflows, or folders of them

options:
  -h, --help  show this help message and exit
  --implementation ID  draw one implementation of one workflow
  --untrusted  draw where untrusted text reaches in one workflow
`},
	"convert": {"usage: awml convert [-h] [--implementation ID] file [PLATFORM]\n", `
Upgrade a file of an older version into one 0.7 file, or join a legacy security view and platform
view into one.

positional arguments:
  file
  PLATFORM    the platform view of a legacy pair, after its security view

options:
  -h, --help  show this help message and exit
  --implementation ID  name a legacy pair's implementation (default: default)
`},
	"page": {"usage: awml page [-h] -o OUT files [files ...]\n", `
positional arguments:
  files

options:
  -h, --help         show this help message and exit
  -o OUT, --out OUT
`},
	"check": {"usage: awml check [-h] [--json] files [files ...]\n", `
positional arguments:
  files       files, or folders of them, checked together

options:
  -h, --help  show this help message and exit
  --json      print every check and its result as JSON
`},
	"fit": {"usage: awml fit [-h] [--json] [--implementation ID] files [files ...]\n", `
Judge each platform described as running every workflow given.

positional arguments:
  files       workflows and platform descriptions, told apart by what they hold, or folders of them

options:
  -h, --help  show this help message and exit
  --json      print JSON
  --implementation ID  judge one implementation
`},
	"summary": {"usage: awml summary [-h] [--json] files [files ...]\n", `
positional arguments:
  files       workflows, or folders of them, summarised together

options:
  -h, --help  show this help message and exit
  --json      print JSON
`},
	"login": {"usage: awml login [-h] [--registry URL]\n", `
options:
  -h, --help      show this help message and exit
  --registry URL  registry URL
`},
	"push": {"usage: awml push [-h] [--private] [--registry URL] owner/slug file\n", `
positional arguments:
  owner/slug
  file

options:
  -h, --help      show this help message and exit
  --private       make a new workflow private
  --registry URL  registry URL
`},
	"pull": {"usage: awml pull [-h] [-o DIR] [--registry URL] owner/slug[@n]\n", `
positional arguments:
  owner/slug[@n]     a workflow, or owner/platforms/slug[@n] for a platform

options:
  -h, --help         show this help message and exit
  -o DIR, --out DIR  output directory
  --registry URL     registry URL
`},
}

// run reads its arguments as the Python tool's argparse did, with the same messages, and gives the
// exit code: 0 when all is well, 1 for a file with mistakes, 2 for a mistake in the command.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func(use, prog, msg string) int {
		fmt.Fprintf(stderr, "%s%s: error: %s\n", use, prog, msg)
		return 2
	}
	var command string
	var files, unknown []string
	var out *string
	implementation := ""
	jsonOutput := false
	untrusted := false
	private := false
	var registryFlag *string
	flags := true
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case flags && a == "--":
			flags = false
		case flags && a == "--version":
			fmt.Fprintf(stdout, "awml %s\n", currentVersion())
			return 0
		case flags && (a == "-h" || a == "--help"):
			if command == "" {
				fmt.Fprint(stdout, help)
			} else {
				fmt.Fprint(stdout, commands[command].usage+commands[command].help)
			}
			return 0
		case flags && (command == "draw" || command == "fit" || command == "convert") && a == "--implementation":
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "-") {
				return fail(commands[command].usage, "awml "+command, "argument --implementation: expected one argument")
			}
			i++
			implementation = args[i]
		case flags && (command == "draw" || command == "fit" || command == "convert") && strings.HasPrefix(a, "--implementation="):
			implementation = strings.TrimPrefix(a, "--implementation=")
		case flags && a == "--registry":
			if i+1 == len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-") {
				use, prog := usage, "awml"
				if command != "" {
					use, prog = commands[command].usage, "awml "+command
				}
				return fail(use, prog, "argument --registry: expected one argument")
			}
			i++
			registryFlag = &args[i]
		case flags && strings.HasPrefix(a, "--registry="):
			v := strings.TrimPrefix(a, "--registry=")
			registryFlag = &v
		case flags && (command == "page" || command == "pull") && (a == "-o" || a == "--out"):
			if i+1 == len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-") {
				return fail(commands[command].usage, "awml "+command, "argument -o/--out: expected one argument")
			}
			i++
			out = &args[i]
		case flags && (command == "page" || command == "pull") && strings.HasPrefix(a, "--out="):
			v := strings.TrimPrefix(a, "--out=")
			out = &v
		case flags && (command == "page" || command == "pull") && strings.HasPrefix(a, "-o") && !strings.HasPrefix(a, "--"):
			v := strings.TrimPrefix(a, "-o")
			out = &v
		case flags && (command == "summary" || command == "fit" || command == "check") && a == "--json":
			jsonOutput = true
		case flags && command == "draw" && a == "--untrusted":
			untrusted = true
		case flags && command == "push" && a == "--private":
			private = true
		case flags && strings.HasPrefix(a, "-") && a != "-":
			unknown = append(unknown, a)
		case command == "":
			if _, ok := commands[a]; !ok {
				return fail(usage, "awml", fmt.Sprintf("argument command: invalid choice: '%s' (choose from 'draw', 'page', 'check', 'fit', 'summary', 'convert', 'login', 'push', 'pull')", a))
			}
			command = a
		default:
			files = append(files, a)
		}
	}
	if command == "" {
		return fail(usage, "awml", "the following arguments are required: command")
	}
	if isOneOf(command, "check", "draw", "fit", "summary", "page") {
		var empty string
		if files, empty = expandFolders(files); empty != "" {
			return fail(commands[command].usage, "awml "+command, empty+" holds no .yaml files")
		}
	}
	var missing []string
	if len(files) == 0 && command != "login" {
		missing = append(missing, map[string]string{"draw": "files", "page": "files", "check": "files", "fit": "files", "convert": "file", "summary": "files", "push": "owner/slug, file", "pull": "owner/slug[@n]"}[command])
	}
	if command == "page" && out == nil {
		missing = append(missing, "-o/--out")
	}
	if missing != nil {
		return fail(commands[command].usage, "awml "+command, "the following arguments are required: "+strings.Join(missing, ", "))
	}
	if command == "convert" && len(files) > 2 {
		return fail(commands[command].usage, "awml convert", "expected one file, or a security view and its PLATFORM view")
	}
	if command == "convert" && len(files) == 1 && implementation != "" {
		return fail(commands[command].usage, "awml convert", "argument --implementation: only a legacy pair has an implementation to name")
	}
	if command == "draw" && untrusted && implementation != "" {
		return fail(commands[command].usage, "awml draw", "argument --untrusted: not allowed with argument --implementation")
	}
	if command == "draw" && len(files) > 1 && untrusted {
		return fail(commands[command].usage, "awml draw", "argument --untrusted: draws one workflow, not several")
	}
	if command == "draw" && len(files) > 1 && implementation != "" {
		return fail(commands[command].usage, "awml draw", "argument --implementation: draws one workflow, not several")
	}
	if command == "login" && len(files) > 0 {
		unknown = append(files, unknown...)
	}
	if command == "pull" && len(files) > 1 {
		unknown = append(files[1:], unknown...)
	}
	if command == "push" && len(files) == 1 {
		return fail(commands[command].usage, "awml push", "the following arguments are required: file")
	}
	if unknown != nil {
		return fail(usage, "awml", "unrecognized arguments: "+strings.Join(unknown, " "))
	}

	registry := registryURL(registryFlag)
	if command == "login" {
		return login(stdin, stdout, stderr, registry)
	}
	if command == "push" {
		return push(files[0], files[1:], private, registry, stdout, stderr)
	}
	if command == "pull" {
		dir := "."
		if out != nil {
			dir = *out
		}
		return pull(files[0], dir, registry, stdout, stderr)
	}

	if command == "check" && jsonOutput {
		return checkJSON(files, stdout, stderr)
	}
	if command == "check" {
		code := 0
		docs := make([]*awml.Map, len(files))
		errors := make([][]string, len(files))
		var given []awml.Workflow
		var at []int
		for i, path := range files {
			docs[i], errors[i] = read(path)
			if awml.IsCanonical(docs[i]) {
				given = append(given, workflowAt(path, docs[i]))
				at = append(at, i)
			}
		}
		// Each ref between the workflows given is followed.
		warnings := make([][]string, len(files))
		mistakes, shared := awml.TogetherMistakes(given), awml.TogetherWarnings(given)
		for n, i := range at {
			errors[i] = append(errors[i], mistakes[n]...)
			warnings[i] = shared[n]
		}
		for i, path := range files {
			doc, errs := docs[i], errors[i]
			for _, warning := range append(awml.Warnings(doc), warnings[i]...) {
				fmt.Fprintf(stdout, "%s: warning: %s\n", path, warning)
			}
			if len(errs) == 0 {
				fmt.Fprintf(stdout, "%s: ok\n", path)
				continue
			}
			for _, e := range errs {
				fmt.Fprintf(stdout, "%s: %s\n", path, e)
			}
			code = 1
		}
		return code
	}
	if command == "convert" {
		return convert(files, implementation, stdout, stderr)
	}
	docs, ok := sound(files, stderr, command == "draw" || command == "page" || command == "summary")
	if !ok {
		return 1
	}
	if command == "draw" && len(docs) > 1 {
		var given []awml.Workflow
		for i, doc := range docs {
			if !awml.IsCanonical(doc) {
				_, err := awml.View(doc, "")
				fmt.Fprintf(stderr, "%s: %s\n", files[i], err)
				return 1
			}
			given = append(given, workflowAt(files[i], doc))
		}
		fmt.Fprintln(stdout, awml.DrawMap(given))
		return 0
	}
	if command == "draw" {
		view, err := awml.View(docs[0], implementation)
		if untrusted {
			view, err = awml.UntrustedView(docs[0])
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, awml.Draw(view))
		return 0
	}
	if command == "fit" {
		// A file with platform is a platform description, and any other is a workflow.
		var given []awml.Workflow
		var descriptions []*awml.Map
		for i, doc := range docs {
			if _, ok := doc.Get("platform"); ok {
				descriptions = append(descriptions, doc)
			} else {
				given = append(given, workflowAt(files[i], doc))
			}
		}
		if len(given) == 0 {
			return fail(commands[command].usage, "awml fit", "no workflow was given")
		}
		if implementation != "" && len(given) > 1 {
			return fail(commands[command].usage, "awml fit", "argument --implementation: judges one workflow, not several")
		}
		var report awml.FitReport
		if implementation != "" {
			placement, err := awml.View(given[0].Doc, implementation)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			report = awml.Judge(given[0].Doc, placement, descriptions)
		} else if len(descriptions) == 0 {
			return fail(commands[command].usage, "awml fit", "no platform descriptions were given")
		} else {
			report = awml.JudgeTogether(given, descriptions)
		}
		output := awml.FitLines(report.Results)
		code := 0
		if len(report.Gaps) != 0 {
			output = report.Gaps
			code = 1
		}
		if jsonOutput {
			if err := json.NewEncoder(stdout).Encode(report); err != nil {
				fmt.Fprintf(stderr, "awml fit: can't write its output: %s\n", err)
				return 1
			}
		} else {
			for _, line := range output {
				fmt.Fprintln(stdout, line)
			}
		}
		return code
	}
	if command == "summary" {
		var given []awml.Workflow
		for i, doc := range docs {
			given = append(given, workflowAt(files[i], doc))
		}
		summary := awml.SummariseTogether(given)
		if jsonOutput {
			if err := json.NewEncoder(stdout).Encode(summary); err != nil {
				fmt.Fprintf(stderr, "awml summary: can't write its output: %s\n", err)
				return 1
			}
			return 0
		}
		printSummary(stdout, summary, len(given) > 1)
		return 0
	}
	if err := os.WriteFile(*out, []byte(awml.Page(docs)), 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: can't be written: %s\n", *out, reason(err))
		return 1
	}
	return 0
}

// checkJSON prints every check and its result as JSON, and fails as the plain output does: when
// any check finds a mistake.
func checkJSON(files []string, stdout, stderr io.Writer) int {
	sources := make([]awml.Source, 0, len(files))
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			sources = append(sources, awml.Source{File: path, Unreadable: "can't be read: " + reason(err)})
			continue
		}
		sources = append(sources, awml.Source{File: path, Path: where(path), Text: src})
	}
	report := awml.CheckSources(currentVersion(), sources)
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		fmt.Fprintf(stderr, "awml check: can't write its output: %s\n", err)
		return 1
	}
	for _, check := range report.Checks {
		if check.Result == "fail" {
			return 1
		}
	}
	return 0
}

// convert prints one 0.7 file, upgraded from a file of an older version or joined from a legacy
// pair, as awml.ConvertSources writes it. It fails when the files can't be converted, when it notes
// something it couldn't carry over, and when the converted file has mistakes, which newer versions'
// rules can find in a file that older ones passed. It lists those on standard error.
func convert(files []string, implementation string, stdout, stderr io.Writer) int {
	var sources []awml.Source
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "%s: can't be read: %s\n", path, reason(err))
			return 1
		}
		sources = append(sources, awml.Source{File: path, Text: src})
	}
	data, notes, mistakes, err := awml.ConvertSources(sources, implementation)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	stdout.Write(data)
	if len(notes) == 0 && len(mistakes) == 0 {
		return 0
	}
	if len(notes) != 0 {
		fmt.Fprintln(stderr, "awml convert: converted, but do this by hand:")
		for _, note := range notes {
			fmt.Fprintf(stderr, "  %s\n", note)
		}
	}
	if len(mistakes) != 0 {
		fmt.Fprintln(stderr, "awml convert: converted, but awml check finds these mistakes in the result:")
		for _, mistake := range mistakes {
			fmt.Fprintf(stderr, "  %s\n", mistake)
		}
	}
	return 1
}

func currentVersion() string {
	if version != "" {
		return version
	}
	return awml.BuildVersion()
}

// expandFolders gives the files named, with each folder standing for every .yaml file directly
// inside it, in the order of their names. A path that can't be read as a folder is a file. A file
// named twice is given once, where it's first named. It also gives the first folder that holds no
// .yaml file, if there is one.
func expandFolders(paths []string) ([]string, string) {
	var files []string
	seen := map[string]bool{}
	add := func(path string) {
		at := where(path)
		if at == "" {
			at = path
		}
		if !seen[at] {
			seen[at] = true
			files = append(files, path)
		}
	}
	for _, path := range paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			add(path)
			continue
		}
		held := false
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
				add(filepath.Join(path, entry.Name()))
				held = true
			}
		}
		if !held {
			return nil, path
		}
	}
	return files, ""
}

// where gives where a file named by path is, as an absolute path, which its refs are followed from.
func where(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return ""
}

// workflowAt gives a workflow given by path, which messages name it by, and where it is.
func workflowAt(path string, doc *awml.Map) awml.Workflow {
	return awml.Workflow{File: path, Path: where(path), Doc: doc}
}

func isOneOf(s string, choices ...string) bool {
	for _, c := range choices {
		if s == c {
			return true
		}
	}
	return false
}

// printSummary prints a summary. Workflows given together also have the parts they share.
func printSummary(out io.Writer, s awml.Summary, together bool) {
	fmt.Fprintf(out, "steps: AI %d, code %d, person %d\n", s.Steps.AI, s.Steps.Code, s.Steps.Person)
	var operations []string
	for op, count := range s.Operations {
		operations = append(operations, fmt.Sprintf("%s %d", op, count))
	}
	sort.Strings(operations)
	if len(operations) == 0 {
		operations = []string{"none"}
	}
	fmt.Fprintf(out, "operations: %s\n", strings.Join(operations, ", "))
	fmt.Fprintf(out, "outside actions: Allow %d, Ask %d, Block %d\n", s.OutsideActions.Allow, s.OutsideActions.Ask, s.OutsideActions.Block)
	fmt.Fprintf(out, "triggers: %d\nreviews: %d\ncontexts: %d\nconnectors: %d\nsurfaces: %d\nscopes: %d\nplatforms: %d\n", s.Triggers, s.Reviews, s.Contexts, s.Connectors, s.Surfaces, s.Scopes, s.Platforms)
	fmt.Fprintf(out, "publishing lines: %d\n", s.PublishingLines.Count)
	for _, label := range s.PublishingLines.Labels {
		fmt.Fprintf(out, "  %s\n", label)
	}
	fmt.Fprintf(out, "refused reads: %d\nred lines: %d\n", s.RefusedReads, s.RedLines)
	fmt.Fprintf(out, "untrusted parts: %d\n", len(s.Untrusted))
	for _, u := range s.Untrusted {
		fmt.Fprintf(out, "  %s\n", u.Part)
		for _, list := range [][2]any{{"steps", u.Steps}, {"outside actions", u.OutsideActions}, {"stopped by", u.StoppedBy}} {
			names := list[1].([]string)
			if len(names) == 0 {
				fmt.Fprintf(out, "    %s: none\n", list[0])
				continue
			}
			// A label may hold commas, so each has a line of its own.
			fmt.Fprintf(out, "    %s:\n", list[0])
			for _, name := range names {
				fmt.Fprintf(out, "      %s\n", name)
			}
		}
	}
	if !together {
		return
	}
	fmt.Fprintf(out, "shared parts: %d\n", len(s.Shared))
	for _, shared := range s.Shared {
		fmt.Fprintf(out, "  %s (%s): %s\n", shared.Label, shared.Kind, strings.Join(shared.Workflows, ", "))
	}
}

// read gives the document in a file, and every mistake awml check reports in it.
func read(path string) (*awml.Map, []string) { return readWith(path, awml.Check) }

// readWith gives the document in a file, and the mistakes find reports in it.
func readWith(path string, find func(any) []string) (*awml.Map, []string) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"can't be read: " + reason(err)}
	}
	doc, err := awml.Parse(src)
	if err != nil {
		return nil, []string{"isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")}
	}
	if errs := find(doc); len(errs) > 0 {
		file, _ := doc.(*awml.Map)
		return file, errs
	}
	return doc.(*awml.Map), nil
}

// sound gives every file's document, or false once any file has mistakes, which are printed: its own,
// then those following each ref between the workflows given finds. A command that draws or counts
// red lines takes a file whose only mistakes are lines that break a rule.
func sound(paths []string, stderr io.Writer, drawn bool) ([]*awml.Map, bool) {
	find := awml.Check
	if drawn {
		find = awml.Unsound
	}
	docs := make([]*awml.Map, len(paths))
	errs := make([][]string, len(paths))
	var given []awml.Workflow
	var at []int
	for i, path := range paths {
		docs[i], errs[i] = readWith(path, find)
		if awml.IsCanonical(docs[i]) {
			given = append(given, workflowAt(path, docs[i]))
			at = append(at, i)
		}
	}
	for n, mistakes := range awml.TogetherMistakes(given) {
		errs[at[n]] = append(errs[at[n]], mistakes...)
	}
	ok := true
	for i, path := range paths {
		for _, e := range errs[i] {
			fmt.Fprintf(stderr, "%s: %s\n", path, e)
		}
		ok = ok && len(errs[i]) == 0
	}
	return docs, ok
}

// reason is why a file couldn't be read or written, as the system words it, such as "No such file
// or directory".
func reason(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		err = errno
	}
	s := err.Error()
	return strings.ToUpper(s[:1]) + s[1:]
}
