package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrismdp/awml"
)

// encoded is a graph as awml graph prints it.
func encoded(t *testing.T, g awml.Graph) string {
	t.Helper()
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

const placedOrders = orders + `implementations:
  home:
    label: At home
    blocks:
      - { id: laptop, label: "Ana's laptop", parts: [keep, read, note] }
      - { id: board_app, label: The board app, parts: [board] }
`

func TestGraphPrintsWhatDrawDrawsAsJSON(t *testing.T) {
	path := write(t, "orders.yaml", placedOrders)
	workflow := doc(t, placedOrders)
	security, _ := awml.View(workflow, "")
	home, _ := awml.View(workflow, "home")
	untrusted, _ := awml.UntrustedView(workflow)
	for _, tc := range []struct {
		args []string
		view *awml.Map
	}{
		{[]string{path}, security},
		{[]string{"--implementation", "home", path}, home},
		{[]string{"--implementation=home", path}, home},
		{[]string{"--untrusted", path}, untrusted},
	} {
		if r := awmlRun(append([]string{"graph"}, tc.args...)...); r != (result{0, encoded(t, awml.GraphOf(tc.view)), ""}) {
			t.Errorf("graph %v gave %+v", tc.args, r)
		}
	}
	if r := awmlRun("graph", "--implementation", "home", path); !strings.Contains(r.stdout, `"places":[{"id":"laptop"`) {
		t.Errorf("graph of the platform view gave %+v", r)
	}
}

func TestGraphRefusesWhatDrawRefuses(t *testing.T) {
	path := write(t, "orders.yaml", placedOrders)
	usage := commands["graph"].usage
	if r := awmlRun("graph", "--untrusted", "--implementation", "home", path); r != (result{2, "", usage + "awml graph: error: argument --untrusted: not allowed with argument --implementation\n"}) {
		t.Errorf("graph --untrusted --implementation gave %+v", r)
	}
	if r := awmlRun("graph", "--implementation", "away", path); r != (result{1, "", "unknown implementation \"away\"\n"}) {
		t.Errorf("graph of an unknown implementation gave %+v", r)
	}
	plain := write(t, "plain.yaml", good)
	if r := awmlRun("graph", "--untrusted", plain); r != (result{1, "", "the file marks no part untrusted, so it has no untrusted view\n"}) {
		t.Errorf("graph --untrusted of a file with nothing untrusted gave %+v", r)
	}
	if r := awmlRun("graph"); r.code != 2 || !strings.Contains(r.stderr, "the following arguments are required: files") {
		t.Errorf("graph of nothing gave %+v", r)
	}
	if r := awmlRun("graph", write(t, "broken.yaml", "awml: \"0.8\"\ntitle: T\n")); r.code != 1 || r.stdout != "" || r.stderr == "" {
		t.Errorf("graph of a file with mistakes gave %+v", r)
	}
}

func TestGraphOfSeveralWorkflowsIsTheirMap(t *testing.T) {
	dir := folder(t, "plan.yaml", planWorkflow, "shop.yaml", shopWorkflow)
	plan, shopping := filepath.Join(dir, "plan.yaml"), filepath.Join(dir, "shop.yaml")
	want := encoded(t, awml.GraphMap([]awml.Workflow{{File: plan, Doc: doc(t, planWorkflow)}, {File: shopping, Doc: doc(t, shopWorkflow)}}))
	if got := awmlRun("graph", plan, shopping); got != (result{0, want, ""}) {
		t.Errorf("graph gave %+v", got)
	}
	if got := awmlRun("graph", dir); got != (result{0, want, ""}) {
		t.Errorf("graph of a folder gave %+v", got)
	}
	for _, option := range []string{"--untrusted", "--implementation=home"} {
		if got := awmlRun("graph", option, plan, shopping); got.code != 2 || !strings.Contains(got.stderr, "awml graph: error: ") || !strings.Contains(got.stderr, "takes one workflow, not several") {
			t.Errorf("graph %s of two gave %+v", option, got)
		}
	}
}

func TestTheHelpListsGraph(t *testing.T) {
	if r := awmlRun("--help"); !strings.Contains(r.stdout, "{draw,graph,page,") || !strings.Contains(r.stdout, "    graph            print what a file's drawing shows, as JSON\n") {
		t.Errorf("help gave %s", r.stdout)
	}
	if r := awmlRun("graph", "--help"); r.code != 0 || !strings.HasPrefix(r.stdout, "usage: awml graph [-h] [--implementation ID | --untrusted] files [files ...]\n") {
		t.Errorf("graph --help gave %+v", r)
	}
}
