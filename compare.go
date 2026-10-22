package awml

import "fmt"

// CompareViews finds parts and lines that differ between the two views of one workflow. It expects
// a sound security view followed by a sound platform view with the same title. Other documents do
// not form a pair, so it reports nothing for them.
func CompareViews(security, platform any) []string {
	securityFile, securityOK := security.(*Map)
	platformFile, platformOK := platform.(*Map)
	if !securityOK || !platformOK || len(Check(securityFile)) != 0 || len(Check(platformFile)) != 0 ||
		str(securityFile, "view") != "security" || str(platformFile, "view") != "platform" ||
		str(securityFile, "title") != str(platformFile, "title") {
		return nil
	}

	securityParts := partsByID(securityFile)
	platformParts := partsByID(platformFile)
	var mistakes []string
	for _, part := range items(platformFile, "parts") {
		id := str(part, "id")
		other := securityParts[id]
		if other == nil {
			if str(part, "kind") == "comment" {
				continue
			}
			mistakes = append(mistakes, fmt.Sprintf(`part "%s" is in the platform view but not the security view`, id))
			continue
		}
		if platformKind, securityKind := str(part, "kind"), str(other, "kind"); platformKind != securityKind {
			mistakes = append(mistakes, fmt.Sprintf(`part "%s" has kind "%s" in the platform view but "%s" in the security view`,
				id, platformKind, securityKind))
		}
	}

	// The platform view may leave out a trigger or comment that does not help it. Lines touching
	// one of those parts are absent for the same reason.
	omitted := map[string]bool{}
	for _, part := range items(securityFile, "parts") {
		id, kind := str(part, "id"), str(part, "kind")
		if platformParts[id] != nil {
			continue
		}
		if kind == "trigger" || kind == "comment" {
			omitted[id] = true
			continue
		}
		mistakes = append(mistakes, fmt.Sprintf(`part "%s" is in the security view but not the platform view`, id))
	}

	securityLines := comparableLines(securityFile, omitted)
	platformLines := comparableLines(platformFile, omitted)
	reported := map[comparableLineKey]bool{}
	for _, line := range items(securityFile, "lines") {
		key, compare := comparableLine(line, omitted)
		if compare && !platformLines[key] && !reported[key] {
			mistakes = append(mistakes, lineDifference(key, "security", "platform"))
			reported[key] = true
		}
	}
	reported = map[comparableLineKey]bool{}
	for _, line := range items(platformFile, "lines") {
		key, compare := comparableLine(line, omitted)
		if compare && !securityLines[key] && !reported[key] {
			mistakes = append(mistakes, lineDifference(key, "platform", "security"))
			reported[key] = true
		}
	}
	return mistakes
}

func partsByID(file *Map) map[string]*Map {
	parts := map[string]*Map{}
	for _, part := range items(file, "parts") {
		parts[str(part, "id")] = part
	}
	return parts
}

type comparableLineKey struct {
	from string
	to   string
	kind string
}

func comparableLines(file *Map, omitted map[string]bool) map[comparableLineKey]bool {
	lines := map[comparableLineKey]bool{}
	for _, line := range items(file, "lines") {
		if key, compare := comparableLine(line, omitted); compare {
			lines[key] = true
		}
	}
	return lines
}

func comparableLine(line *Map, omitted map[string]bool) (comparableLineKey, bool) {
	key := comparableLineKey{str(line, "from"), str(line, "to"), str(line, "kind")}
	return key, key.kind != "tie" && key.kind != "hidden" && !omitted[key.from] && !omitted[key.to]
}

func lineDifference(line comparableLineKey, in, notIn string) string {
	return fmt.Sprintf(`the %s line from "%s" to "%s" is in the %s view but not the %s view`,
		line.kind, line.from, line.to, in, notIn)
}
