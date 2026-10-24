// The render helper writes every view of an example and lists its Mermaid paths for render.sh.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chrismdp/awml"
)

func main() {
	for _, path := range os.Args[1:] {
		source, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		value, err := awml.Parse(source)
		if err != nil {
			fail(err)
		}
		if mistakes := awml.Check(value); len(mistakes) > 0 {
			fail(fmt.Errorf("%s: %s", path, strings.Join(mistakes, "; ")))
		}
		doc := value.(*awml.Map)
		for _, implementation := range append([]string{""}, awml.ImplementationIDs(doc)...) {
			view, err := awml.View(doc, implementation)
			if err != nil {
				fail(err)
			}
			stem := strings.TrimSuffix(path, ".yaml")
			if implementation != "" {
				stem += "-" + implementation
			}
			if err := os.WriteFile(stem+".mmd", []byte(awml.Draw(view)+"\n"), 0644); err != nil {
				fail(err)
			}
			fmt.Println(stem + ".mmd")
		}
		// No implementation id has a hyphen, so this name is never an implementation's.
		if view, err := awml.UntrustedView(doc); err == nil {
			stem := strings.TrimSuffix(path, ".yaml") + "-untrusted-view"
			if err := os.WriteFile(stem+".mmd", []byte(awml.Draw(view)+"\n"), 0644); err != nil {
				fail(err)
			}
			fmt.Println(stem + ".mmd")
		}
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
