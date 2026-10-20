package awml

import (
	"reflect"
	"strings"
	"testing"
)

func parsed(t *testing.T, src string) *Map {
	t.Helper()
	v, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, ok := v.(*Map)
	if !ok {
		t.Fatalf("Parse gave %T, not a mapping", v)
	}
	return m
}

func TestAMappingKeepsTheOrderOfItsKeys(t *testing.T) {
	m := parsed(t, "view: security\nawml: \"0.2\"\ntitle: T\n")
	if got := m.Keys(); !reflect.DeepEqual(got, []string{"view", "awml", "title"}) {
		t.Errorf("keys = %q", got)
	}
}

func TestWordsAndTimesYAML11ReadAsOtherThingsAreText(t *testing.T) {
	m := parsed(t, "a: yes\nb: no\nc: on\nd: off\ne: 19:00\n")
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		v, _ := m.Get(k)
		if _, ok := v.(string); !ok {
			t.Errorf("%s read as %T %v, not text", k, v, v)
		}
	}
}

func TestTrueNumbersAndDatesArentText(t *testing.T) {
	m := parsed(t, "a: true\nb: 12\nc: 0.1\nd: 2026-10-20\ne: null\n")
	for k, want := range map[string]string{"a": "true", "b": "12", "c": "0.1", "d": "2026-10-20", "e": "null"} {
		v, _ := m.Get(k)
		if _, ok := v.(string); ok {
			t.Errorf("%s read as text", k)
		}
		if got := show(v); got != want {
			t.Errorf("show(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestListsAndMappingsShowAsYAMLWritesThem(t *testing.T) {
	m := parsed(t, "a: [x, 1, {b: c}]\n")
	v, _ := m.Get("a")
	if got := show(v); got != "[x, 1, {b: c}]" {
		t.Errorf("show = %q", got)
	}
}

func TestAnAliasAndAMergeAreFollowed(t *testing.T) {
	m := parsed(t, "base: &b {kind: code, label: Copy}\npart: {<<: *b, id: copy, label: Copy the calendar}\nsame: *b\n")
	v, _ := m.Get("part")
	part := v.(*Map)
	if got := part.Keys(); !reflect.DeepEqual(got, []string{"kind", "label", "id"}) {
		t.Errorf("merged keys = %q", got)
	}
	if label, _ := part.Get("label"); label != "Copy the calendar" {
		t.Errorf("label = %v: the mapping's own key should win", label)
	}
	v, _ = m.Get("same")
	if kind, _ := v.(*Map).Get("kind"); kind != "code" {
		t.Errorf("alias kind = %v", kind)
	}
}

func TestAKeyGivenTwiceKeepsItsPlaceAndItsLastValue(t *testing.T) {
	m := parsed(t, "a: 1\nb: 2\na: 3\n")
	a, _ := m.Get("a")
	if !reflect.DeepEqual(m.Keys(), []string{"a", "b"}) || a != 3 {
		t.Errorf("keys %q, a = %v", m.Keys(), a)
	}
}

func TestAnEmptyFileIsNull(t *testing.T) {
	v, err := Parse([]byte("# nothing here\n"))
	if v != nil || err != nil {
		t.Errorf("Parse = %v, %v", v, err)
	}
}

func TestMoreThanOneDocumentIsAMistake(t *testing.T) {
	_, err := Parse([]byte("a: 1\n---\nb: 2\n"))
	if err == nil || err.Error() != "the file holds more than one document" {
		t.Errorf("err = %v", err)
	}
}

func TestAnAliasInsideItselfIsAMistake(t *testing.T) {
	_, err := Parse([]byte("a: &x [1, *x]\n"))
	if err == nil || !strings.Contains(err.Error(), "inside itself") {
		t.Errorf("err = %v", err)
	}
}

func TestBrokenYAMLSaysWhereWithoutTheLibrarysName(t *testing.T) {
	_, err := Parse([]byte("parts: [\n"))
	if err == nil || err.Error() != "line 1: did not find expected node content" {
		t.Errorf("err = %v", err)
	}
}
