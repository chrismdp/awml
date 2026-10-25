package awml

import (
	"errors"
	"fmt"
	"strings"
)

// Upgrade gives a file of an older version as one of the current version, losing nothing: a
// workflow, a legacy security view on its own, or a platform description. A file of the current
// version comes back as it is. The file needn't pass Check, and the result may not either, since
// newer versions hold a file to rules older ones didn't.
//
// From 0.5, each publish line becomes a data line, since a data line publishes whenever its scopes
// make it one. A line's breaks goes, since the check works out which lines are red. A line marked
// red that the check doesn't find gets a comment tied to both its ends, so the finding stays on
// the drawing.
func Upgrade(doc any) (*Map, error) {
	file, ok := doc.(*Map)
	if !ok {
		return nil, errors.New("the file must be a mapping of fields, starting with awml")
	}
	version := file.get("awml")
	switch {
	case version == currentFormat:
		return file, nil
	case !isOneOf(version, olderFormats...):
		return nil, fmt.Errorf(`awml convert reads versions %s to %s, not "%s"`, olderFormats[0], currentFormat, show(version))
	case file.has("platform"):
		out := copyMap(file)
		out.set("awml", currentFormat)
		return out, nil
	}
	legacy := isOneOf(version, "0.2", "0.3")
	if legacy && file.get("view") == "platform" {
		return nil, errors.New("a platform view converts only with its security view: awml convert security.yaml platform.yaml")
	}
	if legacy && file.get("view") != "security" {
		return nil, fmt.Errorf(`view must be "security" or "platform", not "%s"`, show(file.get("view")))
	}
	if err := soundLists(file, "groups", "scopes", "parts", "lines"); err != nil {
		return nil, err
	}
	out := &Map{}
	var red []*Map
	for _, key := range file.Keys() {
		value := file.get(key)
		switch {
		case key == "awml":
			out.set(key, currentFormat)
		case legacy && key == "view":
		case legacy && key == "groups":
			out.set("scopes", value)
		case key == "lines" && value != nil:
			var lines []any
			for _, line := range soundMaps(value) {
				upgraded := copyMap(line, "breaks")
				if line.get("kind") == "publish" {
					upgraded.set("kind", "data")
				}
				// A breaks that isn't true or false stays, for the check to report.
				if breaks, ok := line.get("breaks").(bool); !ok && line.has("breaks") {
					upgraded.set("breaks", line.get("breaks"))
				} else if breaks {
					red = append(red, upgraded)
				}
				lines = append(lines, upgraded)
			}
			out.set(key, lines)
		default:
			out.set(key, value)
		}
	}
	commentRedLines(out, red)
	return out, nil
}

// soundLists reports the first of a file's lists that isn't a list of mappings, which no version
// allows, so there would be nothing to carry over.
func soundLists(file *Map, keys ...string) error {
	for _, key := range keys {
		value := file.get(key)
		if value == nil {
			continue
		}
		list, ok := value.([]any)
		if ok && len(soundMaps(list)) == len(list) {
			continue
		}
		return fmt.Errorf("%s must be a list of mappings", key)
	}
	return nil
}

// commentRedLines ties a comment to both ends of each line that was marked red by hand but that
// the check doesn't find, saying what its label said.
func commentRedLines(file *Map, red []*Map) {
	if red == nil {
		return
	}
	found, _ := checkDoc(file)
	reported := map[[2]string]bool{}
	for _, m := range found {
		for _, line := range m.lines {
			reported[[2]string{str(line, "from"), str(line, "to")}] = true
		}
	}
	used := map[string]bool{}
	for _, key := range []string{"scopes", "parts"} {
		for _, item := range soundMaps(file.get(key)) {
			used[str(item, "id")] = true
		}
	}
	if implementations, ok := file.get("implementations").(*Map); ok {
		for _, name := range implementations.Keys() {
			impl, ok := implementations.get(name).(*Map)
			for _, key := range []string{"blocks", "comments"} {
				if !ok {
					break
				}
				for _, item := range soundMaps(impl.get(key)) {
					used[str(item, "id")] = true
				}
			}
		}
	}
	parts, _ := file.get("parts").([]any)
	lines, _ := file.get("lines").([]any)
	for _, line := range red {
		from, to := str(line, "from"), str(line, "to")
		if reported[[2]string{from, to}] {
			continue
		}
		id := "red_" + from + "_" + to
		for n := 2; used[id]; n++ {
			id = fmt.Sprintf("red_%s_%s_%d", from, to, n)
		}
		used[id] = true
		label := fmt.Sprintf(`Red line from "%s" to "%s"`, from, to)
		if said := str(line, "label"); said != "" {
			label = "Red line: " + said
		}
		comment := &Map{}
		comment.set("id", id)
		comment.set("kind", "comment")
		comment.set("label", label)
		parts = append(parts, comment)
		for _, end := range []string{from, to} {
			tie := &Map{}
			tie.set("from", id)
			tie.set("to", end)
			tie.set("kind", "tie")
			lines = append(lines, tie)
		}
	}
	file.set("parts", parts)
	file.set("lines", lines)
}

// Convert joins a matching legacy pair into one file of the current version. The security view
// supplies the design's wording and order.
func Convert(security, platform *Map) (*Map, error) {
	return convert(security, platform, "default")
}

// ConvertWithImplementation joins a matching legacy pair and names its implementation.
func ConvertWithImplementation(security, platform *Map, implementation string) (*Map, error) {
	return convert(security, platform, implementation)
}

func convert(security, platform *Map, name string) (*Map, error) {
	if security == nil || platform == nil || !isOneOf(security.get("awml"), "0.2", "0.3") || !isOneOf(platform.get("awml"), "0.2", "0.3") ||
		security.get("view") != "security" || platform.get("view") != "platform" {
		return nil, errors.New("convert needs a 0.3 or 0.2 security view followed by a platform view")
	}
	for _, view := range []*Map{security, platform} {
		if err := soundLists(view, "groups", "parts", "lines"); err != nil {
			return nil, err
		}
	}
	platform = copyMap(platform)
	platform.set("title", security.get("title"))
	if differences, _ := compareViews(security, platform); len(differences) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(differences.messages(), "\n"))
	}
	out := &Map{}
	out.set("awml", security.get("awml"))
	out.set("title", security.get("title"))
	out.set("view", "security")
	if security.has("groups") {
		out.set("groups", security.get("groups"))
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
	return Upgrade(out)
}
