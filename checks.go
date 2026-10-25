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

// CheckedFile names a file and what kind of file it was read as: workflow or platform-description.
// A file that couldn't be read as one, such as one of a version this release doesn't read, has no
// view.
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

// FileMistake is one mistake, word for word as awml check prints it, and the file it is in.
type FileMistake struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

var (
	everyFile = []string{"workflow", "platform-description"}
	workflows = []string{"workflow"}
)

// checkDefinition describes one check. early is whether it runs before a broken list stops the
// rest, and warn whether it warns rather than fails. none says why it was skipped when no file
// given is one it applies to.
type checkDefinition struct {
	id, name, about string
	appliesTo       []string
	early, warn     bool
	none            string
}

var checkDefinitions = []checkDefinition{
	{id: "file.yaml", name: "The file is YAML", about: "Each file can be read, and holds a YAML mapping of fields.", appliesTo: everyFile, early: true},
	{id: "file.version", name: "The file is of the version this release reads", about: "awml is \"0.6\". An older file is upgraded with awml convert.", appliesTo: everyFile, early: true},
	{id: "fields.known", name: "Every field is known and none is missing", about: "Each file, scope, part, line, implementation, block and comment has the fields it needs and no others, and each list is a list of mappings.", appliesTo: everyFile, early: true},
	{id: "kinds.known", name: "Every kind is known", about: "Each part and line has a kind the spec defines.", appliesTo: workflows},
	{id: "ids.unique", name: "No id is used twice", about: "Scope, part, block and comment ids are unique across the file.", appliesTo: workflows},
	{id: "ids.written", name: "Every id is well written", about: "An id is letters, digits and underscores, starting with a letter, and a block's platform is a company id or owner/platforms/slug.", appliesTo: workflows},
	{id: "values.text", name: "Every value that should be text is text", about: "Ids, labels, kinds and references are text, not something YAML read as true, false, a number or a date.", appliesTo: everyFile, early: true},
	{id: "references.known", name: "Every reference names something in the file", about: "Each in, from and to, each part a block names and each part a comment ties names a scope, block or part the file has.", appliesTo: workflows},
	{id: "nesting.acyclic", name: "No scope or block is nested in itself", about: "Following in from any scope or block never leads back to it.", appliesTo: workflows},
	{id: "comments.tie-only", name: "A comment is linked only by tie lines", about: "A comment moves nothing, so only a tie, or a hidden line, may touch it.", appliesTo: workflows},
	{id: "ties.comment", name: "Every tie has a comment at one end", about: "A tie links a comment to what it explains, so one of its ends is a comment.", appliesTo: workflows},
	{id: "ops.known", name: "Every op is known and on a step", about: "An op is one of the operations the spec lists, on an ai, code or person step.", appliesTo: workflows},
	{id: "operations.only-do-acts", name: "Only a do step leads to an outside action", about: "A step that leads to an outside action, straight or through a review or a fork, is a do step.", appliesTo: workflows},
	{id: "operations.answer-check-read-only", name: "An answer or check step never writes to a context", about: "An answer step and a check step read contexts and speak on surfaces, but never write to a context.", appliesTo: workflows},
	{id: "tiers.written", name: "Every outside action's label ends with its tier", about: "An outside action's label ends with Allow, Ask or Block after a colon.", appliesTo: workflows},
	{id: "tiers.shape", name: "Every tier agrees with what stands in front of it", about: "An Ask action has a review, a standing approval or a person's own step on every way in, an Allow action has no review or standing approval in front of it, and only red lines lead to a Block action.", appliesTo: workflows},
	{id: "approvals.listed", name: "Every standing approval lists its actions on a surface", about: "A standing approval leads to the surface where each action it covers is listed with how to undo it.", appliesTo: workflows},
	{id: "processing.written", name: "Every processing is well written", about: "A scope's processing is any, none, or a map from company ids to any or a list of known operations.", appliesTo: workflows},
	{id: "processing.nested", name: "An inner scope allows no more processing than its outer one", about: "A scope inside another allows no company or operation that the outer scope doesn't.", appliesTo: workflows},
	{id: "platform.blocks-only", name: "Only a block takes a platform", about: "platform belongs to a block of an implementation, never to a scope.", appliesTo: workflows},
	{id: "wait.duration", name: "Every wait is a duration on a trigger", about: "wait is an ISO 8601 duration in weeks, days, hours, minutes or seconds, given only on a trigger.", appliesTo: workflows},
	{id: "at.path", name: "Every at is a path on a context", about: "at is a relative path of plain names, given only on a context.", appliesTo: workflows},
	{id: "untrusted.parts", name: "Only a trigger, connector or context is marked untrusted", about: "untrusted is true or false, and is true only on a trigger, connector or context.", appliesTo: workflows},
	{id: "cleans.code-only", name: "Only a code step cleans", about: "cleans is true or false, and is true only on a code step, since strangers' text can steer an AI step however it's written.", appliesTo: workflows},
	{id: "placement.once", name: "No part is placed twice in an implementation", about: "Each part sits in at most one block of each implementation.", appliesTo: workflows},
	{id: "placement.complete", name: "Every part that needs a block has one", about: "Every part but a trigger or comment sits in a block of each implementation.", appliesTo: workflows},
	{id: "warn.do-by-code", name: "No do step is decided by AI", about: "A do step should be code acting on what an earlier step decided; this warns and never fails.", appliesTo: workflows, warn: true},
	{id: "warn.one-change", name: "Each do step makes one change", about: "A do step should lead to one change, counting each outside action and surface and all contexts together as one; this warns and never fails.", appliesTo: workflows, warn: true},
	{id: "warn.untrusted-outside", name: "No untrusted text reaches an outside action", about: "Text from a part marked untrusted reaches no outside action without a review, a person's own step or a code step marked cleans to stop it; this warns and never fails.", appliesTo: workflows, warn: true},
	{id: "description.values", name: "Every platform description value is of the right kind", about: "platform is a mapping, off_for_up_to a duration, steps code or agent, agents and calls lists of company ids, and holds a list.", appliesTo: []string{"platform-description"}, none: "no file given is a platform description"},
	{id: "description.ids", name: "Every company id and path is well written", about: "A platform description's id is a company id or owner/platforms/slug, its company ids are lower-case letters, digits and hyphens, and what it holds are relative paths.", appliesTo: []string{"platform-description"}, none: "no file given is a platform description"},
}

// checkedSource is what CheckSources learnt of one file.
type checkedSource struct {
	read     bool
	view     string
	stopped  bool
	mistakes mistakeList
}

// CheckSources makes every check on the files given and reports each check's result, passing ones
// included. engine names the release making the checks.
func CheckSources(engine string, sources []Source) CheckReport {
	report := CheckReport{Engine: engine, Files: []CheckedFile{}}
	checked := make([]checkedSource, len(sources))
	for i, source := range sources {
		checked[i] = checkSource(source)
		report.Files = append(report.Files, CheckedFile{source.File, checked[i].view})
	}

	for _, definition := range checkDefinitions {
		result := CheckResult{ID: definition.id, Name: definition.name, About: definition.about,
			AppliesTo: definition.appliesTo, Mistakes: []FileMistake{}}
		ran, applies := false, false
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
		return checkedSource{mistakes: mistakeList{{check: "file.yaml", message: source.Unreadable}}, stopped: true}
	}
	parsed, err := Parse(source.Text)
	if err != nil {
		return checkedSource{mistakes: mistakeList{{check: "file.yaml", message: "isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")}}, stopped: true}
	}
	found, stopped := checkDoc(parsed)
	file, ok := parsed.(*Map)
	if !ok {
		return checkedSource{mistakes: found, stopped: true}
	}
	result := checkedSource{read: true, stopped: stopped, mistakes: append(found, warningList(file)...)}
	switch {
	case checkVersion(file) != nil:
	case file.has("platform"):
		result.view = "platform-description"
	default:
		result.view = "workflow"
	}
	return result
}

// appliesToFile is whether a check is one the file is held to. A file of a version this release
// doesn't read is held only to being YAML of the right version.
func (definition checkDefinition) appliesToFile(file checkedSource) bool {
	if !file.read {
		return definition.id == "file.yaml"
	}
	if file.view == "" {
		return isOneOf(definition.id, "file.yaml", "file.version")
	}
	return isOneOf(file.view, definition.appliesTo...)
}

func (definition checkDefinition) skippedBecause(checked []checkedSource, applies bool) string {
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
	return "no file given is a workflow"
}
