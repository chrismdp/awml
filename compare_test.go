package awml

import (
	"reflect"
	"strings"
	"testing"
)

func compared(t *testing.T, security, platform string, want ...string) {
	t.Helper()
	got := CompareViews(parsed(t, security), parsed(t, platform))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CompareViews gave\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestCompareViewsReportsPartsMissingFromEitherViewAndChangedKinds(t *testing.T) {
	security := `
awml: "0.3"
title: Picking an order
view: security
parts:
  - { id: order, kind: context, label: Order }
  - { id: pick, kind: ai, label: Pick it }
  - { id: shelf, kind: surface, label: Shelf }
`
	platform := `
awml: "0.3"
title: Picking an order
view: platform
parts:
  - { id: order, kind: connector, label: Order }
  - { id: pick, kind: ai, label: Pick it }
  - { id: scanner, kind: surface, label: Scanner }
`
	compared(t, security, platform,
		`part "order" has kind "connector" in the platform view but "context" in the security view`,
		`part "scanner" is in the platform view but not the security view`,
		`part "shelf" is in the security view but not the platform view`)
}

func TestCompareViewsAllowsPlatformViewToLeaveOutTriggersAndComments(t *testing.T) {
	security := `
awml: "0.3"
title: Picking an order
view: security
parts:
  - { id: morning, kind: trigger, label: Morning }
  - { id: pick, kind: ai, label: Pick it }
  - { id: note, kind: comment, label: Check the shelf }
lines:
  - { from: morning, to: pick, kind: start }
  - { from: note, to: pick, kind: tie }
  - { from: note, to: morning, kind: hidden }
`
	platform := `
awml: "0.3"
title: Picking an order
view: platform
parts:
  - { id: pick, kind: ai, label: Pick it }
`
	compared(t, security, platform)
}

func TestCompareViewsReportsLinesByEndsAndKind(t *testing.T) {
	security := `
awml: "0.3"
title: Picking an order
view: security
parts:
  - { id: order, kind: context, label: Order }
  - { id: pick, kind: ai, label: Pick it }
lines:
  - { from: order, to: pick, kind: data, label: details, breaks: true }
`
	platform := `
awml: "0.3"
title: Picking an order
view: platform
parts:
  - { id: order, kind: context, label: Order }
  - { id: pick, kind: ai, label: Pick it }
lines:
  - { from: order, to: pick, kind: publish, label: order }
`
	compared(t, security, platform,
		`the data line from "order" to "pick" is in the security view but not the platform view`,
		`the publish line from "order" to "pick" is in the platform view but not the security view`)
}

func TestCompareViewsIgnoresLineLabelsBreaksTiesAndHiddenLines(t *testing.T) {
	security := `
awml: "0.3"
title: Picking an order
view: security
parts:
  - { id: order, kind: context, label: Order }
  - { id: pick, kind: ai, label: Pick it }
  - { id: note, kind: comment, label: Check the shelf }
lines:
  - { from: order, to: pick, kind: data, label: details, breaks: true }
  - { from: note, to: pick, kind: tie }
`
	platform := `
awml: "0.3"
title: Picking an order
view: platform
parts:
  - { id: order, kind: context, label: The order }
  - { id: pick, kind: ai, label: Pick it }
  - { id: note, kind: comment, label: Use the scanner }
  - { id: hint, kind: comment, label: Keep the label visible }
lines:
  - { from: order, to: pick, kind: data, label: order }
  - { from: note, to: order, kind: hidden }
  - { from: hint, to: pick, kind: tie }
`
	compared(t, security, platform)
}
