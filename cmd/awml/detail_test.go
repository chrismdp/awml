package main

import (
	"strings"
	"testing"
)

// described is a workflow whose step has a title and a description.
var described = strings.Replace(good, `label: "Plan next week's dinners", in: family }`,
	`label: "Plan next week's dinners", in: family, description: "from the favourites, chosen by the parents in March" }`, 1)

func TestDrawGivesTheDetailWithDetail(t *testing.T) {
	path := write(t, "dinners.yaml", described)
	plain := awmlRun("draw", path)
	if plain.code != 0 || strings.Contains(plain.stdout, "plan_detail") {
		t.Errorf("draw gave %+v", plain)
	}
	note := `plan_detail@{ shape: notch-rect, label: "<b>Plan next week#39;s dinners</b><br>from the favourites#44; chosen by the parents in March" }`
	for _, args := range [][]string{{"draw", "--detail", path}, {"draw", path, "--detail"}} {
		if r := awmlRun(args...); r.code != 0 || !strings.Contains(r.stdout, note) || !strings.Contains(r.stdout, "  plan -.- plan_detail\n") {
			t.Errorf("awml %q gave %+v", args, r)
		}
	}
	if r := awmlRun("draw", "-h"); !strings.Contains(r.stdout, "--detail") {
		t.Errorf("draw -h doesn't name --detail:\n%s", r.stdout)
	}
	// graph gives each field apart already, so it takes no --detail.
	if r := awmlRun("graph", "--detail", path); r.code != 2 {
		t.Errorf("graph --detail gave %+v", r)
	}
}

func TestGraphGivesEachFieldApart(t *testing.T) {
	r := awmlRun("graph", write(t, "dinners.yaml", described))
	if want := `"description":"from the favourites, chosen by the parents in March"`; r.code != 0 || !strings.Contains(r.stdout, want) {
		t.Errorf("graph gave %+v", r)
	}
}

func TestSummaryDoesNotListTheDescription(t *testing.T) {
	r := awmlRun("summary", write(t, "dinners.yaml", described))
	if want := "red lines: 0\nuntrusted parts: 0\n"; r.code != 0 || !strings.HasSuffix(r.stdout, want) {
		t.Errorf("summary gave %+v, want it to end\n%s", r, want)
	}
}

func TestCheckAcceptsDescriptionAndRefusesOldDetail(t *testing.T) {
	if r := awmlRun("check", write(t, "description.yaml", described)); r.code != 0 || !strings.HasSuffix(r.stdout, ": ok\n") {
		t.Errorf("check description gave %+v", r)
	}
	old := strings.Replace(described, "description:", "how:", 1)
	if r := awmlRun("check", write(t, "old.yaml", old)); r.code != 1 || !strings.Contains(r.stdout, `part "plan" has unknown field "how"`) {
		t.Errorf("check old field gave %+v", r)
	}
}
