package awml

import (
	"strings"
	"testing"
)

// A workflow with each field that awml fit reads, and a block that names a platform description.
const fitFields = `
awml: "0.8"
title: A reply
scopes:
  - {id: private, label: private, processing: any}
  - {id: business, label: business, in: private, processing: {anthropic: any, openai: [sort, draft]}}
parts:
  - {id: hourly, kind: trigger, label: Hourly, wait: PT2H, in: private}
  - {id: notes, kind: context, label: Notes, at: notes/projects, in: business}
  - {id: choose, kind: ai, op: sort, label: Choose, in: business}
lines:
  - {from: hourly, to: choose, kind: start}
  - {from: notes, to: choose, kind: data}
implementations:
  home:
    label: At home
    blocks:
      - {id: server, label: Server, platform: worker, parts: [notes, choose]}
`

func TestAWorkflowTakesTheFieldsFitReads(t *testing.T) {
	mistakes(t, parsed(t, fitFields))
}

func TestProcessingBelongsOnlyToScopesAndPlatformOnlyToBlocks(t *testing.T) {
	mistakes(t, parsed(t, strings.Replace(fitFields, "processing: any", "platform: worker", 1)),
		`group "private" has a platform, but only a group in the platform view takes one`)
	mistakes(t, parsed(t, strings.Replace(fitFields, "platform: worker", "platform: worker, processing: any", 1)),
		`block "server" has unknown field "processing"`)
}

func TestProcessingNamesCompaniesAndKnownOperations(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{"sometimes", `group "private": processing must be any, none or a map from company ids to any or operations`},
		{"{OpenAI: any}", `group "private": processing company "OpenAI" must be lower-case letters, digits and hyphens, starting with a letter`},
		{"{true: any}", `group "private": processing company "true" must be text: put it in quotes`},
		{"{openai: none}", `group "private": processing for "openai" must be any or a list of operations`},
		{"{openai: sort}", `group "private": processing for "openai" must be any or a list of operations`},
		{"{openai: [sort, write-up]}", `group "private": processing for "openai" has unknown operation "write-up"`},
	} {
		t.Run(tc.value, func(t *testing.T) {
			src := strings.Replace(fitFields, "processing: any", "processing: "+tc.value, 1)
			mistakes(t, parsed(t, src), tc.want)
		})
	}
}

func TestAnInnerScopeCannotLoosenProcessing(t *testing.T) {
	src := strings.Replace(fitFields, "processing: any", "processing: {anthropic: [sort], openai: [sort]}", 1)
	mistakes(t, parsed(t, src),
		`group "business" allows anthropic to process any operation, but outer group "private" allows it only for sort`,
		`group "business" allows openai to draft, but outer group "private" doesn't`)
}

func TestDurationsUseOnlyFixedLengthISO8601Units(t *testing.T) {
	for _, duration := range []string{"P2W", "P3DT4H5M6S", "PT0S", "PT1.5H"} {
		t.Run("good_"+duration, func(t *testing.T) {
			src := strings.Replace(fitFields, "PT2H", duration, 1)
			mistakes(t, parsed(t, src))
		})
	}
	for _, duration := range []string{"P1Y", "P2M", "PT", "2 hours"} {
		t.Run("bad_"+duration, func(t *testing.T) {
			src := strings.Replace(fitFields, "PT2H", duration, 1)
			mistakes(t, parsed(t, src), `part "hourly": wait must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "`+duration+`"`)
		})
	}
}

func TestWaitBelongsToATriggerAndAtToAContext(t *testing.T) {
	src := strings.Replace(fitFields, "kind: trigger", "kind: code", 1)
	src = strings.Replace(src, "parts: [notes, choose]", "parts: [hourly, notes, choose]", 1)
	src = strings.Replace(src, "kind: context", "kind: surface", 1)
	mistakes(t, parsed(t, src),
		`part "hourly" has a wait, but only a trigger takes one`,
		`part "notes" has an at, but only a context takes one`)
}

func TestAtIsARelativeStorePath(t *testing.T) {
	for _, path := range []string{"/notes/projects", "notes/../projects", "notes//projects", "notes projects"} {
		t.Run(path, func(t *testing.T) {
			src := strings.Replace(fitFields, "notes/projects", path, 1)
			mistakes(t, parsed(t, src), `part "notes": at must be a path, not "`+path+`"`)
		})
	}
}

const goodPlatform = `
awml: "0.8"
platform: {id: worker, label: Worker}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic, openai]
holds: [notes, mail/drafts]
`

func TestASoundPlatformDescriptionHasNoMistakes(t *testing.T) {
	mistakes(t, parsed(t, goodPlatform))
}

func TestAPlatformDescriptionNeedsAllItsFields(t *testing.T) {
	src := strings.Replace(goodPlatform, "steps: code\n", "title: Worker\n", 1)
	mistakes(t, parsed(t, src),
		`the file has no steps`,
		`the file has unknown field "title"`)
	mistakes(t, parsed(t, strings.Replace(goodPlatform, "awml: \"0.8\"\n", "", 1)), `the file has no awml`)
}

func TestAPlatformDescriptionChecksEveryKindAndValue(t *testing.T) {
	src := `
awml: "0.8"
platform: {id: Worker_1, label: 1, extra: x}
off_for_up_to: P1M
steps: person
agents: [OpenAI, 3]
calls: none
holds: [/notes, notes/..]
`
	mistakes(t, parsed(t, src),
		`platform has unknown field "extra"`,
		`platform: YAML read its label as 1, not text: put it in quotes`,
		`platform id "Worker_1" must be lower-case letters, digits and hyphens, starting with a letter, or owner/platforms/slug for a registered platform`,
		`off_for_up_to must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "P1M"`,
		`steps must be "code" or "agent", not "person"`,
		`agents item "OpenAI" must be a company id`,
		`agents item "3" must be text`,
		`calls must be any or a list of company ids`,
		`holds item "/notes" must be a path`,
		`holds item "notes/.." must be a path`)
}
