package awml

import (
	"reflect"
	"strings"
	"testing"
)

const fitSecurity = `
awml: "0.7"
title: Mail
scopes:
  - {id: private, label: private, processing: any}
  - {id: business, label: business, processing: {anthropic: any}}
parts:
  - {id: hourly, kind: trigger, label: Hourly, wait: PT2H, in: private}
  - {id: bring, kind: code, op: bring-in, label: Bring in, in: private}
  - {id: mail, kind: connector, label: Mail, in: private}
  - {id: notes, kind: context, label: Notes, at: notes/projects, in: business}
  - {id: choose, kind: ai, op: sort, label: Choose, in: business}
lines:
  - {from: hourly, to: bring, kind: start}
  - {from: mail, to: bring, kind: data}
  - {from: bring, to: choose, kind: data}
  - {from: notes, to: choose, kind: data}
`

// placedView is the fit workflow with one implementation, whose blocks are given, and that
// implementation's platform view.
func placedView(t *testing.T, blocks string) (*Map, *Map) {
	t.Helper()
	doc := fitDocs(t, fitSecurity+"implementations:\n  placed:\n    label: Placed\n    blocks:\n"+blocks)[0]
	view, err := View(doc, "placed")
	if err != nil {
		t.Fatal(err)
	}
	return doc, view
}

func fitDocs(t *testing.T, sources ...string) []*Map {
	t.Helper()
	var docs []*Map
	for _, source := range sources {
		doc := parsed(t, source)
		if errors := Check(doc); len(errors) != 0 {
			t.Fatalf("bad fit fixture: %v", errors)
		}
		docs = append(docs, doc)
	}
	return docs
}

func TestFitAppliesAllFiveRules(t *testing.T) {
	platforms := []string{
		`{awml: "0.7", platform: {id: fitting, label: Fitting}, off_for_up_to: PT1H, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.7", platform: {id: slow, label: Slow}, off_for_up_to: P1D, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.7", platform: {id: exposed, label: Exposed}, off_for_up_to: PT0S, steps: code, agents: [openai], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.7", platform: {id: no-model, label: No model}, off_for_up_to: PT0S, steps: code, agents: [], calls: [openai], holds: [notes]}`,
		`{awml: "0.7", platform: {id: no-notes, label: No notes}, off_for_up_to: PT0S, steps: code, agents: [], calls: [anthropic], holds: []}`,
		`{awml: "0.7", platform: {id: agent, label: Agent}, off_for_up_to: PT0S, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`,
	}
	docs := fitDocs(t, append([]string{fitSecurity}, platforms...)...)
	got, gaps := Fit(docs[0], nil, docs[1:])
	want := []string{
		"fitting   fits: choose may use anthropic",
		"slow      no: hourly may wait PT2H, but the platform may be off for P1D",
		"exposed   no: openai's agents run here, and business doesn't allow openai",
		"no-model  no: choose has no company it may use",
		"no-notes  no: notes is kept at notes/projects, which the platform doesn't hold",
		"agent     no: bring is a code step, which anthropic's agent would carry out",
	}
	if !reflect.DeepEqual(got, want) || gaps != nil {
		t.Errorf("Fit gave\n%q\ngaps %q\nwant\n%q", got, gaps, want)
	}
}

func TestFitReportsSecurityGaps(t *testing.T) {
	security := `
awml: "0.7"
title: Gaps
scopes: [{id: private, label: private}]
parts:
  - {id: hourly, kind: trigger, label: Hourly, in: private}
  - {id: loose, kind: ai, op: sort, label: Loose}
lines:
  - {from: hourly, to: loose, kind: start}
`
	docs := fitDocs(t, security, goodPlatform)
	lines, gaps := Fit(docs[0], nil, docs[1:])
	want := []string{`group "private" has no processing`, `part "loose" is outside every scope`, `trigger "hourly" has no wait`}
	if lines != nil || !reflect.DeepEqual(gaps, want) {
		t.Errorf("Fit gave lines %q, gaps %q", lines, gaps)
	}
}

func TestFitCannotChooseBetweenDuplicatePlatformDescriptions(t *testing.T) {
	docs := fitDocs(t, fitSecurity, goodPlatform, goodPlatform)
	lines, gaps := Fit(docs[0], nil, docs[1:])
	want := []string{`platform description "worker" was given twice`}
	if lines != nil || !reflect.DeepEqual(gaps, want) {
		t.Errorf("Fit gave lines %q, gaps %q", lines, gaps)
	}
}

func TestFitJudgesADrawnPlacement(t *testing.T) {
	doc, placement := placedView(t, `      - {id: machine, label: Machine, platform: host, parts: [notes, choose]}
      - {id: session, label: Session, in: machine, platform: session, parts: [bring, mail]}
      - {id: phone, label: Phone, parts: []}
`)
	host := `{awml: "0.7", platform: {id: host, label: Host}, off_for_up_to: PT0S, steps: code, agents: [openai], calls: [anthropic], holds: [notes]}`
	session := `{awml: "0.7", platform: {id: session, label: Session}, off_for_up_to: P1D, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`
	got, gaps := Fit(doc, placement, fitDocs(t, host, session))
	want := []string{
		"machine  no: openai's agents run here, and business doesn't allow openai",
		"session  no: hourly may wait PT2H, but the platform may be off for P1D",
		"         no: bring is a code step, which anthropic's agent would carry out",
		"phone    not judged: no platform",
	}
	if !reflect.DeepEqual(got, want) || gaps != nil {
		t.Errorf("Fit gave\n%q\ngaps %q\nwant\n%q", got, gaps, want)
	}
}

func TestFitReportsBlocksWithoutDescriptionsAsNotJudged(t *testing.T) {
	doc, placement := placedView(t, `      - {id: machine, label: Machine, platform: host, parts: [bring, mail, notes, choose]}
      - {id: phone, label: Phone, parts: []}
`)
	report := Judge(doc, placement, nil)
	want := FitReport{
		Results: []FitResult{
			{Platform: "machine", Reasons: []string{`platform "host" wasn't described`}, Models: map[string][]string{}},
			{Platform: "phone", Reasons: []string{}, Models: map[string][]string{}},
		},
		Gaps: []string{},
	}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("Judge gave\n%+v\nwant\n%+v", report, want)
	}
	lines := FitLines(report.Results)
	wantLines := []string{
		`machine  not judged: platform "host" wasn't described`,
		"phone    not judged: no platform",
	}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Errorf("FitLines gave\n%q\nwant\n%q", lines, wantLines)
	}
}

// Every trigger is one the workflow runs on, so even a trigger with no line out needs a wait, and is
// judged against the platform.
func TestFitAsksForAWaitOnATriggerWithNoLineOut(t *testing.T) {
	security := `
awml: "0.7"
title: Gaps
scopes: [{id: private, label: private, processing: any}]
parts:
  - {id: hourly, kind: trigger, label: Hourly, in: private}
  - {id: daily, kind: trigger, label: Daily, wait: P2D, in: private}
  - {id: loose, kind: ai, op: sort, label: Loose, in: private}
`
	docs := fitDocs(t, security)
	if _, gaps := Fit(docs[0], nil, nil); !reflect.DeepEqual(gaps, []string{`trigger "hourly" has no wait`}) {
		t.Errorf("Fit gave gaps %q", gaps)
	}
	docs = fitDocs(t, strings.Replace(security, "Hourly,", "Hourly, wait: PT1H,", 1), codeServer)
	if lines, _ := Fit(docs[0], nil, docs[1:]); !reflect.DeepEqual(lines, []string{"server  no: hourly may wait PT1H, but the platform may be off for P1D"}) {
		t.Errorf("Fit gave %q", lines)
	}
}
