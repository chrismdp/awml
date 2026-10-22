package awml

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// FitResult is the verdict on one platform, or on one block of a drawn placement: whether it was
// judged, whether the workflow fits, every reason it doesn't, and the companies each AI step there
// may use. It is what `awml fit --json` prints for each platform.
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

// Fit judges a security view against platform descriptions and gives the lines `awml fit` prints.
// Gaps mean the placement could not be judged.
func Fit(security, placement *Map, descriptions []*Map) (lines, gaps []string) {
	results, gaps := judge(security, placement, descriptions)
	if len(gaps) != 0 {
		return nil, gaps
	}
	return FitLines(results), nil
}

// Judge judges a security view against platform descriptions, either as whole-workflow candidates
// or in the placement drawn by a platform view. The documents must have passed Check.
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

func judge(security, placement *Map, descriptions []*Map) (results []FitResult, gaps []string) {
	security, _ = View(security, "")
	model := newFitModel(security)
	if gaps = model.gaps(); len(gaps) != 0 {
		return nil, gaps
	}
	described := map[string]fitPlatform{}
	for _, description := range descriptions {
		platform := readFitPlatform(description)
		if _, exists := described[platform.id]; exists {
			gaps = append(gaps, fmt.Sprintf(`platform description "%s" was given twice`, platform.id))
			continue
		}
		described[platform.id] = platform
	}
	if len(gaps) != 0 {
		return nil, gaps
	}
	if placement == nil {
		for _, description := range descriptions {
			platform := readFitPlatform(description)
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
		if !ok {
			gaps = append(gaps, fmt.Sprintf(`group "%s" names platform "%s", but its description wasn't given`, groupID, platformID))
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

type fitModel struct {
	parts      []*Map
	groups     []*Map
	byID       map[string]*Map
	where      map[string]string
	scopes     map[string]*Map
	processing map[string]processingTerms
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
			gaps = append(gaps, fmt.Sprintf(`group "%s" has no processing`, str(scope, "id")))
		}
	}
	for _, part := range model.parts {
		if model.where[str(part, "id")] == "" && isScopedPart(str(part, "kind")) {
			gaps = append(gaps, fmt.Sprintf(`part "%s" is outside every scope`, str(part, "id")))
		}
	}
	for _, part := range model.parts {
		if str(part, "kind") == "trigger" && !part.has("wait") {
			gaps = append(gaps, fmt.Sprintf(`trigger "%s" has no wait`, str(part, "id")))
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
		if str(part, "kind") != "trigger" {
			continue
		}
		securityPart := model.byID[str(part, "id")]
		if securityPart == nil {
			continue
		}
		waitText := str(securityPart, "wait")
		wait, _ := duration(waitText)
		if wait < platform.off {
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s may wait %s, but the platform may be off for %s", str(part, "id"), waitText, platform.offText))
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
				result.Reasons = append(result.Reasons, fmt.Sprintf("%s's agents run here, and %s doesn't allow %s", company, scope, company))
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
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s has no company it may use", str(part, "id")))
		} else {
			result.Models[str(part, "id")] = companies
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
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s is kept at %s, which the platform doesn't hold", str(part, "id"), at))
		}
	}

	// Rule 5: code-shaped work needs the workflow's own code runner.
	if platform.steps != "code" {
		var code []string
		for _, part := range own {
			if securityPart := model.byID[str(part, "id")]; securityPart != nil && str(securityPart, "kind") == "code" {
				code = append(code, str(part, "id"))
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
		if _, ok := result.Models[str(part, "id")]; ok {
			result.steps = append(result.steps, str(part, "id"))
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
