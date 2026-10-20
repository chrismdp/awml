package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrismdp/awml"
)

const good = `awml: "0.1"
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
	var out, errs bytes.Buffer
	code := run(args, &out, &errs)
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
	if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml [-h] {draw,page,check} ...\n\nDraw agentic workflows written in AWML.\n") {
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
		{nil, "usage: awml [-h] {draw,page,check} ...\nawml: error: the following arguments are required: command\n"},
		{[]string{"drew", "a.yaml"}, "usage: awml [-h] {draw,page,check} ...\nawml: error: argument command: invalid choice: 'drew' (choose from 'draw', 'page', 'check')\n"},
		{[]string{"draw"}, "usage: awml draw [-h] file\nawml draw: error: the following arguments are required: file\n"},
		{[]string{"draw", "a.yaml", "b.yaml"}, "usage: awml [-h] {draw,page,check} ...\nawml: error: unrecognized arguments: b.yaml\n"},
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
