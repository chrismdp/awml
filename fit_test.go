package awml

import (
	"reflect"
	"strings"
	"testing"
)

const fitSecurity = `
awml: "0.9"
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
		`{awml: "0.9", platform: {id: fitting, label: Fitting}, off_for_up_to: PT1H, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.9", platform: {id: slow, label: Slow}, off_for_up_to: P1D, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.9", platform: {id: exposed, label: Exposed}, off_for_up_to: PT0S, steps: code, agents: [openai], calls: [anthropic], holds: [notes]}`,
		`{awml: "0.9", platform: {id: no-model, label: No model}, off_for_up_to: PT0S, steps: code, agents: [], calls: [openai], holds: [notes]}`,
		`{awml: "0.9", platform: {id: no-notes, label: No notes}, off_for_up_to: PT0S, steps: code, agents: [], calls: [anthropic], holds: []}`,
		`{awml: "0.9", platform: {id: agent, label: Agent}, off_for_up_to: PT0S, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`,
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
awml: "0.9"
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
	host := `{awml: "0.9", platform: {id: host, label: Host}, off_for_up_to: PT0S, steps: code, agents: [openai], calls: [anthropic], holds: [notes]}`
	session := `{awml: "0.9", platform: {id: session, label: Session}, off_for_up_to: P1D, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`
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
awml: "0.9"
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

// Three platforms that each hold only some of the fit workflow's parts: the laptop is off too long
// for the trigger, the server holds no notes and calls no model, and the agent can't run code.
var splitPlatforms = []string{
	`{awml: "0.9", platform: {id: laptop, label: Laptop}, off_for_up_to: P1D, steps: code, agents: [], calls: [anthropic], holds: [notes]}`,
	`{awml: "0.9", platform: {id: server, label: Server}, off_for_up_to: PT0S, steps: code, agents: [], calls: [], holds: []}`,
	`{awml: "0.9", platform: {id: cowork, label: Cowork}, off_for_up_to: PT0S, steps: agent, agents: [anthropic], calls: any, holds: [notes]}`,
}

func TestFitPartsListsEveryOptionWithNoSparePlatformAndItsPlacement(t *testing.T) {
	docs := fitDocs(t, append([]string{fitSecurity}, splitPlatforms...)...)
	report := JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:])
	on := func(part string, platforms ...string) PartPlacement { return PartPlacement{Part: part, Platforms: platforms} }
	want := PartsReport{
		Parts: []PartFit{
			{Part: "hourly", Platforms: []string{"server", "cowork"}, Reasons: []PlatformReasons{{Platform: "laptop", Reasons: []string{"hourly may wait PT2H, but the platform may be off for P1D"}}}, Models: map[string][]string{}},
			{Part: "bring", Platforms: []string{"laptop", "server"}, Reasons: []PlatformReasons{{Platform: "cowork", Reasons: []string{"bring is a code step, which anthropic's agent would carry out"}}}, Models: map[string][]string{}},
			{Part: "mail", Platforms: []string{"laptop", "server", "cowork"}, Reasons: []PlatformReasons{}, Models: map[string][]string{}},
			{Part: "notes", Platforms: []string{"laptop", "cowork"}, Reasons: []PlatformReasons{{Platform: "server", Reasons: []string{"notes is kept at notes/projects, which the platform doesn't hold"}}}, Models: map[string][]string{}},
			{Part: "choose", Platforms: []string{"laptop", "cowork"}, Reasons: []PlatformReasons{{Platform: "server", Reasons: []string{"choose has no company it may use"}}}, Models: map[string][]string{"laptop": {"anthropic"}, "cowork": {"anthropic"}}},
		},
		Options: []PartsOption{
			{Platforms: []string{"laptop", "server"}, Placement: []PartPlacement{on("hourly", "server"), on("bring", "laptop", "server"), on("mail", "laptop", "server"), on("notes", "laptop"), on("choose", "laptop")}},
			{Platforms: []string{"laptop", "cowork"}, Placement: []PartPlacement{on("hourly", "cowork"), on("bring", "laptop"), on("mail", "laptop", "cowork"), on("notes", "laptop", "cowork"), on("choose", "laptop", "cowork")}},
			{Platforms: []string{"server", "cowork"}, Placement: []PartPlacement{on("hourly", "server", "cowork"), on("bring", "server"), on("mail", "server", "cowork"), on("notes", "cowork"), on("choose", "cowork")}},
		},
		Gaps: []string{},
	}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("JudgeParts gave\n%+v\nwant\n%+v", report, want)
	}
	wantLines := []string{
		"hourly  held by server and cowork",
		"        not by laptop: hourly may wait PT2H, but the platform may be off for P1D",
		"bring   held by laptop and server",
		"        not by cowork: bring is a code step, which anthropic's agent would carry out",
		"mail    held by laptop, server and cowork",
		"notes   held by laptop and cowork",
		"        not by server: notes is kept at notes/projects, which the platform doesn't hold",
		"choose  held by laptop and cowork",
		"        not by server: choose has no company it may use",
		"option: laptop and server",
		"  hourly  on server",
		"  bring   on laptop or server",
		"  mail    on laptop or server",
		"  notes   on laptop",
		"  choose  on laptop",
		"option: laptop and cowork",
		"  hourly  on cowork",
		"  bring   on laptop",
		"  mail    on laptop or cowork",
		"  notes   on laptop or cowork",
		"  choose  on laptop or cowork",
		"option: server and cowork",
		"  hourly  on server or cowork",
		"  bring   on server",
		"  mail    on server or cowork",
		"  notes   on cowork",
		"  choose  on cowork",
		"an option on several platforms is judged part by part only: whether its parts can reach each other isn't judged",
	}
	if lines := PartsLines(report); !reflect.DeepEqual(lines, wantLines) {
		t.Errorf("PartsLines gave\n%q\nwant\n%q", lines, wantLines)
	}
}

// Where one platform holds every part, it is an option of its own, listed first, and no option that
// adds a platform to it is listed. The options of two that leave it out still are.
func TestFitPartsListsAnOptionOfOnePlatformBeforeLargerOnes(t *testing.T) {
	fitting := `{awml: "0.9", platform: {id: fitting, label: Fitting}, off_for_up_to: PT1H, steps: code, agents: [], calls: [anthropic], holds: [notes]}`
	docs := fitDocs(t, append([]string{fitSecurity}, append(append([]string{}, splitPlatforms...), fitting)...)...)
	report := JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:])
	var got [][]string
	for _, option := range report.Options {
		got = append(got, option.Platforms)
	}
	if want := [][]string{{"fitting"}, {"laptop", "server"}, {"laptop", "cowork"}, {"server", "cowork"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("JudgeParts gave options %q, want %q", got, want)
	}
	if want := []PartPlacement{{Part: "hourly", Platforms: []string{"fitting"}}, {Part: "bring", Platforms: []string{"fitting"}}}; !reflect.DeepEqual(report.Options[0].Placement[:2], want) {
		t.Errorf("JudgeParts placed the option of one as %+v", report.Options[0].Placement)
	}

	// With only the platform that holds every part, there's its option, and no note about several.
	docs = fitDocs(t, fitSecurity, fitting)
	lines := PartsLines(JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:]))
	want := []string{"option: fitting", "  hourly  on fitting", "  bring   on fitting", "  mail    on fitting", "  notes   on fitting", "  choose  on fitting"}
	if len(lines) < len(want) || !reflect.DeepEqual(lines[len(lines)-len(want):], want) {
		t.Errorf("PartsLines gave %q", lines)
	}
}

// A part no platform holds is named, and then there's no placement.
func TestFitPartsNamesAPartNoPlatformHolds(t *testing.T) {
	docs := fitDocs(t, fitSecurity, splitPlatforms[1])
	report := JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:])
	if len(report.Options) != 0 || report.Options == nil {
		t.Errorf("JudgeParts gave options %+v", report.Options)
	}
	lines := PartsLines(report)
	if lines[len(lines)-1] != "no placement: notes and choose are held by no platform given" {
		t.Errorf("PartsLines gave %q", lines)
	}
	docs = fitDocs(t, fitSecurity, splitPlatforms[0], splitPlatforms[1])
	lines = PartsLines(JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:]))
	if !reflect.DeepEqual(lines[len(lines)-8:len(lines)-6], []string{"        not by server: choose has no company it may use", "option: laptop and server"}) {
		t.Errorf("PartsLines gave %q", lines)
	}
	docs = fitDocs(t, strings.Replace(fitSecurity, "at: notes/projects", "at: elsewhere", 1), splitPlatforms[0], splitPlatforms[1])
	lines = PartsLines(JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:]))
	if lines[len(lines)-1] != "no placement: notes is held by no platform given" {
		t.Errorf("PartsLines gave %q", lines)
	}
}

func TestFitPartsReportsGapsAndDuplicateDescriptions(t *testing.T) {
	docs := fitDocs(t, fitSecurity, goodPlatform, goodPlatform)
	report := JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:])
	want := PartsReport{Parts: []PartFit{}, Options: []PartsOption{}, Gaps: []string{`platform description "worker" was given twice`}}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("JudgeParts gave %+v", report)
	}
	docs = fitDocs(t, strings.Replace(fitSecurity, "wait: PT2H, ", "", 1), goodPlatform)
	if gaps := JudgeParts([]Workflow{{Doc: docs[0]}}, docs[1:]).Gaps; !reflect.DeepEqual(gaps, []string{`trigger "hourly" has no wait`}) {
		t.Errorf("JudgeParts gave gaps %q", gaps)
	}
}
