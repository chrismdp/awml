package awml

import "strings"

// Summary holds the things in a workflow that an inventory can count, and the names of its AI
// steps, people, scopes and platforms. Groups, parts and lines that appear in both views are
// counted and named once within their kind of view.
type Summary struct {
	Steps           StepCounts        `json:"steps"`
	Operations      map[string]int    `json:"operations"`
	OutsideActions  TierCounts        `json:"outsideActions"`
	Triggers        int               `json:"triggers"`
	Reviews         int               `json:"reviews"`
	Contexts        int               `json:"contexts"`
	Connectors      int               `json:"connectors"`
	Surfaces        int               `json:"surfaces"`
	Scopes          int               `json:"scopes"`
	Platforms       int               `json:"platforms"`
	PublishingLines PublishingSummary `json:"publishingLines"`
	RefusedReads    int               `json:"refusedReads"`
	RedLines        int               `json:"redLines"`
	AISteps         []string          `json:"aiSteps"`
	People          []PersonSummary   `json:"people"`
	ScopeNames      []string          `json:"scopeNames"`
	PlatformNames   []string          `json:"platformNames"`
}

// PersonSummary names a place where a person decides: a person step or a review.
type PersonSummary struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// StepCounts counts steps by who decides them.
type StepCounts struct {
	AI     int `json:"ai"`
	Code   int `json:"code"`
	Person int `json:"person"`
}

// TierCounts counts outside actions by their tier.
type TierCounts struct {
	Allow int `json:"allow"`
	Ask   int `json:"ask"`
	Block int `json:"block"`
}

// PublishingSummary keeps both the number of publishing lines and the label on each one.
type PublishingSummary struct {
	Count  int      `json:"count"`
	Labels []string `json:"labels"`
}

// Summarise counts one workflow from either view or from both views. It expects sound documents,
// as Draw does. A part id identifies the same part in both views; a line is the same when its
// endpoints, kind and label are the same.
func Summarise(docs ...*Map) Summary {
	docs = workflowViews(docs)
	var out Summary
	seenParts := map[string]bool{}
	seenLines := map[string]bool{}
	seenGroups := map[string]bool{}
	for _, doc := range docs {
		view := str(doc, "view")
		for _, group := range items(doc, "groups") {
			key := view + "\x00" + str(group, "id")
			if seenGroups[key] {
				continue
			}
			seenGroups[key] = true
			if view == "security" {
				out.Scopes++
				out.ScopeNames = append(out.ScopeNames, scopeName(str(group, "label")))
			} else if view == "platform" {
				out.Platforms++
				out.PlatformNames = append(out.PlatformNames, str(group, "label"))
			}
		}
		for _, part := range items(doc, "parts") {
			partID := str(part, "id")
			if seenParts[partID] {
				continue
			}
			seenParts[partID] = true
			switch kind := str(part, "kind"); kind {
			case "ai":
				out.Steps.AI++
				out.AISteps = append(out.AISteps, str(part, "label"))
			case "code":
				out.Steps.Code++
			case "person":
				out.Steps.Person++
				out.People = append(out.People, PersonSummary{Label: str(part, "label"), Kind: kind})
			case "outside":
				switch outsideTier(str(part, "label")) {
				case "Allow":
					out.OutsideActions.Allow++
				case "Ask":
					out.OutsideActions.Ask++
				case "Block":
					out.OutsideActions.Block++
				}
			case "trigger":
				out.Triggers++
			case "review":
				out.Reviews++
				out.People = append(out.People, PersonSummary{Label: str(part, "label"), Kind: kind})
			case "context":
				out.Contexts++
			case "connector":
				out.Connectors++
			case "surface":
				out.Surfaces++
			}
			if op := str(part, "op"); op != "" {
				if out.Operations == nil {
					out.Operations = map[string]int{}
				}
				out.Operations[op]++
			}
		}
		for _, line := range items(doc, "lines") {
			key := strings.Join([]string{str(line, "from"), str(line, "to"), str(line, "kind"), str(line, "label")}, "\x00")
			if line.get("breaks") == true {
				key += "\x00red"
			}
			if seenLines[key] {
				continue
			}
			seenLines[key] = true
			if line.get("breaks") == true {
				out.RedLines++
				continue
			}
			switch str(line, "kind") {
			case "publish":
				out.PublishingLines.Count++
				out.PublishingLines.Labels = append(out.PublishingLines.Labels, str(line, "label"))
			case "refused":
				out.RefusedReads++
			}
		}
	}
	return out
}

// TotalSummaries adds per-workflow summaries into an organisation inventory total.
func TotalSummaries(summaries ...Summary) Summary {
	var out Summary
	for _, s := range summaries {
		out.Steps.AI += s.Steps.AI
		out.Steps.Code += s.Steps.Code
		out.Steps.Person += s.Steps.Person
		for op, count := range s.Operations {
			if out.Operations == nil {
				out.Operations = map[string]int{}
			}
			out.Operations[op] += count
		}
		out.OutsideActions.Allow += s.OutsideActions.Allow
		out.OutsideActions.Ask += s.OutsideActions.Ask
		out.OutsideActions.Block += s.OutsideActions.Block
		out.Triggers += s.Triggers
		out.Reviews += s.Reviews
		out.Contexts += s.Contexts
		out.Connectors += s.Connectors
		out.Surfaces += s.Surfaces
		out.Scopes += s.Scopes
		out.Platforms += s.Platforms
		out.PublishingLines.Count += s.PublishingLines.Count
		out.PublishingLines.Labels = append(out.PublishingLines.Labels, s.PublishingLines.Labels...)
		out.RefusedReads += s.RefusedReads
		out.RedLines += s.RedLines
		out.AISteps = append(out.AISteps, s.AISteps...)
		out.People = append(out.People, s.People...)
		out.ScopeNames = append(out.ScopeNames, s.ScopeNames...)
		out.PlatformNames = append(out.PlatformNames, s.PlatformNames...)
	}
	return out
}

// scopeName reads a scope's name from its label, which gives the name before a colon and the
// members after it, as in `family: parents, kids`.
func scopeName(label string) string {
	if name, _, found := strings.Cut(label, ":"); found {
		return strings.TrimSpace(name)
	}
	return label
}

// outsideTier reads the tier after the last colon in the outside action's label. Text after the
// tier holds limits or the name of a standing approval.
func outsideTier(label string) string {
	if colon := strings.LastIndex(label, ":"); colon >= 0 {
		label = label[colon+1:]
	}
	words := strings.Fields(label)
	if len(words) == 0 {
		return ""
	}
	word := strings.Trim(words[0], ",.;")
	if word == "Allow" || word == "Ask" || word == "Block" {
		return word
	}
	return ""
}
