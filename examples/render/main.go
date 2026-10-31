// The render helper writes every view of an example and lists its Mermaid paths for render.sh.
// Given -map OUT and several examples, it writes their map, as awml draw does given them together.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chrismdp/awml"
)

func main() {
	mapOut := flag.String("map", "", "write the workflows' map to this Mermaid file")
	platforms := flag.String("descriptions", "", "folder of platform descriptions")
	flag.Parse()
	if *mapOut != "" {
		if err := drawMap(*mapOut, flag.Args()); err != nil {
			fail(err)
		}
		fmt.Println(*mapOut)
		return
	}
	var descriptions []awml.Source
	if *platforms != "" {
		paths, err := filepath.Glob(filepath.Join(*platforms, "*.yaml"))
		if err != nil {
			fail(err)
		}
		for _, path := range paths {
			text, err := os.ReadFile(path)
			if err != nil {
				fail(err)
			}
			descriptions = append(descriptions, awml.Source{File: path, Text: text})
		}
	}
	for _, path := range flag.Args() {
		drawings, err := drawExample(path, descriptions...)
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
func drawExample(path string, descriptions ...awml.Source) ([]string, error) {
	doc, err := read(path)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sources := append([]awml.Source{{File: path, Text: text}}, descriptions...)
	var drawings []string
	for _, implementation := range append([]string{""}, awml.ImplementationIDs(doc)...) {
		drawn, err := awml.DrawSources(sources, implementation)
		if err != nil {
			return nil, err
		}
		stem := strings.TrimSuffix(path, ".yaml")
		if implementation != "" {
			stem += "-" + implementation
		}
		if err := os.WriteFile(stem+".mmd", []byte(drawn+"\n"), 0644); err != nil {
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
