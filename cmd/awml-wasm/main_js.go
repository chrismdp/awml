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
	expose("__awmlDraw", func(args []js.Value) string {
		doc, err := awml.Parse([]byte(args[0].String()))
		if err != nil || len(awml.Check(doc)) != 0 {
			return ""
		}
		return awml.Draw(doc.(*awml.Map))
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

func errorJSON(message string) string {
	encoded, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{message})
	return string(encoded)
}
