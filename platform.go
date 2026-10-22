package awml

import "fmt"

func checkPlatformDescription(file *Map) []string {
	var errors []string
	if file.has("awml") {
		version, ok := file.get("awml").(string)
		if !ok {
			errors = append(errors, fmt.Sprintf("awml: YAML read it as %s, not text: put it in quotes", show(file.get("awml"))))
		} else if version != "0.3" && version != "0.4" {
			errors = append(errors, fmt.Sprintf(`awml must be "0.3" for a platform description, not "%s"`, version))
		}
	}
	platformFileFields(file, &errors)
	if value := file.get("platform"); file.has("platform") {
		platform, ok := value.(*Map)
		if !ok {
			errors = append(errors, "platform must be a mapping of fields")
		} else {
			fieldsExact("platform", platform, []string{"id", "label"}, nil, &errors)
			if id, ok := platform.get("id").(string); ok && !companyID.MatchString(id) {
				errors = append(errors, fmt.Sprintf(`platform id "%s" must be lower-case letters, digits and hyphens, starting with a letter`, id))
			}
		}
	}
	if value := file.get("off_for_up_to"); file.has("off_for_up_to") {
		text, ok := value.(string)
		if !ok || !validDuration(text) {
			errors = append(errors, fmt.Sprintf(`off_for_up_to must be an ISO 8601 duration using weeks, days, hours, minutes or seconds, not "%s"`, show(value)))
		}
	}
	if value := file.get("steps"); file.has("steps") && !isOneOf(value, "code", "agent") {
		errors = append(errors, fmt.Sprintf(`steps must be "code" or "agent", not "%s"`, show(value)))
	}
	checkCompanyList(file, "agents", false, &errors)
	checkCompanyList(file, "calls", true, &errors)
	if value := file.get("holds"); file.has("holds") {
		list, ok := value.([]any)
		if !ok {
			errors = append(errors, "holds must be a list of paths")
		} else {
			for _, item := range list {
				path, ok := item.(string)
				if !ok || !validPath(path) {
					errors = append(errors, fmt.Sprintf(`holds item "%s" must be a path`, show(item)))
				}
			}
		}
	}
	return errors
}

func platformFileFields(file *Map, errors *[]string) {
	need := []string{"awml", "platform", "off_for_up_to", "steps", "agents", "calls", "holds"}
	allowed := setOf(need)
	for _, field := range need {
		if !file.has(field) {
			*errors = append(*errors, "the file has no "+field)
		}
	}
	for _, field := range file.Keys() {
		if !allowed[field] {
			*errors = append(*errors, fmt.Sprintf(`the file has unknown field "%s"`, field))
		}
	}
}

func fieldsExact(name string, item *Map, needFields, mayFields []string, errors *[]string) {
	need, may := setOf(needFields), setOf(mayFields)
	for _, field := range needFields {
		if !item.has(field) {
			*errors = append(*errors, name+" has no "+field)
		}
	}
	for _, field := range item.Keys() {
		if !need[field] && !may[field] {
			*errors = append(*errors, fmt.Sprintf(`%s has unknown field "%s"`, name, field))
		}
	}
	for _, field := range item.Keys() {
		if (field == "id" || field == "label") && item.get(field) != nil {
			if _, ok := item.get(field).(string); !ok {
				*errors = append(*errors, fmt.Sprintf("%s: YAML read its %s as %s, not text: put it in quotes", name, field, show(item.get(field))))
			}
		}
	}
}

func checkCompanyList(file *Map, field string, acceptsAny bool, errors *[]string) {
	value := file.get(field)
	if !file.has(field) {
		return
	}
	if acceptsAny && value == "any" {
		return
	}
	list, ok := value.([]any)
	if !ok {
		wording := "a list of company ids"
		if acceptsAny {
			wording = "any or " + wording
		}
		*errors = append(*errors, field+" must be "+wording)
		return
	}
	for _, item := range list {
		company, ok := item.(string)
		if !ok {
			*errors = append(*errors, fmt.Sprintf(`%s item "%s" must be text`, field, show(item)))
		} else if !companyID.MatchString(company) {
			*errors = append(*errors, fmt.Sprintf(`%s item "%s" must be a company id`, field, company))
		}
	}
}
