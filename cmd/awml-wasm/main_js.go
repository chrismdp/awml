//go:build js && wasm

// The awml-wasm command exposes the AWML package to JavaScript.
package main

import (
	"encoding/json"
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
	expose("__awmlSummary", func(args []js.Value) string {
		var sources []string
		if err := json.Unmarshal([]byte(args[0].String()), &sources); err != nil {
			return errorJSON(err.Error())
		}
		docs := make([]*awml.Map, 0, len(sources))
		for _, source := range sources {
			doc, err := awml.Parse([]byte(source))
			if err != nil {
				return errorJSON("isn't YAML: " + strings.Join(strings.Fields(err.Error()), " "))
			}
			if errors := awml.Unsound(doc); len(errors) != 0 {
				return errorJSON(strings.Join(errors, "\n"))
			}
			docs = append(docs, doc.(*awml.Map))
		}
		encoded, err := json.Marshal(awml.Summarise(docs...))
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	expose("__awmlFit", func(args []js.Value) string {
		var sources []string
		if err := json.Unmarshal([]byte(args[0].String()), &sources); err != nil {
			return errorJSON(err.Error())
		}
		if len(sources) == 0 {
			return errorJSON("fit needs a workflow")
		}
		docs := make([]*awml.Map, 0, len(sources))
		for _, source := range sources {
			doc, err := awml.Parse([]byte(source))
			if err != nil {
				return errorJSON("isn't YAML: " + strings.Join(strings.Fields(err.Error()), " "))
			}
			if errors := awml.Check(doc); len(errors) != 0 {
				return errorJSON(strings.Join(errors, "\n"))
			}
			docs = append(docs, doc.(*awml.Map))
		}
		if !awml.IsCanonical(docs[0]) {
			return errorJSON("the first file must be a workflow")
		}
		var placement *awml.Map
		if implementation := optionalImplementation(args); implementation != "" {
			var err error
			placement, err = awml.View(docs[0], implementation)
			if err != nil {
				return errorJSON(err.Error())
			}
		}
		if placement == nil && len(docs) == 1 {
			return errorJSON("no platform descriptions were given")
		}
		for _, doc := range docs[1:] {
			if _, ok := doc.Get("platform"); !ok {
				return errorJSON("fit takes only platform descriptions after its workflow")
			}
		}
		encoded, err := json.Marshal(awml.Judge(docs[0], placement, docs[1:]))
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	// __awmlConvert gives one 0.6 file, upgraded from one file or joined from a legacy pair, with the
	// YAML comments of each file at its top, and the mistakes the current rules find in it, as awml
	// convert writes them.
	expose("__awmlConvert", func(args []js.Value) string {
		var sources []string
		if err := json.Unmarshal([]byte(args[0].String()), &sources); err != nil {
			return errorJSON(err.Error())
		}
		implementation := optionalImplementation(args)
		if len(sources) == 0 || len(sources) > 2 {
			return errorJSON("convert takes one file, or a security view and its platform view")
		}
		if len(sources) == 1 && implementation != "" {
			return errorJSON("only a legacy pair has an implementation to name")
		}
		var parsed []any
		var comments []string
		for _, source := range sources {
			doc, err := awml.Parse([]byte(source))
			if err != nil {
				return errorJSON("isn't YAML: " + strings.Join(strings.Fields(err.Error()), " "))
			}
			found, err := awml.Comments([]byte(source))
			if err != nil {
				return errorJSON(err.Error())
			}
			parsed = append(parsed, doc)
			comments = append(comments, found...)
		}
		var converted *awml.Map
		var err error
		if len(parsed) == 1 {
			converted, err = awml.Upgrade(parsed[0])
		} else {
			if implementation == "" {
				implementation = "default"
			}
			security, _ := parsed[0].(*awml.Map)
			platform, _ := parsed[1].(*awml.Map)
			converted, err = awml.ConvertWithImplementation(security, platform, implementation)
		}
		if err != nil {
			return errorJSON(err.Error())
		}
		data, err := awml.Marshal(converted)
		if err != nil {
			return errorJSON(err.Error())
		}
		var file strings.Builder
		for _, comment := range comments {
			file.WriteString(comment + "\n")
		}
		file.Write(data)
		mistakes := awml.Check(converted)
		if mistakes == nil {
			mistakes = []string{}
		}
		encoded, err := json.Marshal(struct {
			File     string   `json:"file"`
			Mistakes []string `json:"mistakes"`
		}{file.String(), mistakes})
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
