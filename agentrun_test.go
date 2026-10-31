package awml

import (
	"encoding/json"
	"strings"
	"testing"
)

const agentRunWorkflow = `awml: "0.10"
title: The bakery's stock
scopes:
  - { id: bakery, label: Bakery }
parts:
  - { id: morning, kind: trigger, label: Every morning, untrusted: true }
  - { id: count, kind: code, op: check, label: Count the stock, in: bakery }
  - { id: suggest, kind: ai, op: draft, label: Suggest an order, in: bakery }
  - { id: notes, kind: context, label: Stock notes, in: bakery }
  - { id: local, kind: code, label: Read the local count, in: bakery }
  - { id: scripted, kind: code, label: Read the saved count, in: bakery }
lines:
  - { from: morning, to: count, kind: start }
  - { from: count, to: suggest, kind: start }
  - { from: notes, to: count, kind: data }
  - { from: count, to: local, kind: start }
  - { from: count, to: scripted, kind: start }
implementations:
  office:
    label: In the office
    blocks:
      - { id: session, label: Agent session, platform: scripts-server, parts: [count, suggest, notes] }
      - { id: child, label: Local script, in: session, parts: [local] }
      - { id: scheduler, label: Scheduler, in: session, platform: scheduler, parts: [scripted] }
`

func TestPlatformViewOutlinesOnlyCodeInItsOwnAgentBlock(t *testing.T) {
	for _, tc := range []struct {
		name, workflow, description, implementation string
		outlined                                    bool
	}{
		{"agent", agentRunWorkflow, strings.Replace(soundDescription, "steps: code", "steps: agent", 1), "office", true},
		{"code", agentRunWorkflow, soundDescription, "office", false},
		{"without descriptions", agentRunWorkflow, "", "office", false},
		{"another platform", agentRunWorkflow, strings.Replace(soundDescription, "id: scripts-server", "id: another", 1), "office", false},
		{"unnamed platform", strings.Replace(agentRunWorkflow, "platform: scripts-server, ", "", 1), strings.Replace(soundDescription, "steps: code", "steps: agent", 1), "office", false},
		{"published platform", strings.Replace(agentRunWorkflow, "platform: scripts-server", "platform: acme/platforms/session", 1), strings.Replace(strings.Replace(soundDescription, "id: scripts-server", "id: acme/platforms/session", 1), "steps: code", "steps: agent", 1), "office", true},
		{"security", agentRunWorkflow, strings.Replace(soundDescription, "steps: code", "steps: agent", 1), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := []Source{source("stock.yaml", tc.workflow)}
			if tc.description != "" {
				sources = append(sources, source("session.yaml", tc.description))
			}
			out, err := DrawSources(sources, tc.implementation)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(out, "  class count agentrun\n"); got != tc.outlined {
				t.Errorf("count outlined = %t, want %t:\n%s", got, tc.outlined, out)
			}
			if tc.outlined {
				has(t, out, "  classDef agentrun stroke:#d97a00,stroke-width:3px,stroke-dasharray:6 3\n")
				if strings.Count(out, " agentrun\n") != 1 {
					t.Errorf("outlined a part other than count:\n%s", out)
				}
			} else if strings.Contains(out, "agentrun") {
				t.Errorf("an outline without a code step run by an agent:\n%s", out)
			}
			has(t, out, "  class morning untrusted\n")
			if tc.implementation == "" {
				plain, err := DrawSources(sources[:1], "")
				if err != nil || out != plain {
					t.Errorf("descriptions changed the security view: %v", err)
				}
			}
			graph, err := GraphSources(sources, tc.implementation)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(graph)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(encoded), `"agentRun":true`); got != map[bool]int{false: 0, true: 1}[tc.outlined] {
				t.Errorf("graph has %d agent steps: %s", got, encoded)
			}
			var read Graph
			if err := json.Unmarshal(encoded, &read); err != nil || read.Mermaid() != out {
				t.Errorf("graph read back draws differently: %v", err)
			}
		})
	}
}

func TestPlatformDescriptionsDoNotBecomeWorkflowsOnAMap(t *testing.T) {
	sources := []Source{source("plan.yaml", planWorkflow), source("shop.yaml", shopWorkflow)}
	want, err := DrawSources(sources, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DrawSources(append(sources, source("server.yaml", soundDescription)), "")
	if err != nil || got != want {
		t.Errorf("description changed the map: %v\n%s", err, got)
	}
}
