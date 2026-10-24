package awml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func checkResult(t *testing.T, report CheckReport, id string) CheckResult {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("no check %q in the report", id)
	return CheckResult{}
}

func source(file, text string) Source { return Source{File: file, Text: []byte(text)} }

const soundWorkflow = `awml: "0.6"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: plan, kind: ai, op: draft, label: "Plan next week's dinners", in: family }
  - { id: notes, kind: context, label: Notes, in: family, at: notes }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: plan, to: notes, kind: data }
implementations:
  home:
    label: At home
    blocks:
      - { id: server, label: Server, parts: [plan, notes] }
`

const soundDescription = `awml: "0.4"
platform: { id: scripts-server, label: Scripts on a server }
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: [notes]
`

func TestTheChecksKeepTheirIDsAndTheSpecsOrder(t *testing.T) {
	var ids []string
	for _, check := range CheckSources("dev", nil).Checks {
		ids = append(ids, check.ID)
	}
	want := []string{
		"file.yaml", "file.version", "fields.known", "kinds.known", "ids.unique", "ids.written",
		"values.text", "lines.breaks", "references.known", "nesting.acyclic", "comments.tie-only",
		"ties.comment", "ops.known", "operations.only-do-acts", "operations.answer-check-read-only",
		"tiers.written", "tiers.shape", "approvals.listed", "lines.data-narrows", "processing.written", "processing.nested", "processing.scopes-only",
		"platform.blocks-only", "wait.duration", "at.path", "untrusted.parts", "cleans.code-only", "placement.once", "placement.complete",
		"warn.do-by-code", "warn.one-change", "warn.untrusted-outside", "description.values", "description.ids",
		"views.same-parts", "views.same-lines",
	}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("the checks are\n  %s\nwant\n  %s", strings.Join(ids, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestEveryCheckSaysWhatItIsAndWhatItAppliesTo(t *testing.T) {
	for _, check := range CheckSources("dev", nil).Checks {
		if check.Name == "" || check.About == "" || len(check.AppliesTo) == 0 {
			t.Errorf("%s has name %q, about %q and applies to %v", check.ID, check.Name, check.About, check.AppliesTo)
		}
		if !strings.HasSuffix(check.About, ".") {
			t.Errorf("%s: about should be a sentence: %q", check.ID, check.About)
		}
	}
}

func TestASoundWorkflowPassesEveryCheckThatAppliesToIt(t *testing.T) {
	report := CheckSources("0.4.1", []Source{source("dinners.yaml", soundWorkflow)})
	if report.Engine != "0.4.1" {
		t.Errorf("engine is %q", report.Engine)
	}
	if !reflect.DeepEqual(report.Files, []CheckedFile{{File: "dinners.yaml", View: "workflow"}}) {
		t.Errorf("files are %+v", report.Files)
	}
	for _, check := range report.Checks {
		want := "pass"
		// A 0.6 workflow has no breaks.
		if strings.HasPrefix(check.ID, "description.") || strings.HasPrefix(check.ID, "views.") || check.ID == "lines.breaks" {
			want = "skipped"
		}
		if check.Result != want || len(check.Mistakes) != 0 {
			t.Errorf("%s gave %s with %+v, want %s", check.ID, check.Result, check.Mistakes, want)
		}
		if (check.Result == "skipped") != (check.SkippedBecause != "") {
			t.Errorf("%s is %s but says it was skipped because %q", check.ID, check.Result, check.SkippedBecause)
		}
	}
	if got := checkResult(t, report, "description.ids").SkippedBecause; got != "no file given is a platform description" {
		t.Errorf("description.ids was skipped because %q", got)
	}
}

func TestAMistakeFailsItsCheckWithTodaysMessage(t *testing.T) {
	src := strings.Replace(soundWorkflow, "to: notes", "to: note", 1)
	report := CheckSources("dev", []Source{source("dinners.yaml", src)})
	references := checkResult(t, report, "references.known")
	want := []FileMistake{{File: "dinners.yaml", Message: `the line from "plan" to "note" ends at unknown part "note"`}}
	if references.Result != "fail" || !reflect.DeepEqual(references.Mistakes, want) {
		t.Errorf("references.known gave %s with %+v", references.Result, references.Mistakes)
	}
	if kinds := checkResult(t, report, "kinds.known"); kinds.Result != "pass" {
		t.Errorf("kinds.known gave %s", kinds.Result)
	}
}

func TestWarningsRideInTheListAsWarn(t *testing.T) {
	src := strings.Replace(soundWorkflow, "kind: ai, op: draft", "kind: ai, op: do", 1)
	report := CheckSources("dev", []Source{source("dinners.yaml", src)})
	check := checkResult(t, report, "warn.do-by-code")
	want := []FileMistake{{File: "dinners.yaml", Message: Warnings(parsed(t, src))[0]}}
	if check.Result != "warn" || !reflect.DeepEqual(check.Mistakes, want) {
		t.Errorf("warn.do-by-code gave %s with %+v", check.Result, check.Mistakes)
	}
	if one := checkResult(t, report, "warn.one-change"); one.Result != "pass" {
		t.Errorf("warn.one-change gave %s", one.Result)
	}
}

func TestAFileThatIsntYAMLRunsNoOtherCheck(t *testing.T) {
	report := CheckSources("dev", []Source{
		source("broken.yaml", "awml: [\n"),
		{File: "missing.yaml", Unreadable: "can't be read: No such file or directory"},
	})
	if !reflect.DeepEqual(report.Files, []CheckedFile{{File: "broken.yaml"}, {File: "missing.yaml"}}) {
		t.Errorf("files are %+v", report.Files)
	}
	yaml := checkResult(t, report, "file.yaml")
	if yaml.Result != "fail" || len(yaml.Mistakes) != 2 || !strings.HasPrefix(yaml.Mistakes[0].Message, "isn't YAML: ") ||
		yaml.Mistakes[1] != (FileMistake{"missing.yaml", "can't be read: No such file or directory"}) {
		t.Errorf("file.yaml gave %s with %+v", yaml.Result, yaml.Mistakes)
	}
	if fields := checkResult(t, report, "fields.known"); fields.Result != "skipped" || fields.SkippedBecause != "no file given could be read as YAML" {
		t.Errorf("fields.known gave %s because %q", fields.Result, fields.SkippedBecause)
	}
}

func TestAChecksStoppedByAnEarlierMistakeIsSkipped(t *testing.T) {
	src := "awml: \"0.3\"\ntitle: Broken\nview: security\nparts: [nonsense]\n"
	report := CheckSources("dev", []Source{source("broken.yaml", src)})
	if fields := checkResult(t, report, "fields.known"); fields.Result != "fail" {
		t.Errorf("fields.known gave %s", fields.Result)
	}
	kinds := checkResult(t, report, "kinds.known")
	if kinds.Result != "skipped" || kinds.SkippedBecause != "an earlier mistake stopped it in every file it applies to" {
		t.Errorf("kinds.known gave %s because %q", kinds.Result, kinds.SkippedBecause)
	}
}

func TestChecksFor03FieldsDontApplyToA02File(t *testing.T) {
	src := `awml: "0.2"
title: Dinners
view: security
parts:
  - { id: thu, kind: trigger, label: Thursday }
`
	report := CheckSources("dev", []Source{source("old.yaml", src)})
	if wait := checkResult(t, report, "wait.duration"); wait.Result != "skipped" {
		t.Errorf("wait.duration gave %s", wait.Result)
	}
	if scope := checkResult(t, report, "lines.data-narrows"); scope.Result != "pass" {
		t.Errorf("lines.data-narrows gave %s", scope.Result)
	}
	if !reflect.DeepEqual(report.Files, []CheckedFile{{File: "old.yaml", View: "security"}}) {
		t.Errorf("files are %+v", report.Files)
	}
}

func TestOnlyAFileBefore06HasBreaks(t *testing.T) {
	report := CheckSources("dev", []Source{source("live.yaml", fridaySent)})
	if breaks := checkResult(t, report, "lines.breaks"); breaks.Result != "skipped" || breaks.SkippedBecause != "no file given is a workflow or view before 0.6" {
		t.Errorf("lines.breaks gave %s because %q", breaks.Result, breaks.SkippedBecause)
	}
	report = CheckSources("dev", []Source{source("old.yaml", good)})
	if breaks := checkResult(t, report, "lines.breaks"); breaks.Result != "pass" {
		t.Errorf("lines.breaks gave %s", breaks.Result)
	}
}

func TestAPlatformDescriptionIsCheckedOnItsOwnTerms(t *testing.T) {
	src := strings.Replace(soundDescription, "holds: [notes]", "holds: [/notes]", 1)
	report := CheckSources("dev", []Source{source("server.yaml", src)})
	if !reflect.DeepEqual(report.Files, []CheckedFile{{File: "server.yaml", View: "platform-description"}}) {
		t.Errorf("files are %+v", report.Files)
	}
	ids := checkResult(t, report, "description.ids")
	if ids.Result != "fail" || !reflect.DeepEqual(ids.Mistakes, []FileMistake{{"server.yaml", `holds item "/notes" must be a path`}}) {
		t.Errorf("description.ids gave %s with %+v", ids.Result, ids.Mistakes)
	}
	if values := checkResult(t, report, "description.values"); values.Result != "pass" {
		t.Errorf("description.values gave %s", values.Result)
	}
	if kinds := checkResult(t, report, "kinds.known"); kinds.Result != "skipped" {
		t.Errorf("kinds.known gave %s", kinds.Result)
	}
}

func TestTwoViewsWithTheSameTitleAreCompared(t *testing.T) {
	security := `awml: "0.3"
title: Dinners
view: security
parts:
  - { id: thu, kind: trigger, label: Thursday }
  - { id: plan, kind: ai, label: Plan }
lines:
  - { from: thu, to: plan, kind: start }
`
	platform := strings.Replace(strings.Replace(security, "view: security", "view: platform", 1), "kind: ai", "kind: code", 1)
	report := CheckSources("dev", []Source{source("s.yaml", security), source("p.yaml", platform)})
	parts := checkResult(t, report, "views.same-parts")
	want := []FileMistake{{"s.yaml and p.yaml", `part "plan" has kind "code" in the platform view but "ai" in the security view`}}
	if parts.Result != "fail" || !reflect.DeepEqual(parts.Mistakes, want) {
		t.Errorf("views.same-parts gave %s with %+v", parts.Result, parts.Mistakes)
	}
	if lines := checkResult(t, report, "views.same-lines"); lines.Result != "pass" {
		t.Errorf("views.same-lines gave %s", lines.Result)
	}

	alone := CheckSources("dev", []Source{source("s.yaml", security)})
	if lines := checkResult(t, alone, "views.same-lines"); lines.Result != "skipped" || lines.SkippedBecause != "no sound security view and platform view share a title" {
		t.Errorf("views.same-lines gave %s because %q", lines.Result, lines.SkippedBecause)
	}
}

func TestTheReportsJSONUsesTheIssuesNames(t *testing.T) {
	src := strings.Replace(soundWorkflow, "to: notes", "to: note", 1)
	encoded, err := json.Marshal(CheckSources("dev", []Source{source("dinners.yaml", src)}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"engine":"dev","files":[{"file":"dinners.yaml","view":"workflow"}],"checks":[{"id":"file.yaml","name":`,
		`"applies_to":["workflow","security","platform","platform-description"],"result":"pass","mistakes":[]}`,
		`"result":"fail","mistakes":[{"file":"dinners.yaml","message":"the line from \"plan\" to \"note\" ends at unknown part \"note\""}]}`,
		`"result":"skipped","skipped_because":"no file given is a platform description","mistakes":[]}`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the JSON lacks %s:\n%s", want, encoded)
		}
	}
}

// Every mistake Check finds is filed under one check, word for word, so the JSON loses nothing the
// plain output says.
func TestEveryMistakeIsFiledUnderACheck(t *testing.T) {
	sources := map[string]string{
		"sound":       soundWorkflow,
		"description": soundDescription,
		"bad description": `awml: 3
platform: { id: Server, label: 4 }
off_for_up_to: P1M
steps: robots
agents: [Anthropic, 3]
calls: some
holds: [/x, 4]
extra: 1
`,
		"not a mapping": "- one\n- two\n",
		"bad version":   "awml: \"0.1\"\ntitle: x\nview: sideways\nparts: []\n",
		"broken lists":  "awml: \"0.3\"\ntitle: x\nview: security\nparts: 3\nlines: [3]\n",
		"legacy": `awml: "0.3"
title: Legacy
view: security
groups:
  - { id: a, label: A, in: b, platform: Pc, processing: { anthropic: [fly], 3: any, Bad: soon } }
  - { id: b, label: B, in: a, processing: { anthropic: [sort] } }
  - { id: c, label: C, in: b, processing: any }
  - { id: 1x, label: X, extra: 1 }
parts:
  - { id: t, kind: trigger, label: T, wait: soon, in: nowhere }
  - { id: s, kind: ai, op: answer, label: S, in: a, at: /x }
  - { id: s, kind: robot, label: 4 }
  - { id: d, kind: code, op: draft, label: D, wait: PT1M }
  - { id: o, kind: outside, label: "O: Ask", op: do }
  - { id: n, kind: context, label: N, in: c }
  - { id: m, kind: comment, label: M }
  - { id: f, kind: fork, op: fly }
lines:
  - { from: t, to: s, kind: start, breaks: maybe }
  - { from: s, to: n, kind: data }
  - { from: d, to: o, kind: data }
  - { from: m, to: s, kind: data }
  - { from: t, to: d, kind: tie }
  - { from: t, to: zz, kind: sideways }
  - { to: t }
`,
		"operations": `awml: "0.3"
title: Operations
view: security
groups:
  - { id: a, label: A }
parts:
  - { id: s, kind: ai, op: answer, label: S, in: a }
  - { id: n, kind: context, label: N }
  - { id: d, kind: ai, op: do, label: D }
  - { id: o, kind: outside, label: "O: Ask" }
  - { id: p, kind: surface, label: P }
lines:
  - { from: s, to: n, kind: data }
  - { from: d, to: o, kind: data }
  - { from: d, to: p, kind: data }
`,
		"platform view": `awml: "0.3"
title: Legacy
view: platform
groups:
  - { id: box, label: Box, processing: any, platform: 3 }
parts:
  - { id: t, kind: trigger, label: T, in: box }
`,
		"canonical": `awml: "0.4"
title: Canonical
extra: 1
scopes:
  - { id: family, label: Family, platform: x }
parts:
  - { id: plan, kind: ai, label: Plan, in: family }
  - { id: notes, kind: context, label: Notes, in: family }
implementations:
  1bad: 3
  home:
    label: Home
    blocks:
      - { id: server, label: Server, parts: [plan, plan, 3, ghost], processing: any }
      - { id: family, label: Again, parts: oops }
    comments:
      - { id: c, label: C, in: "", ties: [ghost, 4], extra: 1 }
      - { id: d, label: D, in: server, ties: oops }
  away:
    label: Away
    blocks: 3
`,
		"tiers": `awml: "0.5"
title: Tiers
parts:
  - { id: d, kind: code, op: do, label: D }
  - { id: ok, kind: approval, label: OK }
  - { id: o, kind: outside, label: O }
  - { id: a, kind: outside, label: "A: Allow" }
  - { id: k, kind: outside, label: "K: Ask" }
  - { id: b, kind: outside, label: "B: Block" }
lines:
  - { from: d, to: ok, kind: data }
  - { from: ok, to: a, kind: data }
  - { from: d, to: k, kind: data }
  - { from: d, to: b, kind: data }
  - { from: d, to: o, kind: data }
`,
		"untrusted": `awml: "0.6"
title: Untrusted
parts:
  - { id: form, kind: trigger, label: Form, untrusted: true }
  - { id: read, kind: ai, label: Read, untrusted: true, cleans: true }
  - { id: keep, kind: code, label: Keep, cleans: "yes" }
  - { id: d, kind: code, op: do, label: D }
  - { id: a, kind: outside, label: "A: Allow" }
lines:
  - { from: form, to: read, kind: start }
  - { from: read, to: d, kind: start }
  - { from: d, to: a, kind: start }
`,
		"canonical lists":           "awml: \"0.4\"\ntitle: x\nparts: [3]\nimplementations: 3\n",
		"canonical implementations": "awml: \"0.4\"\ntitle: x\nparts: []\nimplementations: 3\n",
	}
	examples, _ := filepath.Glob("examples/*.yaml")
	more, _ := filepath.Glob("examples/*/*.yaml")
	for _, path := range append(examples, more...) {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(text)
	}
	failed := map[string]bool{}
	for name, text := range sources {
		doc, err := Parse([]byte(text))
		if err != nil {
			t.Fatalf("%s: %s", name, err)
		}
		want := append(append([]string{}, Check(doc)...), Warnings(doc)...)
		got := []string{}
		for _, check := range CheckSources("dev", []Source{source(name, text)}).Checks {
			for _, mistake := range check.Mistakes {
				got = append(got, mistake.Message)
				failed[check.ID] = true
			}
		}
		sort.Strings(want)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the checks hold\n  %s\nbut Check and Warnings give\n  %s", name, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	}
	// Two views and a file that isn't YAML are tested on their own above.
	for _, check := range CheckSources("dev", nil).Checks {
		if !failed[check.ID] && check.ID != "file.yaml" && !strings.HasPrefix(check.ID, "views.") {
			t.Errorf("no source here fails %s", check.ID)
		}
	}
}
