package awml

import (
	"runtime/debug"
	"strings"
)

// BuildVersion gives the version Go stamped on the program built from this module, or dev when it
// stamped none. A release names its version itself.
func BuildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		buildVersion := strings.TrimPrefix(info.Main.Version, "v")
		if buildVersion != "" && buildVersion != "(devel)" {
			return buildVersion
		}
	}
	return "dev"
}

// Source is one file given to CheckSources: its name, its text, and why it couldn't be read, if it
// couldn't.
type Source struct {
	File       string
	Text       []byte
	Unreadable string
}

// CheckReport lists every check the engine makes, in the order the spec lists them, with what each
// found in the files given.
type CheckReport struct {
	Engine string        `json:"engine"`
	Files  []CheckedFile `json:"files"`
	Checks []CheckResult `json:"checks"`
}

// CheckedFile names a file and what kind of file it was read as: workflow, security, platform or
// platform-description. A file that couldn't be read as one has no view.
type CheckedFile struct {
	File string `json:"file"`
	View string `json:"view,omitempty"`
}

// CheckResult is one check: pass, fail, skipped or warn, with the mistakes it found. Its id stays
// the same from release to release, and a check that changes meaning gets a new one.
type CheckResult struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	About          string        `json:"about"`
	AppliesTo      []string      `json:"applies_to"`
	Result         string        `json:"result"`
	SkippedBecause string        `json:"skipped_because,omitempty"`
	Mistakes       []FileMistake `json:"mistakes"`
}

// FileMistake is one mistake, word for word as awml check prints it, and the file it is in. A
// difference between two views names both files.
type FileMistake struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

var (
	everyFile = []string{"workflow", "security", "platform", "platform-description"}
	workflows = []string{"workflow", "security", "platform"}
)

// checkDefinition describes one check. early is whether it runs before a broken list stops the
// rest, warn whether it warns rather than fails, and legacy03 whether a legacy view must be 0.3
// for it to apply. none says why it was skipped when no file given is one it applies to.
type checkDefinition struct {
	id, name, about string
	appliesTo       []string
	early, warn     bool
	legacy03        bool
	none            string
}

var checkDefinitions = []checkDefinition{
	{id: "file.yaml", name: "The file is YAML", about: "Each file can be read, and holds a YAML mapping of fields.", appliesTo: everyFile, early: true},
	{id: "file.version", name: "The file says which version and view it is", about: "awml names a version this release reads, and a legacy view says it is security or platform.", appliesTo: everyFile, early: true},
	{id: "fields.known", name: "Every field is known and none is missing", about: "Each file, scope, part, line, implementation, block and comment has the fields it needs and no others, and each list is a list of mappings.", appliesTo: everyFile, early: true},
	{id: "kinds.known", name: "Every kind is known", about: "Each part and line has a kind the spec defines.", appliesTo: workflows},
	{id: "ids.unique", name: "No id is used twice", about: "Scope, part, block and comment ids are unique across the file.", appliesTo: workflows},
	{id: "ids.written", name: "Every id is well written", about: "An id is letters, digits and underscores, starting with a letter, and a block's platform is a company id.", appliesTo: workflows},
	{id: "values.text", name: "Every value that should be text is text", about: "Ids, labels, kinds and references are text, not something YAML read as true, false, a number or a date.", appliesTo: everyFile, early: true},
	{id: "lines.breaks", name: "Every breaks is true or false", about: "A line's breaks, which draws it red, is true or false, unquoted.", appliesTo: workflows},
	{id: "references.known", name: "Every reference names something in the file", about: "Each in, from and to, each part a block names and each part a comment ties names a scope, block or part the file has.", appliesTo: workflows},
	{id: "nesting.acyclic", name: "No scope or block is nested in itself", about: "Following in from any scope or block never leads back to it.", appliesTo: workflows},
	{id: "comments.tie-only", name: "A comment is linked only by tie lines", about: "A comment moves nothing, so only a tie, or a hidden line, may touch it.", appliesTo: workflows},
	{id: "ties.comment", name: "Every tie has a comment at one end", about: "A tie links a comment to what it explains, so one of its ends is a comment.", appliesTo: workflows},
	{id: "ops.known", name: "Every op is known and on a step", about: "An op is one of the operations the spec lists, on an ai, code or person step.", appliesTo: workflows},
	{id: "operations.only-do-acts", name: "Only a do step leads to an outside action", about: "A step that leads to an outside action, straight or through a review or a fork, is a do step.", appliesTo: workflows},
	{id: "operations.answer-check-read-only", name: "An answer or check step never writes to a context", about: "An answer step and a check step read contexts and speak on surfaces, but never write to a context.", appliesTo: workflows},
	{id: "lines.data-narrows", name: "Every data line stays in its scope or narrows", about: "A data line moves content only within its scope or into one inside it, since any other move is publishing drawn as routing.", appliesTo: []string{"workflow", "security"}, none: "no file given is a workflow or a security view"},
	{id: "processing.written", name: "Every processing is well written", about: "A scope's processing is any, none, or a map from company ids to any or a list of known operations.", appliesTo: []string{"workflow", "security"}, legacy03: true, none: "no file given is a 0.4 workflow or a 0.3 security view"},
	{id: "processing.nested", name: "An inner scope allows no more processing than its outer one", about: "A scope inside another allows no company or operation that the outer scope doesn't.", appliesTo: []string{"workflow", "security"}, legacy03: true, none: "no file given is a 0.4 workflow or a 0.3 security view"},
	{id: "processing.scopes-only", name: "Only a scope takes processing", about: "processing belongs to a scope, never to a block of the platform view.", appliesTo: []string{"workflow", "platform"}, legacy03: true, none: "no file given is a 0.4 workflow or a 0.3 platform view"},
	{id: "platform.blocks-only", name: "Only a block takes a platform", about: "platform belongs to a block of the platform view, never to a scope.", appliesTo: []string{"workflow", "security"}, legacy03: true, none: "no file given is a 0.4 workflow or a 0.3 security view"},
	{id: "wait.duration", name: "Every wait is a duration on a trigger", about: "wait is an ISO 8601 duration in weeks, days, hours, minutes or seconds, given only on a trigger.", appliesTo: workflows, legacy03: true, none: "no file given is a 0.4 workflow or a 0.3 view"},
	{id: "at.path", name: "Every at is a path on a context", about: "at is a relative path of plain names, given only on a context.", appliesTo: workflows, legacy03: true, none: "no file given is a 0.4 workflow or a 0.3 view"},
	{id: "placement.once", name: "No part is placed twice in an implementation", about: "Each part sits in at most one block of each implementation.", appliesTo: []string{"workflow"}, none: "no file given is a 0.4 workflow"},
	{id: "placement.complete", name: "Every part that needs a block has one", about: "Every part but a trigger or comment sits in a block of each implementation.", appliesTo: []string{"workflow"}, none: "no file given is a 0.4 workflow"},
	{id: "warn.do-by-code", name: "No do step is decided by AI", about: "A do step should be code acting on what an earlier step decided; this warns and never fails.", appliesTo: workflows, warn: true},
	{id: "warn.one-change", name: "Each do step makes one change", about: "A do step should lead to one change, counting each outside action and surface and all contexts together as one; this warns and never fails.", appliesTo: workflows, warn: true},
	{id: "description.values", name: "Every platform description value is of the right kind", about: "platform is a mapping, off_for_up_to a duration, steps code or agent, agents and calls lists of company ids, and holds a list.", appliesTo: []string{"platform-description"}, none: "no file given is a platform description"},
	{id: "description.ids", name: "Every company id and path is well written", about: "A platform description's ids and company ids are lower-case letters, digits and hyphens, and what it holds are relative paths.", appliesTo: []string{"platform-description"}, none: "no file given is a platform description"},
	{id: "views.same-parts", name: "Both views have the same parts", about: "A legacy security view and platform view with the same title have the same parts, of the same kinds, apart from comments and triggers the platform view leaves out.", appliesTo: []string{"security", "platform"}},
	{id: "views.same-lines", name: "Both views have the same lines", about: "A legacy security view and platform view with the same title have the same lines by their ends and kind, apart from ties, hidden lines and lines to what the platform view leaves out.", appliesTo: []string{"security", "platform"}},
}

// checkedSource is what CheckSources learnt of one file.
type checkedSource struct {
	doc      *Map
	read     bool
	view     string
	version  string
	stopped  bool
	mistakes mistakeList
}

// CheckSources makes every check on the files given and reports each check's result, passing ones
// included. engine names the release making the checks.
func CheckSources(engine string, sources []Source) CheckReport {
	report := CheckReport{Engine: engine, Files: []CheckedFile{}}
	checked := make([]checkedSource, len(sources))
	docs := make([]*Map, len(sources))
	for i, source := range sources {
		checked[i] = checkSource(source)
		docs[i] = checked[i].doc
		report.Files = append(report.Files, CheckedFile{source.File, checked[i].view})
	}
	var compared int
	var differences []FileMistake
	var differenceChecks []string
	for _, pair := range ViewPairs(docs) {
		found, ok := compareViews(docs[pair[0]], docs[pair[1]])
		if !ok {
			continue
		}
		compared++
		for _, difference := range found {
			differences = append(differences, FileMistake{sources[pair[0]].File + " and " + sources[pair[1]].File, difference.message})
			differenceChecks = append(differenceChecks, difference.check)
		}
	}

	for _, definition := range checkDefinitions {
		result := CheckResult{ID: definition.id, Name: definition.name, About: definition.about,
			AppliesTo: definition.appliesTo, Mistakes: []FileMistake{}}
		ran, applies := false, false
		if strings.HasPrefix(definition.id, "views.") {
			ran = compared > 0
			for i, difference := range differences {
				if differenceChecks[i] == definition.id {
					result.Mistakes = append(result.Mistakes, difference)
				}
			}
		} else {
			for i, file := range checked {
				found := false
				for _, m := range file.mistakes {
					if m.check == definition.id {
						result.Mistakes = append(result.Mistakes, FileMistake{sources[i].File, m.message})
						found = true
					}
				}
				if found || definition.appliesToFile(file) {
					applies = true
					ran = ran || found || definition.early || !file.stopped
				}
			}
		}
		switch {
		case len(result.Mistakes) > 0 && definition.warn:
			result.Result = "warn"
		case len(result.Mistakes) > 0:
			result.Result = "fail"
		case ran:
			result.Result = "pass"
		default:
			result.Result = "skipped"
			result.SkippedBecause = definition.skippedBecause(checked, applies)
		}
		report.Checks = append(report.Checks, result)
	}
	return report
}

func checkSource(source Source) checkedSource {
	if source.Unreadable != "" {
		return checkedSource{mistakes: mistakeList{{"file.yaml", source.Unreadable}}, stopped: true}
	}
	parsed, err := Parse(source.Text)
	if err != nil {
		return checkedSource{mistakes: mistakeList{{"file.yaml", "isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")}}, stopped: true}
	}
	found, stopped := checkDoc(parsed)
	file, ok := parsed.(*Map)
	if !ok {
		return checkedSource{mistakes: found, stopped: true}
	}
	result := checkedSource{doc: file, read: true, version: show(file.get("awml")), stopped: stopped,
		mistakes: append(found, warningList(file)...)}
	switch {
	case IsCanonical(file):
		result.view = "workflow"
	case file.has("platform"):
		result.view = "platform-description"
	case isOneOf(file.get("view"), "security", "platform"):
		result.view = str(file, "view")
	}
	return result
}

// appliesToFile is whether a check is one the file is held to. A legacy view whose view isn't
// known is held to the checks that every view is.
func (definition checkDefinition) appliesToFile(file checkedSource) bool {
	if !file.read {
		return definition.id == "file.yaml"
	}
	if file.view == "" {
		return isOneOf("security", definition.appliesTo...) && isOneOf("platform", definition.appliesTo...) &&
			(!definition.legacy03 || file.version == "0.3")
	}
	if !isOneOf(file.view, definition.appliesTo...) {
		return false
	}
	return !definition.legacy03 || file.view == "workflow" || file.view == "platform-description" || file.version == "0.3"
}

func (definition checkDefinition) skippedBecause(checked []checkedSource, applies bool) string {
	if strings.HasPrefix(definition.id, "views.") {
		return "no sound security view and platform view share a title"
	}
	if applies {
		return "an earlier mistake stopped it in every file it applies to"
	}
	if len(checked) == 0 {
		return "no file was given"
	}
	read := false
	for _, file := range checked {
		read = read || file.read
	}
	if !read {
		return "no file given could be read as YAML"
	}
	if definition.none != "" {
		return definition.none
	}
	return "no file given is a workflow or a view"
}

// ViewPairs gives the legacy security and platform views that awml check compares: one of each
// with the same title, where no other view shares it. Each pair holds the security view's index
// and then the platform view's.
func ViewPairs(docs []*Map) [][2]int {
	type views struct{ security, platform []int }
	byTitle := map[string]*views{}
	var titles []string
	for i, doc := range docs {
		if doc == nil || !isOneOf(doc.get("view"), "security", "platform") {
			continue
		}
		title := str(doc, "title")
		if byTitle[title] == nil {
			byTitle[title] = &views{}
			titles = append(titles, title)
		}
		if doc.get("view") == "security" {
			byTitle[title].security = append(byTitle[title].security, i)
		} else {
			byTitle[title].platform = append(byTitle[title].platform, i)
		}
	}
	var pairs [][2]int
	for _, title := range titles {
		if v := byTitle[title]; len(v.security) == 1 && len(v.platform) == 1 {
			pairs = append(pairs, [2]int{v.security[0], v.platform[0]})
		}
	}
	return pairs
}
