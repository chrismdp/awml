package awml

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// A ref names a part of another workflow: owner/slug#id for a published one, or a path to a file
// alongside, ending .yaml, such as shopping.yaml#list. The owner and slug follow the registry's rule
// for names, and a path is names joined by /, which may start by going up with ...
var (
	publishedRef = regexp.MustCompile(`^([a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?/[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?)#([A-Za-z][A-Za-z0-9_]*)$`)
	pathRef      = regexp.MustCompile(`^((?:\.\./)*(?:[A-Za-z0-9_-][A-Za-z0-9_.-]*/)*[A-Za-z0-9_-][A-Za-z0-9_.-]*\.yaml)#([A-Za-z][A-Za-z0-9_]*)$`)
)

// The kinds of part a workflow can share with another: what hands work on, and what both reach.
var refKinds = setOf([]string{"trigger", "context", "connector", "surface"})

// refMistakes holds a part's ref to how it's written and the kinds of part that take one.
func refMistakes(name string, p *Map) mistakeList {
	if !p.has("ref") {
		return nil
	}
	var errors mistakeList
	if k, _ := p.get("kind").(string); !refKinds[k] {
		errors.add("refs.parts", name+" has a ref, but only a trigger, context, connector or surface takes one")
	}
	if _, _, ok := readRef(p.get("ref")); !ok {
		errors.add("refs.written", fmt.Sprintf(`%s: ref must be owner/slug#id, or a path to a file alongside such as shopping.yaml#list, not "%s"`, name, show(p.get("ref"))))
	}
	return errors
}

// readRef gives the workflow a ref names, as written before its #, and the id of the part.
func readRef(value any) (workflow, part string, ok bool) {
	text, _ := value.(string)
	for _, form := range []*regexp.Regexp{publishedRef, pathRef} {
		if found := form.FindStringSubmatch(text); found != nil {
			return found[1], found[2], true
		}
	}
	return "", "", false
}

// refersTo gives the workflow among those given that a ref from the file named refers to, or -1
// when none of them is it. A path is followed from the referring file's folder. owner/slug is
// followed to a file at a path ending owner/slug.yaml, where awml pull -o owner puts it.
func refersTo(workflows []Workflow, from, workflow string) int {
	want := ""
	if strings.HasSuffix(workflow, ".yaml") {
		want = path.Clean(path.Join(path.Dir(filepath.ToSlash(from)), workflow))
	}
	for i, w := range workflows {
		file := path.Clean(filepath.ToSlash(w.File))
		if want != "" && file == want || want == "" && (file == workflow+".yaml" || strings.HasSuffix(file, "/"+workflow+".yaml")) {
			return i
		}
	}
	return -1
}
