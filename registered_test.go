package awml

import (
	"reflect"
	"strings"
	"testing"
)

// A block may name a registered platform as owner/platforms/slug.
func TestABlockMayNameARegisteredPlatform(t *testing.T) {
	registered := strings.Replace(canonical, "platform: scripts", "platform: acme/platforms/ops-runner", 1)
	if got := Check(parseCanonical(t, registered)); len(got) != 0 {
		t.Fatalf("a registered platform: %q", got)
	}
	if got := Check(parseCanonical(t, canonical)); len(got) != 0 {
		t.Fatalf("a bare platform: %q", got)
	}
	// The owner and slug follow the registry's rule for names, so either may start with a digit.
	for _, name := range []string{"42labs/platforms/worker", "acme/platforms/2nd-runner", "a/platforms/9", strings.Repeat("a", 40) + "/platforms/" + strings.Repeat("9", 40)} {
		source := strings.Replace(canonical, "platform: scripts", "platform: "+name, 1)
		if got := Check(parseCanonical(t, source)); len(got) != 0 {
			t.Fatalf("%s: %q", name, got)
		}
	}
	for _, platform := range []string{
		"acme/ops-runner",
		"acme/workflows/ops-runner",
		"Acme/platforms/ops-runner",
		"acme-/platforms/ops-runner",
		"acme/platforms/-ops-runner",
		strings.Repeat("a", 41) + "/platforms/ops-runner",
		"acme/platforms/" + strings.Repeat("a", 41),
		"42labs",
	} {
		t.Run(platform, func(t *testing.T) {
			source := strings.Replace(canonical, "platform: scripts", "platform: "+platform, 1)
			want := []string{`group "server": platform "` + platform + `" must be a company id, or owner/platforms/slug for a registered platform`}
			if got := Check(parseCanonical(t, source)); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %q; want %q", got, want)
			}
		})
	}
}

const registeredDescription = `awml: "0.8"
platform: { id: acme/platforms/ops-runner, label: "The ops runner" }
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: [notes]
`

func TestADescriptionMayHaveARegisteredID(t *testing.T) {
	if got := Check(parsed(t, registeredDescription)); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
	if got := Check(parsed(t, strings.Replace(registeredDescription, "acme/platforms/ops-runner", "42labs/platforms/9-runner", 1))); len(got) != 0 {
		t.Fatalf("an owner and slug starting with a digit: %q", got)
	}
	for _, id := range []string{"acme/ops-runner", "acme/platforms/ops-runner-"} {
		t.Run(id, func(t *testing.T) {
			got := Check(parsed(t, strings.Replace(registeredDescription, "acme/platforms/ops-runner", id, 1)))
			want := `platform id "` + id + `" must be lower-case letters, digits and hyphens, starting with a letter, or owner/platforms/slug for a registered platform`
			if !reflect.DeepEqual(got, []string{want}) {
				t.Fatalf("got %q; want %q", got, want)
			}
		})
	}
}

// awml fit matches a registered platform by its plain id, and says how to fetch one it wasn't given.
func TestFitMatchesARegisteredPlatformByItsID(t *testing.T) {
	workflow := parseCanonical(t, strings.Replace(canonical, "platform: scripts", "platform: acme/platforms/ops-runner", 1))
	placement, err := View(workflow, "home")
	if err != nil {
		t.Fatal(err)
	}
	description := parsed(t, registeredDescription)
	got, gaps := Fit(workflow, placement, []*Map{description})
	if want := []string{"server  fits: plan may use any"}; !reflect.DeepEqual(got, want) || gaps != nil {
		t.Fatalf("got %q, gaps %q", got, gaps)
	}
	other := parsed(t, strings.Replace(registeredDescription, "acme/platforms/ops-runner", "other", 1))
	_, gaps = Fit(workflow, placement, []*Map{other})
	want := []string{`group "server" names platform "acme/platforms/ops-runner", but its description wasn't given: pull it with awml pull acme/platforms/ops-runner`}
	if !reflect.DeepEqual(gaps, want) {
		t.Fatalf("gaps %q; want %q", gaps, want)
	}
}
