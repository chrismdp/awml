//go:build js && wasm

// The awml-wasm command exposes the AWML package to JavaScript.
package main

import (
	"encoding/json"
	"errors"
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
		for _, file := range files {
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
		workflows, err := readGiven(args[0].String(), awml.Unsound)
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
		docs, err := readGiven(args[0].String(), awml.Check)
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
		workflows, err := readGiven(args[0].String(), awml.Unsound)
		if err != nil {
			return errorJSON(err.Error())
		}
		for _, workflow := range workflows {
			if !awml.IsCanonical(workflow.Doc) {
				_, err := awml.View(workflow.Doc, "")
				return errorJSON(err.Error())
			}
		}
		return awml.DrawMap(workflows)
	})
	// __awmlConvert gives the 0.6 file awml convert writes, from one file or a legacy pair, and the
	// mistakes the current rules find in it.
	expose("__awmlConvert", func(args []js.Value) string {
		var texts []string
		if err := json.Unmarshal([]byte(args[0].String()), &texts); err != nil {
			return errorJSON(err.Error())
		}
		sources := make([]awml.Source, 0, len(texts))
		for _, text := range texts {
			sources = append(sources, awml.Source{Text: []byte(text)})
		}
		file, mistakes, err := awml.ConvertSources(sources, optionalImplementation(args))
		if err != nil {
			return errorJSON(err.Error())
		}
		if mistakes == nil {
			mistakes = []string{}
		}
		encoded, err := json.Marshal(struct {
			File     string   `json:"file"`
			Mistakes []string `json:"mistakes"`
		}{string(file), mistakes})
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	expose("__awmlDraw", func(args []js.Value) string {
		doc, err := awml.Parse([]byte(args[0].String()))
		if err != nil || len(awml.Unsound(doc)) != 0 {
			return ""
		}
		view, err := awml.View(doc.(*awml.Map), optionalImplementation(args))
		if err != nil {
			return errorJSON(err.Error())
		}
		return awml.Draw(view)
	})
	expose("__awmlDrawUntrusted", func(args []js.Value) string {
		doc, err := awml.Parse([]byte(args[0].String()))
		if err != nil || len(awml.Unsound(doc)) != 0 {
			return ""
		}
		view, err := awml.UntrustedView(doc.(*awml.Map))
		if err != nil {
			return errorJSON(err.Error())
		}
		return awml.Draw(view)
	})
	select {}
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
// is what a ref's path is followed from. It fails on the first file find reports mistakes in.
func readGiven(list string, find func(any) []string) ([]awml.Workflow, error) {
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(list), &items); err != nil {
		return nil, err
	}
	workflows := make([]awml.Workflow, 0, len(items))
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
		doc, err := awml.Parse([]byte(given.Source))
		if err != nil {
			return nil, errors.New("isn't YAML: " + strings.Join(strings.Fields(err.Error()), " "))
		}
		if mistakes := find(doc); len(mistakes) != 0 {
			return nil, errors.New(strings.Join(mistakes, "\n"))
		}
		workflows = append(workflows, awml.Workflow{File: given.File, Doc: doc.(*awml.Map)})
	}
	return workflows, nil
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
