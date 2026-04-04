package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateSpecQueue validates the spec queue input JSON.
func ValidateSpecQueue(data []byte) []string {
	var errs []string
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

	// Check for unexpected top-level keys.
	for k := range raw {
		if k != "specs" {
			errs = append(errs, fmt.Sprintf("unexpected field %q", k))
		}
	}

	specsRaw, ok := raw["specs"]
	if !ok {
		return append(errs, "missing required field \"specs\"")
	}

	var specs []json.RawMessage
	if err := json.Unmarshal(specsRaw, &specs); err != nil {
		return append(errs, fmt.Sprintf("\"specs\" must be an array: %s", err))
	}

	if len(specs) == 0 {
		return append(errs, "\"specs\" array must not be empty")
	}

	requiredFields := []string{"name", "domain", "topic", "file", "planning_sources", "depends_on"}

	for i, specRaw := range specs {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(specRaw, &entry); err != nil {
			errs = append(errs, fmt.Sprintf("specs[%d]: invalid object: %s", i, err))
			continue
		}
		for _, field := range requiredFields {
			if _, ok := entry[field]; !ok {
				errs = append(errs, fmt.Sprintf("specs[%d]: missing required field %q", i, field))
			}
		}
		allowedFields := map[string]bool{
			"name": true, "domain": true, "topic": true,
			"file": true, "planning_sources": true, "depends_on": true,
		}
		for k := range entry {
			if !allowedFields[k] {
				errs = append(errs, fmt.Sprintf("specs[%d]: unexpected field %q", i, k))
			}
		}
	}

	return errs
}

// ValidatePlanQueue validates the plan queue input JSON.
func ValidatePlanQueue(data []byte) []string {
	var errs []string
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

	for k := range raw {
		if k != "plans" {
			errs = append(errs, fmt.Sprintf("unexpected field %q", k))
		}
	}

	plansRaw, ok := raw["plans"]
	if !ok {
		return append(errs, "missing required field \"plans\"")
	}

	var plans []json.RawMessage
	if err := json.Unmarshal(plansRaw, &plans); err != nil {
		return append(errs, fmt.Sprintf("\"plans\" must be an array: %s", err))
	}

	if len(plans) == 0 {
		return append(errs, "\"plans\" array must not be empty")
	}

	requiredFields := []string{"name", "domain", "topic", "file", "specs", "code_search_roots"}

	for i, planRaw := range plans {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(planRaw, &entry); err != nil {
			errs = append(errs, fmt.Sprintf("plans[%d]: invalid object: %s", i, err))
			continue
		}
		for _, field := range requiredFields {
			if _, ok := entry[field]; !ok {
				errs = append(errs, fmt.Sprintf("plans[%d]: missing required field %q", i, field))
			}
		}
		allowedFields := map[string]bool{
			"name": true, "domain": true, "topic": true,
			"file": true, "specs": true, "spec_commits": true, "code_search_roots": true,
		}
		for k := range entry {
			if !allowedFields[k] {
				errs = append(errs, fmt.Sprintf("plans[%d]: unexpected field %q", i, k))
			}
		}
	}

	return errs
}

// ValidatePlanJSON validates a plan.json file for the implementing phase.
// baseDir is the directory from which ref paths are resolved.
func ValidatePlanJSON(data []byte, baseDir string) []string {
	var errs []string

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

	// Check top-level fields.
	requiredTop := []string{"context", "layers", "items"}
	for _, f := range requiredTop {
		if _, ok := raw[f]; !ok {
			errs = append(errs, fmt.Sprintf("missing required field %q", f))
		}
	}
	allowedTop := map[string]bool{
		"context": true, "refs": true, "layers": true, "items": true,
	}
	for k := range raw {
		if !allowedTop[k] {
			errs = append(errs, fmt.Sprintf("unexpected field %q", k))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	var plan PlanJSON
	if err := json.Unmarshal(data, &plan); err != nil {
		return []string{fmt.Sprintf("parse error: %s", err)}
	}

	// Context fields.
	if plan.Context.Domain == "" {
		errs = append(errs, "context.domain must be a non-empty string")
	}
	if plan.Context.Module == "" {
		errs = append(errs, "context.module must be a non-empty string")
	}

	// Refs exist check.
	for _, ref := range plan.Refs {
		refPath := filepath.Join(baseDir, ref.Path)
		if _, err := os.Stat(refPath); err != nil {
			errs = append(errs, fmt.Sprintf("refs: path %q does not exist", ref.Path))
		}
	}

	// Build item index.
	itemIDs := map[string]int{}
	for i, item := range plan.Items {
		if item.ID == "" {
			errs = append(errs, fmt.Sprintf("items[%d]: missing required field \"id\"", i))
			continue
		}
		if _, exists := itemIDs[item.ID]; exists {
			errs = append(errs, fmt.Sprintf("items[%d]: duplicate item ID %q", i, item.ID))
			continue
		}
		itemIDs[item.ID] = i

		// Item schema check.
		if item.Name == "" {
			errs = append(errs, fmt.Sprintf("items[%d] (%s): missing required field \"name\"", i, item.ID))
		}
		if item.Description == "" {
			errs = append(errs, fmt.Sprintf("items[%d] (%s): missing required field \"description\"", i, item.ID))
		}
		if item.DependsOn == nil {
			errs = append(errs, fmt.Sprintf("items[%d] (%s): missing required field \"depends_on\"", i, item.ID))
		}
		if item.Tests == nil {
			errs = append(errs, fmt.Sprintf("items[%d] (%s): missing required field \"tests\"", i, item.ID))
		}

		// Test schema.
		validCategories := map[string]bool{"functional": true, "rejection": true, "edge_case": true}
		for j, t := range item.Tests {
			if t.Category == "" {
				errs = append(errs, fmt.Sprintf("items[%d].tests[%d]: missing \"category\"", i, j))
			} else if !validCategories[t.Category] {
				errs = append(errs, fmt.Sprintf("items[%d].tests[%d]: invalid category %q (must be functional, rejection, or edge_case)", i, j, t.Category))
			}
			if t.Description == "" {
				errs = append(errs, fmt.Sprintf("items[%d].tests[%d]: missing \"description\"", i, j))
			}
		}

		// Notes file check (refs are validated on disk, relative to plan.json dir).
		for _, ref := range item.Refs {
			refPath := filepath.Join(baseDir, ref)
			if _, err := os.Stat(refPath); err != nil {
				errs = append(errs, fmt.Sprintf("items[%d] (%s): ref %q does not exist", i, item.ID, ref))
			}
		}
	}

	// Layer coverage: every item in exactly one layer.
	itemInLayer := map[string]string{}
	for _, layer := range plan.Layers {
		for _, itemID := range layer.Items {
			if _, exists := itemIDs[itemID]; !exists {
				errs = append(errs, fmt.Sprintf("layers[%s].items: references non-existent item %q", layer.ID, itemID))
				continue
			}
			if prevLayer, already := itemInLayer[itemID]; already {
				errs = append(errs, fmt.Sprintf("item %q appears in both layer %q and %q", itemID, prevLayer, layer.ID))
				continue
			}
			itemInLayer[itemID] = layer.ID
		}
	}
	for id := range itemIDs {
		if _, covered := itemInLayer[id]; !covered {
			errs = append(errs, fmt.Sprintf("item %q not assigned to any layer", id))
		}
	}

	// Layer ordering: items only depend on items in equal or earlier layers.
	layerOrder := map[string]int{}
	for i, layer := range plan.Layers {
		layerOrder[layer.ID] = i
	}
	for _, item := range plan.Items {
		itemLayer := itemInLayer[item.ID]
		itemLayerIdx, ok := layerOrder[itemLayer]
		if !ok {
			continue
		}
		for _, depID := range item.DependsOn {
			depLayer := itemInLayer[depID]
			depLayerIdx, ok := layerOrder[depLayer]
			if !ok {
				continue
			}
			if depLayerIdx > itemLayerIdx {
				errs = append(errs, fmt.Sprintf("item %q (layer %s) depends on %q (layer %s) which is a later layer", item.ID, itemLayer, depID, depLayer))
			}
		}
	}

	// DAG validity: depends_on references valid items, no cycles.
	for _, item := range plan.Items {
		for _, depID := range item.DependsOn {
			if _, exists := itemIDs[depID]; !exists {
				errs = append(errs, fmt.Sprintf("item %q depends on non-existent item %q", item.ID, depID))
			}
		}
	}
	if cycle := detectCycle(plan.Items); cycle != "" {
		errs = append(errs, fmt.Sprintf("dependency cycle detected: %s", cycle))
	}

	return errs
}

// detectCycle finds a cycle in item dependencies using DFS.
func detectCycle(items []PlanItem) string {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := map[string]int{}
	parent := map[string]string{}
	deps := map[string][]string{}
	for _, item := range items {
		deps[item.ID] = item.DependsOn
	}

	var cyclePath string
	var dfs func(id string) bool
	dfs = func(id string) bool {
		color[id] = gray
		for _, dep := range deps[id] {
			if color[dep] == gray {
				// Build cycle path.
				path := []string{dep, id}
				curr := id
				for curr != dep {
					curr = parent[curr]
					path = append([]string{curr}, path...)
				}
				cyclePath = strings.Join(path, " → ")
				return true
			}
			if color[dep] == white {
				parent[dep] = id
				if dfs(dep) {
					return true
				}
			}
		}
		color[id] = black
		return false
	}

	for _, item := range items {
		if color[item.ID] == white {
			if dfs(item.ID) {
				return cyclePath
			}
		}
	}
	return ""
}

// SpecQueueSchema returns the valid schema description for spec queue files.
func SpecQueueSchema() string {
	return `{
  "specs": [
    {
      "name": "<string>",
      "domain": "<string>",
      "topic": "<string>",
      "file": "<string>",
      "planning_sources": ["<string>", ...],
      "depends_on": ["<string>", ...]
    }
  ]
}`
}

// PlanQueueSchema returns the valid schema description for plan queue files.
func PlanQueueSchema() string {
	return `{
  "plans": [
    {
      "name": "<string>",
      "domain": "<string>",
      "topic": "<string>",
      "file": "<string>",
      "specs": ["<string>", ...],
      "code_search_roots": ["<string>", ...]
    }
  ]
}`
}
