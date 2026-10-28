// The render helper writes every view of an example and lists its Mermaid paths for render.sh.
// Given -map OUT and several examples, it writes their map, as awml draw does given them together.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chrismdp/awml"
)

func main() {
	if len(os.Args) > 2 && os.Args[1] == "-map" {
		if err := drawMap(os.Args[2], os.Args[3:]); err != nil {
			fail(err)
		}
		fmt.Println(os.Args[2])
		return
	}
	for _, path := range os.Args[1:] {
		drawings, err := drawExample(path)
		if err != nil {
			fail(err)
		}
		for _, drawing := range drawings {
			fmt.Println(drawing)
		}
	}
}

// read gives the document in an example, refusing it as awml draw does: for any mistake but a line
// that breaks a rule, which the drawing draws red.
func read(path string) (*awml.Map, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	value, err := awml.Parse(source)
	if err != nil {
		return nil, err
	}
	if mistakes := awml.Unsound(value); len(mistakes) > 0 {
		return nil, fmt.Errorf("%s: %s", path, strings.Join(mistakes, "; "))
	}
	return value.(*awml.Map), nil
}

// drawExample writes every view of the example at path, and gives the Mermaid paths it wrote.
func drawExample(path string) ([]string, error) {
	doc, err := read(path)
	if err != nil {
		return nil, err
	}
	var drawings []string
	for _, implementation := range append([]string{""}, awml.ImplementationIDs(doc)...) {
		view, err := awml.View(doc, implementation)
		if err != nil {
			return nil, err
		}
		stem := strings.TrimSuffix(path, ".yaml")
		if implementation != "" {
			stem += "-" + implementation
		}
		if err := os.WriteFile(stem+".mmd", []byte(awml.Draw(view)+"\n"), 0644); err != nil {
			return nil, err
		}
		drawings = append(drawings, stem+".mmd")
	}
	return drawings, nil
}

func drawMap(out string, paths []string) error {
	var workflows []awml.Workflow
	for _, path := range paths {
		doc, err := read(path)
		if err != nil {
			return err
		}
		workflows = append(workflows, awml.Workflow{File: path, Doc: doc})
	}
	return os.WriteFile(out, []byte(awml.DrawMap(workflows)+"\n"), 0644)
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
