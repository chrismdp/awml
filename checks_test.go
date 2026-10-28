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

const soundWorkflow = `awml: "0.9"
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

const soundDescription = `awml: "0.9"
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
		"values.text", "references.known", "nesting.acyclic", "comments.tie-only",
		"ties.comment", "triggers.no-line-in", "contexts.written-by-steps", "ops.known", "operations.only-do-acts", "operations.answer-check-read-only",
		"tiers.written", "tiers.shape", "approvals.listed", "processing.written", "processing.nested",
		"platform.blocks-only", "wait.duration", "at.path", "untrusted.parts", "cleans.code-only", "untrusted.outside", "warn.untrusted-surface", "refs.written", "refs.parts", "placement.once", "placement.complete",
		"refs.known", "refs.agree", "refs.starts", "refs.owner",
		"warn.do-by-code", "warn.one-change",
		"warn.several-workflows", "warn.unlinked-part", "warn.untied-comment", "warn.unstarted-step", "warn.shared-writers", "warn.shared-readers",
		"description.values", "description.ids",
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
		if strings.HasPrefix(check.ID, "description.") {
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
	src := "awml: \"0.9\"\ntitle: Broken\nparts: [nonsense]\n"
	report := CheckSources("dev", []Source{source("broken.yaml", src)})
	if fields := checkResult(t, report, "fields.known"); fields.Result != "fail" {
		t.Errorf("fields.known gave %s", fields.Result)
	}
	kinds := checkResult(t, report, "kinds.known")
	if kinds.Result != "skipped" || kinds.SkippedBecause != "an earlier mistake stopped it in every file it applies to" {
		t.Errorf("kinds.known gave %s because %q", kinds.Result, kinds.SkippedBecause)
	}
}

func TestAnOlderFileFailsOnlyItsVersion(t *testing.T) {
	src := `awml: "0.2"
title: Dinners
view: security
parts:
  - { id: thu, kind: trigger, label: Thursday }
`
	report := CheckSources("dev", []Source{source("old.yaml", src)})
	if !reflect.DeepEqual(report.Files, []CheckedFile{{File: "old.yaml"}}) {
		t.Errorf("files are %+v", report.Files)
	}
	version := checkResult(t, report, "file.version")
	want := []FileMistake{{"old.yaml", `awml is "0.2", which this release no longer reads: upgrade the file with awml convert`}}
	if version.Result != "fail" || !reflect.DeepEqual(version.Mistakes, want) {
		t.Errorf("file.version gave %s with %+v", version.Result, version.Mistakes)
	}
	if yaml := checkResult(t, report, "file.yaml"); yaml.Result != "pass" {
		t.Errorf("file.yaml gave %s", yaml.Result)
	}
	if kinds := checkResult(t, report, "kinds.known"); kinds.Result != "skipped" || kinds.SkippedBecause != "no file given is a workflow" {
		t.Errorf("kinds.known gave %s because %q", kinds.Result, kinds.SkippedBecause)
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

func TestTheReportsJSONUsesTheIssuesNames(t *testing.T) {
	src := strings.Replace(soundWorkflow, "to: notes", "to: note", 1)
	encoded, err := json.Marshal(CheckSources("dev", []Source{source("dinners.yaml", src)}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"engine":"dev","files":[{"file":"dinners.yaml","view":"workflow"}],"checks":[{"id":"file.yaml","name":`,
		`"applies_to":["workflow","platform-description"],"result":"pass","mistakes":[]}`,
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
		"bad description": `awml: "0.9"
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
		"older version": "awml: \"0.5\"\ntitle: x\nparts: []\n",
		"broken lists":  "awml: \"0.9\"\ntitle: x\nparts: 3\nlines: [3]\n",
		"design": `awml: "0.9"
title: Design
scopes:
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
  - { id: f, kind: surface, label: F, op: fly }
  - { id: r, kind: ai, label: R, ref: x.yaml#y }
  - { id: q, kind: context, label: Q, ref: nope }
lines:
  - { from: t, to: s, kind: start, breaks: maybe }
  - { from: s, to: n, kind: data }
  - { from: d, to: o, kind: data }
  - { from: m, to: s, kind: data }
  - { from: t, to: d, kind: tie }
  - { from: t, to: zz, kind: sideways }
  - { to: t }
  - { from: n, to: t, kind: data }
  - { from: o, to: n, kind: data }
`,
		"operations": `awml: "0.9"
title: Operations
scopes:
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
		"canonical": `awml: "0.9"
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
      - { id: server, label: Server, platform: Pc, parts: [plan, plan, 3, ghost], processing: any }
      - { id: family, label: Again, parts: oops }
    comments:
      - { id: c, label: C, in: "", ties: [ghost, 4], extra: 1 }
      - { id: d, label: D, in: server, ties: oops }
  away:
    label: Away
    blocks: 3
`,
		"tiers": `awml: "0.9"
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
		"untrusted": `awml: "0.9"
title: Untrusted
parts:
  - { id: form, kind: trigger, label: Form, untrusted: true }
  - { id: read, kind: ai, label: Read, untrusted: true, cleans: true }
  - { id: keep, kind: code, label: Keep, cleans: "yes" }
  - { id: d, kind: code, op: do, label: D }
  - { id: a, kind: outside, label: "A: Allow" }
  - { id: s, kind: surface, label: S }
lines:
  - { from: form, to: read, kind: start }
  - { from: read, to: d, kind: start }
  - { from: d, to: a, kind: start }
  - { from: read, to: s, kind: data }
`,
		"canonical lists":           "awml: \"0.9\"\ntitle: x\nparts: [3]\nimplementations: 3\n",
		"canonical implementations": "awml: \"0.9\"\ntitle: x\nparts: []\nimplementations: 3\n",
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
	// The checks that follow refs between workflows find mistakes only in files given together, and
	// file each word for word as awml check prints it.
	planMore := strings.Replace(planWorkflow, "  - { id: list,", "  - { id: inbox, kind: connector, label: Inbox, in: family }\n  - { id: list,", 1)
	planMore = strings.Replace(planMore, "lines:\n", "lines:\n  - { from: inbox, to: plan, kind: data }\n", 1)
	shopMore := strings.Replace(shopWorkflow, "  - { from: fill, to: basket", "  - { from: fill, to: list, kind: data }\n  - { from: fill, to: basket", 1)
	shopMore = strings.Replace(shopMore, "  - { id: fill,", "  - { id: inbox, kind: connector, label: Inbox, in: home, ref: plan.yaml#inbox }\n  - { id: fill,", 1)
	shopMore = strings.Replace(shopMore, "lines:\n", "lines:\n  - { from: inbox, to: fill, kind: data }\n", 1)
	for _, pair := range [][2]string{
		{planMore, shopMore},
		{strings.Replace(planWorkflow, "shop.yaml#ready", "shop.yaml#gone", 1), strings.Replace(shopWorkflow, "label: Shopping list", "label: The list", 1)},
		{strings.Replace(planWorkflow, "shop.yaml#ready", "plan.yaml#thu", 1), shopWorkflow},
		{strings.Replace(planWorkflow, "shop.yaml#ready", "shop.yaml#fill", 1), shopWorkflow},
	} {
		workflows := together(t, "plan.yaml", pair[0], "shop.yaml", pair[1])
		mistakes, warnings := TogetherMistakes(workflows), TogetherWarnings(workflows)
		for i, file := range []string{"plan.yaml", "shop.yaml"} {
			want := append(append([]string{}, mistakes[i]...), warnings[i]...)
			got := []string{}
			for _, check := range CheckSources("dev", []Source{source("plan.yaml", pair[0]), source("shop.yaml", pair[1])}).Checks {
				for _, mistake := range check.Mistakes {
					if mistake.File == file && isOneOf(check.ID, "refs.known", "refs.agree", "refs.starts", "refs.owner", "warn.shared-writers", "warn.shared-readers") {
						got = append(got, mistake.Message)
						failed[check.ID] = true
					}
				}
			}
			sort.Strings(want)
			sort.Strings(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s together: the checks hold %q, but TogetherMistakes and TogetherWarnings give %q", file, got, want)
			}
		}
	}
	// A file that isn't YAML is tested on its own above.
	for _, check := range CheckSources("dev", nil).Checks {
		if !failed[check.ID] && check.ID != "file.yaml" {
			t.Errorf("no source here fails %s", check.ID)
		}
	}
}
