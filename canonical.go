package awml

import (
	"fmt"
	"strings"
)

// IsCanonical reports whether a document holds a 0.4, 0.5 or 0.6 workflow.
func IsCanonical(doc *Map) bool {
	return doc != nil && isOneOf(doc.get("awml"), "0.4", "0.5", "0.6") && !doc.has("platform")
}

// since05 is whether a file's version has what 0.5 added: standing approvals, tiers that the check
// reads, and platforms named from the registry.
func since05(format string) bool { return isOneOf(format, "0.5", "0.6") }

// ImplementationIDs gives implementations in the order they were written.
func ImplementationIDs(doc *Map) []string {
	if implementations, ok := doc.get("implementations").(*Map); ok {
		return append([]string{}, implementations.Keys()...)
	}
	return nil
}

func copyMap(source *Map, omit ...string) *Map {
	result := &Map{}
	for _, key := range source.Keys() {
		if !isOneOf(key, omit...) {
			result.set(key, source.get(key))
		}
	}
	return result
}

// View projects a sound workflow into the same representation used by the older views.
// An empty implementation selects the security view. The source is never changed.
func View(doc *Map, implementation string) (*Map, error) {
	if !IsCanonical(doc) {
		if implementation != "" {
			return nil, fmt.Errorf("--implementation needs a 0.4, 0.5 or 0.6 workflow")
		}
		return doc, nil
	}
	out := copyMap(doc, "scopes", "implementations", "groups", "view")
	out.set("awml", "0.3")
	out.set("view", "security")
	if doc.has("scopes") {
		out.set("groups", doc.get("scopes"))
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
	for _, line := range items(doc, "lines") {
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
	fieldsExact("the file", file, []string{"awml", "title", "parts"}, []string{"scopes", "lines", "implementations"}, &errors)
	security, _ := View(file, "")
	// The outer schema was checked above. Keep the existing messages for all design rules.
	clean := &Map{}
	for _, key := range security.Keys() {
		if isOneOf(key, "awml", "title", "parts", "lines", "groups", "view") {
			clean.set(key, security.get(key))
		}
	}
	format := str(file, "awml")
	designErrors, stopped := checkView(clean, false, format)
	errors = appendUnique(errors, designErrors...)
	if !file.has("implementations") {
		return errors, stopped
	}
	implementations, ok := file.get("implementations").(*Map)
	if !ok {
		return append(errors, mistake{"fields.known", "implementations must be a mapping of implementation ids"}), stopped
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
					errors = appendUnique(errors, mistake{"references.known", fmt.Sprintf(`the line from "%s" to "%s" ends at unknown part "%s"`, show(comment.get("id")), show(target), show(target))})
				}
			}
		}
		for _, item := range append(append([]*Map{}, blocks...), comments...) {
			value := item.get("id")
			if value == nil {
				continue
			}
			if seen[keyOf(value)] {
				errors = appendUnique(errors, mistake{"ids.unique", fmt.Sprintf(`id "%s" is used twice`, show(value))})
			}
			seen[keyOf(value)] = true
		}
		platform, _ := View(file, name)
		platformErrors, _ := checkView(platform, true, format)
		errors = appendUnique(errors, platformErrors...)
	}
	return errors, false
}

func canonicalList(file *Map, key, singular string) ([]*Map, mistakeList) {
	if !file.has(key) {
		return nil, nil
	}
	values, ok := file.get(key).([]any)
	if !ok {
		return nil, mistakeList{{"fields.known", key + " must be a list"}}
	}
	var result []*Map
	for _, value := range values {
		item, ok := value.(*Map)
		if !ok {
			return nil, mistakeList{{"fields.known", "each " + singular + " must be a mapping of fields"}}
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

// Convert joins a matching legacy pair. The security view supplies the design's wording and order.
func Convert(security, platform *Map) (*Map, error) {
	return convert(security, platform, "default")
}

// ConvertWithImplementation joins a matching legacy pair and names its implementation.
func ConvertWithImplementation(security, platform *Map, implementation string) (*Map, error) {
	return convert(security, platform, implementation)
}

func convert(security, platform *Map, name string) (*Map, error) {
	for _, doc := range []*Map{security, platform} {
		if mistakes := Check(doc); len(mistakes) != 0 {
			return nil, fmt.Errorf("%s", strings.Join(mistakes, "\n"))
		}
	}
	if !isOneOf(str(security, "awml"), "0.2", "0.3") || !isOneOf(str(platform, "awml"), "0.2", "0.3") || str(security, "view") != "security" || str(platform, "view") != "platform" {
		return nil, fmt.Errorf("convert needs a 0.3 or 0.2 security view followed by a platform view")
	}
	platform = copyMap(platform)
	platform.set("title", security.get("title"))
	if differences := CompareViews(security, platform); len(differences) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(differences, "\n"))
	}
	out := &Map{}
	out.set("awml", "0.4")
	out.set("title", security.get("title"))
	if security.has("groups") {
		out.set("scopes", security.get("groups"))
	}
	out.set("parts", security.get("parts"))
	if security.has("lines") {
		out.set("lines", security.get("lines"))
	}
	var blocks, comments []any
	where := placed(items(platform, "parts"), items(platform, "lines"))
	design := partsByID(security)
	used := map[string]bool{}
	for _, key := range []string{"groups", "parts"} {
		for _, item := range items(security, key) {
			used[str(item, "id")] = true
		}
	}
	renamed := map[string]string{}
	for _, group := range items(platform, "groups") {
		old := str(group, "id")
		name := old
		for used[name] {
			name = "block_" + name
		}
		renamed[old] = name
		used[name] = true
	}
	for _, group := range items(platform, "groups") {
		block := copyMap(group)
		block.set("id", renamed[str(group, "id")])
		if group.has("in") {
			block.set("in", renamed[str(group, "in")])
		}
		ids := []any{}
		for _, part := range items(platform, "parts") {
			if str(part, "in") == str(group, "id") && design[str(part, "id")] != nil {
				ids = append(ids, part.get("id"))
			}
		}
		block.set("parts", ids)
		blocks = append(blocks, block)
	}
	for _, part := range items(platform, "parts") {
		if design[str(part, "id")] != nil {
			continue
		}
		comment := copyMap(part, "kind", "in")
		name := str(part, "id")
		for used[name] {
			name = "comment_" + name
		}
		used[name] = true
		comment.set("id", name)
		comment.set("in", renamed[where[str(part, "id")]])
		ties := []any{}
		for _, line := range items(platform, "lines") {
			if str(line, "kind") != "tie" {
				continue
			}
			from, to := str(line, "from"), str(line, "to")
			if to == str(part, "id") {
				from, to = to, from
			}
			if from == str(part, "id") {
				ties = append(ties, to)
			}
		}
		comment.set("ties", ties)
		comments = append(comments, comment)
	}
	impl := &Map{}
	impl.set("label", "Default")
	impl.set("blocks", blocks)
	if len(comments) > 0 {
		impl.set("comments", comments)
	}
	implementations := &Map{}
	implementations.set(name, impl)
	out.set("implementations", implementations)
	if mistakes := Check(out); len(mistakes) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(mistakes, "\n"))
	}
	return out, nil
}

func workflowViews(docs []*Map) []*Map {
	var views []*Map
	for _, doc := range docs {
		security, _ := View(doc, "")
		views = append(views, security)
		if !IsCanonical(doc) {
			continue
		}
		if untrusted, err := UntrustedView(doc); err == nil {
			views = append(views, untrusted)
		}
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
