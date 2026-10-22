package awml

import (
	"reflect"
	"testing"
)

const fitSecurity = `
awml: "0.3"
title: Mail
view: security
groups:
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
  - {from: bring, to: choose, kind: publish}
  - {from: notes, to: choose, kind: data}
`

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
		`{awml: "0.3", platform: {id: fitting, label: Fitting}, off_for_up_to: PT1H, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.3", platform: {id: slow, label: Slow}, off_for_up_to: P1D, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.3", platform: {id: exposed, label: Exposed}, off_for_up_to: PT0S, steps: code, agents: [openai], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.3", platform: {id: no-model, label: No model}, off_for_up_to: PT0S, steps: code, agents: [], calls: [openai], holds: [notes]}`,
		`{awml: "0.3", platform: {id: no-notes, label: No notes}, off_for_up_to: PT0S, steps: code, agents: [], calls: [anthropic], holds: []}`,
		`{awml: "0.3", platform: {id: agent, label: Agent}, off_for_up_to: PT0S, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`,
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
awml: "0.3"
title: Gaps
view: security
groups: [{id: private, label: private}]
parts:
  - {id: hourly, kind: trigger, label: Hourly, in: private}
  - {id: loose, kind: ai, op: sort, label: Loose}
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
	placement := `
awml: "0.3"
title: Mail placement
view: platform
groups:
  - {id: machine, label: Machine, platform: host}
  - {id: session, label: Session, in: machine, platform: session}
  - {id: phone, label: Phone}
parts:
  - {id: hourly, kind: trigger, label: Hourly}
  - {id: bring, kind: code, op: bring-in, label: Bring in, in: session}
  - {id: mail, kind: connector, label: Mail, in: session}
  - {id: notes, kind: context, label: Notes, in: machine}
  - {id: choose, kind: ai, op: sort, label: Choose, in: machine}
lines:
  - {from: hourly, to: bring, kind: start}
  - {from: mail, to: bring, kind: data}
  - {from: bring, to: choose, kind: publish}
  - {from: notes, to: choose, kind: data}
`
	host := `{awml: "0.3", platform: {id: host, label: Host}, off_for_up_to: PT0S, steps: code, agents: [openai], calls: [anthropic], holds: [notes]}`
	session := `{awml: "0.3", platform: {id: session, label: Session}, off_for_up_to: P1D, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`
	docs := fitDocs(t, fitSecurity, placement, host, session)
	got, gaps := Fit(docs[0], docs[1], docs[2:])
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
