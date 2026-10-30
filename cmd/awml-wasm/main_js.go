//go:build js && wasm

// The awml-wasm command exposes the AWML package to JavaScript.
package main

import (
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/chrismdp/awml"
)

var callbacks []js.Func

// version is the release, when the build names it, as the awml command's does.
var version string

func main() {
	expose("__awmlChecks", func(args []js.Value) string {
		sources, err := given(args[0].String())
		if err != nil {
			return errorJSON(err.Error())
		}
		return encoded(awml.CheckSources(engine(), sources), nil)
	})
	// __awmlCheck gives what awml check reports of a file, warnings aside: its errors, then the
	// mistakes that make it invalid.
	expose("__awmlCheck", func(args []js.Value) string {
		_, errs, invalid := alone(args[0]).Said(0)
		return list(append(errs, invalid...))
	})
	// __awmlErrors gives a file's errors, the lines that break a rule, as awml check says them.
	expose("__awmlErrors", func(args []js.Value) string {
		_, errs, _ := alone(args[0]).Said(0)
		return list(errs)
	})
	// __awmlUnsound gives the mistakes that make a file invalid, as awml draw prints them.
	expose("__awmlUnsound", func(args []js.Value) string {
		_, err := awml.ReadSources([]awml.Source{{Text: []byte(args[0].String())}})
		var mistakes awml.Mistakes
		errors.As(err, &mistakes)
		messages := make([]string, len(mistakes))
		for i, mistake := range mistakes {
			messages[i] = mistake.Message
		}
		return list(messages)
	})
	expose("__awmlWarnings", func(args []js.Value) string {
		warnings, _, _ := alone(args[0]).Said(0)
		return list(warnings)
	})
	// __awmlSummary summarises the workflows given together, as awml summary does.
	expose("__awmlSummary", func(args []js.Value) string {
		sources, err := given(args[0].String())
		if err != nil {
			return errorJSON(err.Error())
		}
		return encoded(awml.SummariseSources(sources))
	})
	// __awmlFit judges each platform described as running every workflow given, as awml fit does.
	expose("__awmlFit", func(args []js.Value) string {
		sources, err := given(args[0].String())
		if err != nil {
			return errorJSON(err.Error())
		}
		return encoded(awml.FitSources(sources, optionalImplementation(args)))
	})
	// __awmlFitParts judges each part alone against each platform described, as awml fit --parts does.
	expose("__awmlFitParts", func(args []js.Value) string {
		sources, err := given(args[0].String())
		if err != nil {
			return errorJSON(err.Error())
		}
		return encoded(awml.FitPartsSources(sources))
	})
	// __awmlConvert gives the 0.10 file awml convert writes, from one file or a legacy pair, what it
	// notes it couldn't carry over, and the mistakes that make it invalid under the current rules.
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
		return encoded(struct {
			File     string   `json:"file"`
			Notes    []string `json:"notes"`
			Mistakes []string `json:"mistakes"`
		}{string(file), notes, mistakes}, nil)
	})
	// __awmlDraw draws one workflow's view, or the map of several, as awml draw does.
	expose("__awmlDraw", func(args []js.Value) string {
		sources, err := given(args[0].String())
		if err != nil {
			return errorJSON(err.Error())
		}
		drawn, err := awml.DrawSources(sources, optionalImplementation(args))
		if err != nil {
			return errorJSON(err.Error())
		}
		return drawn
	})
	// __awmlGraph gives what __awmlDraw draws, as awml graph does. Every label is given exactly as
	// the file wrote it, and no text in the JSON is escaped, so a page that shows it must escape it
	// itself, unlike __awmlDraw, whose Mermaid escapes it.
	expose("__awmlGraph", func(args []js.Value) string {
		sources, err := given(args[0].String())
		if err != nil {
			return errorJSON(err.Error())
		}
		return encoded(awml.GraphSources(sources, optionalImplementation(args)))
	})
	select {}
}

// given decodes the files given as a JSON list, each its text or { file, source }. A file's name is
// its path from one root, which a ref's path is followed from.
func given(list string) ([]awml.Source, error) {
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(list), &items); err != nil {
		return nil, err
	}
	sources := make([]awml.Source, 0, len(items))
	for _, item := range items {
		var file struct {
			File   string `json:"file"`
			Source string `json:"source"`
		}
		if err := json.Unmarshal(item, &file.Source); err != nil {
			if err := json.Unmarshal(item, &file); err != nil {
				return nil, err
			}
		}
		sources = append(sources, awml.Source{File: file.File, Text: []byte(file.Source)})
	}
	return sources, nil
}

// alone checks one file given as text alone.
func alone(text js.Value) awml.CheckReport {
	return awml.CheckSources(engine(), []awml.Source{{Text: []byte(text.String())}})
}

// engine names the release, as awml check --json does.
func engine() string {
	if version != "" {
		return version
	}
	return awml.BuildVersion()
}

// encoded gives a result as JSON, or the error in its place.
func encoded(result any, err error) string {
	if err != nil {
		return errorJSON(err.Error())
	}
	out, err := json.Marshal(result)
	if err != nil {
		return errorJSON(err.Error())
	}
	return string(out)
}

// list gives messages as a JSON list, empty when there are none.
func list(messages []string) string {
	if messages == nil {
		messages = []string{}
	}
	out, _ := json.Marshal(messages)
	return string(out)
}

func expose(name string, call func([]js.Value) string) {
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any { return call(args) })
	callbacks = append(callbacks, callback)
	js.Global().Set(name, callback)
}

func errorJSON(message string) string {
	out, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{message})
	return string(out)
}

func optionalImplementation(args []js.Value) string {
	if len(args) > 1 && args[1].Type() == js.TypeString {
		return args[1].String()
	}
	return ""
}
