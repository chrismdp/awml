package awml

import (
	"fmt"
	"testing"
)

// shapes is a made-up bakery's file, given its parts and lines.
func bakery(t *testing.T, parts string, lines string) *Map {
	t.Helper()
	return parsed(t, "awml: \"0.7\"\ntitle: Shapes\nparts:\n"+parts+"lines:\n"+lines)
}

const unstarted = `step "%s" is started by no trigger: add the start line that leads into it, or, if it begins work of its own, move it into another workflow that an outside action starts`

func TestTwoTriggersWhoseRunsShareAStepAreOneWorkflow(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: label, kind: trigger, label: "The build label is added"}
  - {id: nightly, kind: trigger, label: "Every night"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: box, kind: code, label: "Box the bread"}
`, `
  - {from: label, to: bake, kind: start}
  - {from: nightly, to: bake, kind: start}
  - {from: bake, to: box, kind: start}
`))
}

func TestTwoTriggersWhoseRunsShareOnlyAContextAreTwoWorkflows(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: evening, kind: trigger, label: "Every evening"}
  - {id: order, kind: trigger, label: "An order arrives"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: count, kind: code, label: "Count what's left"}
  - {id: stock, kind: context, label: "The stock"}
`, `
  - {from: morning, to: bake, kind: start}
  - {from: order, to: bake, kind: start}
  - {from: evening, to: count, kind: start}
  - {from: bake, to: stock, kind: data}
  - {from: stock, to: count, kind: data}
`),
		`this file holds 2 workflows: the one started by "Every morning" or "An order arrives" and the one started by "Every evening"`)
}

func TestADataLineDoesNotCarryARunOn(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: box, kind: code, label: "Box the bread"}
`, `
  - {from: morning, to: bake, kind: start}
  - {from: bake, to: box, kind: data}
`),
		fmt.Sprintf(unstarted, "box"))
}

func TestAPartNoTriggerReachesWarns(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: recipes, kind: context, label: "The recipes"}
  - {id: bank, kind: connector, label: "The bakery's bank account"}
  - {id: till, kind: surface, label: "The till"}
`, `
  - {from: morning, to: bake, kind: start}
  - {from: recipes, to: bake, kind: data}
  - {from: bank, to: till, kind: data}
`),
		`part "bank" is linked to no trigger: connect it to the work, or remove it`,
		`part "till" is linked to no trigger: connect it to the work, or remove it`)
}

func TestAHiddenLineLinksNothing(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: till, kind: surface, label: "The till"}
`, `
  - {from: morning, to: bake, kind: start}
  - {from: bake, to: till, kind: hidden}
`),
		`part "till" is linked to no trigger: connect it to the work, or remove it`)
}

func TestAStepThatOnlyReadsAContextAnotherWroteWarnsUntilAStartLineLeadsIn(t *testing.T) {
	parts := `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: stock, kind: context, label: "The stock"}
  - {id: order, kind: ai, label: "Order more flour"}
`
	lines := `
  - {from: morning, to: bake, kind: start}
  - {from: bake, to: stock, kind: data}
  - {from: stock, to: order, kind: data}
`
	warnings(t, bakery(t, parts, lines), fmt.Sprintf(unstarted, "order"))
	warnings(t, bakery(t, parts, lines+"  - {from: bake, to: order, kind: start}\n"))
}

func TestAStartLineFromAPartThatIsNotAStepCarriesNoRun(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: stock, kind: context, label: "The stock"}
  - {id: order, kind: ai, label: "Order more flour"}
`, `
  - {from: morning, to: bake, kind: start}
  - {from: bake, to: stock, kind: data}
  - {from: stock, to: order, kind: start}
`),
		fmt.Sprintf(unstarted, "order"))
}

func TestARunGoesOnThroughForksReviewsAndStandingApprovals(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: split, kind: fork}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: yes, kind: review, label: "Ana's yes"}
  - {id: box, kind: code, label: "Box the bread"}
  - {id: standing, kind: approval, label: "Ana's standing approval"}
  - {id: label, kind: code, label: "Label the boxes"}
`, `
  - {from: morning, to: split, kind: start}
  - {from: split, to: bake, kind: start}
  - {from: split, to: yes, kind: start}
  - {from: yes, to: box, kind: start, label: "[yes]"}
  - {from: box, to: standing, kind: start}
  - {from: standing, to: label, kind: start}
`))
}

func TestAnUnreachedStepWarnsOnce(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: sweep, kind: person, label: "Sweep the floor"}
`, `
  - {from: morning, to: bake, kind: start}
`),
		`part "sweep" is linked to no trigger: connect it to the work, or remove it`)
}

func TestACommentWithNoTieWarns(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: bake, kind: code, label: "Bake the bread"}
  - {id: why, kind: comment, label: "Bread first, then cakes"}
  - {id: tied, kind: comment, label: "Ben bakes on Sundays"}
`, `
  - {from: morning, to: bake, kind: start}
  - {from: tied, to: bake, kind: tie}
`),
		`comment "why" is tied to nothing: tie it to the part it explains, or remove it`)
}

func TestAFileWithOneTriggerIsOneWorkflow(t *testing.T) {
	warnings(t, bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
`, ""))
}

func TestTheNewWarningsLeaveTheCheckPassing(t *testing.T) {
	doc := bakery(t, `
  - {id: morning, kind: trigger, label: "Every morning"}
  - {id: evening, kind: trigger, label: "Every evening"}
  - {id: sweep, kind: person, label: "Sweep the floor"}
  - {id: why, kind: comment, label: "Bread first, then cakes"}
`, "")
	if len(Warnings(doc)) == 0 {
		t.Fatal("the file gave no warnings")
	}
	if got := Check(doc); len(got) != 0 {
		t.Fatalf("Check returned warnings as mistakes: %v", got)
	}
}
