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

func main() {
	expose("__awmlCheck", func(args []js.Value) string {
		errors := checkSource(args[0].String())
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
			if errors := awml.Check(doc); len(errors) != 0 {
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
		if len(sources) < 2 {
			return errorJSON("fit needs a security view and at least one platform description")
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
		if field(docs[0], "view") != "security" {
			return errorJSON("the first file must be a security view")
		}
		var placement *awml.Map
		start := 1
		if field(docs[1], "view") == "platform" {
			placement = docs[1]
			start = 2
		}
		if start == len(docs) {
			return errorJSON("no platform descriptions were given")
		}
		for _, doc := range docs[start:] {
			if _, ok := doc.Get("platform"); !ok {
				return errorJSON("fit takes only platform descriptions after its views")
			}
		}
		encoded, err := json.Marshal(awml.Judge(docs[0], placement, docs[start:]))
		if err != nil {
			return errorJSON(err.Error())
		}
		return string(encoded)
	})
	expose("__awmlDraw", func(args []js.Value) string {
		doc, err := awml.Parse([]byte(args[0].String()))
		if err != nil || len(awml.Check(doc)) != 0 {
			return ""
		}
		return awml.Draw(doc.(*awml.Map))
	})
	select {}
}

func field(doc *awml.Map, name string) string {
	value, _ := doc.Get(name)
	text, _ := value.(string)
	return text
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
