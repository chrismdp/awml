package awml

import (
	"reflect"
	"strings"
	"testing"
)

// A 0.5 file may name a registered platform as owner/platforms/slug, which a 0.4 file may not.
func TestABlockMayNameARegisteredPlatformIn05(t *testing.T) {
	registered := strings.Replace(strings.Replace(canonical, `awml: "0.4"`, `awml: "0.5"`, 1), "platform: scripts", "platform: acme/platforms/ops-runner", 1)
	if got := Check(parseCanonical(t, registered)); len(got) != 0 {
		t.Fatalf("0.5 with a registered platform: %q", got)
	}
	if got := Check(parseCanonical(t, strings.Replace(canonical, `awml: "0.4"`, `awml: "0.5"`, 1))); len(got) != 0 {
		t.Fatalf("0.5 with a bare platform: %q", got)
	}
	cases := []struct{ version, platform, want string }{
		{"0.4", "acme/platforms/ops-runner", `group "server": platform "acme/platforms/ops-runner" must be a company id`},
		{"0.5", "acme/ops-runner", `group "server": platform "acme/ops-runner" must be a company id, or owner/platforms/slug for a registered platform`},
		{"0.5", "acme/workflows/ops-runner", `group "server": platform "acme/workflows/ops-runner" must be a company id, or owner/platforms/slug for a registered platform`},
		{"0.5", "Acme/platforms/ops-runner", `group "server": platform "Acme/platforms/ops-runner" must be a company id, or owner/platforms/slug for a registered platform`},
	}
	for _, tc := range cases {
		t.Run(tc.version+" "+tc.platform, func(t *testing.T) {
			source := strings.Replace(strings.Replace(canonical, `awml: "0.4"`, `awml: "`+tc.version+`"`, 1), "platform: scripts", "platform: "+tc.platform, 1)
			if got, want := Check(parseCanonical(t, source)), []string{tc.want}; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %q; want %q", got, want)
			}
		})
	}
}

func TestA05FileIsAWorkflow(t *testing.T) {
	doc := parseCanonical(t, strings.Replace(canonical, `awml: "0.4"`, `awml: "0.5"`, 1))
	if !IsCanonical(doc) {
		t.Fatal("a 0.5 file isn't read as a workflow")
	}
	if got := Check(parseCanonical(t, strings.Replace(canonical, `awml: "0.4"`, `awml: "0.6"`, 1))); !strings.Contains(strings.Join(got, "\n"), `awml must be "0.2" or "0.3", not "0.6"`) {
		t.Fatalf("0.6 gave %q", got)
	}
}

const registeredDescription = `awml: "0.5"
platform: { id: acme/platforms/ops-runner, label: "The ops runner" }
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: [notes]
`

func TestADescriptionMayHaveARegisteredIDIn05(t *testing.T) {
	if got := Check(parsed(t, registeredDescription)); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
	cases := []struct{ old, new, want string }{
		{`awml: "0.5"`, `awml: "0.4"`, `platform id "acme/platforms/ops-runner" must be lower-case letters, digits and hyphens, starting with a letter`},
		{"acme/platforms/ops-runner", "acme/ops-runner", `platform id "acme/ops-runner" must be lower-case letters, digits and hyphens, starting with a letter, or owner/platforms/slug for a registered platform`},
	}
	for _, tc := range cases {
		t.Run(tc.new, func(t *testing.T) {
			got := Check(parsed(t, strings.Replace(registeredDescription, tc.old, tc.new, 1)))
			if !reflect.DeepEqual(got, []string{tc.want}) {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}

// awml fit matches a registered platform by its plain id, and says how to fetch one it wasn't given.
func TestFitMatchesARegisteredPlatformByItsID(t *testing.T) {
	workflow := parseCanonical(t, strings.Replace(strings.Replace(canonical, `awml: "0.4"`, `awml: "0.5"`, 1), "platform: scripts", "platform: acme/platforms/ops-runner", 1))
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
