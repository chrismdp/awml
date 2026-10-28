package awml

import (
	"fmt"
	"path"
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

// The kinds of part that refer to another workflow's: a context, connector or surface both
// workflows reach, and an outside action that starts another workflow, which names its trigger.
var refKinds = setOf([]string{"context", "connector", "surface", "outside"})

// refMistakes holds a part's ref to how it's written and the kinds of part that take one. A trigger
// belongs to the workflow it starts, so it never refers.
func refMistakes(name string, p *Map) mistakeList {
	if !p.has("ref") {
		return nil
	}
	var errors mistakeList
	if k, _ := p.get("kind").(string); k == "trigger" {
		errors.add("refs.parts", name+" has a ref, but a trigger belongs to the workflow it starts: start another workflow with an outside action whose ref names its trigger")
	} else if !refKinds[k] {
		errors.add("refs.parts", name+" has a ref, but only a context, connector, surface or outside action takes one")
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

// refersTo gives each workflow among those given that a ref from the one named could refer to, in
// the order given: none when it's to a workflow not given, and more than one when several files are
// at a path ending owner/slug.yaml. A path is followed from the folder the referring file is in.
// owner/slug is followed to a file at a path ending owner/slug.yaml, where awml pull -o owner puts
// it. Both are matched against where each file is, however it was named.
func refersTo(workflows []Workflow, from Workflow, workflow string) []int {
	want := ""
	if strings.HasSuffix(workflow, ".yaml") {
		want = path.Join(path.Dir(from.at()), workflow)
	}
	var found []int
	for i, w := range workflows {
		file := w.at()
		if want != "" && file == want || want == "" && strings.HasSuffix(file, "/"+workflow+".yaml") {
			found = append(found, i)
		}
	}
	return found
}
