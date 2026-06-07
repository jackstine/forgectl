package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSpecQueue_Valid(t *testing.T) {
	input := SpecQueueInput{
		Specs: []SpecQueueEntry{
			{
				Name:            "Test Spec",
				Domain:          "test",
				Topic:           "A test spec",
				File:            "test/specs/test.md",
				PlanningSources: []string{},
				DependsOn:       []string{},
			},
		},
	}
	data, _ := json.Marshal(input)
	errs := ValidateSpecQueue(data)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateSpecQueue_MissingField(t *testing.T) {
	data := []byte(`{"specs": [{"name": "Test", "domain": "test"}]}`)
	errs := ValidateSpecQueue(data)
	if len(errs) == 0 {
		t.Error("expected validation errors for missing fields")
	}
}

func TestValidateSpecQueue_ExtraField(t *testing.T) {
	data := []byte(`{"specs": [{"name": "Test", "domain": "test", "topic": "t", "file": "f", "planning_sources": [], "depends_on": [], "extra": true}]}`)
	errs := ValidateSpecQueue(data)
	if len(errs) == 0 {
		t.Error("expected error for extra field")
	}
}

func TestValidateSpecQueue_InvalidJSON(t *testing.T) {
	errs := ValidateSpecQueue([]byte("{bad"))
	if len(errs) == 0 {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateSpecQueue_EmptyArray(t *testing.T) {
	errs := ValidateSpecQueue([]byte(`{"specs": []}`))
	if len(errs) == 0 {
		t.Error("expected error for empty specs array")
	}
}

func TestValidatePlanQueue_Valid(t *testing.T) {
	input := PlanQueueInput{
		Plans: []PlanQueueEntry{
			{
				Name:            "Test Plan",
				Domain:          "test",
				File:            "test/plan.json",
				Specs:           []string{"spec.md"},
				SpecCommits:     []string{},
				CodeSearchRoots: []string{"test/"},
			},
		},
	}
	data, _ := json.Marshal(input)
	errs := ValidatePlanQueue(data)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidatePlanQueue_MissingField(t *testing.T) {
	data := []byte(`{"plans": [{"name": "Test"}]}`)
	errs := ValidatePlanQueue(data)
	if len(errs) == 0 {
		t.Error("expected validation errors")
	}
}

// Functional: kind "code", kind "ui", and an absent kind field are all accepted
// (absent defaults to code). kind routes the phase shift to the implementation
// phase, so the queue must carry it through without rejection.
func TestValidatePlanQueue_KindAccepted(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind string // "" means the field is omitted
	}{
		{"code", "code"},
		{"ui", "ui"},
		{"absent", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]any{
				"name":              "Test Plan",
				"domain":            "test",
				"file":              "test/plan.json",
				"specs":             []string{"spec.md"},
				"spec_commits":      []string{},
				"code_search_roots": []string{"test/"},
			}
			if tc.kind != "" {
				entry["kind"] = tc.kind
			}
			data, _ := json.Marshal(map[string]any{"plans": []any{entry}})
			errs := ValidatePlanQueue(data)
			if len(errs) > 0 {
				t.Errorf("expected no errors, got %v", errs)
			}
		})
	}
}

// Rejection: a kind other than code/ui is rejected, naming the entry index and
// the offending value.
func TestValidatePlanQueue_KindInvalid(t *testing.T) {
	data := []byte(`{"plans": [{"name": "Test Plan", "domain": "test", "file": "test/plan.json", "specs": ["spec.md"], "spec_commits": [], "code_search_roots": ["test/"], "kind": "frontend"}]}`)
	errs := ValidatePlanQueue(data)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "plans[0]") && strings.Contains(e, "frontend") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected rejection naming plans[0] and \"frontend\", got %v", errs)
	}
}

func TestValidatePlanJSON_Valid(t *testing.T) {
	dir := t.TempDir()

	// Create notes file.
	notesDir := filepath.Join(dir, "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "config.md"), []byte("notes"), 0644)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "test-mod"},
		Layers: []PlanLayerDef{
			{ID: "L0", Name: "Foundation", Items: []string{"item.1"}},
		},
		Items: []PlanItem{
			{
				ID:          "item.1",
				Name:        "First Item",
				Description: "Does the thing",
				DependsOn:   []string{},
				Refs:        []string{"notes/config.md"},
				Tests: []PlanTest{
					{Category: "functional", Description: "it works"},
				},
			},
		},
	}
	data, _ := json.Marshal(plan)
	errs := ValidatePlanJSON(data, dir)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidatePlanJSON_MissingContext(t *testing.T) {
	plan := PlanJSON{
		Context: PlanContext{},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "F", Items: []string{"a"}}},
		Items: []PlanItem{
			{ID: "a", Name: "A", Description: "d", DependsOn: []string{},
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
		},
	}
	data, _ := json.Marshal(plan)
	errs := ValidatePlanJSON(data, t.TempDir())
	found := false
	for _, e := range errs {
		if e == "context.domain must be a non-empty string" {
			found = true
		}
	}
	if !found {
		t.Error("expected error for missing context.domain")
	}
}

func TestValidatePlanJSON_DuplicateItemID(t *testing.T) {
	plan := PlanJSON{
		Context: PlanContext{Domain: "t", Module: "m"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "F", Items: []string{"a"}}},
		Items: []PlanItem{
			{ID: "a", Name: "A", Description: "d", DependsOn: []string{},
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
			{ID: "a", Name: "A2", Description: "d2", DependsOn: []string{},
				Tests: []PlanTest{{Category: "functional", Description: "t2"}}},
		},
	}
	data, _ := json.Marshal(plan)
	errs := ValidatePlanJSON(data, t.TempDir())
	if len(errs) == 0 {
		t.Error("expected error for duplicate item ID")
	}
}

func TestValidatePlanJSON_CycleDetection(t *testing.T) {
	plan := PlanJSON{
		Context: PlanContext{Domain: "t", Module: "m"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "F", Items: []string{"a", "b"}}},
		Items: []PlanItem{
			{ID: "a", Name: "A", Description: "d", DependsOn: []string{"b"},
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
			{ID: "b", Name: "B", Description: "d", DependsOn: []string{"a"},
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
		},
	}
	data, _ := json.Marshal(plan)
	errs := ValidatePlanJSON(data, t.TempDir())
	hasCycle := false
	for _, e := range errs {
		if len(e) > 0 {
			hasCycle = true
		}
	}
	if !hasCycle {
		t.Error("expected cycle detection error")
	}
}

func TestValidatePlanJSON_InvalidTestCategory(t *testing.T) {
	plan := PlanJSON{
		Context: PlanContext{Domain: "t", Module: "m"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "F", Items: []string{"a"}}},
		Items: []PlanItem{
			{ID: "a", Name: "A", Description: "d", DependsOn: []string{},
				Tests: []PlanTest{{Category: "invalid", Description: "t"}}},
		},
	}
	data, _ := json.Marshal(plan)
	errs := ValidatePlanJSON(data, t.TempDir())
	if len(errs) == 0 {
		t.Error("expected error for invalid test category")
	}
}

func TestValidatePlanJSON_LayerOrderViolation(t *testing.T) {
	plan := PlanJSON{
		Context: PlanContext{Domain: "t", Module: "m"},
		Layers: []PlanLayerDef{
			{ID: "L0", Name: "Foundation", Items: []string{"a"}},
			{ID: "L1", Name: "Core", Items: []string{"b"}},
		},
		Items: []PlanItem{
			{ID: "a", Name: "A", Description: "d", DependsOn: []string{"b"},
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
			{ID: "b", Name: "B", Description: "d", DependsOn: []string{},
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
		},
	}
	data, _ := json.Marshal(plan)
	errs := ValidatePlanJSON(data, t.TempDir())
	if len(errs) == 0 {
		t.Error("expected error for layer order violation")
	}
}

// Rejection: a missing/blank concept and an empty domains list are rejected
// with the spec's error messages.
func TestValidateReverseEngineeringInputRejectsMissingConceptAndDomains(t *testing.T) {
	// Blank concept + empty domains.
	errs := ValidateReverseEngineeringInput([]byte(`{"concept": "   ", "domains": []}`))
	if !containsSubstr(errs, "A concept is required to scope the reverse engineering effort.") {
		t.Errorf("expected concept-required error, got: %v", errs)
	}
	if !containsSubstr(errs, "At least one domain is required.") {
		t.Errorf("expected domains-required error, got: %v", errs)
	}

	// Both keys absent.
	errs = ValidateReverseEngineeringInput([]byte(`{}`))
	if !containsSubstr(errs, "A concept is required to scope the reverse engineering effort.") {
		t.Errorf("expected concept-required error for empty object, got: %v", errs)
	}
	if !containsSubstr(errs, "At least one domain is required.") {
		t.Errorf("expected domains-required error for empty object, got: %v", errs)
	}

	// A well-formed input produces no errors (guards against false positives).
	if errs := ValidateReverseEngineeringInput([]byte(`{"concept": "auth refactor", "domains": ["api"]}`)); len(errs) != 0 {
		t.Errorf("valid input should produce no errors, got: %v", errs)
	}
}

// Rejection: a duplicate domain and any extra top-level field are rejected.
func TestValidateReverseEngineeringInputRejectsDuplicateAndExtraField(t *testing.T) {
	errs := ValidateReverseEngineeringInput([]byte(`{"concept": "c", "domains": ["api", "portal", "api"]}`))
	if !containsSubstr(errs, `duplicate domain "api"`) {
		t.Errorf("expected duplicate-domain error, got: %v", errs)
	}

	errs = ValidateReverseEngineeringInput([]byte(`{"concept": "c", "domains": ["api"], "extra": true}`))
	if !containsSubstr(errs, `unexpected field "extra"`) {
		t.Errorf("expected unexpected-field error, got: %v", errs)
	}
}

func containsSubstr(errs []string, want string) bool {
	for _, e := range errs {
		if e == want {
			return true
		}
	}
	return false
}

// buildREProject creates a temp project root with the given domain/subdir
// directories and returns the project root.
func buildREProject(t *testing.T, dirs map[string][]string) string {
	t.Helper()
	root := t.TempDir()
	for domain, subs := range dirs {
		for _, sub := range subs {
			if err := os.MkdirAll(filepath.Join(root, domain, sub), 0755); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// Functional: a well-formed queue with existing code_search_roots passes.
func TestValidateReverseEngineeringQueue_Valid(t *testing.T) {
	root := buildREProject(t, map[string][]string{
		"optimizer": {"src/repo", "src/config"},
		"api":       {"handlers"},
	})
	queue := `{
  "specs": [
    {"name": "Repo Loading", "domain": "optimizer", "topic": "loads a repo",
     "file": "specs/repo-loading.md", "action": "create",
     "code_search_roots": ["src/repo/", "src/config/"], "depends_on": []},
    {"name": "API Handlers", "domain": "api", "topic": "request handlers",
     "file": "specs/handlers.md", "action": "update",
     "code_search_roots": ["handlers/"], "depends_on": ["Repo Loading"]}
  ]
}`
	errs := ValidateReverseEngineeringQueue([]byte(queue), root, []string{"optimizer", "api"})
	if len(errs) != 0 {
		t.Errorf("valid queue should produce no errors, got: %v", errs)
	}
}

// Rejection: bad action, unknown domain, missing field, and a nonexistent
// code_search_roots directory are all reported.
func TestValidateReverseEngineeringQueue_Rejections(t *testing.T) {
	root := buildREProject(t, map[string][]string{"api": {"handlers"}})

	// Bad action + unknown domain + nonexistent root.
	queue := `{
  "specs": [
    {"name": "Bad", "domain": "ghost", "topic": "t", "file": "specs/x.md",
     "action": "delete", "code_search_roots": ["nope/"], "depends_on": []}
  ]
}`
	errs := ValidateReverseEngineeringQueue([]byte(queue), root, []string{"api"})
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, `action "delete" must be`) {
		t.Errorf("expected action enum error, got: %v", errs)
	}
	if !strings.Contains(joined, `has domain "ghost"`) || !strings.Contains(joined, "forgectl add-domain") {
		t.Errorf("expected domain-membership error with add-domain hint, got: %v", errs)
	}
	if !strings.Contains(joined, "code_search_roots directory does not exist") {
		t.Errorf("expected missing-directory error, got: %v", errs)
	}

	// Missing required field (no "topic").
	queue2 := `{"specs": [{"name": "X", "domain": "api", "file": "specs/x.md",
     "action": "create", "code_search_roots": ["handlers/"], "depends_on": []}]}`
	errs2 := ValidateReverseEngineeringQueue([]byte(queue2), root, []string{"api"})
	if !strings.Contains(strings.Join(errs2, "\n"), `missing required field "topic"`) {
		t.Errorf("expected missing-topic error, got: %v", errs2)
	}

	// Empty specs array.
	errs3 := ValidateReverseEngineeringQueue([]byte(`{"specs": []}`), root, []string{"api"})
	if len(errs3) == 0 {
		t.Error("expected error for empty specs array")
	}
}

// Edge case: a circular depends_on chain is detected.
func TestValidateReverseEngineeringQueue_Cycle(t *testing.T) {
	root := buildREProject(t, map[string][]string{"api": {"a", "b"}})
	queue := `{
  "specs": [
    {"name": "A", "domain": "api", "topic": "t", "file": "specs/a.md",
     "action": "create", "code_search_roots": ["a/"], "depends_on": ["B"]},
    {"name": "B", "domain": "api", "topic": "t", "file": "specs/b.md",
     "action": "create", "code_search_roots": ["b/"], "depends_on": ["A"]}
  ]
}`
	errs := ValidateReverseEngineeringQueue([]byte(queue), root, []string{"api"})
	if !strings.Contains(strings.Join(errs, "\n"), "circular dependency detected") {
		t.Errorf("expected circular dependency error, got: %v", errs)
	}
}
