// The awml command checks, draws, summarises and exchanges workflow files with a registry.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
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
    check            report warnings, structural mistakes and differences between views
    fit              judge where a workflow can run
    summary          count the parts and lines in one workflow
    convert          join a legacy pair into one 0.4 workflow
    login            save a registry API token
    push             push a new workflow version
    pull             download a workflow version

options:
  -h, --help         show this help message and exit
  --version          show program's version number and exit
  --registry URL     registry URL (default: AWML_REGISTRY_URL or https://awml.io)
`

var commands = map[string]struct{ usage, help string }{
	"draw": {"usage: awml draw [-h] [--implementation ID] file\n", `
positional arguments:
  file

options:
  -h, --help  show this help message and exit
  --implementation ID  draw one implementation
`},
	"convert": {"usage: awml convert [-h] [--implementation ID] SECURITY PLATFORM\n", `
Join a legacy security and platform pair into one 0.4 file.

options:
  -h, --help  show this help message and exit
  --implementation ID  name the implementation (default: default)
`},
	"page": {"usage: awml page [-h] -o OUT files [files ...]\n", `
positional arguments:
  files

options:
  -h, --help         show this help message and exit
  -o OUT, --out OUT
`},
	"check": {"usage: awml check [-h] files [files ...]\n", `
positional arguments:
  files

options:
  -h, --help  show this help message and exit
`},
	"fit": {"usage: awml fit [-h] [--json] [--implementation ID] SECURITY {DESC... | PLATFORM_VIEW DESC...}\n", `
positional arguments:
  SECURITY
  DESC
  PLATFORM_VIEW

options:
  -h, --help  show this help message and exit
  --json      print JSON
  --implementation ID  judge one implementation
`},
	"summary": {"usage: awml summary [-h] [--json] files [files ...]\n", `
positional arguments:
  files

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
  owner/slug[@n]

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
		case flags && (command == "summary" || command == "fit") && a == "--json":
			jsonOutput = true
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
	var missing []string
	if len(files) == 0 && command != "login" {
		missing = append(missing, map[string]string{"draw": "file", "page": "files", "check": "files", "fit": "SECURITY, DESC", "convert": "SECURITY, PLATFORM", "summary": "files", "push": "owner/slug, file", "pull": "owner/slug[@n]"}[command])
	} else if command == "fit" && len(files) == 1 {
		missing = append(missing, "DESC")
	}
	if command == "page" && out == nil {
		missing = append(missing, "-o/--out")
	}
	if missing != nil {
		return fail(commands[command].usage, "awml "+command, "the following arguments are required: "+strings.Join(missing, ", "))
	}
	if command == "convert" && len(files) != 2 {
		return fail(commands[command].usage, "awml convert", "expected SECURITY and PLATFORM")
	}
	if command == "draw" && len(files) > 1 {
		unknown = append(files[1:], unknown...)
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

	if command == "check" {
		type checkedFile struct {
			path     string
			doc      *awml.Map
			errs     []string
			warnings []string
		}
		type viewsOfTitle struct {
			security []int
			platform []int
		}
		checked := make([]checkedFile, 0, len(files))
		byTitle := map[string]*viewsOfTitle{}
		var titleOrder []string
		for _, path := range files {
			doc, errs := read(path)
			checked = append(checked, checkedFile{path, doc, errs, awml.Warnings(doc)})
			if doc == nil {
				continue
			}
			view := strField(doc, "view")
			if view != "security" && view != "platform" {
				continue
			}
			title := strField(doc, "title")
			views := byTitle[title]
			if views == nil {
				views = &viewsOfTitle{}
				byTitle[title] = views
				titleOrder = append(titleOrder, title)
			}
			if view == "security" {
				views.security = append(views.security, len(checked)-1)
			} else {
				views.platform = append(views.platform, len(checked)-1)
			}
		}

		type comparison struct {
			security int
			platform int
			mistakes []string
		}
		var comparisons []comparison
		failedComparison := make([]bool, len(checked))
		for _, title := range titleOrder {
			views := byTitle[title]
			if len(views.security) != 1 || len(views.platform) != 1 {
				continue
			}
			security, platform := views.security[0], views.platform[0]
			mistakes := awml.CompareViews(checked[security].doc, checked[platform].doc)
			if len(mistakes) == 0 {
				continue
			}
			failedComparison[security], failedComparison[platform] = true, true
			comparisons = append(comparisons, comparison{security, platform, mistakes})
		}

		code := 0
		for i, file := range checked {
			for _, warning := range file.warnings {
				fmt.Fprintf(stdout, "%s: warning: %s\n", file.path, warning)
			}
			if len(file.errs) == 0 {
				if !failedComparison[i] {
					fmt.Fprintf(stdout, "%s: ok\n", file.path)
				}
				continue
			}
			for _, e := range file.errs {
				fmt.Fprintf(stdout, "%s: %s\n", file.path, e)
			}
			code = 1
		}
		for _, comparison := range comparisons {
			security, platform := checked[comparison.security], checked[comparison.platform]
			for _, mistake := range comparison.mistakes {
				fmt.Fprintf(stdout, "%s and %s: %s\n", security.path, platform.path, mistake)
			}
			code = 1
		}
		return code
	}
	docs, ok := sound(files, stderr)
	if !ok {
		return 1
	}
	if command == "draw" {
		view, err := awml.View(docs[0], implementation)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, awml.Draw(view))
		return 0
	}
	if command == "convert" {
		name := implementation
		if name == "" {
			name = "default"
		}
		converted, err := awml.ConvertWithImplementation(docs[0], docs[1], name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		data, err := awml.Marshal(converted)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, path := range files {
			src, _ := os.ReadFile(path)
			comments, err := awml.Comments(src)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			for _, comment := range comments {
				fmt.Fprintln(stdout, comment)
			}
		}
		fmt.Fprint(stdout, string(data))
		return 0
	}
	if command == "fit" {
		if !awml.IsCanonical(docs[0]) && strField(docs[0], "view") != "security" {
			fmt.Fprintf(stderr, "%s: must be a security view\n", files[0])
			return 1
		}
		var placement *awml.Map
		start := 1
		if implementation != "" {
			var err error
			placement, err = awml.View(docs[0], implementation)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		} else if !awml.IsCanonical(docs[0]) && strField(docs[1], "view") == "platform" {
			placement = docs[1]
			start = 2
		}
		if start == len(docs) {
			fmt.Fprintf(stderr, "awml fit: no platform descriptions were given\n")
			return 1
		}
		for i := start; i < len(docs); i++ {
			if _, ok := docs[i].Get("platform"); !ok {
				fmt.Fprintf(stderr, "%s: must be a platform description\n", files[i])
				return 1
			}
		}
		report := awml.Judge(docs[0], placement, docs[start:])
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
		summary := awml.Summarise(docs...)
		if jsonOutput {
			if err := json.NewEncoder(stdout).Encode(summary); err != nil {
				fmt.Fprintf(stderr, "awml summary: can't write its output: %s\n", err)
				return 1
			}
			return 0
		}
		printSummary(stdout, summary)
		return 0
	}
	if err := os.WriteFile(*out, []byte(awml.Page(docs)), 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: can't be written: %s\n", *out, reason(err))
		return 1
	}
	return 0
}

func currentVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		buildVersion := strings.TrimPrefix(info.Main.Version, "v")
		if buildVersion != "" && buildVersion != "(devel)" {
			return buildVersion
		}
	}
	return "dev"
}

func strField(doc *awml.Map, field string) string {
	value, _ := doc.Get(field)
	text, _ := value.(string)
	return text
}

func printSummary(out io.Writer, s awml.Summary) {
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
}

// read gives the document in a file, and the mistakes that stop it being drawn.
func read(path string) (*awml.Map, []string) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"can't be read: " + reason(err)}
	}
	doc, err := awml.Parse(src)
	if err != nil {
		return nil, []string{"isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")}
	}
	if errs := awml.Check(doc); len(errs) > 0 {
		file, _ := doc.(*awml.Map)
		return file, errs
	}
	return doc.(*awml.Map), nil
}

// sound gives every file's document, or false once any file has mistakes, which are printed.
func sound(paths []string, stderr io.Writer) ([]*awml.Map, bool) {
	var docs []*awml.Map
	ok := true
	for _, path := range paths {
		doc, errs := read(path)
		for _, e := range errs {
			fmt.Fprintf(stderr, "%s: %s\n", path, e)
		}
		ok = ok && len(errs) == 0
		docs = append(docs, doc)
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
