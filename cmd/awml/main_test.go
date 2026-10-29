package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chrismdp/awml"
)

const good = `awml: "0.9"
title: Dinners
scopes:
  - { id: family, label: "family" }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
lines:
  - { from: thu, to: plan, kind: start }
`

var bad = strings.Replace(good, "to: plan", "to: plann", 1)

// A legacy pair, which this release reads only to convert.
const legacySecurity = `awml: "0.2"
title: Dinners
view: security
groups:
  - { id: family, label: "family" }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
lines:
  - { from: thu, to: plan, kind: start }
`

var legacyPlatform = strings.Replace(legacySecurity, "view: security", "view: platform", 1)

type result struct {
	code           int
	stdout, stderr string
}

func awmlRun(args ...string) result {
	return awmlRunInput("", args...)
}

func awmlRunInput(input string, args ...string) result {
	var out, errs bytes.Buffer
	code := run(args, strings.NewReader(input), &out, &errs)
	return result{code, out.String(), errs.String()}
}

func write(t *testing.T, name, src string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func doc(t *testing.T, src string) *awml.Map {
	t.Helper()
	d, err := awml.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return d.(*awml.Map)
}

func TestDrawPrintsMermaid(t *testing.T) {
	r := awmlRun("draw", write(t, "good.yaml", good))
	if r.code != 0 || r.stdout != awml.Draw(doc(t, good))+"\n" {
		t.Errorf("draw gave %d and\n%s", r.code, r.stdout)
	}
}

func TestDrawRefusesAFileWithMistakes(t *testing.T) {
	f := write(t, "bad.yaml", bad)
	r := awmlRun("draw", f)
	want := result{1, "", f + `: the line from "thu" to "plann" ends at unknown part "plann"` + "\n"}
	if r != want {
		t.Errorf("draw gave %+v", r)
	}
}

// A 0.9 drawing of a live system, whose one line into a Block action breaks a rule.
const live = `awml: "0.9"
title: Old files
parts:
  - { id: night, kind: trigger, label: "Every night" }
  - { id: tidy, kind: code, op: do, label: "Tidy the old files" }
  - { id: wipe, kind: outside, label: "Delete a file: Block" }
lines:
  - { from: tidy, to: wipe, kind: start, label: "one night in June" }
  - { from: night, to: tidy, kind: start }
`

// A drawing with errors is valid, so check passes it, saying each error, and every command takes it.
func TestADrawingWithErrorsPassesTheCheckAndIsDrawnRed(t *testing.T) {
	path := write(t, "live.yaml", live)
	message := `outside action "wipe" is Block, but part "tidy" leads to it: a blocked action is never taken, so a line into it is red`
	if r := awmlRun("check", path); r != (result{0, path + ": error: " + message + "\n", ""}) {
		t.Errorf("check gave %+v", r)
	}
	if r := awmlRun("check", "--json", path); r.code != 0 ||
		!strings.Contains(r.stdout, `"result":"error","mistakes":[{"file":"`+path+`","message":"outside action \"wipe\"`) ||
		!strings.Contains(r.stdout, `so a line into it is red","severity":"error"}`) {
		t.Errorf("check --json gave %+v", r)
	}
	description := write(t, "laptop.yaml", "awml: \"0.9\"\nplatform: { id: laptop, label: Laptop }\noff_for_up_to: PT0S\nsteps: code\nagents: []\ncalls: any\nholds: []\n")
	for _, args := range [][]string{{"fit", path, description}, {"fit", "--parts", path, description}} {
		if r := awmlRun(args...); r.stderr != "" || r.stdout == "" {
			t.Errorf("awml %q gave %+v", args, r)
		}
	}
	if r := awmlRun("convert", path); r != (result{0, live, ""}) {
		t.Errorf("convert gave %+v", r)
	}
	if r := awmlRun("draw", path); r.code != 0 || r.stdout != awml.Draw(doc(t, live))+"\n" || !strings.Contains(r.stdout, "linkStyle 0 stroke:#d0342c") {
		t.Errorf("draw gave %+v", r)
	}
	if r := awmlRun("summary", "--json", path); r.code != 0 || !strings.Contains(r.stdout, `"redLines":1`) {
		t.Errorf("summary gave %+v", r)
	}
	out := filepath.Join(t.TempDir(), "out.html")
	if r := awmlRun("page", path, "-o", out); r.code != 0 {
		t.Errorf("page gave %+v", r)
	}
}

func TestCheckPassesASoundFile(t *testing.T) {
	f := write(t, "good.yaml", good)
	if r := awmlRun("check", f); r != (result{0, f + ": ok\n", ""}) {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckPrintsWarningsBeforeOkWithoutFailing(t *testing.T) {
	src := strings.Replace(good, "kind: ai, label:", "kind: ai, op: do, label:", 1)
	f := write(t, "warning.yaml", src)
	want := f + `: warning: step "plan" is a do step decided by AI: a do step should be code acting on what an earlier step decided` + "\n" + f + ": ok\n"
	if r := awmlRun("check", f); r != (result{0, want, ""}) {
		t.Errorf("check gave %+v", r)
	}
}

// The line with a mistake in it links nothing, so bad's plan step is left on its own.
const (
	unlinkedPlan  = `part "plan" has no line linking it to a trigger: connect it or remove it`
	unstartedPlan = `step "plan" is started by no trigger: add the start line it's missing, or split its work into a workflow of its own that an outside action starts`
)

func TestCheckPrintsWarningsBeforeMistakesAndStillFailsForTheMistakes(t *testing.T) {
	src := strings.Replace(bad, "kind: ai, label:", "kind: ai, op: do, label:", 1)
	f := write(t, "warning-and-mistake.yaml", src)
	want := f + `: warning: step "plan" is a do step decided by AI: a do step should be code acting on what an earlier step decided` + "\n" +
		f + `: warning: ` + unlinkedPlan + "\n" +
		f + `: warning: ` + unstartedPlan + "\n" +
		f + `: the line from "thu" to "plann" ends at unknown part "plann"` + "\n"
	if r := awmlRun("check", f); r != (result{1, want, ""}) {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckReportsEachMistakeAndFails(t *testing.T) {
	g, b := write(t, "good.yaml", good), write(t, "bad.yaml", bad)
	r := awmlRun("check", g, b)
	want := result{1, g + ": ok\n" + b + ": warning: " + unlinkedPlan + "\n" + b + ": warning: " + unstartedPlan + "\n" +
		b + `: the line from "thu" to "plann" ends at unknown part "plann"` + "\n", ""}
	if r != want {
		t.Errorf("check gave %+v", r)
	}
}

func TestEveryCommandRefusesAnOlderFileAndSaysToConvertIt(t *testing.T) {
	path := write(t, "old.yaml", strings.Replace(good, `awml: "0.9"`, `awml: "0.5"`, 1))
	message := path + `: awml is "0.5", which this release no longer reads: upgrade the file with awml convert` + "\n"
	if r := awmlRun("check", path); r != (result{1, message, ""}) {
		t.Errorf("check gave %+v", r)
	}
	desc := write(t, "worker.yaml", goodPlatformForCommand)
	out := filepath.Join(t.TempDir(), "out.html")
	for _, args := range [][]string{{"draw", path}, {"summary", path}, {"fit", path, desc}, {"page", path, "-o", out}} {
		if r := awmlRun(args...); r != (result{1, "", message}) {
			t.Errorf("%v gave %+v", args, r)
		}
	}
}

func TestConvertUpgradesOneOlderFile(t *testing.T) {
	old := "# Dinners at home.\n" + strings.Replace(strings.Replace(good, `awml: "0.9"`, `awml: "0.5"`, 1),
		"kind: start }", "kind: start, breaks: false }", 1)
	r := awmlRun("convert", write(t, "old.yaml", old))
	if r != (result{0, strings.Replace(good, "title: Dinners\n", "title: Dinners\ndescription: |\n  Dinners at home.\n", 1), ""}) {
		t.Errorf("convert gave %+v\nwant stdout:\n%s", r, good)
	}
	security := awmlRun("convert", write(t, "security.yaml", legacySecurity))
	if security != (result{0, good, ""}) {
		t.Errorf("convert of a security view gave %+v", security)
	}
	if r := awmlRun("convert", write(t, "current.yaml", good)); r != (result{0, good, ""}) {
		t.Errorf("convert of a current file gave %+v", r)
	}
	platform := write(t, "platform.yaml", legacyPlatform)
	if r := awmlRun("convert", platform); r != (result{1, "", platform + ": a platform view converts only with its security view: awml convert security.yaml platform.yaml\n"}) {
		t.Errorf("convert of a platform view gave %+v", r)
	}
	if r := awmlRun("convert", "--implementation", "home", write(t, "old.yaml", old)); r.code != 2 || !strings.Contains(r.stderr, "only a legacy pair has an implementation to name") {
		t.Errorf("convert --implementation of one file gave %+v", r)
	}
}

func TestConvertGivesA07FileAsItWasWithItsComments(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "tour", "08-gap.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\n  # ") || !strings.Contains(string(src), "\ndescription: |\n") {
		t.Fatalf("%s no longer has a comment and a description", path)
	}
	if r := awmlRun("convert", path); r != (result{0, string(src), ""}) {
		t.Errorf("convert gave %+v\nwant stdout:\n%s", r, src)
	}
}

func TestConvertMovesEachCommentOfAnOlderFileToTheTopOnce(t *testing.T) {
	old := `awml: "0.5" # Dinners at home.
title: Dinners
scopes:
  - { id: family, label: "family" }
parts:
  # The week starts on Thursday.
  - { id: thu, kind: trigger, label: "Thursday 19:00" } # After the school run.
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
lines:
  # Thursday starts the plan.
  - { from: thu, to: plan, kind: start }
`
	want := "# Dinners at home.\n# The week starts on Thursday.\n# After the school run.\n# Thursday starts the plan.\n" + good
	if r := awmlRun("convert", write(t, "old.yaml", old)); r != (result{0, want, ""}) {
		t.Errorf("convert gave %+v\nwant stdout:\n%s", r, want)
	}
}

// A file older ones passed may break a rule a newer one holds it to. awml convert still writes all
// of it, and lists what awml check will report.
func TestConvertListsTheMistakesNewerRulesFind(t *testing.T) {
	old := `awml: "0.4"
title: Send
parts:
  - { id: act, kind: code, op: do, label: "Act" }
  - { id: out, kind: outside, label: "Send it" }
lines:
  - { from: act, to: out, kind: data }
`
	r := awmlRun("convert", write(t, "old.yaml", old))
	want := "awml convert: converted, but the result is invalid:\n" +
		`  outside action "out" has no tier: end its label with Allow, Ask or Block after a colon, as in "Send the mail: Ask"` + "\n"
	if r != (result{1, strings.Replace(old, `awml: "0.4"`, `awml: "0.9"`, 1), want}) {
		t.Errorf("convert gave %+v", r)
	}
}

// A trigger a do step fired becomes its outside action, which can't be marked untrusted. awml convert
// says where the mark belongs, and exits 1, since the author has that to do.
func TestConvertSaysWhatItCouldntCarryOver(t *testing.T) {
	old := `awml: "0.6"
title: Dinners
parts:
  - { id: tell, kind: code, op: do, label: "Tell the shopping" }
  - { id: ready, kind: trigger, label: "The list is ready", untrusted: true, ref: shopping.yaml#ready }
lines:
  - { from: tell, to: ready, kind: start }
`
	converted := `awml: "0.9"
title: Dinners
parts:
  - { id: tell, kind: code, op: do, label: "Tell the shopping" }
  - { id: ready, kind: outside, label: "The list is ready: Allow", ref: shopping.yaml#ready }
lines:
  - { from: tell, to: ready, kind: start }
`
	want := "awml convert: converted, but do this by hand:\n" +
		`  part "ready" was a trigger marked untrusted, and an outside action isn't: mark the trigger it starts, shopping.yaml#ready, untrusted instead` + "\n"
	if r := awmlRun("convert", write(t, "old.yaml", old)); r != (result{1, converted, want}) {
		t.Errorf("convert gave %+v\nwant stderr %q", r, want)
	}
}

func TestCheckReportsAFileThatIsntYAML(t *testing.T) {
	f := write(t, "broken.yaml", "parts: [\n")
	r := awmlRun("check", f)
	if r.code != 1 || r.stdout != f+": isn't YAML: line 1: did not find expected node content\n" {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckJSONListsEveryCheckAndFailsLikeThePlainOutput(t *testing.T) {
	previous := version
	version = "0.4.1"
	t.Cleanup(func() { version = previous })
	g, b := write(t, "good.yaml", good), write(t, "bad.yaml", bad)
	missing := filepath.Join(t.TempDir(), "none.yaml")
	r := awmlRun("check", "--json", g, b, missing)
	want, _ := json.Marshal(awml.CheckSources("0.4.1", []awml.Source{
		{File: g, Text: []byte(good)},
		{File: b, Text: []byte(bad)},
		{File: missing, Unreadable: "can't be read: No such file or directory"},
	}))
	if r != (result{1, string(want) + "\n", ""}) {
		t.Errorf("check --json gave %+v\nwant %s", r, want)
	}
	if r := awmlRun("check", g, "--json"); r.code != 0 || !strings.HasPrefix(r.stdout, `{"engine":"0.4.1","files":[{"file":"`+g+`","view":"workflow"}],"checks":[`) {
		t.Errorf("check --json gave %+v", r)
	}
}

func TestCheckHelpMentionsJSON(t *testing.T) {
	if r := awmlRun("check", "-h"); r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml check [-h] [--json] files [files ...]\n") || !strings.Contains(r.stdout, "  --json      print every check and its result as JSON\n") {
		t.Errorf("check -h gave %+v", r)
	}
}

func TestCheckReportsAMissingFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "none.yaml")
	r := awmlRun("check", f)
	if r.code != 1 || r.stdout != f+": can't be read: No such file or directory\n" {
		t.Errorf("check gave %+v", r)
	}
}

func TestFitPrintsPlainAndJSONResults(t *testing.T) {
	security := write(t, "security.yaml", `
awml: "0.9"
title: Reply
scopes: [{id: business, label: business, processing: {anthropic: any}}]
parts:
  - {id: draft, kind: ai, op: draft, label: Draft, in: business}
`)
	platform := write(t, "worker.yaml", `
awml: "0.9"
platform: {id: worker, label: Worker}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: []
`)
	plain := awmlRun("fit", security, platform)
	if plain != (result{0, "worker  fits: draft may use anthropic\n", ""}) {
		t.Errorf("plain fit gave %+v", plain)
	}
	json := awmlRun("fit", "--json", security, platform)
	want := `{"results":[{"platform":"worker","judged":true,"fits":true,"reasons":[],"models":{"draft":["anthropic"]}}],"gaps":[]}`
	if json != (result{0, want + "\n", ""}) {
		t.Errorf("JSON fit gave %+v", json)
	}
	cowork := write(t, "cowork.yaml", `
awml: "0.9"
platform: {id: cowork, label: Cowork}
off_for_up_to: PT0S
steps: agent
agents: [openai]
calls: any
holds: []
`)
	no := awmlRun("fit", "--json", security, cowork)
	wantNo := `{"results":[{"platform":"cowork","judged":true,"fits":false,"reasons":["openai's agents run here, and business doesn't allow openai"],"models":{"draft":["anthropic"]}}],"gaps":[]}`
	if no != (result{0, wantNo + "\n", ""}) {
		t.Errorf("JSON fit for a platform that doesn't fit gave %+v", no)
	}
}

func TestFitFailsForAGapOrTheWrongKindsOfFile(t *testing.T) {
	security := write(t, "security.yaml", `
awml: "0.9"
title: Reply
scopes: [{id: business, label: business}]
parts: [{id: draft, kind: ai, op: draft, label: Draft, in: business}]
`)
	platform := write(t, "worker.yaml", goodPlatformForCommand)
	if r := awmlRun("fit", security, platform); r.code != 1 || r.stdout != `group "business" has no processing`+"\n" {
		t.Errorf("gap fit gave %+v", r)
	}
	if r := awmlRun("fit", "--json", security, platform); r.code != 1 || r.stdout != `{"results":[],"gaps":["group \"business\" has no processing"]}`+"\n" {
		t.Errorf("gap fit as JSON gave %+v", r)
	}
	// A fit with nothing to judge, or nothing to judge it against, is a mistake in the command.
	usage := "usage: awml fit [-h] [--json] [--parts] [--implementation ID] files [files ...]\nawml fit: error: "
	for _, tc := range []struct {
		files []string
		want  string
	}{
		{[]string{platform}, "no workflow was given"},
		{[]string{platform, platform}, "no workflow was given"},
		{[]string{security}, "no platform descriptions were given"},
		{[]string{security, security}, "no platform descriptions were given"},
	} {
		if r := awmlRun(append([]string{"fit"}, tc.files...)...); r != (result{2, "", usage + tc.want + "\n"}) {
			t.Errorf("fit of %d files gave %+v", len(tc.files), r)
		}
	}
}

func TestFitPartsJudgesEachPartAlone(t *testing.T) {
	workflow := write(t, "reply.yaml", `
awml: "0.9"
title: Reply
scopes: [{id: business, label: business, processing: {anthropic: any}}]
parts:
  - {id: mail, kind: trigger, label: Mail comes, wait: PT1H, in: business}
  - {id: draft, kind: ai, op: draft, label: Draft, in: business}
lines: [{from: mail, to: draft, kind: start}]
`)
	worker := write(t, "worker.yaml", goodPlatformForCommand)
	laptop := write(t, "laptop.yaml", strings.Replace(strings.Replace(goodPlatformForCommand, "worker", "laptop", 2), "PT0S", "P1D", 1))
	plain := awmlRun("fit", "--parts", workflow, laptop, worker)
	want := "mail   held by worker\n" +
		"       not by laptop: mail may wait PT1H, but the platform may be off for P1D\n" +
		"draft  held by laptop and worker\n"
	if plain != (result{0, want, ""}) {
		t.Errorf("fit --parts gave %+v", plain)
	}
	json := awmlRun("fit", "--parts", "--json", workflow, laptop, worker)
	wantJSON := `{"parts":[{"part":"mail","platforms":["worker"],"reasons":[{"platform":"laptop","reasons":["mail may wait PT1H, but the platform may be off for P1D"]}],"models":{}},` +
		`{"part":"draft","platforms":["laptop","worker"],"reasons":[],"models":{"laptop":["anthropic"],"worker":["anthropic"]}}],"placements":[],"gaps":[]}`
	if json != (result{0, wantJSON + "\n", ""}) {
		t.Errorf("fit --parts --json gave %+v", json)
	}
	gap := write(t, "gap.yaml", strings.Replace(mustRead(t, workflow), "wait: PT1H, ", "", 1))
	if r := awmlRun("fit", "--parts", gap, worker); r != (result{1, `trigger "mail" has no wait` + "\n", ""}) {
		t.Errorf("fit --parts of a gap gave %+v", r)
	}
	usage := "usage: awml fit [-h] [--json] [--parts] [--implementation ID] files [files ...]\nawml fit: error: "
	if r := awmlRun("fit", "--parts", "--implementation", "office", workflow, worker); r != (result{2, "", usage + "argument --parts: not allowed with argument --implementation\n"}) {
		t.Errorf("fit --parts of an implementation gave %+v", r)
	}
	if r := awmlRun("fit", "--parts", workflow); r != (result{2, "", usage + "no platform descriptions were given\n"}) {
		t.Errorf("fit --parts without descriptions gave %+v", r)
	}
	if r := awmlRun("fit", "-h"); !strings.Contains(r.stdout, "  --parts     judge each part alone, and give the fewest platforms that hold them all\n") {
		t.Errorf("fit -h gave %+v", r)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFitJudgesAnImplementation(t *testing.T) {
	workflow := write(t, "reply.yaml", `
awml: "0.9"
title: Reply
scopes: [{id: business, label: business, processing: {anthropic: any}}]
parts: [{id: draft, kind: ai, op: draft, label: Draft, in: business}]
implementations:
  office:
    label: Office
    blocks: [{id: box, label: Box, platform: worker, parts: [draft]}]
`)
	platform := write(t, "worker.yaml", goodPlatformForCommand)
	if r := awmlRun("fit", "--implementation", "office", workflow, platform); r != (result{0, "box  fits: draft may use anthropic\n", ""}) {
		t.Errorf("placement fit gave %+v", r)
	}
}

const goodPlatformForCommand = `
awml: "0.9"
platform: {id: worker, label: Worker}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: []
`

func TestSummaryPrintsJSONForOneOrMoreFiles(t *testing.T) {
	lunches := strings.Replace(strings.Replace(good, "title: Dinners", "title: Lunches", 1), "id: plan, kind: ai", "id: plan, kind: code", 1)
	dinners, other := write(t, "dinners.yaml", good), write(t, "lunches.yaml", lunches)
	wantSingle, err := json.Marshal(awml.Summarise(doc(t, good)))
	if err != nil {
		t.Fatal(err)
	}
	wantBoth, err := json.Marshal(awml.SummariseTogether([]awml.Workflow{{File: dinners, Doc: doc(t, good)}, {File: other, Doc: doc(t, lunches)}}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want []byte
	}{{[]string{"summary", "--json", dinners}, wantSingle}, {[]string{"summary", dinners, other, "--json"}, wantBoth}} {
		r := awmlRun(tc.args...)
		if r != (result{0, string(tc.want) + "\n", ""}) {
			t.Errorf("awml %q gave %+v", tc.args, r)
		}
	}
}

func TestSummaryPrintsPlainCounts(t *testing.T) {
	r := awmlRun("summary", write(t, "good.yaml", good))
	want := "steps: AI 1, code 0, person 0\noperations: none\noutside actions: Allow 0, Ask 0, Block 0\ntriggers: 1\nreviews: 0\ncontexts: 0\nconnectors: 0\nsurfaces: 0\nscopes: 1\nplatforms: 0\npublishing lines: 0\nrefused reads: 0\nred lines: 0\nuntrusted parts: 0\n"
	if r != (result{0, want, ""}) {
		t.Errorf("summary gave %+v", r)
	}
}

const orders = `awml: "0.9"
title: Cake orders
scopes:
  - { id: bakery, label: "bakery: Ana, Ben", processing: any }
parts:
  - { id: form, kind: trigger, label: "An order on the website's form", untrusted: true }
  - { id: keep, kind: code, op: bring-in, label: "Keep only the order number and quantity", in: bakery, cleans: true }
  - { id: read, kind: ai, op: bring-in, label: "Read the order", in: bakery }
  - { id: note, kind: code, op: do, label: "Note it", in: bakery }
  - { id: board, kind: outside, label: "Pin it to the board: Allow", in: bakery }
lines:
  - { from: form, to: keep, kind: start }
  - { from: keep, to: read, kind: start }
  - { from: form, to: note, kind: start }
  - { from: note, to: board, kind: start }
`

func TestSummaryListsWhatEachUntrustedPartsTextReaches(t *testing.T) {
	r := awmlRun("summary", write(t, "orders.yaml", orders))
	want := "untrusted parts: 1\n  An order on the website's form\n    steps:\n      Keep only the order number and quantity\n      Note it\n    outside actions:\n      Pin it to the board: Allow\n    stopped by:\n      Keep only the order number and quantity\n    risks accepted: none\n"
	if r := awmlRun("summary", write(t, "nothing.yaml", strings.Replace(orders, "{ from: form, to: note, kind: start }", "{ from: keep, to: note, kind: start }", 1))); !strings.HasSuffix(r.stdout, "    outside actions: none\n    stopped by:\n      Keep only the order number and quantity\n    risks accepted: none\n") {
		t.Errorf("summary gave %+v", r)
	}
	if r.code != 0 || !strings.HasSuffix(r.stdout, want) {
		t.Errorf("summary gave %+v", r)
	}
	// A standing approval in front of the action accepts the risk, and the summary lists it under it.
	accepted := strings.Replace(orders, "Pin it to the board: Allow", "Pin it to the board: Ask", 1)
	accepted = strings.Replace(accepted, "  - { id: board,", "  - { id: yes, kind: approval, label: \"Ana's standing approval, risk accepted by Ana\", in: bakery }\n  - { id: list, kind: surface, label: \"Ana's list\", in: bakery }\n  - { id: board,", 1)
	accepted = strings.Replace(accepted, "  - { from: note, to: board, kind: start }", "  - { from: note, to: yes, kind: start }\n  - { from: yes, to: board, kind: start }\n  - { from: yes, to: list, kind: data }", 1)
	path := write(t, "accepted.yaml", accepted)
	if r := awmlRun("check", path); r.code != 0 {
		t.Errorf("check gave %+v", r)
	}
	if r := awmlRun("summary", path); !strings.HasSuffix(r.stdout, "    risks accepted:\n      Pin it to the board: Ask\n        under Ana's standing approval, risk accepted by Ana\n") {
		t.Errorf("summary gave %+v", r)
	}
}

// Untrusted text that reaches an outside action with nothing to stop it or accept the risk breaks
// a rule, so it's an error, which check says, and draw draws red, as every line that breaks a rule.
// The security view shows where the text goes, so there's no untrusted view to ask for.
func TestUntrustedTextThatReachesAnActionUnstoppedIsAnError(t *testing.T) {
	path := write(t, "orders.yaml", orders)
	words := `untrusted text from part "form" reaches outside action "board" with nothing to stop it or accept the risk`
	if r := awmlRun("check", path); r != (result{0, path + ": error: " + words + ": a review, a person's own step or a code step marked cleans on the way would stop it, or a standing approval in front of the action would accept the risk\n", ""}) {
		t.Errorf("check gave %+v", r)
	}
	if r := awmlRun("draw", path); r.code != 0 || !strings.Contains(r.stdout, "  linkStyle 2,3 stroke:#d0342c") || !strings.Contains(r.stdout, "  class form unstopped\n") {
		t.Errorf("draw gave %+v", r)
	}
	if r := awmlRun("draw", "--untrusted", path); r != (result{2, "", usage + "awml: error: unrecognized arguments: --untrusted\n"}) {
		t.Errorf("draw --untrusted gave %+v", r)
	}
}

func TestPageWritesOneHTMLFile(t *testing.T) {
	lunches := strings.Replace(good, "title: Dinners", "title: Lunches", 1)
	a, b := write(t, "a.yaml", good), write(t, "b.yaml", lunches)
	out := filepath.Join(t.TempDir(), "out.html")
	if r := awmlRun("page", a, b, "-o", out); r != (result{0, "", ""}) {
		t.Errorf("page gave %+v", r)
	}
	html, err := os.ReadFile(out)
	if err != nil || string(html) != awml.Page([]awml.Workflow{{File: a, Path: a, Doc: doc(t, good)}, {File: b, Path: b, Doc: doc(t, lunches)}}) {
		t.Errorf("the page isn't the map and the two drawings: %v", err)
	}
}

func TestPageWritesNothingWhenAFileHasMistakes(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.html")
	r := awmlRun("page", write(t, "a.yaml", good), write(t, "bad.yaml", bad), "--out", out)
	if r.code != 1 || !strings.Contains(r.stderr, "unknown part") {
		t.Errorf("page gave %+v", r)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("page wrote %s", out)
	}
}

func TestConvertWritesAHandWrittenFileAndNamesItsImplementation(t *testing.T) {
	security := write(t, "security.yaml", `# A bakery plans tomorrow's loaves. The bakery is fictional.
awml: "0.3"
title: Tomorrow's loaves
view: security
groups:
  - { id: "bakery", label: "bakery: Jo, Kit", processing: { anthropic: any } }
parts:
  - { id: morning, kind: "trigger", label: "Tomorrow 07:00", wait: "PT1H" }
  - { id: "plan", kind: ai, op: "draft", label: "Plan a \"special\" loaf \\ batch", in: bakery }
  - { id: notes, kind: context, label: "Notes", at: "notes/bread", in: bakery }
lines:
  - { from: morning, to: "plan", kind: "start" }
  - { from: plan, to: notes, kind: data, label: "yes" }
`)
	platform := write(t, "platform.yaml", `# It runs in the bakery office.
awml: "0.3"
title: Tomorrow's loaves
view: platform
groups:
  - { id: laptop, label: "Laptop: always on" }
parts:
  - { id: morning, kind: trigger, label: "Tomorrow 07:00", wait: PT1H }
  - { id: "plan", kind: ai, op: draft, label: "Plan a \"special\" loaf \\ batch", in: laptop }
  - { id: notes, kind: context, label: "Notes", at: notes/bread, in: laptop }
  - { id: key, kind: comment, label: "The key: read only", in: laptop }
lines:
  - { from: morning, to: plan, kind: start }
  - { from: plan, to: notes, kind: data, label: "yes" }
  - { from: key, to: plan, kind: tie }
`)
	r := awmlRun("convert", "--implementation", "office", security, platform)
	want := `# It runs in the bakery office.
awml: "0.9"
title: Tomorrow's loaves
description: |
  A bakery plans tomorrow's loaves. The bakery is fictional.
scopes:
  - { id: bakery, label: "bakery: Jo, Kit", processing: { anthropic: any } }
parts:
  - { id: morning, kind: trigger, label: "Tomorrow 07:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan a \"special\" loaf \\ batch", in: bakery }
  - { id: notes, kind: context, label: "Notes", at: notes/bread, in: bakery }
lines:
  - { from: morning, to: plan, kind: start }
  - { from: plan, to: notes, kind: data, label: "yes" }
implementations:
  office:
    label: Default
    blocks:
      - { id: laptop, label: "Laptop: always on", parts: [plan, notes] }
    comments:
      - { id: key, label: "The key: read only", in: laptop, ties: [plan] }
`
	if r != (result{0, want, ""}) {
		t.Errorf("convert gave %+v\nwant stdout:\n%s", r, want)
	}
}

func TestTheOutputMayComeFirstOrBeJoinedToItsFlag(t *testing.T) {
	a := write(t, "a.yaml", good)
	dir := t.TempDir()
	for _, args := range [][]string{{"-o", filepath.Join(dir, "1.html"), a}, {"-o" + filepath.Join(dir, "2.html"), a}, {"--out=" + filepath.Join(dir, "3.html"), a}} {
		if r := awmlRun(append([]string{"page"}, args...)...); r.code != 0 {
			t.Errorf("page %q gave %+v", args, r)
		}
	}
	for _, n := range []string{"1", "2", "3"} {
		if _, err := os.Stat(filepath.Join(dir, n+".html")); err != nil {
			t.Errorf("no %s.html", n)
		}
	}
}

func TestHelpIsPrintedAndSucceeds(t *testing.T) {
	r := awmlRun("-h")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml [-h] [--version] [--registry URL] {draw,graph,page,check,fit,summary,convert,login,push,pull} ...\n\nDraw agentic workflows written in AWML.\n") || !strings.Contains(r.stdout, "  --version          show program's version number and exit\n") {
		t.Errorf("-h gave %+v", r)
	}
	if r := awmlRun("page", "--help"); r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml page [-h] -o OUT files [files ...]\n") {
		t.Errorf("page --help gave %+v", r)
	}
}

func TestHelpListsEachCommandsDescriptionInOneColumn(t *testing.T) {
	_, list, _ := strings.Cut(awmlRun("-h").stdout, "positional arguments:\n")
	list, _, _ = strings.Cut(list, "\n\n")
	column := map[int][]string{}
	for _, line := range strings.Split(list, "\n")[1:] {
		name := strings.Fields(line)[0]
		after := strings.TrimLeft(line[strings.Index(line, name)+len(name):], " ")
		column[len(line)-len(after)] = append(column[len(line)-len(after)], name)
	}
	if len(column) != 1 {
		t.Errorf("the commands' descriptions start in different columns: %v", column)
	}
}

func TestVersionPrintsTheReleaseVersion(t *testing.T) {
	previous := version
	version = "0.3.1"
	t.Cleanup(func() { version = previous })

	if r := awmlRun("--version"); r != (result{0, "awml 0.3.1\n", ""}) {
		t.Errorf("--version gave %+v", r)
	}
}

func TestVersionFallsBackToDev(t *testing.T) {
	previous := version
	version = ""
	t.Cleanup(func() { version = previous })

	if r := awmlRun("--version"); r != (result{0, "awml dev\n", ""}) {
		t.Errorf("--version gave %+v", r)
	}
}

func TestAMistakeInTheCommandShowsHowToUseIt(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "usage: awml [-h] [--version] [--registry URL] {draw,graph,page,check,fit,summary,convert,login,push,pull} ...\nawml: error: the following arguments are required: command\n"},
		{[]string{"drew", "a.yaml"}, "usage: awml [-h] [--version] [--registry URL] {draw,graph,page,check,fit,summary,convert,login,push,pull} ...\nawml: error: argument command: invalid choice: 'drew' (choose from 'draw', 'graph', 'page', 'check', 'fit', 'summary', 'convert', 'login', 'push', 'pull')\n"},
		{[]string{"draw"}, "usage: awml draw [-h] [--implementation ID] files [files ...]\nawml draw: error: the following arguments are required: files\n"},
		{[]string{"check"}, "usage: awml check [-h] [--json] files [files ...]\nawml check: error: the following arguments are required: files\n"},
		{[]string{"page", "a.yaml"}, "usage: awml page [-h] -o OUT files [files ...]\nawml page: error: the following arguments are required: -o/--out\n"},
		{[]string{"page", "a.yaml", "-o"}, "usage: awml page [-h] -o OUT files [files ...]\nawml page: error: argument -o/--out: expected one argument\n"},
		{[]string{"page", "-o", "out.html"}, "usage: awml page [-h] -o OUT files [files ...]\nawml page: error: the following arguments are required: files\n"},
	} {
		if r := awmlRun(c.args...); r != (result{2, "", c.want}) {
			t.Errorf("awml %q gave %+v", c.args, r)
		}
	}
}

func TestADoubleDashEndsTheFlags(t *testing.T) {
	r := awmlRun("check", "--", "-a.yaml")
	if r.code != 1 || r.stdout != "-a.yaml: can't be read: No such file or directory\n" {
		t.Errorf("check gave %+v", r)
	}
}

func TestLoginWritesCredentialsWithMode0600(t *testing.T) {
	config := useTemporaryHome(t)
	r := awmlRunInput("secret-token\n", "login", "--registry", "https://registry.example/")
	if r != (result{0, "Logged in to https://registry.example\n", ""}) {
		t.Fatalf("login gave %+v", r)
	}
	path := filepath.Join(config, "awml", "credentials")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("credentials mode = %04o, want 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\n  \"https://registry.example\": \"secret-token\"\n}\n" {
		t.Errorf("credentials = %s", data)
	}
}

func TestPushRefusesABadFileWithoutAnHTTPCall(t *testing.T) {
	useTemporaryHome(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	r := awmlRun("push", "acme/mail", write(t, "bad.yaml", bad), "--registry", server.URL)
	if r.code != 1 || !strings.Contains(r.stderr, `ends at unknown part "plann"`) {
		t.Fatalf("push gave %+v", r)
	}
	if calls != 0 {
		t.Errorf("server received %d calls", calls)
	}
}

func TestPushChecksThenSendsTheWorkflowAndToken(t *testing.T) {
	useTemporaryHome(t)
	var gotPath, gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"owner":"acme","slug":"mail","number":3,"url":"/acme/mail/v/3"}`)
	}))
	defer server.Close()
	loginForTest(t, server.URL, "push-token")
	workflow := write(t, "workflow.yaml", canonicalFile)
	r := awmlRun("push", "acme/mail", workflow, "--private", "--registry", server.URL)
	if r != (result{0, "Pushed acme/mail version 3: /acme/mail/v/3\n", ""}) {
		t.Fatalf("push gave %+v", r)
	}
	if gotPath != "/api/v1/acme/mail/versions" || gotAuth != "Bearer push-token" {
		t.Errorf("request path/auth = %q, %q", gotPath, gotAuth)
	}
	wantBody := map[string]any{"workflow": canonicalFile, "visibility": "private"}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %#v", gotBody)
	}
}

// The registry stores a drawing with errors, so push sends one.
func TestPushSendsADrawingWithErrors(t *testing.T) {
	useTemporaryHome(t)
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"owner":"acme","slug":"files","number":1,"url":"/acme/files/v/1"}`)
	}))
	defer server.Close()
	loginForTest(t, server.URL, "push-token")
	r := awmlRun("push", "acme/files", write(t, "live.yaml", live), "--registry", server.URL)
	if r != (result{0, "Pushed acme/files version 1: /acme/files/v/1\n", ""}) || gotBody["workflow"] != live {
		t.Errorf("push gave %+v and sent %#v", r, gotBody)
	}
}

func TestPushRefusesOlderOrMultipleFilesWithoutAnHTTPCall(t *testing.T) {
	useTemporaryHome(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	security := write(t, "security.yaml", legacySecurity)
	platform := write(t, "platform.yaml", legacyPlatform)
	older := write(t, "older.yaml", strings.Replace(canonicalFile, `awml: "0.9"`, `awml: "0.5"`, 1))

	for _, files := range [][]string{{security}, {older}, {security, platform}} {
		args := append([]string{"push", "acme/mail"}, files...)
		args = append(args, "--registry", server.URL)
		r := awmlRun(args...)
		if r.code != 1 || !strings.Contains(r.stderr, "awml convert") {
			t.Errorf("push %v gave %+v", files, r)
		}
	}
	if r := awmlRun("push", "acme/mail", write(t, "worker.yaml", goodPlatformForCommand), "--registry", server.URL); r.code != 1 || !strings.Contains(r.stderr, "push one 0.9 workflow") {
		t.Errorf("push of a platform description gave %+v", r)
	}
	if calls != 0 {
		t.Errorf("server received %d calls", calls)
	}
}

func TestPushPrints422Errors(t *testing.T) {
	useTemporaryHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"errors":["workflow: a mistake"]}`)
	}))
	defer server.Close()
	loginForTest(t, server.URL, "token")
	r := awmlRun("push", "acme/mail", write(t, "workflow.yaml", canonicalFile), "--registry", server.URL)
	want := "workflow: a mistake\n"
	if r != (result{1, "", want}) {
		t.Errorf("push gave %+v", r)
	}
}

func TestPullDownloadsAnExplicitVersion(t *testing.T) {
	useTemporaryHome(t)
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/v1/acme/mail/v/2" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprintf(w, `{"number":2,"files":{"security":%q,"platform":%q}}`, legacySecurity, legacyPlatform)
	}))
	defer server.Close()
	loginForTest(t, server.URL, "pull-token")
	out := filepath.Join(t.TempDir(), "download")
	r := awmlRun("pull", "acme/mail@2", "-o", out, "--registry", server.URL)
	if r != (result{0, "Pulled acme/mail version 2 to " + out + "\n", ""}) {
		t.Fatalf("pull gave %+v", r)
	}
	if gotAuth != "Bearer pull-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	for name, want := range map[string]string{"security.yaml": legacySecurity, "platform.yaml": legacyPlatform} {
		got, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v", name, got, err)
		}
	}
}

func TestPullFailsWhenTheVersionHasNoFiles(t *testing.T) {
	useTemporaryHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"number":3,"files":{}}`)
	}))
	defer server.Close()
	out := filepath.Join(t.TempDir(), "download")
	r := awmlRun("pull", "acme/mail@3", "-o", out, "--registry", server.URL)
	if r != (result{1, "", "awml pull: version 3 of acme/mail came with no files\n"}) {
		t.Fatalf("pull gave %+v", r)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("pull made %s, err %v", out, err)
	}
}

func TestPullFindsTheLatestVersion(t *testing.T) {
	useTemporaryHome(t)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/v/4") {
			fmt.Fprintf(w, `{"number":4,"files":{"security":%q}}`, good)
			return
		}
		fmt.Fprint(w, `{"versions":[{"number":2},{"number":4},{"number":3}]}`)
	}))
	defer server.Close()
	out := filepath.Join(t.TempDir(), "download")
	r := awmlRun("pull", "acme/mail", "-o", out, "--registry", server.URL)
	if r.code != 0 {
		t.Fatalf("pull gave %+v", r)
	}
	wantPaths := []string{"/api/v1/acme/mail", "/api/v1/acme/mail/v/4"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Errorf("paths = %q, want %q", paths, wantPaths)
	}
}

func TestRegistryFlagIsHonoured(t *testing.T) {
	useTemporaryHome(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprintf(w, `{"number":1,"files":{"security":%q}}`, good)
	}))
	defer server.Close()
	out := filepath.Join(t.TempDir(), "out")
	if r := awmlRun("--registry", server.URL, "pull", "acme/mail@1", "-o", out); r.code != 0 {
		t.Fatalf("pull gave %+v", r)
	}
	if calls != 1 {
		t.Errorf("registry got %d calls", calls)
	}
}

// An admin points everyone at their own registry with AWML_REGISTRY_URL, so login, push and pull
// all use it without --registry.
func TestAWMLRegistryURLEnvironmentIsHonoured(t *testing.T) {
	useTemporaryHome(t)
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"owner":"acme","slug":"mail","number":1,"url":"/acme/mail/v/1"}`)
			return
		}
		fmt.Fprintf(w, `{"number":1,"files":{"workflow":%q}}`, canonicalFile)
	}))
	defer server.Close()
	t.Setenv("AWML_REGISTRY_URL", server.URL+"/")
	if r := awmlRunInput("env-token\n", "login"); r != (result{0, "Logged in to " + server.URL + "\n", ""}) {
		t.Fatalf("login gave %+v", r)
	}
	if r := awmlRun("push", "acme/mail", write(t, "workflow.yaml", canonicalFile)); r.code != 0 {
		t.Fatalf("push gave %+v", r)
	}
	if r := awmlRun("pull", "acme/mail@1", "-o", filepath.Join(t.TempDir(), "out")); r.code != 0 {
		t.Fatalf("pull gave %+v", r)
	}
	want := []string{"POST /api/v1/acme/mail/versions Bearer env-token", "GET /api/v1/acme/mail/v/1 Bearer env-token"}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("registry got %q, want %q", calls, want)
	}
}

func TestTheRegistryFlagWinsOverAWMLRegistryURL(t *testing.T) {
	useTemporaryHome(t)
	t.Setenv("AWML_REGISTRY_URL", "https://registry.example")
	r := awmlRunInput("token\n", "login", "--registry", "https://other.example")
	if r != (result{0, "Logged in to https://other.example\n", ""}) {
		t.Fatalf("login gave %+v", r)
	}
}

func useTemporaryHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("AWML_REGISTRY_URL", "")
	return config
}

func loginForTest(t *testing.T, registry, token string) {
	t.Helper()
	if r := awmlRunInput(token+"\n", "login", "--registry", registry); r.code != 0 {
		t.Fatalf("login gave %+v", r)
	}
}

const canonicalFile = `# Bakery workflow
awml: "0.9"
title: Bread
scopes: [{id: bakery, label: Bakery, processing: any}]
parts:
  - {id: plan, kind: ai, label: Plan, in: bakery}
implementations:
  home:
    label: At home
    blocks: [{id: server, label: Server, platform: scripts, parts: [plan]}]
`

func TestCanonicalCommands(t *testing.T) {
	path := write(t, "bread.yaml", canonicalFile)
	desc := write(t, "scripts.yaml", `awml: "0.9"
platform: {id: scripts, label: Scripts}
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: []
`)
	for _, args := range [][]string{{"check", path}, {"draw", path}, {"draw", "--implementation", "home", path}, {"fit", path, desc}, {"fit", "--implementation=home", path, desc}, {"summary", "--json", path}} {
		got := awmlRun(args...)
		if got.code != 0 {
			t.Fatalf("%v: %+v", args, got)
		}
	}
	for _, command := range []string{"draw", "fit"} {
		args := []string{command, "--implementation", "missing", path}
		if command == "fit" {
			args = append(args, desc)
		}
		got := awmlRun(args...)
		if got.code != 1 || !strings.Contains(got.stderr, `unknown implementation "missing"`) {
			t.Fatalf("%+v", got)
		}
	}
	page := filepath.Join(t.TempDir(), "bread.html")
	if got := awmlRun("page", path, "-o", page); got.code != 0 {
		t.Fatal(got)
	}
	data, _ := os.ReadFile(page)
	if !strings.Contains(string(data), "At home") {
		t.Fatal("missing implementation heading")
	}
}

func TestFitAnImplementationWithoutDescriptions(t *testing.T) {
	path := write(t, "bread.yaml", canonicalFile)
	if got := awmlRun("fit", "--implementation", "home", path); got != (result{0, `server  not judged: platform "scripts" wasn't described` + "\n", ""}) {
		t.Errorf("fit without descriptions gave %+v", got)
	}
	want := `{"results":[{"platform":"server","judged":false,"fits":false,"reasons":["platform \"scripts\" wasn't described"],"models":{}}],"gaps":[]}` + "\n"
	if got := awmlRun("fit", "--json", "--implementation", "home", path); got != (result{0, want, ""}) {
		t.Errorf("fit --json without descriptions gave %+v", got)
	}
	if got := awmlRun("fit", path); got != (result{2, "", "usage: awml fit [-h] [--json] [--parts] [--implementation ID] files [files ...]\nawml fit: error: no platform descriptions were given\n"}) {
		t.Errorf("design fit without descriptions gave %+v", got)
	}
}

func TestConvertCommand(t *testing.T) {
	security := write(t, "security.yaml", "# Keep this comment\n"+legacySecurity)
	platform := write(t, "platform.yaml", legacyPlatform)
	got := awmlRun("convert", security, platform)
	if got.code != 0 || !strings.Contains(got.stdout, "\ndescription: |\n  Keep this comment\n") || !strings.Contains(got.stdout, `awml: "0.9"`) {
		t.Fatalf("%+v", got)
	}
	parsed, err := awml.Parse([]byte(got.stdout))
	if err != nil || len(awml.Check(parsed)) > 0 {
		t.Fatalf("%v %v", err, awml.Check(parsed))
	}
	platform = write(t, "bad-pair.yaml", strings.Replace(legacyPlatform, "kind: ai", "kind: code", 1))
	got = awmlRun("convert", security, platform)
	if got.code != 1 || !strings.Contains(got.stderr, `has kind "code"`) {
		t.Fatalf("%+v", got)
	}
}

func TestCanonicalPushAndPull(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"number":1,"url":"http://example.test/bread"}`)
		} else {
			json.NewEncoder(w).Encode(map[string]any{"number": 1, "files": map[string]string{"workflow": canonicalFile}})
		}
	}))
	defer server.Close()
	if err := writeCredentials(map[string]string{server.URL: "test-token"}); err != nil {
		t.Fatal(err)
	}
	path := write(t, "bread.yaml", canonicalFile)
	if got := awmlRun("push", "--registry", server.URL, "bakery/bread", path); got.code != 0 {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(payload, map[string]any{"workflow": canonicalFile}) {
		t.Fatalf("payload = %#v", payload)
	}
	dir := t.TempDir()
	if got := awmlRun("pull", "--registry", server.URL, "bakery/bread@1", "-o", dir); got.code != 0 {
		t.Fatal(got)
	}
	source, err := os.ReadFile(filepath.Join(dir, "bread.yaml"))
	if err != nil || string(source) != canonicalFile {
		t.Fatalf("%s %v", source, err)
	}
	for _, name := range []string{"security.yaml", "platform.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s", name)
		}
	}
}

func TestConvertCollectsInlineComments(t *testing.T) {
	security := write(t, "security.yaml", strings.Replace(legacySecurity, "title: Dinners", "title: Dinners # Dinner plans", 1))
	platform := write(t, "platform.yaml", legacyPlatform)
	got := awmlRun("convert", security, platform)
	if got.code != 0 || !strings.HasPrefix(got.stdout, "# Dinner plans\n") {
		t.Fatalf("%+v", got)
	}
}

func TestPushRefusesACanonicalFileWithALegacyPlatformFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	canonical := write(t, "bread.yaml", canonicalFile)
	platform := write(t, "platform.yaml", legacyPlatform)
	got := awmlRun("push", "bakery/bread", canonical, platform)
	if got.code != 1 || !strings.Contains(got.stderr, "awml convert security.yaml platform.yaml > workflow.yaml") {
		t.Fatalf("%+v", got)
	}
}
