package awml

import (
	"strings"
)

// A Refusal is a command refusing what it was asked to do, rather than a mistake in a file, such as
// fit given no workflow. awml says it as a mistake in how it was called.
type Refusal string

func (r Refusal) Error() string { return string(r) }

// Mistakes are what stop a command reading the files it was given, each with its file, in the order
// the files were given.
type Mistakes []FileMistake

// Error says each mistake on a line of its own, after its file's name, as awml prints them. A file
// given without a name has its mistakes said alone.
func (m Mistakes) Error() string {
	lines := make([]string, len(m))
	for i, mistake := range m {
		lines[i] = mistake.Message
		if mistake.File != "" {
			lines[i] = mistake.File + ": " + mistake.Message
		}
	}
	return strings.Join(lines, "\n")
}

// ReadSources reads files given together, as every command but convert reads them, and gives each
// as a workflow, platform descriptions included. When any file has mistakes it gives them all: each
// file's own, then those following each ref between the workflows given finds. A command that draws
// or counts red lines reads them drawn, taking a file whose only mistakes are lines that break a
// rule.
func ReadSources(sources []Source, drawn bool) ([]Workflow, error) {
	find, together := Check, TogetherMistakes
	if drawn {
		find, together = Unsound, TogetherUnsound
	}
	sources = once(sources)
	docs := make([]*Map, len(sources))
	found := make([][]string, len(sources))
	var given []Workflow
	var at []int
	for i, source := range sources {
		docs[i], found[i] = readSource(source, find)
		if IsCanonical(docs[i]) {
			given = append(given, source.workflow(docs[i]))
			at = append(at, i)
		}
	}
	for n, mistakes := range together(given) {
		found[at[n]] = append(found[at[n]], mistakes...)
	}
	var mistakes Mistakes
	workflows := make([]Workflow, 0, len(sources))
	for i, source := range sources {
		for _, message := range found[i] {
			mistakes = append(mistakes, FileMistake{source.File, message})
		}
		workflows = append(workflows, source.workflow(docs[i]))
	}
	if mistakes != nil {
		return nil, mistakes
	}
	return workflows, nil
}

// readSource gives the document in a file, if it holds a mapping, and the mistakes find reports in
// it.
func readSource(source Source, find func(any) []string) (*Map, []string) {
	if source.Unreadable != "" {
		return nil, []string{source.Unreadable}
	}
	doc, err := Parse(source.Text)
	if err != nil {
		return nil, []string{notYAML(err)}
	}
	file, _ := doc.(*Map)
	return file, find(doc)
}

// notYAML says why a file isn't YAML, on one line.
func notYAML(err error) string {
	return "isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")
}

func (s Source) workflow(doc *Map) Workflow {
	return Workflow{File: s.File, Path: s.Path, Doc: doc}
}

// once gives the files given, each once, where it's first given, telling a file given again by where
// it is. A file given as text alone, with no name, is never given again.
func once(sources []Source) []Source {
	var given []Source
	seen := map[string]bool{}
	for _, source := range sources {
		if source.File != "" || source.Path != "" {
			at := source.workflow(nil).at()
			if seen[at] {
				continue
			}
			seen[at] = true
		}
		given = append(given, source)
	}
	return given
}

// DrawSources draws what GraphSources gives, as awml draw does.
func DrawSources(sources []Source, implementation string) (string, error) {
	graph, err := graphSources(sources, implementation, "draws")
	if err != nil {
		return "", err
	}
	return graph.Mermaid(), nil
}

// GraphSources gives what awml graph gives: the view of one workflow, its security view or the
// implementation named, or the map of several drawn together.
func GraphSources(sources []Source, implementation string) (Graph, error) {
	return graphSources(sources, implementation, "takes")
}

// graphSources gives what draw and graph draw. verb is what the command does with one workflow, as
// its refusal of an implementation of several says.
func graphSources(sources []Source, implementation, verb string) (Graph, error) {
	sources = once(sources)
	if len(sources) == 0 {
		return Graph{}, Refusal("no workflow was given")
	}
	if implementation != "" && len(sources) > 1 {
		return Graph{}, Refusal("argument --implementation: " + verb + " one workflow, not several")
	}
	workflows, err := ReadSources(sources, true)
	if err != nil {
		return Graph{}, err
	}
	if len(workflows) > 1 {
		for _, w := range workflows {
			if !IsCanonical(w.Doc) {
				_, err := View(w.Doc, "")
				return Graph{}, Mistakes{{w.File, err.Error()}}
			}
		}
		return GraphMap(workflows), nil
	}
	view, err := View(workflows[0].Doc, implementation)
	if err != nil {
		return Graph{}, err
	}
	return GraphOf(view), nil
}

// FitSources judges each platform described as running every workflow given, as awml fit does,
// telling the two apart by what they hold: a file with platform is a platform description. Given an
// implementation, it judges where that implementation of one workflow places its parts.
func FitSources(sources []Source, implementation string) (FitReport, error) {
	workflows, descriptions, err := fitGiven(sources)
	if err != nil {
		return FitReport{}, err
	}
	if implementation != "" {
		if len(workflows) > 1 {
			return FitReport{}, Refusal("argument --implementation: judges one workflow, not several")
		}
		placement, err := View(workflows[0].Doc, implementation)
		if err != nil {
			return FitReport{}, err
		}
		return Judge(workflows[0].Doc, placement, descriptions), nil
	}
	if len(descriptions) == 0 {
		return FitReport{}, Refusal("no platform descriptions were given")
	}
	return JudgeTogether(workflows, descriptions), nil
}

// FitPartsSources judges each part of the workflows given on its own against each platform
// described, and gives the fewest platforms that between them hold every part, as awml fit --parts
// does.
func FitPartsSources(sources []Source) (PartsReport, error) {
	workflows, descriptions, err := fitGiven(sources)
	if err != nil {
		return PartsReport{}, err
	}
	if len(descriptions) == 0 {
		return PartsReport{}, Refusal("no platform descriptions were given")
	}
	return JudgeParts(workflows, descriptions), nil
}

// fitGiven reads the files fit is given and tells the workflows from the platform descriptions by
// what they hold: a file with platform is a platform description. It refuses where no workflow was
// given.
func fitGiven(sources []Source) ([]Workflow, []*Map, error) {
	read, err := ReadSources(sources, false)
	if err != nil {
		return nil, nil, err
	}
	var workflows []Workflow
	var descriptions []*Map
	for _, w := range read {
		if w.Doc.has("platform") {
			descriptions = append(descriptions, w.Doc)
		} else {
			workflows = append(workflows, w)
		}
	}
	if len(workflows) == 0 {
		return nil, nil, Refusal("no workflow was given")
	}
	return workflows, descriptions, nil
}

// SummariseSources summarises the workflows given together, as awml summary does.
func SummariseSources(sources []Source) (Summary, error) {
	workflows, err := ReadSources(sources, true)
	if err != nil {
		return Summary{}, err
	}
	return SummariseTogether(workflows), nil
}
