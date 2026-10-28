//go:build js && wasm

// The awml-wasm command exposes the AWML package to JavaScript.
package main

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"syscall/js"

	"github.com/chrismdp/awml"
)

var callbacks []js.Func

// version is the release, when the build names it, as the awml command's does.
var version string

func main() {
	expose("__awmlChecks", func(args []js.Value) string {
		var files []struct {
			File   string `json:"file"`
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(args[0].String()), &files); err != nil {
			return errorJSON(err.Error())
		}
		sources := make([]awml.Source, 0, len(files))
		named := names{}
		for _, file := range files {
			if named.again(file.File) {
				continue
			}
			sources = append(sources, awml.Source{File: file.File, Text: []byte(file.Source)})
		}
		engine := version
		if engine == "" {
			engine = awml.BuildVersion()
		}
		encoded, err := json.Marshal(awml.CheckSources(engine, sources))
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	expose("__awmlCheck", func(args []js.Value) string {
		errors := checkSource(args[0].String())
		encoded, _ := json.Marshal(errors)
		return string(encoded)
	})
	// __awmlUnsound gives the mistakes that stop a file being drawn, as awml draw prints them.
	expose("__awmlUnsound", func(args []js.Value) string {
		errors := unsoundSource(args[0].String())
		encoded, _ := json.Marshal(errors)
		return string(encoded)
	})
	expose("__awmlWarnings", func(args []js.Value) string {
		warnings := warningsSource(args[0].String())
		encoded, _ := json.Marshal(warnings)
		return string(encoded)
	})
	// __awmlSummary summarises the workflows given together, as awml summary does.
	expose("__awmlSummary", func(args []js.Value) string {
		workflows, err := readGiven(args[0].String(), true)
		if err != nil {
			return errorJSON(err.Error())
		}
		encoded, err := json.Marshal(awml.SummariseTogether(workflows))
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	// __awmlFit judges each platform described as running every workflow given, telling the two apart
	// by what they hold, as awml fit does.
	expose("__awmlFit", func(args []js.Value) string {
		docs, err := readGiven(args[0].String(), false)
		if err != nil {
			return errorJSON(err.Error())
		}
		var workflows []awml.Workflow
		var descriptions []*awml.Map
		for _, doc := range docs {
			if _, ok := doc.Doc.Get("platform"); ok {
				descriptions = append(descriptions, doc.Doc)
			} else {
				workflows = append(workflows, doc)
			}
		}
		if len(workflows) == 0 {
			return errorJSON("no workflow was given")
		}
		var report awml.FitReport
		if implementation := optionalImplementation(args); implementation != "" {
			if len(workflows) > 1 {
				return errorJSON("an implementation is judged for one workflow, not several")
			}
			placement, err := awml.View(workflows[0].Doc, implementation)
			if err != nil {
				return errorJSON(err.Error())
			}
			report = awml.Judge(workflows[0].Doc, placement, descriptions)
		} else if len(descriptions) == 0 {
			return errorJSON("no platform descriptions were given")
		} else {
			report = awml.JudgeTogether(workflows, descriptions)
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	// __awmlDrawMap draws the map of the workflows given together, as awml draw does given several.
	expose("__awmlDrawMap", func(args []js.Value) string {
		graph, err := mapOf(args)
		if err != nil {
			return errorJSON(err.Error())
		}
		return graph.Mermaid()
	})
	// __awmlGraphMap gives what the map of the workflows given together draws, as awml graph does
	// given several, with its text unescaped as __awmlGraph gives it.
	expose("__awmlGraphMap", func(args []js.Value) string {
		graph, err := mapOf(args)
		if err != nil {
			return errorJSON(err.Error())
		}
		return graphJSON(graph)
	})
	// __awmlConvert gives the 0.8 file awml convert writes, from one file or a legacy pair, what it
	// notes it couldn't carry over, and the mistakes the current rules find in it.
	expose("__awmlConvert", func(args []js.Value) string {
		var texts []string
		if err := json.Unmarshal([]byte(args[0].String()), &texts); err != nil {
			return errorJSON(err.Error())
		}
		sources := make([]awml.Source, 0, len(texts))
		for _, text := range texts {
			sources = append(sources, awml.Source{Text: []byte(text)})
		}
		file, notes, mistakes, err := awml.ConvertSources(sources, optionalImplementation(args))
		if err != nil {
			return errorJSON(err.Error())
		}
		if notes == nil {
			notes = []string{}
		}
		if mistakes == nil {
			mistakes = []string{}
		}
		encoded, err := json.Marshal(struct {
			File     string   `json:"file"`
			Notes    []string `json:"notes"`
			Mistakes []string `json:"mistakes"`
		}{string(file), notes, mistakes})
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	// __awmlDraw draws one workflow's view. Given the file's name too, it refuses a ref the file makes
	// to itself, as awml draw does.
	expose("__awmlDraw", func(args []js.Value) string {
		view, err := viewOf(args)
		if err != nil || view == nil {
			return errorText(err)
		}
		return awml.Draw(view)
	})
	// __awmlGraph gives what __awmlDraw draws, as awml graph does. Every label is given exactly as
	// the file wrote it, and no text in the JSON is escaped, so a page that shows it must escape it
	// itself, unlike __awmlDraw, whose Mermaid escapes it.
	expose("__awmlGraph", func(args []js.Value) string {
		view, err := viewOf(args)
		if err != nil || view == nil {
			return errorText(err)
		}
		return graphJSON(awml.GraphOf(view))
	})
	select {}
}

// viewOf gives the view of one workflow that draw draws: the implementation given, or its security
// view. Given the file's name too, it refuses a ref the file makes to itself, as awml draw does. It
// gives no view for a file with mistakes, which the caller checks first.
func viewOf(args []js.Value) (*awml.Map, error) {
	doc, err := awml.Parse([]byte(args[0].String()))
	if err != nil || len(awml.Unsound(doc)) != 0 {
		return nil, nil
	}
	if len(args) > 2 && args[2].Type() == js.TypeString {
		if err := refMistakes([]awml.Workflow{{File: args[2].String(), Doc: doc.(*awml.Map)}}); err != nil {
			return nil, err
		}
	}
	return awml.View(doc.(*awml.Map), optionalImplementation(args))
}

// mapOf gives the map of the workflows given together, which draw draws given several.
func mapOf(args []js.Value) (awml.Graph, error) {
	workflows, err := readGiven(args[0].String(), true)
	if err != nil {
		return awml.Graph{}, err
	}
	for _, workflow := range workflows {
		if !awml.IsCanonical(workflow.Doc) {
			_, err := awml.View(workflow.Doc, "")
			return awml.Graph{}, err
		}
	}
	return awml.GraphMap(workflows), nil
}

func graphJSON(graph awml.Graph) string {
	encoded, err := json.Marshal(graph)
	if err != nil {
		return errorJSON(err.Error())
	}
	return string(encoded)
}

// errorText gives an error as JSON, or nothing for a file with mistakes, which has no view.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return errorJSON(err.Error())
}

func expose(name string, call func([]js.Value) string) {
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any { return call(args) })
	callbacks = append(callbacks, callback)
	js.Global().Set(name, callback)
}

func checkSource(source string) []string {
	doc, err := awml.Parse([]byte(source))
	if err != nil {
		return []string{"isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")}
	}
	errors := awml.Check(doc)
	if errors == nil {
		return []string{}
	}
	return errors
}

func unsoundSource(source string) []string {
	doc, err := awml.Parse([]byte(source))
	if err != nil {
		return []string{"isn't YAML: " + strings.Join(strings.Fields(err.Error()), " ")}
	}
	errors := awml.Unsound(doc)
	if errors == nil {
		return []string{}
	}
	return errors
}

func warningsSource(source string) []string {
	doc, err := awml.Parse([]byte(source))
	if err != nil {
		return []string{}
	}
	warnings := awml.Warnings(doc)
	if warnings == nil {
		return []string{}
	}
	return warnings
}

// readGiven reads the files given as a JSON list, each its text or { file, source }. A file's name
// is its path from one root, which a ref's path is followed from. It fails on the first file find
// reports mistakes in, and then on the mistakes following each ref between the workflows given
// finds, one to a line after the file's name, as awml prints them. A command that draws or counts
// red lines is drawn, and takes files whose only mistakes are lines that break a rule.
func readGiven(list string, drawn bool) ([]awml.Workflow, error) {
	find, together := awml.Check, awml.TogetherMistakes
	if drawn {
		find, together = awml.Unsound, awml.TogetherUnsound
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(list), &items); err != nil {
		return nil, err
	}
	// As awml does, every file's own mistakes and those following the refs between them are said
	// together, each after its file's name, in the order the files were given.
	var files []string
	var docs []*awml.Map
	var errs [][]string
	named := names{}
	for _, item := range items {
		var given struct {
			File   string `json:"file"`
			Source string `json:"source"`
		}
		if err := json.Unmarshal(item, &given.Source); err != nil {
			if err := json.Unmarshal(item, &given); err != nil {
				return nil, err
			}
		}
		if named.again(given.File) {
			continue
		}
		doc, err := awml.Parse([]byte(given.Source))
		if err != nil {
			return nil, errors.New("isn't YAML: " + strings.Join(strings.Fields(err.Error()), " "))
		}
		// A file that isn't a mapping has its mistakes said, as any other's, but isn't a workflow.
		m, _ := doc.(*awml.Map)
		files = append(files, given.File)
		docs = append(docs, m)
		errs = append(errs, find(doc))
	}
	var workflows, canonical []awml.Workflow
	var at []int
	for i, doc := range docs {
		if doc == nil {
			continue
		}
		workflows = append(workflows, awml.Workflow{File: files[i], Doc: doc})
		if awml.IsCanonical(doc) {
			canonical = append(canonical, workflows[len(workflows)-1])
			at = append(at, i)
		}
	}
	for n, mistakes := range together(canonical) {
		errs[at[n]] = append(errs[at[n]], mistakes...)
	}
	var lines []string
	for i, mistakes := range errs {
		for _, mistake := range mistakes {
			if files[i] != "" {
				mistake = files[i] + ": " + mistake
			}
			lines = append(lines, mistake)
		}
	}
	if lines != nil {
		return nil, errors.New(strings.Join(lines, "\n"))
	}
	return workflows, nil
}

// refMistakes gives the mistakes together finds following each ref between the workflows given, one
// to a line after the file's name, as awml prints them, or nil when there are none.
func refMistakes(given []awml.Workflow) error {
	var workflows []awml.Workflow
	for _, w := range given {
		if awml.IsCanonical(w.Doc) {
			workflows = append(workflows, w)
		}
	}
	var lines []string
	for n, mistakes := range awml.TogetherUnsound(workflows) {
		for _, mistake := range mistakes {
			if workflows[n].File != "" {
				mistake = workflows[n].File + ": " + mistake
			}
			lines = append(lines, mistake)
		}
	}
	if lines == nil {
		return nil
	}
	return errors.New(strings.Join(lines, "\n"))
}

// names holds the files given so far, by their paths from one root.
type names map[string]bool

// again is whether a file of this name was given already, as awml reads a file named twice once. A
// file given as text alone has no name, so it's never given again.
func (n names) again(file string) bool {
	if file == "" {
		return false
	}
	at := path.Join("/", file)
	if n[at] {
		return true
	}
	n[at] = true
	return false
}

func errorJSON(message string) string {
	encoded, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{message})
	return string(encoded)
}

func optionalImplementation(args []js.Value) string {
	if len(args) > 1 && args[1].Type() == js.TypeString {
		return args[1].String()
	}
	return ""
}
