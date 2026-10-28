package awml

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// IsCanonical reports whether a document holds a workflow of the current version.
func IsCanonical(doc *Map) bool {
	return doc != nil && doc.get("awml") == currentFormat && !doc.has("platform")
}

// ImplementationIDs gives implementations in the order they were written.
func ImplementationIDs(doc *Map) []string {
	if implementations, ok := doc.get("implementations").(*Map); ok {
		return append([]string{}, implementations.Keys()...)
	}
	return nil
}

// copyMap copies a mapping but the keys omitted, keeping how YAML wrote each value, so a copy
// written out again quotes what the file quoted.
func copyMap(source *Map, omit ...string) *Map {
	result := &Map{}
	for _, key := range source.Keys() {
		if isOneOf(key, omit...) {
			continue
		}
		result.set(key, source.get(key))
		if style, ok := source.scalarStyles[key]; ok {
			if result.scalarStyles == nil {
				result.scalarStyles = map[string]yaml.Style{}
			}
			result.scalarStyles[key] = style
		}
		if source.nonTextKeys[key] {
			if result.nonTextKeys == nil {
				result.nonTextKeys = map[string]bool{}
			}
			result.nonTextKeys[key] = true
		}
	}
	return result
}

// View projects a sound workflow into one of its views, whose boxes are groups. An empty
// implementation selects the security view. The source is never changed. Each line the check
// reports as breaking a rule is red: its breaks holds the check's words for each rule it breaks.
func View(doc *Map, implementation string) (*Map, error) {
	if doc != nil && doc.has("platform") {
		return nil, fmt.Errorf("a platform description has no drawing")
	}
	if !IsCanonical(doc) {
		return nil, fmt.Errorf(`awml draws only a workflow whose awml is "%s": upgrade an older file with awml convert`, currentFormat)
	}
	view, err := project(doc, implementation)
	if err != nil {
		return view, err
	}
	// A line that publishes is a fresh copy each time the workflow is projected, so the check's
	// lines are found by their places, which every projection keeps.
	found, _ := checkDoc(doc)
	words := map[int][]any{}
	for _, m := range found {
		for _, place := range m.places {
			words[place] = append(words[place], m.breach)
		}
	}
	// Each line that publishes keeps, in widens, what it adds or loosens, which its label ends with.
	widens := widenings(doc)
	// Each line untrusted text touches keeps how it's drawn for it, in untrusted, and each part marked
	// untrusted whose text reaches an outside action unstopped is marked unstopped.
	looks, unstopped := untrustedLooks(items(view, "parts"), items(view, "lines"))
	var lines []any
	for i, line := range items(view, "lines") {
		said, publishes := widens[i]
		look := looks[line]
		if breaches := words[i]; breaches != nil || publishes || look != "" {
			line = copyMap(line)
			if breaches != nil {
				line.set("breaks", breaches)
			}
			if publishes {
				list := make([]any, len(said))
				for n, words := range said {
					list[n] = words
				}
				line.set("widens", list)
			}
			if look != "" {
				line.set("untrusted", look)
			}
		}
		lines = append(lines, line)
	}
	if lines != nil {
		view.set("lines", lines)
	}
	if len(unstopped) > 0 {
		var parts []any
		for _, part := range items(view, "parts") {
			if unstopped[part] {
				part = copyMap(part)
				part.set("unstopped", true)
			}
			parts = append(parts, part)
		}
		view.set("parts", parts)
	}
	return view, nil
}

// project projects a workflow into a view as View does, with no line red. The file may not be
// checked yet.
func project(doc *Map, implementation string) (*Map, error) {
	out := copyMap(doc, "scopes", "implementations", "groups", "view")
	// A view keeps the shape of the separate views files once had, which Draw and the checks read.
	out.set("awml", "0.3")
	out.set("view", "security")
	if doc.has("scopes") {
		out.set("groups", doc.get("scopes"))
	}
	if doc.has("lines") {
		out.set("lines", publishedLines(doc))
	}
	if implementation == "" {
		return out, nil
	}
	implementations, _ := doc.get("implementations").(*Map)
	if implementations == nil || !implementations.has(implementation) {
		return nil, fmt.Errorf(`unknown implementation "%s"`, implementation)
	}
	impl, ok := implementations.get(implementation).(*Map)
	if !ok {
		return nil, fmt.Errorf(`implementation "%s" must be a mapping of fields`, implementation)
	}
	out.set("view", "platform")
	var groups, parts, lines []any
	where := map[string]string{}
	for _, block := range items(impl, "blocks") {
		groups = append(groups, copyMap(block, "parts"))
		if ids, ok := block.get("parts").([]any); ok {
			for _, part := range ids {
				where[show(part)] = str(block, "id")
			}
		}
	}
	for _, part := range items(doc, "parts") {
		p := copyMap(part, "in")
		if home := where[str(p, "id")]; home != "" {
			p.set("in", home)
		}
		parts = append(parts, p)
	}
	for _, line := range items(out, "lines") {
		lines = append(lines, line)
	}
	for _, comment := range items(impl, "comments") {
		p := copyMap(comment, "ties")
		p.set("kind", "comment")
		parts = append(parts, p)
		if ties, ok := comment.get("ties").([]any); ok {
			for _, target := range ties {
				line := &Map{}
				line.set("from", comment.get("id"))
				line.set("to", target)
				line.set("kind", "tie")
				lines = append(lines, line)
			}
		}
	}
	out.set("groups", groups)
	out.set("parts", parts)
	out.set("lines", lines)
	return out, nil
}

func checkCanonical(file *Map) (mistakeList, bool) {
	var errors mistakeList
	fieldsExact("the file", file, []string{"awml", "title", "parts"}, []string{"description", "scopes", "lines", "implementations"}, &errors)
	// A description is Markdown, which awml reads only as text.
	if file.has("description") {
		if _, ok := file.get("description").(string); !ok {
			errors.add("values.text", fmt.Sprintf("the file: YAML read its description as %s, not text: put it in quotes", show(file.get("description"))))
		}
	}
	security, _ := project(file, "")
	// The outer schema was checked above. Keep the existing messages for all design rules.
	clean := &Map{}
	for _, key := range security.Keys() {
		if isOneOf(key, "awml", "title", "parts", "lines", "groups", "view") {
			clean.set(key, security.get(key))
		}
	}
	designErrors, stopped := checkView(clean, false)
	errors = appendUnique(errors, placeLines(designErrors, clean, file)...)
	for n, line := range soundMaps(file.get("lines")) {
		if line.get("kind") == "publish" {
			errors.add("kinds.known", fmt.Sprintf(`%s has unknown kind "publish": write it as data, since a data line publishes whenever its scopes make it one`, lineName(n, line)))
		}
	}
	if !file.has("implementations") {
		return errors, stopped
	}
	implementations, ok := file.get("implementations").(*Map)
	if !ok {
		return append(errors, mistake{check: "fields.known", message: "implementations must be a mapping of implementation ids"}), stopped
	}
	// Avoid projecting malformed design lists. Check has already explained those mistakes.
	if stopped {
		return errors, true
	}
	knownParts := partsByID(file)
	seen := map[any]bool{}
	for _, key := range []string{"scopes", "parts"} {
		for _, item := range items(file, key) {
			seen[keyOf(item.get("id"))] = true
		}
	}
	for _, name := range implementations.Keys() {
		if implementations.nonTextKeys[name] || !id.MatchString(name) {
			errors.add("ids.written", fmt.Sprintf(`implementation id "%s" must be letters, digits and underscores, starting with a letter`, name))
		}
		impl, ok := implementations.get(name).(*Map)
		if !ok {
			errors.add("fields.known", fmt.Sprintf(`implementation "%s" must be a mapping of fields`, name))
			continue
		}
		fieldsExact(fmt.Sprintf(`implementation "%s"`, name), impl, []string{"label", "blocks"}, []string{"comments"}, &errors)
		blocks, blockErrors := canonicalList(impl, "blocks", "block")
		comments, commentErrors := canonicalList(impl, "comments", "comment")
		errors = append(errors, blockErrors...)
		errors = append(errors, commentErrors...)
		if len(blockErrors)+len(commentErrors) > 0 {
			continue
		}
		placedParts := map[string]bool{}
		for _, block := range blocks {
			blockName := fmt.Sprintf(`block "%s"`, show(block.get("id")))
			fieldsExact(blockName, block, []string{"id", "label", "parts"}, []string{"platform", "in"}, &errors)
			ids, ok := block.get("parts").([]any)
			if !ok {
				if block.has("parts") {
					errors.add("fields.known", blockName+": parts must be a list")
				}
				continue
			}
			for _, value := range ids {
				partID, text := value.(string)
				if !text {
					errors.add("values.text", fmt.Sprintf("%s: part id %s must be text: put it in quotes", blockName, show(value)))
					continue
				}
				if knownParts[partID] == nil {
					errors.add("references.known", fmt.Sprintf(`%s names unknown part "%s"`, blockName, partID))
					continue
				}
				if placedParts[partID] {
					errors.add("placement.once", fmt.Sprintf(`part "%s" is placed twice in implementation "%s"`, partID, name))
				}
				placedParts[partID] = true
			}
		}
		for _, part := range items(file, "parts") {
			if !placedParts[str(part, "id")] && !isOneOf(part.get("kind"), "trigger", "comment") {
				errors.add("placement.complete", fmt.Sprintf(`part "%s" is not placed in implementation "%s"`, str(part, "id"), name))
			}
		}
		for _, comment := range comments {
			commentName := fmt.Sprintf(`comment "%s"`, show(comment.get("id")))
			fieldsExact(commentName, comment, []string{"id", "label", "in", "ties"}, nil, &errors)
			if comment.has("in") && (comment.get("in") == nil || comment.get("in") == "") {
				errors.add("references.known", fmt.Sprintf(`part "%s" is in unknown group "%s"`, show(comment.get("id")), show(comment.get("in"))))
			}

			ties, ok := comment.get("ties").([]any)
			if !ok {
				if comment.has("ties") {
					errors.add("fields.known", commentName+": ties must be a list")
				}
				continue
			}
			for _, target := range ties {
				if _, ok := target.(string); !ok {
					errors.add("values.text", fmt.Sprintf("%s: tie id %s must be text: put it in quotes", commentName, show(target)))
				}
				// Implementation comments can only refer to design parts.
				if knownParts[show(target)] == nil {
					errors = appendUnique(errors, mistake{check: "references.known", message: fmt.Sprintf(`the line from "%s" to "%s" ends at unknown part "%s"`, show(comment.get("id")), show(target), show(target))})
				}
			}
		}
		for _, item := range append(append([]*Map{}, blocks...), comments...) {
			value := item.get("id")
			if value == nil {
				continue
			}
			if seen[keyOf(value)] {
				errors = appendUnique(errors, mistake{check: "ids.unique", message: fmt.Sprintf(`id "%s" is used twice`, show(value))})
			}
			seen[keyOf(value)] = true
		}
		platform, _ := project(file, name)
		platformErrors, _ := checkView(platform, true)
		errors = appendUnique(errors, placeLines(platformErrors, platform, file)...)
	}
	return errors, false
}

// placeLines gives each mistake that is lines breaking a rule the places of its lines in the file's
// list of lines. The view is the one the check found them in, which starts with the file's lines in
// their order, then a platform view's ties.
func placeLines(found mistakeList, view, file *Map) mistakeList {
	own, _ := file.get("lines").([]any)
	list, _ := view.get("lines").([]any)
	place := map[*Map]int{}
	for i, item := range list {
		if line, ok := item.(*Map); ok && i < len(own) {
			place[line] = i
		}
	}
	for n := range found {
		for _, line := range found[n].lines {
			if i, ok := place[line]; ok {
				found[n].places = append(found[n].places, i)
			}
		}
	}
	return found
}

func canonicalList(file *Map, key, singular string) ([]*Map, mistakeList) {
	if !file.has(key) {
		return nil, nil
	}
	values, ok := file.get(key).([]any)
	if !ok {
		return nil, mistakeList{{check: "fields.known", message: key + " must be a list"}}
	}
	var result []*Map
	for _, value := range values {
		item, ok := value.(*Map)
		if !ok {
			return nil, mistakeList{{check: "fields.known", message: "each " + singular + " must be a mapping of fields"}}
		}
		result = append(result, item)
	}
	return result, nil
}

// appendUnique adds each mistake whose message isn't already there.
func appendUnique(existing mistakeList, added ...mistake) mistakeList {
	for _, message := range added {
		found := false
		for _, old := range existing {
			if old.message == message.message {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, message)
		}
	}
	return existing
}

func workflowViews(docs []*Map) []*Map {
	var views []*Map
	for _, doc := range docs {
		security, err := View(doc, "")
		if err != nil {
			continue
		}
		views = append(views, security)
		implementations := doc.get("implementations")
		for _, name := range ImplementationIDs(doc) {
			platform, _ := View(doc, name)
			impl := implementations.(*Map).get(name).(*Map)
			platform.set("implementationLabel", str(impl, "label"))
			views = append(views, platform)
		}
	}
	return views
}
