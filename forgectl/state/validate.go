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

	requiredFields := []string{"name", "domain", "file", "specs", "spec_commits", "code_search_roots"}

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
			"name": true, "domain": true,
			"file": true, "specs": true, "spec_commits": true, "code_search_roots": true,
			"kind": true,
		}
		for k := range entry {
			if !allowedFields[k] {
				errs = append(errs, fmt.Sprintf("plans[%d]: unexpected field %q", i, k))
			}
		}
		// kind, when present, routes the phase shift: only "code" or "ui" are
		// valid (absent defaults to code).
		if kindRaw, ok := entry["kind"]; ok {
			var kind string
			if err := json.Unmarshal(kindRaw, &kind); err != nil {
				errs = append(errs, fmt.Sprintf("plans[%d]: \"kind\" must be a string: %s", i, err))
			} else if kind != "code" && kind != "ui" {
				errs = append(errs, fmt.Sprintf("plans[%d]: invalid kind %q (must be \"code\" or \"ui\")", i, kind))
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
      "file": "<string>",
      "specs": ["<string>", ...],
      "spec_commits": ["<string>", ...],
      "code_search_roots": ["<string>", ...]
    }
  ]
}`
}

// ValidateReverseEngineeringInput validates the reverse_engineering init input
// JSON: a non-empty concept, a non-empty domains list with no duplicates, and
// no additional fields. Shared by init and any other entry point.
func ValidateReverseEngineeringInput(data []byte) []string {
	var errs []string
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

	allowedFields := map[string]bool{"concept": true, "domains": true}
	for k := range raw {
		if !allowedFields[k] {
			errs = append(errs, fmt.Sprintf("unexpected field %q", k))
		}
	}

	// concept: required, non-empty string.
	if conceptRaw, ok := raw["concept"]; !ok {
		errs = append(errs, "A concept is required to scope the reverse engineering effort.")
	} else {
		var concept string
		if err := json.Unmarshal(conceptRaw, &concept); err != nil {
			errs = append(errs, "\"concept\" must be a string")
		} else if strings.TrimSpace(concept) == "" {
			errs = append(errs, "A concept is required to scope the reverse engineering effort.")
		}
	}

	// domains: required, non-empty array of strings, no duplicates.
	if domainsRaw, ok := raw["domains"]; !ok {
		errs = append(errs, "At least one domain is required.")
	} else {
		var domains []string
		if err := json.Unmarshal(domainsRaw, &domains); err != nil {
			errs = append(errs, "\"domains\" must be an array of strings")
		} else if len(domains) == 0 {
			errs = append(errs, "At least one domain is required.")
		} else {
			seen := map[string]bool{}
			for _, d := range domains {
				if seen[d] {
					errs = append(errs, fmt.Sprintf("duplicate domain %q", d))
				}
				seen[d] = true
			}
		}
	}

	return errs
}

// ReverseEngineeringInitSchema returns the printable schema for the
// reverse_engineering init input file.
func ReverseEngineeringInitSchema() string {
	return `{
  "concept": "<string>",
  "domains": ["<string>", ...]
}`
}

// ValidateReverseEngineeringQueue validates the reverse engineering queue JSON.
// projectRoot is the absolute project root; validDomains is the initialized
// domain list. Each entry's file and code_search_roots paths resolve against
// the domain root <projectRoot>/<domain>/. Shared by the QUEUE advance.
func ValidateReverseEngineeringQueue(data []byte, projectRoot string, validDomains []string) []string {
	var errs []string
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %s", err)}
	}

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

	validDomainSet := map[string]bool{}
	for _, d := range validDomains {
		validDomainSet[d] = true
	}

	requiredFields := []string{"name", "domain", "topic", "file", "action", "code_search_roots", "depends_on"}
	allowedFields := map[string]bool{
		"name": true, "domain": true, "topic": true, "file": true,
		"action": true, "code_search_roots": true, "depends_on": true,
	}

	entries := make([]REQueueEntry, 0, len(specs))
	for i, specRaw := range specs {
		var rawEntry map[string]json.RawMessage
		if err := json.Unmarshal(specRaw, &rawEntry); err != nil {
			errs = append(errs, fmt.Sprintf("specs[%d]: invalid object: %s", i, err))
			continue
		}
		for _, field := range requiredFields {
			if _, ok := rawEntry[field]; !ok {
				errs = append(errs, fmt.Sprintf("specs[%d]: missing required field %q", i, field))
			}
		}
		for k := range rawEntry {
			if !allowedFields[k] {
				errs = append(errs, fmt.Sprintf("specs[%d]: unexpected field %q", i, k))
			}
		}

		var entry REQueueEntry
		if err := json.Unmarshal(specRaw, &entry); err != nil {
			errs = append(errs, fmt.Sprintf("specs[%d]: invalid object: %s", i, err))
			continue
		}
		entries = append(entries, entry)

		// action enum.
		if entry.Action != "" && entry.Action != "create" && entry.Action != "update" {
			errs = append(errs, fmt.Sprintf("specs[%d]: action %q must be \"create\" or \"update\"", i, entry.Action))
		}

		// domain membership.
		if entry.Domain != "" && !validDomainSet[entry.Domain] {
			errs = append(errs, fmt.Sprintf(
				"Queue entry %q has domain %q which is not in the\ninitialized domain list.\n\nValid domains: %s\n\nTo add a new domain, run:\n  forgectl add-domain <domain>",
				entry.Name, entry.Domain, strings.Join(validDomains, ", ")))
		}

		// code_search_roots: non-empty, and each directory exists under the domain root.
		if _, ok := rawEntry["code_search_roots"]; ok {
			if len(entry.CodeSearchRoots) == 0 {
				errs = append(errs, fmt.Sprintf("specs[%d] %q: code_search_roots must not be empty", i, entry.Name))
			}
			for _, root := range entry.CodeSearchRoots {
				resolved := filepath.Join(projectRoot, entry.Domain, root)
				if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
					errs = append(errs, fmt.Sprintf("specs[%d] %q: code_search_roots directory does not exist: %s", i, entry.Name, resolved))
				}
			}
		}
	}

	// Acyclic depends_on graph; entry names are the node IDs.
	items := make([]PlanItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, PlanItem{ID: e.Name, DependsOn: e.DependsOn})
	}
	if cycle := detectCycle(items); cycle != "" {
		errs = append(errs, fmt.Sprintf("circular dependency detected: %s", cycle))
	}

	return errs
}

// ReverseEngineeringQueueSchema returns the printable schema for the reverse
// engineering queue file.
func ReverseEngineeringQueueSchema() string {
	return `{
  "specs": [
    {
      "name": "<string>",
      "domain": "<string>",
      "topic": "<string>",
      "file": "<string>",
      "action": "create" | "update",
      "code_search_roots": ["<string>", ...],
      "depends_on": ["<string>", ...]
    }
  ]
}`
}
