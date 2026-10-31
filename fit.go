package awml

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// FitResult is the verdict on one platform, or on one block of a drawn placement: whether it was
// judged, whether the workflow fits, every reason it doesn't, and the companies each AI step there
// may use. A block that wasn't judged because no descriptions were given has that as its reason.
// It is what `awml fit --json` prints for each platform.
type FitResult struct {
	Platform string              `json:"platform"`
	Judged   bool                `json:"judged"`
	Fits     bool                `json:"fits"`
	Reasons  []string            `json:"reasons"`
	Models   map[string][]string `json:"models"`
	steps    []string
}

// FitReport is the whole of `awml fit --json`: a result for each platform, or the gaps that kept
// the workflow from being judged.
type FitReport struct {
	Results []FitResult `json:"results"`
	Gaps    []string    `json:"gaps"`
}

// Fit judges a workflow, or its security view, against platform descriptions and gives the lines
// `awml fit` prints.
// Gaps mean the placement could not be judged.
func Fit(security, placement *Map, descriptions []*Map) (lines, gaps []string) {
	results, gaps := judge(security, placement, descriptions)
	if len(gaps) != 0 {
		return nil, gaps
	}
	return FitLines(results), nil
}

// Judge judges a workflow, or its security view, against platform descriptions, either as
// whole-workflow candidates or in the placement drawn by one of its platform views. The documents
// must have no mistakes but lines that break a rule.
func Judge(security, placement *Map, descriptions []*Map) FitReport {
	results, gaps := judge(security, placement, descriptions)
	if results == nil {
		results = []FitResult{}
	}
	if gaps == nil {
		gaps = []string{}
	}
	return FitReport{Results: results, Gaps: gaps}
}

func judge(security, placement *Map, descriptions []*Map) ([]FitResult, []string) {
	return judgeModel(securityFitModel(security), placement, descriptions)
}

// securityFitModel gives what fit judges of one workflow, or of its security view.
func securityFitModel(security *Map) fitModel {
	if IsCanonical(security) {
		security, _ = View(security, "")
	}
	return newFitModel(security)
}

func judgeModel(model fitModel, placement *Map, descriptions []*Map) (results []FitResult, gaps []string) {
	platforms, described, gaps := describe(model, descriptions)
	if len(gaps) != 0 {
		return nil, gaps
	}
	if placement == nil {
		for _, platform := range platforms {
			results = append(results, judgePlatform(platform.id, platform, model, model.parts, model.parts))
		}
		return results, nil
	}

	groups := items(placement, "groups")
	parts := items(placement, "parts")
	where := placed(parts, items(placement, "lines"))
	parents := map[string]string{}
	for _, group := range groups {
		parents[str(group, "id")] = str(group, "in")
	}
	for _, group := range groups {
		groupID, platformID := str(group, "id"), str(group, "platform")
		if platformID == "" {
			results = append(results, FitResult{Platform: groupID, Reasons: []string{}, Models: map[string][]string{}})
			continue
		}
		platform, ok := described[platformID]
		if !ok && len(descriptions) == 0 {
			results = append(results, FitResult{Platform: groupID, Reasons: []string{fmt.Sprintf(`platform "%s" wasn't described`, platformID)}, Models: map[string][]string{}})
			continue
		}
		if !ok {
			gap := fmt.Sprintf(`group "%s" names platform "%s", but its description wasn't given`, groupID, platformID)
			if registeredPlatform.MatchString(platformID) {
				gap += ": pull it with awml pull " + platformID
			}
			gaps = append(gaps, gap)
			continue
		}
		var own, visible []*Map
		for _, part := range parts {
			home := where[str(part, "id")]
			if home == groupID {
				own = append(own, part)
			}
			if within(home, groupID, parents) {
				visible = append(visible, part)
			}
		}
		results = append(results, judgePlatform(groupID, platform, model, own, visible))
	}
	if len(gaps) != 0 {
		return nil, gaps
	}
	return results, nil
}

// describe reads the platform descriptions, in the order given and by id, or gives the gaps that
// keep the workflow from being judged: its own, and a description given twice.
func describe(model fitModel, descriptions []*Map) ([]fitPlatform, map[string]fitPlatform, []string) {
	if gaps := model.gaps(); len(gaps) != 0 {
		return nil, nil, gaps
	}
	return describePlatforms(descriptions)
}

// describePlatforms reads descriptions in their given order and matches them by platform id.
func describePlatforms(descriptions []*Map) ([]fitPlatform, map[string]fitPlatform, []string) {
	var gaps []string
	var platforms []fitPlatform
	described := map[string]fitPlatform{}
	for _, description := range descriptions {
		platform := readFitPlatform(description)
		if _, exists := described[platform.id]; exists {
			gaps = append(gaps, fmt.Sprintf(`platform description "%s" was given twice`, platform.id))
			continue
		}
		described[platform.id] = platform
		platforms = append(platforms, platform)
	}
	return platforms, described, gaps
}

type fitModel struct {
	parts      []*Map
	groups     []*Map
	byID       map[string]*Map
	where      map[string]string
	scopes     map[string]*Map
	processing map[string]processingTerms
	// names gives the name a part or scope is reported by, where it isn't its id.
	names map[string]string
}

// name gives the name a part or scope is reported by.
func (model fitModel) name(id string) string {
	if name, ok := model.names[id]; ok {
		return name
	}
	return id
}

func newFitModel(security *Map) fitModel {
	model := fitModel{
		parts:      items(security, "parts"),
		groups:     items(security, "groups"),
		byID:       map[string]*Map{},
		scopes:     map[string]*Map{},
		processing: map[string]processingTerms{},
	}
	model.where = placed(model.parts, items(security, "lines"))
	for _, part := range model.parts {
		model.byID[str(part, "id")] = part
	}
	for _, scope := range model.groups {
		id := str(scope, "id")
		model.scopes[id] = scope
		if scope.has("processing") {
			model.processing[id], _ = readProcessing(scope.get("processing"), "")
		}
	}
	return model
}

func (model fitModel) gaps() []string {
	var gaps []string
	for _, scope := range model.scopesInOrder() {
		if !scope.has("processing") {
			gaps = append(gaps, fmt.Sprintf(`group "%s" has no processing`, model.name(str(scope, "id"))))
		}
	}
	for _, part := range model.parts {
		if model.where[str(part, "id")] == "" && isScopedPart(str(part, "kind")) {
			gaps = append(gaps, fmt.Sprintf(`part "%s" is outside every scope`, model.name(str(part, "id"))))
		}
	}
	for _, part := range model.parts {
		if str(part, "kind") == "trigger" && !part.has("wait") {
			gaps = append(gaps, fmt.Sprintf(`trigger "%s" has no wait`, model.name(str(part, "id"))))
		}
	}
	return gaps
}

func (model fitModel) scopesInOrder() []*Map {
	return model.groups
}

func isScopedPart(kind string) bool {
	return kind == "ai" || kind == "code" || kind == "person" || kind == "context" || kind == "connector"
}

type fitPlatform struct {
	id, offText string
	off         time.Duration
	steps       string
	agents      []string
	calls       []string
	callsAny    bool
	holds       []string
}

func readFitPlatform(description *Map) fitPlatform {
	platform := description.get("platform").(*Map)
	off, _ := duration(str(description, "off_for_up_to"))
	result := fitPlatform{id: str(platform, "id"), offText: str(description, "off_for_up_to"), off: off, steps: str(description, "steps")}
	result.agents = textList(description.get("agents"))
	if description.get("calls") == "any" {
		result.callsAny = true
	} else {
		result.calls = textList(description.get("calls"))
	}
	result.holds = textList(description.get("holds"))
	return result
}

func textList(value any) []string {
	list, _ := value.([]any)
	result := make([]string, 0, len(list))
	for _, item := range list {
		result = append(result, item.(string))
	}
	return result
}

func judgePlatform(name string, platform fitPlatform, model fitModel, own, visible []*Map) FitResult {
	result := FitResult{Platform: name, Judged: true, Reasons: []string{}, Models: map[string][]string{}}

	// Rule 1: each trigger belongs to its own platform, including one placed by its first line.
	for _, part := range own {
		securityPart := model.byID[str(part, "id")]
		if securityPart == nil || str(securityPart, "kind") != "trigger" {
			continue
		}
		waitText := str(securityPart, "wait")
		wait, _ := duration(waitText)
		if wait < platform.off {
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s may wait %s, but the platform may be off for %s", model.name(str(part, "id")), waitText, platform.offText))
		}
	}

	// Rule 2: agents can read every relevant thing in their block and blocks nested inside it.
	seenExposure := map[string]bool{}
	for _, company := range platform.agents {
		for _, part := range visible {
			if !countsForAccess(str(part, "kind")) {
				continue
			}
			scope := model.where[str(part, "id")]
			if scope == "" || model.allowsAny(scope, company) {
				continue
			}
			key := company + "\x00" + scope
			if !seenExposure[key] {
				seenExposure[key] = true
				result.Reasons = append(result.Reasons, fmt.Sprintf("%s's agents run here, and %s doesn't allow %s", company, model.name(scope), company))
			}
		}
	}

	// Rule 3: each AI step has at least one reachable, allowed company.
	for _, part := range own {
		securityPart := model.byID[str(part, "id")]
		if securityPart == nil || str(securityPart, "kind") != "ai" {
			continue
		}
		companies := model.companiesFor(model.where[str(part, "id")], str(securityPart, "op"), platform)
		if len(companies) == 0 {
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s has no company it may use", model.name(str(part, "id"))))
		} else {
			result.Models[model.name(str(part, "id"))] = companies
		}
	}

	// Rule 4: every existing context is held by the platform.
	for _, part := range own {
		securityPart := model.byID[str(part, "id")]
		if securityPart == nil || str(securityPart, "kind") != "context" || !securityPart.has("at") {
			continue
		}
		at := str(securityPart, "at")
		if !holds(platform.holds, at) {
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s is kept at %s, which the platform doesn't hold", model.name(str(part, "id")), at))
		}
	}

	// Rule 5: code-shaped work needs the workflow's own code runner.
	if platform.steps != "code" {
		var code []string
		for _, part := range own {
			if securityPart := model.byID[str(part, "id")]; securityPart != nil && str(securityPart, "kind") == "code" {
				code = append(code, model.name(str(part, "id")))
			}
		}
		if len(code) != 0 {
			predicate := "are code steps"
			if len(code) == 1 {
				predicate = "is a code step"
			}
			who := "an agent"
			if len(platform.agents) == 1 {
				who = platform.agents[0] + "'s agent"
			} else if len(platform.agents) > 1 {
				who = naturalList(platform.agents) + " agents"
			}
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s %s, which %s would carry out", naturalList(code), predicate, who))
		}
	}

	for _, part := range own {
		if _, ok := result.Models[model.name(str(part, "id"))]; ok {
			result.steps = append(result.steps, model.name(str(part, "id")))
		}
	}
	result.Fits = len(result.Reasons) == 0
	return result
}

func countsForAccess(kind string) bool {
	return kind == "ai" || kind == "code" || kind == "context" || kind == "connector"
}

func (model fitModel) allowsAny(scope, company string) bool {
	terms := model.processing[scope]
	return terms.all || terms.companies[company].all
}

func (model fitModel) companiesFor(scope, operation string, platform fitPlatform) []string {
	terms := model.processing[scope]
	if terms.all && platform.callsAny {
		return []string{"any"}
	}
	reachable := map[string]bool{}
	for _, company := range append(append([]string{}, platform.agents...), platform.calls...) {
		reachable[company] = true
	}
	if platform.callsAny {
		for company := range terms.companies {
			reachable[company] = true
		}
	}
	var companies []string
	for company := range reachable {
		allowed := terms.all
		if companyTerms := terms.companies[company]; companyTerms.all || companyTerms.ops[operation] {
			allowed = true
		}
		if allowed {
			companies = append(companies, company)
		}
	}
	sort.Strings(companies)
	return companies
}

func holds(roots []string, path string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func fitMessage(result FitResult) string {
	type choice struct {
		companies string
		steps     []string
	}
	var choices []choice
	byCompanies := map[string]int{}
	for _, id := range result.steps {
		allowed := result.Models[id]
		key := strings.Join(allowed, " or ")
		position, found := byCompanies[key]
		if !found {
			position = len(choices)
			byCompanies[key] = position
			choices = append(choices, choice{companies: key})
		}
		choices[position].steps = append(choices[position].steps, id)
	}
	if len(choices) == 0 {
		return "fits"
	}
	clauses := make([]string, 0, len(choices))
	for _, choice := range choices {
		clauses = append(clauses, naturalList(choice.steps)+" may use "+choice.companies)
	}
	return "fits: " + strings.Join(clauses, "; ")
}

func within(group, outer string, parents map[string]string) bool {
	for seen := map[string]bool{}; group != "" && !seen[group]; group = parents[group] {
		if group == outer {
			return true
		}
		seen[group] = true
	}
	return false
}

// FitLines gives the lines `awml fit` prints for each result: one that names the companies each AI
// step may use where the workflow fits, and one for each reason where it doesn't.
func FitLines(results []FitResult) []string {
	width := 0
	for _, result := range results {
		if len(result.Platform) > width {
			width = len(result.Platform)
		}
	}
	var lines []string
	for _, result := range results {
		var messages []string
		switch {
		case !result.Judged && len(result.Reasons) != 0:
			messages = []string{"not judged: " + result.Reasons[0]}
		case !result.Judged:
			messages = []string{"not judged: no platform"}
		case result.Fits:
			messages = []string{fitMessage(result)}
		default:
			for _, reason := range result.Reasons {
				messages = append(messages, "no: "+reason)
			}
		}
		for i, message := range messages {
			name := result.Platform
			if i > 0 {
				name = ""
			}
			lines = append(lines, fmt.Sprintf("%-*s  %s", width, name, message))
		}
	}
	return lines
}

// PartsReport is the whole of `awml fit --parts --json`: for each part, the platforms that can hold
// it alone; each option of platforms that between them hold every part, with none to spare; and the
// gaps that kept the workflow from being judged.
type PartsReport struct {
	Parts   []PartFit     `json:"parts"`
	Options []PartsOption `json:"options"`
	Gaps    []string      `json:"gaps"`
}

// PartsOption is one set of platforms that between them hold every part, from which none could be
// dropped, in the order the platforms were given, and its placement: for each part, in the order of
// the parts, the platforms in the option that can hold it.
type PartsOption struct {
	Platforms []string        `json:"platforms"`
	Placement []PartPlacement `json:"placement"`
}

// PartPlacement is where one part of an option may go: every platform in the option that holds it.
type PartPlacement struct {
	Part      string   `json:"part"`
	Platforms []string `json:"platforms"`
}

// PartFit is the verdict on one part: the platforms that can hold it, in the order given, why each
// of the others can't, and the companies an AI step may use on each platform that holds it.
type PartFit struct {
	Part      string              `json:"part"`
	Platforms []string            `json:"platforms"`
	Reasons   []PlatformReasons   `json:"reasons"`
	Models    map[string][]string `json:"models"`
}

// PlatformReasons is every reason one platform can't hold a part.
type PlatformReasons struct {
	Platform string   `json:"platform"`
	Reasons  []string `json:"reasons"`
}

// JudgeParts judges each trigger, step that runs on a platform, context and connector of the
// workflows given on its own against each platform, by the same rules as Judge, and finds every
// option of platforms that between them hold them all with none to spare. Whether parts on different
// platforms can reach each other isn't judged.
func JudgeParts(workflows []Workflow, descriptions []*Map) PartsReport {
	model := securityFitModel(workflows[0].Doc)
	if len(workflows) > 1 {
		model = togetherFitModel(workflows)
	}
	report := PartsReport{Parts: []PartFit{}, Options: []PartsOption{}, Gaps: []string{}}
	platforms, _, gaps := describe(model, descriptions)
	if len(gaps) != 0 {
		report.Gaps = gaps
		return report
	}
	var held [][]bool
	for _, part := range model.parts {
		if kind := str(part, "kind"); kind != "trigger" && !countsForAccess(kind) {
			continue
		}
		name := model.name(str(part, "id"))
		verdict := PartFit{Part: name, Platforms: []string{}, Reasons: []PlatformReasons{}, Models: map[string][]string{}}
		holders := make([]bool, len(platforms))
		for i, platform := range platforms {
			result := judgePlatform(platform.id, platform, model, []*Map{part}, []*Map{part})
			if !result.Fits {
				verdict.Reasons = append(verdict.Reasons, PlatformReasons{Platform: platform.id, Reasons: result.Reasons})
				continue
			}
			holders[i] = true
			verdict.Platforms = append(verdict.Platforms, platform.id)
			if companies, ok := result.Models[name]; ok {
				verdict.Models[platform.id] = companies
			}
		}
		report.Parts = append(report.Parts, verdict)
		held = append(held, holders)
	}
	report.Options = options(platforms, report.Parts, held)
	return report
}

// options gives every set of platforms that between them hold every part and from which no platform
// could be dropped, smallest first, then in the order the platforms were given, each with its
// placement. A set that adds a platform to one of these isn't given, and none is given where some
// part no platform holds. Trying every combination is fine for the handful of platforms a person
// describes.
func options(platforms []fitPlatform, parts []PartFit, held [][]bool) []PartsOption {
	found := []PartsOption{}
	// covers says whether the chosen platforms, leaving out the one at skip, hold every part.
	covers := func(chosen []int, skip int) bool {
		for _, holders := range held {
			covered := false
			for j, i := range chosen {
				covered = covered || (j != skip && holders[i])
			}
			if !covered {
				return false
			}
		}
		return true
	}
	var chosen []int
	var choose func(from, size int)
	choose = func(from, size int) {
		if len(chosen) == size {
			if !covers(chosen, -1) {
				return
			}
			for skip := range chosen {
				if covers(chosen, skip) {
					return
				}
			}
			option := PartsOption{Platforms: make([]string, 0, size), Placement: make([]PartPlacement, 0, len(parts))}
			for _, i := range chosen {
				option.Platforms = append(option.Platforms, platforms[i].id)
			}
			for p, part := range parts {
				placement := PartPlacement{Part: part.Part, Platforms: []string{}}
				for _, i := range chosen {
					if held[p][i] {
						placement.Platforms = append(placement.Platforms, platforms[i].id)
					}
				}
				option.Placement = append(option.Placement, placement)
			}
			found = append(found, option)
			return
		}
		for i := from; i <= len(platforms)-(size-len(chosen)); i++ {
			chosen = append(chosen, i)
			choose(i+1, size)
			chosen = chosen[:len(chosen)-1]
		}
	}
	for size := 1; size <= len(platforms); size++ {
		choose(0, size)
	}
	return found
}

// PartsLines gives the lines `awml fit --parts` prints: for each part, the platforms that hold it
// and why each other one doesn't, then each option with where each part goes in it, or the parts
// that no platform holds.
func PartsLines(report PartsReport) []string {
	width := 0
	for _, part := range report.Parts {
		if len(part.Part) > width {
			width = len(part.Part)
		}
	}
	var lines, unheld []string
	for _, part := range report.Parts {
		holders := "none"
		if len(part.Platforms) != 0 {
			holders = naturalList(part.Platforms)
		} else {
			unheld = append(unheld, part.Part)
		}
		lines = append(lines, fmt.Sprintf("%-*s  held by %s", width, part.Part, holders))
		for _, platform := range part.Reasons {
			for _, reason := range platform.Reasons {
				lines = append(lines, fmt.Sprintf("%-*s  not by %s: %s", width, "", platform.Platform, reason))
			}
		}
	}
	if len(unheld) != 0 {
		verb := "are"
		if len(unheld) == 1 {
			verb = "is"
		}
		return append(lines, fmt.Sprintf("no placement: %s %s held by no platform given", naturalList(unheld), verb))
	}
	several := false
	for _, option := range report.Options {
		lines = append(lines, "option: "+naturalList(option.Platforms))
		for _, placement := range option.Placement {
			lines = append(lines, fmt.Sprintf("  %-*s  on %s", width, placement.Part, strings.Join(placement.Platforms, " or ")))
		}
		several = several || len(option.Platforms) > 1
	}
	if several {
		lines = append(lines, "an option on several platforms is judged part by part only: whether its parts can reach each other isn't judged")
	}
	return lines
}
