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

func TestCheckReportsEachMistakeAndFails(t *testing.T) {
	g, b := write(t, "good.yaml", good), write(t, "bad.yaml", bad)
	r := awmlRun("check", g, b)
	want := result{1, g + ": ok\n" + b + `: the line from "thu" to "plann" ends at unknown part "plann"` + "\n", ""}
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
	if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml [-h] [--registry URL] {draw,page,check,summary,login,push,pull} ...\n\nDraw agentic workflows written in AWML.\n") {
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
		{nil, "usage: awml [-h] [--registry URL] {draw,page,check,summary,login,push,pull} ...\nawml: error: the following arguments are required: command\n"},
		{[]string{"drew", "a.yaml"}, "usage: awml [-h] [--registry URL] {draw,page,check,summary,login,push,pull} ...\nawml: error: argument command: invalid choice: 'drew' (choose from 'draw', 'page', 'check', 'summary', 'login', 'push', 'pull')\n"},
		{[]string{"draw"}, "usage: awml draw [-h] file\nawml draw: error: the following arguments are required: file\n"},
		{[]string{"draw", "a.yaml", "b.yaml"}, "usage: awml [-h] [--registry URL] {draw,page,check,summary,login,push,pull} ...\nawml: error: unrecognized arguments: b.yaml\n"},
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
	if r != (result{0, "Logged in to https://registry.example\n", "Token: "}) {
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
		fmt.Fprintf(w, `{"number":2,"security":%q,"platform":%q}`, good, strings.Replace(good, "view: security", "view: platform", 1))
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

func TestPullFindsTheLatestVersion(t *testing.T) {
	useTemporaryHome(t)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/v/4") {
			fmt.Fprintf(w, `{"number":4,"security":%q}`, good)
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
		fmt.Fprintf(w, `{"number":1,"security":%q}`, good)
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

func TestAWMLRegistryEnvironmentIsHonoured(t *testing.T) {
	useTemporaryHome(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprintf(w, `{"number":1,"security":%q}`, good)
	}))
	defer server.Close()
	t.Setenv("AWML_REGISTRY", server.URL)
	out := filepath.Join(t.TempDir(), "out")
	if r := awmlRun("pull", "acme/mail@1", "-o", out); r.code != 0 {
		t.Fatalf("pull gave %+v", r)
	}
	if calls != 1 {
		t.Errorf("registry got %d calls", calls)
	}
}

func useTemporaryHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	config := filepath.Join(root, "config")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("AWML_REGISTRY", "")
	return config
}

func loginForTest(t *testing.T, registry, token string) {
	t.Helper()
	if r := awmlRunInput(token+"\n", "login", "--registry", registry); r.code != 0 {
		t.Fatalf("login gave %+v", r)
	}
}
