package state

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ValidSchema returns the expected JSON schema as a formatted string.
func ValidSchema() string {
	return `{
  "specs": [
    {
      "name": "<string, required>",
      "domain": "<string, required>",
      "topic": "<string, required>",
      "file": "<string, required>",
      "planning_sources": ["<string, ...>"],
      "depends_on": ["<string, ...>"]
    }
  ]
}`
}

var requiredSpecFields = map[string]string{
	"name":             "string",
	"domain":           "string",
	"topic":            "string",
	"file":             "string",
	"planning_sources": "array of strings",
	"depends_on":       "array of strings",
}

// ValidateQueueInput validates raw JSON bytes against the queue input schema.
// Returns a list of errors. Empty list means valid.
func ValidateQueueInput(data []byte) []string {
	var errs []string

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

	// Check for extra top-level keys.
	for key := range raw {
		if key != "specs" {
			errs = append(errs, fmt.Sprintf("extra top-level field: %q", key))
		}
	}

	specsRaw, ok := raw["specs"]
	if !ok {
		errs = append(errs, "missing required top-level field: \"specs\"")
		return errs
	}

	var specs []json.RawMessage
	if err := json.Unmarshal(specsRaw, &specs); err != nil {
		errs = append(errs, fmt.Sprintf("\"specs\" must be an array: %s", err))
		return errs
	}

	if len(specs) == 0 {
		errs = append(errs, "\"specs\" array must be non-empty")
		return errs
	}

	for i, specRaw := range specs {
		specErrs := validateSpecEntry(i, specRaw)
		errs = append(errs, specErrs...)
	}

	sort.Strings(errs)
	return errs
}

func validateSpecEntry(index int, data json.RawMessage) []string {
	var errs []string
	prefix := fmt.Sprintf("specs[%d]", index)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("%s: not a JSON object", prefix)}
	}

	// Check for extra fields.
	for key := range raw {
		if _, ok := requiredSpecFields[key]; !ok {
			errs = append(errs, fmt.Sprintf("%s: extra field: %q", prefix, key))
		}
	}

	// Check for missing and type-incorrect fields.
	stringFields := []string{"name", "domain", "topic", "file"}
	for _, field := range stringFields {
		val, ok := raw[field]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: missing required field: %q", prefix, field))
			continue
		}
		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			errs = append(errs, fmt.Sprintf("%s.%s: expected string, got %s", prefix, field, describeJSONType(val)))
			continue
		}
		if strings.TrimSpace(s) == "" {
			errs = append(errs, fmt.Sprintf("%s.%s: must be a non-empty string", prefix, field))
		}
	}

	arrayFields := []string{"planning_sources", "depends_on"}
	for _, field := range arrayFields {
		val, ok := raw[field]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: missing required field: %q", prefix, field))
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(val, &arr); err != nil {
			errs = append(errs, fmt.Sprintf("%s.%s: expected array of strings, got %s", prefix, field, describeJSONType(val)))
			continue
		}
		for j, elem := range arr {
			var s string
			if err := json.Unmarshal(elem, &s); err != nil {
				errs = append(errs, fmt.Sprintf("%s.%s[%d]: expected string, got %s", prefix, field, j, describeJSONType(elem)))
			}
		}
	}

	return errs
}

func describeJSONType(data json.RawMessage) string {
	s := strings.TrimSpace(string(data))
	if len(s) == 0 {
		return "empty"
	}
	switch s[0] {
	case '"':
		return "string"
	case '[':
		return "array"
	case '{':
		return "object"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// CheckDependencies checks that all depends_on references exist in the queue.
// Returns warnings (not errors) for missing dependencies.
func CheckDependencies(specs []QueueSpec) []string {
	names := make(map[string]bool)
	for _, s := range specs {
		names[s.Name] = true
	}

	var warnings []string
	for _, s := range specs {
		for _, dep := range s.DependsOn {
			if !names[dep] {
				warnings = append(warnings, fmt.Sprintf("Warning: %q depends on %q which is not in the queue", s.Name, dep))
			}
		}
	}
	return warnings
}
