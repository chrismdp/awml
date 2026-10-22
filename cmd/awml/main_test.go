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

const good = `awml: "0.2"
title: Dinners
view: security
groups:
  - { id: family, label: family }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00" }
  - { id: plan, kind: ai, label: "Plan next week's dinners", in: family }
lines:
  - { from: thu, to: plan, kind: start }
`

var bad = strings.Replace(good, "to: plan", "to: plann", 1)

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

func TestCheckPrintsWarningsBeforeMistakesAndStillFailsForTheMistakes(t *testing.T) {
	src := strings.Replace(bad, "kind: ai, label:", "kind: ai, op: do, label:", 1)
	f := write(t, "warning-and-mistake.yaml", src)
	want := f + `: warning: step "plan" is a do step decided by AI: a do step should be code acting on what an earlier step decided` + "\n" +
		f + `: the line from "thu" to "plann" ends at unknown part "plann"` + "\n"
	if r := awmlRun("check", f); r != (result{1, want, ""}) {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckReportsEachMistakeAndFails(t *testing.T) {
	g, b := write(t, "good.yaml", good), write(t, "bad.yaml", bad)
	r := awmlRun("check", g, b)
	want := result{1, g + ": ok\n" + b + `: the line from "thu" to "plann" ends at unknown part "plann"` + "\n", ""}
	if r != want {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckComparesTwoViewsWithTheSameTitle(t *testing.T) {
	securitySource := strings.Replace(good, "  - { from: thu, to: plan, kind: start }\n", "", 1)
	security := write(t, "security.yaml", securitySource)
	platform := write(t, "platform.yaml", strings.Replace(strings.Replace(securitySource, "view: security", "view: platform", 1),
		"  - { id: plan, kind: ai, label: \"Plan next week's dinners\", in: family }\n", "", 1))
	r := awmlRun("check", security, platform)
	want := result{1, security + " and " + platform + `: part "plan" is in the security view but not the platform view` + "\n", ""}
	if r != want {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckComparesOnlyOneSecurityAndOnePlatformViewOfATitle(t *testing.T) {
	security := write(t, "security.yaml", good)
	secondSecurity := write(t, "second-security.yaml", good)
	platformSource := strings.Replace(good, "view: security", "view: platform", 1)
	platform := write(t, "platform.yaml", platformSource)
	r := awmlRun("check", security, secondSecurity, platform)
	want := result{0, security + ": ok\n" + secondSecurity + ": ok\n" + platform + ": ok\n", ""}
	if r != want {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckCountsAFaultyViewWhenDecidingWhetherATitleHasOnePair(t *testing.T) {
	securitySource := strings.Replace(good, "  - { from: thu, to: plan, kind: start }\n", "", 1)
	security := write(t, "security.yaml", securitySource)
	faulty := write(t, "faulty-security.yaml", strings.Replace(securitySource, "in: family }", "in: unknown }", 1))
	platformSource := strings.Replace(strings.Replace(securitySource, "view: security", "view: platform", 1),
		"  - { id: plan, kind: ai, label: \"Plan next week's dinners\", in: family }\n", "", 1)
	platform := write(t, "platform.yaml", platformSource)
	r := awmlRun("check", security, faulty, platform)
	want := result{1, security + ": ok\n" + faulty + `: part "plan" is in unknown group "unknown"` + "\n" + platform + ": ok\n", ""}
	if r != want {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckDoesNotCompareDifferentTitles(t *testing.T) {
	security := write(t, "security.yaml", good)
	platformSource := strings.Replace(strings.Replace(good, "view: security", "view: platform", 1), "title: Dinners", "title: Lunches", 1)
	platform := write(t, "platform.yaml", platformSource)
	r := awmlRun("check", security, platform)
	want := result{0, security + ": ok\n" + platform + ": ok\n", ""}
	if r != want {
		t.Errorf("check gave %+v", r)
	}
}

func TestCheckReportsAFileThatIsntYAML(t *testing.T) {
	f := write(t, "broken.yaml", "parts: [\n")
	r := awmlRun("check", f)
	if r.code != 1 || r.stdout != f+": isn't YAML: line 1: did not find expected node content\n" {
		t.Errorf("check gave %+v", r)
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
awml: "0.3"
title: Reply
view: security
groups: [{id: business, label: business, processing: {anthropic: any}}]
parts:
  - {id: draft, kind: ai, op: draft, label: Draft, in: business}
`)
	platform := write(t, "worker.yaml", `
awml: "0.3"
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
awml: "0.3"
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
awml: "0.3"
title: Reply
view: security
groups: [{id: business, label: business}]
parts: [{id: draft, kind: ai, op: draft, label: Draft, in: business}]
`)
	platform := write(t, "worker.yaml", goodPlatformForCommand)
	if r := awmlRun("fit", security, platform); r.code != 1 || r.stdout != `group "business" has no processing`+"\n" {
		t.Errorf("gap fit gave %+v", r)
	}
	if r := awmlRun("fit", "--json", security, platform); r.code != 1 || r.stdout != `{"results":[],"gaps":["group \"business\" has no processing"]}`+"\n" {
		t.Errorf("gap fit as JSON gave %+v", r)
	}
	view := write(t, "view.yaml", strings.Replace(good, "view: security", "view: platform", 1))
	if r := awmlRun("fit", view, platform); r.code != 1 || !strings.Contains(r.stderr, "must be a security view") {
		t.Errorf("wrong-view fit gave %+v", r)
	}
}

func TestFitJudgesAPlatformView(t *testing.T) {
	security := write(t, "security.yaml", `
awml: "0.3"
title: Reply
view: security
groups: [{id: business, label: business, processing: {anthropic: any}}]
parts: [{id: draft, kind: ai, op: draft, label: Draft, in: business}]
`)
	placement := write(t, "placement.yaml", `
awml: "0.3"
title: Reply placement
view: platform
groups: [{id: box, label: Box, platform: worker}]
parts: [{id: draft, kind: ai, op: draft, label: Draft, in: box}]
`)
	platform := write(t, "worker.yaml", goodPlatformForCommand)
	if r := awmlRun("fit", security, placement, platform); r != (result{0, "box  fits: draft may use anthropic\n", ""}) {
		t.Errorf("placement fit gave %+v", r)
	}
}

const goodPlatformForCommand = `
awml: "0.3"
platform: {id: worker, label: Worker}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: []
`

func TestSummaryPrintsJSONForOneOrBothViews(t *testing.T) {
	security := write(t, "security.yaml", good)
	platform := write(t, "platform.yaml", strings.Replace(good, "view: security", "view: platform", 1))
	wantSingle, err := json.Marshal(awml.Summarise(doc(t, good)))
	if err != nil {
		t.Fatal(err)
	}
	wantBoth, err := json.Marshal(awml.Summarise(doc(t, good), doc(t, strings.Replace(good, "view: security", "view: platform", 1))))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want []byte
	}{{[]string{"summary", "--json", security}, wantSingle}, {[]string{"summary", security, platform, "--json"}, wantBoth}} {
		r := awmlRun(tc.args...)
		if r != (result{0, string(tc.want) + "\n", ""}) {
			t.Errorf("awml %q gave %+v", tc.args, r)
		}
	}
}

func TestSummaryPrintsPlainCounts(t *testing.T) {
	r := awmlRun("summary", write(t, "good.yaml", good))
	want := "steps: AI 1, code 0, person 0\noperations: none\noutside actions: Allow 0, Ask 0, Block 0\ntriggers: 1\nreviews: 0\ncontexts: 0\nconnectors: 0\nsurfaces: 0\nscopes: 1\nplatforms: 0\npublishing lines: 0\nrefused reads: 0\nred lines: 0\n"
	if r != (result{0, want, ""}) {
		t.Errorf("summary gave %+v", r)
	}
}

func TestPageWritesOneHTMLFile(t *testing.T) {
	platform := strings.Replace(good, "view: security", "view: platform", 1)
	a, b := write(t, "a.yaml", good), write(t, "b.yaml", platform)
	out := filepath.Join(t.TempDir(), "out.html")
	if r := awmlRun("page", a, b, "-o", out); r != (result{0, "", ""}) {
		t.Errorf("page gave %+v", r)
	}
	html, err := os.ReadFile(out)
	if err != nil || string(html) != awml.Page([]*awml.Map{doc(t, good), doc(t, platform)}) {
		t.Errorf("the page isn't the two drawings: %v", err)
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
	want := `# A bakery plans tomorrow's loaves. The bakery is fictional.
# It runs in the bakery office.
awml: "0.4"
title: Tomorrow's loaves
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
	if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml [-h] [--registry URL] {draw,page,check,fit,summary,convert,login,push,pull} ...\n\nDraw agentic workflows written in AWML.\n") {
		t.Errorf("-h gave %+v", r)
	}
	if r := awmlRun("page", "--help"); r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml page [-h] -o OUT files [files ...]\n") {
		t.Errorf("page --help gave %+v", r)
	}
}

func TestAMistakeInTheCommandShowsHowToUseIt(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "usage: awml [-h] [--registry URL] {draw,page,check,fit,summary,convert,login,push,pull} ...\nawml: error: the following arguments are required: command\n"},
		{[]string{"drew", "a.yaml"}, "usage: awml [-h] [--registry URL] {draw,page,check,fit,summary,convert,login,push,pull} ...\nawml: error: argument command: invalid choice: 'drew' (choose from 'draw', 'page', 'check', 'fit', 'summary', 'convert', 'login', 'push', 'pull')\n"},
		{[]string{"draw"}, "usage: awml draw [-h] [--implementation ID] file\nawml draw: error: the following arguments are required: file\n"},
		{[]string{"draw", "a.yaml", "b.yaml"}, "usage: awml [-h] [--registry URL] {draw,page,check,fit,summary,convert,login,push,pull} ...\nawml: error: unrecognized arguments: b.yaml\n"},
		{[]string{"check"}, "usage: awml check [-h] files [files ...]\nawml check: error: the following arguments are required: files\n"},
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

func TestPushChecksThenSendsTheViewsAndToken(t *testing.T) {
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
	security := write(t, "security.yaml", good)
	platformText := strings.Replace(good, "view: security", "view: platform", 1)
	platform := write(t, "platform.yaml", platformText)
	r := awmlRun("push", "acme/mail", security, platform, "--private", "--registry", server.URL)
	if r != (result{0, "Pushed acme/mail version 3: /acme/mail/v/3\n", ""}) {
		t.Fatalf("push gave %+v", r)
	}
	if gotPath != "/api/v1/acme/mail/versions" || gotAuth != "Bearer push-token" {
		t.Errorf("request path/auth = %q, %q", gotPath, gotAuth)
	}
	if gotBody["security"] != good || gotBody["platform"] != platformText || gotBody["visibility"] != "private" {
		t.Errorf("body = %#v", gotBody)
	}
}

func TestPushPrints422Errors(t *testing.T) {
	useTemporaryHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"errors":["security: a mistake","platform: another mistake"]}`)
	}))
	defer server.Close()
	loginForTest(t, server.URL, "token")
	r := awmlRun("push", "acme/mail", write(t, "security.yaml", good), "--registry", server.URL)
	want := "security: a mistake\nplatform: another mistake\n"
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
		fmt.Fprintf(w, `{"number":2,"files":{"security":%q,"platform":%q}}`, good, strings.Replace(good, "view: security", "view: platform", 1))
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
	for name, want := range map[string]string{"security.yaml": good, "platform.yaml": strings.Replace(good, "view: security", "view: platform", 1)} {
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
		fmt.Fprintf(w, `{"number":1,"files":{"security":%q}}`, good)
	}))
	defer server.Close()
	t.Setenv("AWML_REGISTRY_URL", server.URL+"/")
	if r := awmlRunInput("env-token\n", "login"); r != (result{0, "Logged in to " + server.URL + "\n", ""}) {
		t.Fatalf("login gave %+v", r)
	}
	if r := awmlRun("push", "acme/mail", write(t, "security.yaml", good)); r.code != 0 {
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
awml: "0.4"
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
	desc := write(t, "scripts.yaml", `awml: "0.3"
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

func TestConvertCommand(t *testing.T) {
	security := write(t, "security.yaml", "# Keep this comment\n"+good)
	platform := write(t, "platform.yaml", strings.Replace(good, "view: security", "view: platform", 1))
	got := awmlRun("convert", security, platform)
	if got.code != 0 || !strings.Contains(got.stdout, "# Keep this comment") || !strings.Contains(got.stdout, `awml: "0.4"`) {
		t.Fatalf("%+v", got)
	}
	parsed, err := awml.Parse([]byte(got.stdout))
	if err != nil || len(awml.Check(parsed)) > 0 {
		t.Fatalf("%v %v", err, awml.Check(parsed))
	}
	platform = write(t, "bad-pair.yaml", strings.Replace(strings.Replace(good, "view: security", "view: platform", 1), "kind: ai", "kind: code", 1))
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
			json.NewEncoder(w).Encode(map[string]any{"number": 1, "files": map[string]string{"security": canonicalFile}})
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
	if payload["security"] != canonicalFile || payload["platform"] != nil {
		t.Fatalf("payload = %#v", payload)
	}
	dir := t.TempDir()
	if got := awmlRun("pull", "--registry", server.URL, "bakery/bread@1", "-o", dir); got.code != 0 {
		t.Fatal(got)
	}
	source, err := os.ReadFile(filepath.Join(dir, "security.yaml"))
	if err != nil || string(source) != canonicalFile {
		t.Fatalf("%s %v", source, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "platform.yaml")); !os.IsNotExist(err) {
		t.Fatal("unexpected platform file")
	}
}

func TestConvertCollectsInlineComments(t *testing.T) {
	security := write(t, "security.yaml", strings.Replace(good, "title: Dinners", "title: Dinners # Dinner plans", 1))
	platform := write(t, "platform.yaml", strings.Replace(good, "view: security", "view: platform", 1))
	got := awmlRun("convert", security, platform)
	if got.code != 0 || !strings.HasPrefix(got.stdout, "# Dinner plans\n") {
		t.Fatalf("%+v", got)
	}
}

func TestPushRefusesACanonicalFileWithALegacyPlatformFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	canonical := write(t, "bread.yaml", canonicalFile)
	platform := write(t, "platform.yaml", strings.Replace(good, "view: security", "view: platform", 1))
	got := awmlRun("push", "bakery/bread", canonical, platform)
	if got.code != 1 || !strings.Contains(got.stderr, "a 0.4 workflow must be pushed on its own") {
		t.Fatalf("%+v", got)
	}
}
