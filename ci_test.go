package awml

import (
	"regexp"
	"strings"
	"testing"
)

// CI runs every check that AGENTS.md says each change passes, so nothing reaches main with CI
// green that those checks would have stopped.
func TestCIRunsEveryCheckAgentsMdLists(t *testing.T) {
	block := regexp.MustCompile("(?s)```sh\n(.*?)```").FindStringSubmatch(readText(t, "AGENTS.md"))
	if block == nil {
		t.Fatal("AGENTS.md lists no checks")
	}
	runs := map[string]bool{}
	for _, line := range strings.Split(readText(t, ".github/workflows/ci.yml"), "\n") {
		line = strings.TrimSpace(line)
		runs[strings.TrimPrefix(line, "- run: ")] = true
		runs[strings.TrimPrefix(line, "run: ")] = true
	}
	for _, check := range strings.Split(strings.TrimSpace(block[1]), "\n") {
		if !runs[strings.TrimSpace(check)] {
			t.Errorf("CI doesn't run %q, which AGENTS.md lists as a check", check)
		}
	}
}
