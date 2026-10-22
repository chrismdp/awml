package awml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// A Map is a YAML mapping that keeps its keys in the order the file gives them, so mistakes are
// reported in the order they were written.
type Map struct {
	keys         []string
	vals         map[string]any
	nonTextKeys  map[string]bool
	headComment  string
	footComment  string
	keyComments  map[string]string
	scalarStyles map[string]yaml.Style
}

// Keys gives the mapping's keys in the file's order.
func (m *Map) Keys() []string { return m.keys }

// Get gives the value of a key, and whether the mapping has it.
func (m *Map) Get(key string) (any, bool) {
	v, ok := m.vals[key]
	return v, ok
}

func (m *Map) has(key string) bool {
	_, ok := m.vals[key]
	return ok
}

// get gives the value of a key, or nil.
func (m *Map) get(key string) any { return m.vals[key] }

// set gives a key a value. A key given twice keeps its first place and its last value.
func (m *Map) set(key string, v any) {
	if m.vals == nil {
		m.vals = map[string]any{}
	}
	if !m.has(key) {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = v
	delete(m.nonTextKeys, key)
	delete(m.scalarStyles, key)
}

// Parse reads one YAML document into plain values: text, numbers, true and false, dates, null,
// lists ([]any) and mappings (*Map). An empty file is null. It reads YAML 1.2, so yes, no, on, off
// and 19:00 are text.
func Parse(src []byte) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, plain(err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); err != io.EOF {
		if err != nil {
			return nil, plain(err)
		}
		return nil, errors.New("the file holds more than one document")
	}
	return value(&doc, map[*yaml.Node]bool{})
}

// plain drops the library's name from a YAML error.
func plain(err error) error {
	return errors.New(strings.TrimPrefix(err.Error(), "yaml: "))
}

// value turns a node into plain values. open holds the nodes being read, so an alias to one of them,
// which would never end, is a mistake.
func value(n *yaml.Node, open map[*yaml.Node]bool) (any, error) {
	if open[n] {
		return nil, fmt.Errorf("line %d: an alias is used inside itself", n.Line)
	}
	open[n] = true
	defer delete(open, n)
	switch n.Kind {
	case yaml.DocumentNode:
		return value(n.Content[0], open)
	case yaml.AliasNode:
		return value(n.Alias, open)
	case yaml.SequenceNode:
		list := []any{}
		for _, c := range n.Content {
			v, err := value(c, open)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case yaml.MappingNode:
		return mapping(n, open)
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, plain(err)
	}
	return v, nil
}

// mapping reads a mapping. A merge key (<<) brings in another mapping's keys first, so the
// mapping's own keys win, and of several merged mappings the first wins.
func mapping(n *yaml.Node, open map[*yaml.Node]bool) (*Map, error) {
	var merged, own [][3]any
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.ShortTag() == "!!merge" {
			from, err := value(v, open)
			if err != nil {
				return nil, err
			}
			maps, ok := from.([]any)
			if !ok {
				maps = []any{from}
			}
			for j := len(maps) - 1; j >= 0; j-- {
				m, ok := maps[j].(*Map)
				if !ok {
					return nil, fmt.Errorf("line %d: a merge (<<) takes only mappings", k.Line)
				}
				for _, key := range m.keys {
					merged = append(merged, [3]any{key, m.vals[key], !m.nonTextKeys[key]})
				}
			}
			continue
		}
		key, err := value(k, open)
		if err != nil {
			return nil, err
		}
		val, err := value(v, open)
		if err != nil {
			return nil, err
		}
		_, textKey := key.(string)
		own = append(own, [3]any{show(key), val, textKey})
	}
	m := &Map{
		vals:         map[string]any{},
		nonTextKeys:  map[string]bool{},
		headComment:  n.HeadComment,
		footComment:  n.FootComment,
		keyComments:  map[string]string{},
		scalarStyles: map[string]yaml.Style{},
	}
	for _, kv := range append(merged, own...) {
		m.set(kv[0].(string), kv[1])
		if !kv[2].(bool) {
			m.nonTextKeys[kv[0].(string)] = true
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := showNodeKey(n.Content[i])
		comment := joinComments(n.Content[i].HeadComment, n.Content[i+1].HeadComment)
		if comment != "" {
			m.keyComments[key] = comment
		}
		m.scalarStyles[key] = n.Content[i+1].Style
	}
	return m, nil
}

func showNodeKey(node *yaml.Node) string {
	value, err := value(node, map[*yaml.Node]bool{})
	if err != nil {
		return node.Value
	}
	return show(value)
}

func joinComments(comments ...string) string {
	var present []string
	for _, comment := range comments {
		if comment != "" {
			present = append(present, comment)
		}
	}
	return strings.Join(present, "\n")
}

// show writes a value as YAML would: text as it is, null, true, a number, a date, a list or a mapping.
func show(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case nil:
		return "null"
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case time.Time:
		if v.Equal(v.Truncate(24 * time.Hour)) {
			return v.Format("2006-01-02")
		}
		return v.Format(time.RFC3339Nano)
	case []any:
		items := make([]string, len(v))
		for i, x := range v {
			items[i] = show(x)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case *Map:
		items := make([]string, len(v.keys))
		for i, k := range v.keys {
			items[i] = k + ": " + show(v.vals[k])
		}
		return "{" + strings.Join(items, ", ") + "}"
	}
	return fmt.Sprint(v)
}

// Marshal writes a document in the same compact style as hand-written AWML files.
func Marshal(doc *Map) ([]byte, error) {
	if doc == nil {
		return nil, errors.New("cannot marshal a nil document")
	}
	var out strings.Builder
	writeComment(&out, doc.headComment, 0)
	writeBlockMap(&out, doc, 0, true)
	writeComment(&out, doc.footComment, 0)
	return []byte(out.String()), nil
}

func writeBlockMap(out *strings.Builder, value *Map, indent int, root bool) {
	for _, key := range marshalKeys(value, root) {
		writeComment(out, value.keyComments[key], indent)
		writeIndent(out, indent)
		out.WriteString(key)
		out.WriteByte(':')
		field := value.get(key)
		switch field := field.(type) {
		case *Map:
			if flowMapKey(key) {
				out.WriteByte(' ')
				writeFlowMap(out, field)
				out.WriteByte('\n')
			} else {
				out.WriteByte('\n')
				writeBlockMap(out, field, indent+2, false)
			}
		case []any:
			if listOfMaps(field) {
				out.WriteByte('\n')
				writeMapList(out, field, indent+2)
			} else {
				out.WriteByte(' ')
				writeFlowList(out, field, key)
				out.WriteByte('\n')
			}
		default:
			out.WriteByte(' ')
			writeScalar(out, field, key, value)
			out.WriteByte('\n')
		}
	}
}

func marshalKeys(value *Map, root bool) []string {
	if !root || str(value, "awml") != "0.4" || value.has("platform") {
		return value.Keys()
	}
	ordered := []string{"awml", "title", "scopes", "parts", "lines", "implementations"}
	var keys []string
	seen := map[string]bool{}
	for _, key := range ordered {
		if value.has(key) {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for _, key := range value.Keys() {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

func flowMapKey(key string) bool {
	return key == "platform" || key == "processing"
}

func listOfMaps(values []any) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if _, ok := value.(*Map); !ok {
			return false
		}
	}
	return true
}

func writeMapList(out *strings.Builder, values []any, indent int) {
	for _, item := range values {
		mapping := item.(*Map)
		writeComment(out, mapping.headComment, indent)
		if len(mapping.Keys()) > 0 {
			writeComment(out, mapping.keyComments[mapping.Keys()[0]], indent)
		}
		writeIndent(out, indent)
		out.WriteString("- ")
		writeFlowMap(out, mapping)
		out.WriteByte('\n')
		writeComment(out, mapping.footComment, indent)
	}
}

func writeFlowMap(out *strings.Builder, value *Map) {
	out.WriteString("{ ")
	for i, key := range value.Keys() {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(key)
		out.WriteString(": ")
		field := value.get(key)
		switch field := field.(type) {
		case *Map:
			writeFlowMap(out, field)
		case []any:
			writeFlowList(out, field, key)
		default:
			writeScalar(out, field, key, value)
		}
	}
	out.WriteString(" }")
}

func writeFlowList(out *strings.Builder, values []any, key string) {
	out.WriteByte('[')
	for i, item := range values {
		if i > 0 {
			out.WriteString(", ")
		}
		switch item := item.(type) {
		case *Map:
			writeFlowMap(out, item)
		default:
			writeScalar(out, item, key, nil)
		}
	}
	out.WriteByte(']')
}

func writeScalar(out *strings.Builder, value any, key string, parent *Map) {
	text, ok := value.(string)
	if !ok {
		out.WriteString(show(value))
		return
	}
	quote := key == "awml" || !plainText(text)
	if key == "label" && parent != nil && parent.scalarStyles[key] == yaml.DoubleQuotedStyle {
		quote = true
	}
	if key == "label" && parent != nil && !parent.has("from") && !parent.has("to") && !isImplementation(parent) {
		quote = true
	}
	if key == "label" && !simpleLabel(text) {
		quote = true
	}
	if quote {
		out.WriteString(doubleQuoted(text))
		return
	}
	out.WriteString(text)
}

func isImplementation(value *Map) bool {
	return value.has("blocks") && !value.has("id")
}

func simpleLabel(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r != ' ' && r != '\'' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return plainText(value)
}

var yaml11Time = regexp.MustCompile(`^[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?$`)

func plainText(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\t") {
		return false
	}
	switch strings.ToLower(value) {
	case "yes", "no", "on", "off":
		return false
	}
	if yaml11Time.MatchString(value) {
		return false
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte("value: "+value+"\n"), &document); err != nil || len(document.Content) == 0 {
		return false
	}
	mapping := document.Content[0]
	if len(mapping.Content) != 2 {
		return false
	}
	var decoded any
	if err := mapping.Content[1].Decode(&decoded); err != nil {
		return false
	}
	text, ok := decoded.(string)
	return ok && text == value
}

func doubleQuoted(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return "\"" + replacer.Replace(value) + "\""
}

func writeComment(out *strings.Builder, comment string, indent int) {
	if comment == "" {
		return
	}
	for _, line := range strings.Split(comment, "\n") {
		writeIndent(out, indent)
		out.WriteString(line)
		out.WriteByte('\n')
	}
}

func writeIndent(out *strings.Builder, indent int) {
	out.WriteString(strings.Repeat(" ", indent))
}

// Comments collects YAML comments in source order, ready to keep above a converted document.
func Comments(source []byte) ([]string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(source, &document); err != nil {
		return nil, plain(err)
	}
	var comments []string
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		for _, comment := range []string{node.HeadComment, node.LineComment} {
			if comment != "" {
				comments = append(comments, comment)
			}
		}
		for _, child := range node.Content {
			visit(child)
		}
		if node.FootComment != "" {
			comments = append(comments, node.FootComment)
		}
	}
	visit(&document)
	return comments, nil
}
