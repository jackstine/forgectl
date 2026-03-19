package state

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateQueueInput checks a raw JSON byte slice against the queue schema.
// Returns a list of validation errors; empty means valid.
func ValidateQueueInput(data []byte) []string {
	var errs []string

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

	// Check for extra top-level fields
	for key := range raw {
		if key != "plans" {
			errs = append(errs, fmt.Sprintf("unexpected top-level field: %q", key))
		}
	}

	plansRaw, ok := raw["plans"]
	if !ok {
		return append(errs, "missing required field: \"plans\"")
	}

	var plans []json.RawMessage
	if err := json.Unmarshal(plansRaw, &plans); err != nil {
		return append(errs, fmt.Sprintf("\"plans\" must be an array: %s", err))
	}

	if len(plans) == 0 {
		return append(errs, "\"plans\" array must not be empty")
	}

	allowedFields := map[string]bool{
		"name":              true,
		"domain":            true,
		"topic":             true,
		"file":              true,
		"specs":             true,
		"code_search_roots": true,
	}

	requiredStringFields := []string{"name", "domain", "topic", "file"}
	requiredArrayFields := []string{"specs", "code_search_roots"}

	for i, planRaw := range plans {
		prefix := fmt.Sprintf("plans[%d]", i)

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(planRaw, &fields); err != nil {
			errs = append(errs, fmt.Sprintf("%s: not a JSON object", prefix))
			continue
		}

		// Check for extra fields
		for key := range fields {
			if !allowedFields[key] {
				errs = append(errs, fmt.Sprintf("%s: unexpected field: %q", prefix, key))
			}
		}

		// Check required string fields
		for _, field := range requiredStringFields {
			val, exists := fields[field]
			if !exists {
				errs = append(errs, fmt.Sprintf("%s: missing required field: %q", prefix, field))
				continue
			}
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				errs = append(errs, fmt.Sprintf("%s.%s: must be a string", prefix, field))
				continue
			}
			if strings.TrimSpace(s) == "" {
				errs = append(errs, fmt.Sprintf("%s.%s: must not be empty", prefix, field))
			}
		}

		// Check required array fields
		for _, field := range requiredArrayFields {
			val, exists := fields[field]
			if !exists {
				errs = append(errs, fmt.Sprintf("%s: missing required field: %q", prefix, field))
				continue
			}
			var arr []json.RawMessage
			if err := json.Unmarshal(val, &arr); err != nil {
				errs = append(errs, fmt.Sprintf("%s.%s: must be an array", prefix, field))
				continue
			}
			// Validate array elements are strings
			for j, elem := range arr {
				var s string
				if err := json.Unmarshal(elem, &s); err != nil {
					errs = append(errs, fmt.Sprintf("%s.%s[%d]: must be a string", prefix, field, j))
				}
			}
		}
	}

	return errs
}

// ParseQueue parses validated JSON into QueuePlan slices with assigned IDs.
func ParseQueue(data []byte) ([]QueuePlan, error) {
	var input struct {
		Plans []QueuePlan `json:"plans"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	for i := range input.Plans {
		input.Plans[i].ID = i + 1
	}
	return input.Plans, nil
}

// ValidSchema returns a human-readable description of the expected queue schema.
func ValidSchema() string {
	return `{
  "plans": [
    {
      "name": "<string, required>",
      "domain": "<string, required>",
      "topic": "<string, required>",
      "file": "<string, required>",
      "specs": ["<string, ...>"],
      "code_search_roots": ["<string, ...>"]
    }
  ]
}`
}
