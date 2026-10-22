package awml

import (
	"strings"
	"testing"
)

const good03 = `
awml: "0.3"
title: A reply
view: security
groups:
  - {id: private, label: private, processing: any}
  - {id: business, label: business, in: private, processing: {anthropic: any, openai: [sort, draft]}}
parts:
  - {id: hourly, kind: trigger, label: Hourly, wait: PT2H, in: private}
  - {id: notes, kind: context, label: Notes, at: notes/projects, in: business}
  - {id: choose, kind: ai, op: sort, label: Choose, in: business}
lines:
  - {from: hourly, to: choose, kind: start}
  - {from: notes, to: choose, kind: data}
`

func TestVersion03TakesTheNewWorkflowFields(t *testing.T) {
	mistakes(t, parsed(t, good03))
	platform := strings.Replace(good03, "view: security", "view: platform", 1)
	platform = strings.Replace(platform, ", processing: any", ", platform: worker", 1)
	platform = strings.Replace(platform, ", processing: {anthropic: any, openai: [sort, draft]}", "", 1)
	mistakes(t, parsed(t, platform))
}

func TestVersion02ReportsEveryVersion03Field(t *testing.T) {
	doc := parsed(t, strings.Replace(good03, `awml: "0.3"`, `awml: "0.2"`, 1))
	mistakes(t, doc,
		`group "private" has unknown field "processing"`,
		`group "business" has unknown field "processing"`,
		`part "hourly" has unknown field "wait"`,
		`part "notes" has unknown field "at"`)
}

func TestProcessingBelongsOnlyToSecurityGroupsAndPlatformOnlyToPlatformGroups(t *testing.T) {
	security := parsed(t, strings.Replace(good03, "processing: any", "platform: worker", 1))
	mistakes(t, security,
		`group "private" has a platform, but only a group in the platform view takes one`)

	platform := strings.Replace(good03, "view: security", "view: platform", 1)
	mistakes(t, parsed(t, platform),
		`group "private" has processing, but only a group in the security view takes it`,
		`group "business" has processing, but only a group in the security view takes it`)
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
			src := strings.Replace(good03, "processing: any", "processing: "+tc.value, 1)
			mistakes(t, parsed(t, src), tc.want)
		})
	}
}

func TestAnInnerScopeCannotLoosenProcessing(t *testing.T) {
	src := strings.Replace(good03, "processing: any", "processing: {anthropic: [sort], openai: [sort]}", 1)
	mistakes(t, parsed(t, src),
		`group "business" allows anthropic to process any operation, but outer group "private" allows it only for sort`,
		`group "business" allows openai to draft, but outer group "private" doesn't`)
}

func TestDurationsUseOnlyFixedLengthISO8601Units(t *testing.T) {
	for _, duration := range []string{"P2W", "P3DT4H5M6S", "PT0S", "PT1.5H"} {
		t.Run("good_"+duration, func(t *testing.T) {
			src := strings.Replace(good03, "PT2H", duration, 1)
			mistakes(t, parsed(t, src))
		})
	}
	for _, duration := range []string{"P1Y", "P2M", "PT", "2 hours"} {
		t.Run("bad_"+duration, func(t *testing.T) {
			src := strings.Replace(good03, "PT2H", duration, 1)
			mistakes(t, parsed(t, src), `part "hourly": wait must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "`+duration+`"`)
		})
	}
}

func TestWaitBelongsToATriggerAndAtToAContext(t *testing.T) {
	src := strings.Replace(good03, "kind: trigger", "kind: code", 1)
	src = strings.Replace(src, "kind: context", "kind: surface", 1)
	mistakes(t, parsed(t, src),
		`part "hourly" has a wait, but only a trigger takes one`,
		`part "notes" has an at, but only a context takes one`)
}

func TestAtIsARelativeStorePath(t *testing.T) {
	for _, path := range []string{"/notes/projects", "notes/../projects", "notes//projects", "notes projects"} {
		t.Run(path, func(t *testing.T) {
			src := strings.Replace(good03, "notes/projects", path, 1)
			mistakes(t, parsed(t, src), `part "notes": at must be a path, not "`+path+`"`)
		})
	}
}

const goodPlatform = `
awml: "0.3"
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

func TestAPlatformDescriptionNeedsVersion03AndAllItsFields(t *testing.T) {
	src := strings.Replace(goodPlatform, `awml: "0.3"`, `awml: "0.2"`, 1)
	src = strings.Replace(src, "steps: code\n", "title: Worker\n", 1)
	mistakes(t, parsed(t, src),
		`awml must be "0.3" for a platform description, not "0.2"`,
		`the file has no steps`,
		`the file has unknown field "title"`)
}

func TestAPlatformDescriptionChecksEveryKindAndValue(t *testing.T) {
	src := `
awml: "0.3"
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
		`platform id "Worker_1" must be lower-case letters, digits and hyphens, starting with a letter`,
		`off_for_up_to must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "P1M"`,
		`steps must be "code" or "agent", not "person"`,
		`agents item "OpenAI" must be a company id`,
		`agents item "3" must be text`,
		`calls must be any or a list of company ids`,
		`holds item "/notes" must be a path`,
		`holds item "notes/.." must be a path`)
}
