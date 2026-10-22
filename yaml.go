package awml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// A Map is a YAML mapping that keeps its keys in the order the file gives them, so mistakes are
// reported in the order they were written.
type Map struct {
	keys        []string
	vals        map[string]any
	nonTextKeys map[string]bool
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
	m := &Map{vals: map[string]any{}, nonTextKeys: map[string]bool{}}
	for _, kv := range append(merged, own...) {
		m.set(kv[0].(string), kv[1])
		if !kv[2].(bool) {
			m.nonTextKeys[kv[0].(string)] = true
		}
	}
	return m, nil
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

// Marshal writes a document with mappings in their original order.
func Marshal(doc *Map) ([]byte, error) {
	return yaml.Marshal(yamlNode(doc))
}

func yamlNode(value any) *yaml.Node {
	switch value := value.(type) {
	case *Map:
		node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, key := range value.Keys() {
			node.Content = append(node.Content, yamlNode(key), yamlNode(value.get(key)))
		}
		return node
	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range value {
			child := yamlNode(item)
			child.Style = yaml.FlowStyle
			node.Content = append(node.Content, child)
		}
		return node
	default:
		node := &yaml.Node{}
		_ = node.Encode(value)
		return node
	}
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
