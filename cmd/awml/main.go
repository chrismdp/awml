// The awml command: draw, page and check.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/chrismdp/awml"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = "usage: awml [-h] {draw,page,check} ...\n"

const help = usage + `
Draw agentic workflows written in AWML.

positional arguments:
  {draw,page,check}
    draw             print a file's Mermaid
    page             write one HTML page that draws each file
    check            report a file's structural mistakes

options:
  -h, --help         show this help message and exit
`

var commands = map[string]struct{ usage, help string }{
	"draw": {"usage: awml draw [-h] file\n", `
positional arguments:
  file

options:
  -h, --help  show this help message and exit
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
}

// run reads its arguments as the Python tool's argparse did, with the same messages, and gives the
// exit code: 0 when all is well, 1 for a file with mistakes, 2 for a mistake in the command.
func run(args []string, stdout, stderr io.Writer) int {
	fail := func(use, prog, msg string) int {
		fmt.Fprintf(stderr, "%s%s: error: %s\n", use, prog, msg)
		return 2
	}
	var command string
	var files, unknown []string
	var out *string
	flags := true
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case flags && a == "--":
			flags = false
		case flags && (a == "-h" || a == "--help"):
			if command == "" {
				fmt.Fprint(stdout, help)
			} else {
				fmt.Fprint(stdout, commands[command].usage+commands[command].help)
			}
			return 0
		case flags && command == "page" && (a == "-o" || a == "--out"):
			if i+1 == len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-") {
				return fail(commands[command].usage, "awml page", "argument -o/--out: expected one argument")
			}
			i++
			out = &args[i]
		case flags && command == "page" && strings.HasPrefix(a, "--out="):
			v := strings.TrimPrefix(a, "--out=")
			out = &v
		case flags && command == "page" && strings.HasPrefix(a, "-o") && !strings.HasPrefix(a, "--"):
			v := strings.TrimPrefix(a, "-o")
			out = &v
		case flags && strings.HasPrefix(a, "-") && a != "-":
			unknown = append(unknown, a)
		case command == "":
			if _, ok := commands[a]; !ok {
				return fail(usage, "awml", fmt.Sprintf("argument command: invalid choice: '%s' (choose from 'draw', 'page', 'check')", a))
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
	if len(files) == 0 {
		missing = append(missing, map[string]string{"draw": "file", "page": "files", "check": "files"}[command])
	}
	if command == "page" && out == nil {
		missing = append(missing, "-o/--out")
	}
	if missing != nil {
		return fail(commands[command].usage, "awml "+command, "the following arguments are required: "+strings.Join(missing, ", "))
	}
	if command == "draw" && len(files) > 1 {
		unknown = append(files[1:], unknown...)
	}
	if unknown != nil {
		return fail(usage, "awml", "unrecognized arguments: "+strings.Join(unknown, " "))
	}

	if command == "check" {
		code := 0
		for _, path := range files {
			_, errs := read(path)
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
	docs, ok := sound(files, stderr)
	if !ok {
		return 1
	}
	if command == "draw" {
		fmt.Fprintln(stdout, awml.Draw(docs[0]))
		return 0
	}
	if err := os.WriteFile(*out, []byte(awml.Page(docs)), 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: can't be written: %s\n", *out, reason(err))
		return 1
	}
	return 0
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
		return nil, errs
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
