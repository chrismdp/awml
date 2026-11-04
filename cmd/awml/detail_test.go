package main

import (
	"strings"
	"testing"
)

// decided is a workflow whose step has a title, and how, why and decided as its detail.
var decided = strings.Replace(good, `label: "Plan next week's dinners", in: family }`,
	`label: "Plan next week's dinners", in: family, how: "from the favourites", decided: "The parents, in March" }`, 1)

func TestDrawGivesTheDetailWithDetail(t *testing.T) {
	path := write(t, "dinners.yaml", decided)
	plain := awmlRun("draw", path)
	if plain.code != 0 || strings.Contains(plain.stdout, "plan_detail") {
		t.Errorf("draw gave %+v", plain)
	}
	note := `plan_detail@{ shape: notch-rect, label: "<b>Plan next week#39;s dinners</b><br>how: from the favourites<br>decided: The parents#44; in March" }`
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
	r := awmlRun("graph", write(t, "dinners.yaml", decided))
	if want := `"how":"from the favourites","decided":"The parents, in March"`; r.code != 0 || !strings.Contains(r.stdout, want) {
		t.Errorf("graph gave %+v", r)
	}
}

func TestSummaryListsEachDecided(t *testing.T) {
	r := awmlRun("summary", write(t, "dinners.yaml", decided))
	if want := "red lines: 0\ndecided: 1\n  Plan next week's dinners\n    The parents, in March\nuntrusted parts: 0\n"; r.code != 0 || !strings.HasSuffix(r.stdout, want) {
		t.Errorf("summary gave %+v, want it to end\n%s", r, want)
	}
}
