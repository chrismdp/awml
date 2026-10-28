package awml

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// Upgrade gives a file of an older version as one of the current version, losing nothing: a
// workflow, a legacy security view on its own, or a platform description. A file of the current
// version comes back as it is. The file needn't pass Check, and the result may not either, since
// newer versions hold a file to rules older ones didn't.
//
// From 0.7, each fork goes, since several start lines out of one part already show work side by
// side, and each line into it joins each line out of it. Otherwise only the version changes, since
// 0.8 changed only rules: what a standing approval may cover, and that untrusted text reaching an
// action unchecked is a mistake. From 0.6, a trigger of another workflow that only do steps lead into becomes their outside action,
// which refers to it, since a trigger is never a destination. From 0.5, each publish line becomes a
// data line, since a data line publishes whenever its scopes make it one. A line's breaks goes,
// since the check works out which lines are red. A line marked red that the check doesn't find gets
// a comment tied to both its ends, so the finding stays on the drawing.
func Upgrade(doc any) (*Map, error) {
	file, _, err := upgrade(doc)
	return file, err
}

// upgrade upgrades a file as Upgrade does, and gives a note of what it couldn't carry over, for the
// author to do by hand.
func upgrade(doc any) (*Map, []string, error) {
	file, err := upgradeFile(doc)
	if err != nil || file == doc || file.has("platform") {
		return file, nil, err
	}
	dropForks(file)
	if !isOneOf(doc.(*Map).get("awml"), rewrittenFormats...) {
		return file, nil, nil
	}
	notes := startOutsideActions(file)
	return file, notes, nil
}

// rewrittenFormats are the versions whose workflows change in more than their version and their
// forks when they're converted, the last of which let a do step lead into another workflow's
// trigger.
var rewrittenFormats = []string{"0.2", "0.3", "0.4", "0.5", "0.6"}

// dropForks takes out each fork, in the order of the parts, and joins each line into it to each
// line out of it, in the place of the line out, with that line's kind and label. A joined line the
// file already has isn't added again. A tie between a comment and a fork ties the comment instead
// to each part a line into the fork came from, as does an implementation comment's tie, and a
// hidden line at a fork goes, with the fork's place in each block.
func dropForks(file *Map) {
	parts, _ := file.get("parts").([]any)
	lines, _ := file.get("lines").([]any)
	var kept []any
	forks, from := map[string]bool{}, map[string][]any{}
	for _, item := range parts {
		p, ok := item.(*Map)
		if !ok || p.get("kind") != "fork" {
			kept = append(kept, item)
			continue
		}
		fork := str(p, "id")
		forks[fork] = true
		var into []*Map
		for _, l := range soundMaps(lines) {
			if l.get("to") == fork && l.get("from") != fork && !isOneOf(l.get("kind"), "tie", "hidden") {
				into = append(into, l)
				from[fork] = append(from[fork], l.get("from"))
			}
		}
		var joined []any
		has := func(line *Map) bool {
			for _, l := range append(soundMaps(lines), soundMaps(joined)...) {
				same := true
				for _, field := range []string{"from", "to", "kind", "label"} {
					same = same && l.has(field) == line.has(field) && keyOf(l.get(field)) == keyOf(line.get(field))
				}
				if same {
					return true
				}
			}
			return false
		}
		for _, item := range lines {
			l, ok := item.(*Map)
			switch {
			case !ok || l.get("from") != fork && l.get("to") != fork:
				joined = append(joined, item)
			case l.get("kind") == "tie":
				for _, part := range from[fork] {
					tie := copyMap(l)
					if l.get("from") == fork {
						tie.set("from", part)
					} else {
						tie.set("to", part)
					}
					if !has(tie) {
						joined = append(joined, tie)
					}
				}
			case l.get("from") == fork && l.get("to") != fork && l.get("kind") != "hidden":
				for _, in := range into {
					line := copyMap(l)
					line.set("from", in.get("from"))
					if !has(line) {
						joined = append(joined, line)
					}
				}
			}
		}
		lines = joined
	}
	if len(kept) == len(parts) {
		return
	}
	file.set("parts", kept)
	file.set("lines", lines)
	implementations, ok := file.get("implementations").(*Map)
	if !ok {
		return
	}
	// split gives the parts a fork's work split from, through any forks in a row, or the part itself.
	// A fork's lines in come only from parts that weren't forks when it went, so this ends.
	var split func(id any) []any
	split = func(id any) []any {
		if s, _ := id.(string); !forks[s] {
			return []any{id}
		}
		var parts []any
		for _, next := range from[id.(string)] {
			parts = append(parts, split(next)...)
		}
		return parts
	}
	out := &Map{}
	for _, name := range implementations.Keys() {
		impl, ok := implementations.get(name).(*Map)
		if !ok {
			out.set(name, implementations.get(name))
			continue
		}
		impl = copyMap(impl)
		for _, list := range [][2]string{{"blocks", "parts"}, {"comments", "ties"}} {
			items, ok := impl.get(list[0]).([]any)
			if !ok {
				continue
			}
			var changed []any
			for _, item := range items {
				m, ok := item.(*Map)
				ids, isList := m.get(list[1]).([]any)
				if !ok || !isList {
					changed = append(changed, item)
					continue
				}
				var to []any
				seen := map[any]bool{}
				for _, id := range ids {
					if s, _ := id.(string); list[0] == "blocks" {
						if !forks[s] {
							to = append(to, id)
						}
						continue
					}
					for _, part := range split(id) {
						if !seen[keyOf(part)] {
							seen[keyOf(part)] = true
							to = append(to, part)
						}
					}
				}
				m = copyMap(m)
				m.set(list[1], to)
				changed = append(changed, m)
			}
			impl.set(list[0], changed)
		}
		out.set(name, impl)
	}
	file.set("implementations", out)
}

func upgradeFile(doc any) (*Map, error) {
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
	case file.has("platform") || !isOneOf(version, rewrittenFormats...):
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

// startOutsideActions turns each trigger of another workflow, which only do steps lead into and
// nothing leads out of, into their outside action, which refers to it as the trigger did. A do step
// leads to it with no review in front, so it's Allow. It waits for nothing, so its wait goes. It sits
// in the scope of the first do step that leads to it, where it has none, and in that step's block in
// each implementation that doesn't place it. Only a trigger, connector or context is marked
// untrusted, so its mark goes too, and it gives a note saying the mark belongs on the trigger it
// starts. Any other line into a trigger stays, for the check to report and the author to redraw.
func startOutsideActions(file *Map) []string {
	parts, ok := file.get("parts").([]any)
	if !ok || file.has("platform") {
		return nil
	}
	parts = append([]any{}, parts...)
	byID := map[string]*Map{}
	for _, p := range soundMaps(parts) {
		byID[str(p, "id")] = p
	}
	lines := soundMaps(file.get("lines"))
	var notes []string
	for n, item := range parts {
		p, ok := item.(*Map)
		if !ok || p.get("kind") != "trigger" || !p.has("ref") {
			continue
		}
		var from []*Map
		onlyDo := true
		for _, l := range lines {
			if !lineLeads(l) {
				continue
			}
			if l.get("from") == p.get("id") {
				onlyDo = false
			}
			if l.get("to") == p.get("id") {
				step := byID[str(l, "from")]
				onlyDo = onlyDo && step != nil && steps[str(step, "kind")] && step.get("op") == "do"
				from = append(from, step)
			}
		}
		if len(from) == 0 || !onlyDo {
			continue
		}
		action := copyMap(p, "wait", "untrusted")
		action.set("kind", "outside")
		action.set("label", str(p, "label")+": Allow")
		if !p.has("in") && from[0].has("in") {
			action.set("in", from[0].get("in"))
		}
		if p.get("untrusted") == true {
			notes = append(notes, fmt.Sprintf(`part "%s" was a trigger marked untrusted, and an outside action isn't: mark the trigger it starts, %s, untrusted instead`, str(p, "id"), show(p.get("ref"))))
		}
		parts[n] = action
		placeBeside(file, str(action, "id"), str(from[0], "id"))
	}
	file.set("parts", parts)
	return notes
}

// placeBeside places a part in the block of another, in each implementation that doesn't place it.
func placeBeside(file *Map, id, beside string) {
	implementations, ok := file.get("implementations").(*Map)
	if !ok {
		return
	}
	out := &Map{}
	for _, name := range implementations.Keys() {
		impl, ok := implementations.get(name).(*Map)
		var blocks []any
		if ok {
			blocks, ok = impl.get("blocks").([]any)
		}
		if !ok {
			out.set(name, implementations.get(name))
			continue
		}
		placed, at := false, -1
		for n, block := range blocks {
			b, ok := block.(*Map)
			if !ok {
				continue
			}
			ids, _ := b.get("parts").([]any)
			for _, part := range ids {
				placed = placed || part == id
				if part == beside && at < 0 {
					at = n
				}
			}
		}
		if placed || at < 0 {
			out.set(name, impl)
			continue
		}
		block := copyMap(blocks[at].(*Map))
		ids, _ := block.get("parts").([]any)
		block.set("parts", append(append([]any{}, ids...), id))
		blocks = append([]any{}, blocks...)
		blocks[at] = block
		impl = copyMap(impl)
		impl.set("blocks", blocks)
		out.set(name, impl)
	}
	file.set("implementations", out)
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

// ConvertSources writes the one file of the current version that awml convert prints, from one file
// or from a legacy security view followed by its platform view, whose implementation is named
// default unless it's given. A file of the current version comes back as it was. Otherwise each
// YAML comment of the files moves to the top once, in the files' order. It also gives the mistakes
// Check finds in the result, since newer rules can find mistakes in a file that older ones passed,
// and before those its notes of what it couldn't carry over, for the author to do by hand. A
// mistake in one of the files starts with its name, when it has one.
func ConvertSources(sources []Source, implementation string) (file []byte, notes, mistakes []string, err error) {
	if len(sources) == 0 || len(sources) > 2 {
		return nil, nil, nil, errors.New("convert takes one file, or a security view and its platform view")
	}
	if len(sources) == 1 && implementation != "" {
		return nil, nil, nil, errors.New("only a legacy pair has an implementation to name")
	}
	var docs []any
	var comments []string
	for _, source := range sources {
		doc, err := Parse(source.Text)
		if err != nil {
			return nil, nil, nil, named(source, errors.New("isn't YAML: "+strings.Join(strings.Fields(err.Error()), " ")))
		}
		found, err := Comments(source.Text)
		if err != nil {
			return nil, nil, nil, named(source, err)
		}
		docs = append(docs, doc)
		comments = append(comments, found...)
	}
	var converted *Map
	if len(docs) == 1 {
		if file, ok := docs[0].(*Map); ok && file.get("awml") == currentFormat {
			return sources[0].Text, nil, Check(file), nil
		}
		if converted, notes, err = upgrade(docs[0]); err != nil {
			return nil, nil, nil, named(sources[0], err)
		}
	} else {
		if implementation == "" {
			implementation = "default"
		}
		security, _ := docs[0].(*Map)
		platform, _ := docs[1].(*Map)
		if converted, err = ConvertWithImplementation(security, platform, implementation); err != nil {
			return nil, nil, nil, err
		}
	}
	dropComments(converted)
	data, err := Marshal(converted)
	if err != nil {
		return nil, nil, nil, err
	}
	var out bytes.Buffer
	for _, comment := range comments {
		out.WriteString(comment + "\n")
	}
	out.Write(data)
	return out.Bytes(), notes, Check(converted), nil
}

// named starts a mistake in one file with the file's name, when it has one.
func named(source Source, err error) error {
	if source.File == "" {
		return err
	}
	return fmt.Errorf("%s: %w", source.File, err)
}

// dropComments takes the YAML comments off a converted file, since they move to its top.
func dropComments(value any) {
	switch value := value.(type) {
	case *Map:
		value.headComment, value.footComment, value.keyComments = "", "", nil
		for _, key := range value.Keys() {
			dropComments(value.get(key))
		}
	case []any:
		for _, item := range value {
			dropComments(item)
		}
	}
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
