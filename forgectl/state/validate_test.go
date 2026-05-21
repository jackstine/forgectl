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

func TestValidateReverseEngineeringInit_Valid(t *testing.T) {
	data := []byte(`{"concept": "auth middleware refactor", "domains": ["optimizer", "api", "portal"]}`)
	errs := ValidateReverseEngineeringInit(data)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateReverseEngineeringInit_MissingConcept(t *testing.T) {
	data := []byte(`{"domains": ["optimizer"]}`)
	errs := ValidateReverseEngineeringInit(data)
	if len(errs) == 0 {
		t.Error("expected error for missing concept")
	}
}

func TestValidateReverseEngineeringInit_EmptyDomains(t *testing.T) {
	data := []byte(`{"concept": "auth refactor", "domains": []}`)
	errs := ValidateReverseEngineeringInit(data)
	if len(errs) == 0 {
		t.Error("expected error for empty domains")
	}
}

func TestValidateReverseEngineeringInit_DuplicateDomain(t *testing.T) {
	data := []byte(`{"concept": "auth refactor", "domains": ["optimizer", "optimizer"]}`)
	errs := ValidateReverseEngineeringInit(data)
	if len(errs) == 0 {
		t.Error("expected error for duplicate domain")
	}
}

func TestValidateReverseEngineeringInit_ExtraField(t *testing.T) {
	data := []byte(`{"concept": "auth refactor", "domains": ["optimizer"], "extra": true}`)
	errs := ValidateReverseEngineeringInit(data)
	if len(errs) == 0 {
		t.Error("expected error for extra field")
	}
}

func validQueueEntry() string {
	return `{"name":"Auth Spec","domain":"optimizer","topic":"auth","file":"specs/auth.md","action":"create","code_search_roots":["src/"],"depends_on":[]}`
}

func TestValidateReverseEngineeringQueue_Valid(t *testing.T) {
	data := []byte(`{"specs": [` + validQueueEntry() + `]}`)
	errs := ValidateReverseEngineeringQueue(data, "", nil)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateReverseEngineeringQueue_MissingField(t *testing.T) {
	data := []byte(`{"specs": [{"name":"A","domain":"optimizer","topic":"t","file":"f","action":"create","code_search_roots":["src/"]}]}`)
	errs := ValidateReverseEngineeringQueue(data, "", nil)
	if len(errs) == 0 {
		t.Error("expected error for missing depends_on")
	}
}

func TestValidateReverseEngineeringQueue_InvalidAction(t *testing.T) {
	data := []byte(`{"specs": [{"name":"A","domain":"optimizer","topic":"t","file":"f","action":"delete","code_search_roots":["src/"],"depends_on":[]}]}`)
	errs := ValidateReverseEngineeringQueue(data, "", nil)
	if len(errs) == 0 {
		t.Error("expected error for invalid action")
	}
}

func TestValidateReverseEngineeringQueue_EmptyCodeSearchRoots(t *testing.T) {
	data := []byte(`{"specs": [{"name":"A","domain":"optimizer","topic":"t","file":"f","action":"create","code_search_roots":[],"depends_on":[]}]}`)
	errs := ValidateReverseEngineeringQueue(data, "", nil)
	if len(errs) == 0 {
		t.Error("expected error for empty code_search_roots")
	}
}

func TestValidateReverseEngineeringQueue_DomainMembership(t *testing.T) {
	data := []byte(`{"specs": [{"name":"A","domain":"unknown","topic":"t","file":"f","action":"create","code_search_roots":["src/"],"depends_on":[]}]}`)
	errs := ValidateReverseEngineeringQueue(data, "", []string{"optimizer"})
	if len(errs) == 0 {
		t.Error("expected error for domain not in initialized list")
	}
}

func TestValidateReverseEngineeringQueue_ForwardDependency(t *testing.T) {
	// B depends on A but A appears after B — forward dependency violation.
	data := []byte(`{"specs": [
		{"name":"B","domain":"optimizer","topic":"t","file":"specs/b.md","action":"create","code_search_roots":["src/"],"depends_on":["A"]},
		{"name":"A","domain":"optimizer","topic":"t","file":"specs/a.md","action":"create","code_search_roots":["src/"],"depends_on":[]}
	]}`)
	errs := ValidateReverseEngineeringQueue(data, "", nil)
	if len(errs) == 0 {
		t.Error("expected error for dependency on entry that appears later")
	}
}

func TestValidateReverseEngineeringQueue_CodeSearchRootsDirExists(t *testing.T) {
	// Create a temp dir structure to simulate project root.
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "optimizer")
	os.MkdirAll(filepath.Join(domainDir, "src"), 0o755)

	data := []byte(`{"specs": [{"name":"A","domain":"optimizer","topic":"t","file":"f","action":"create","code_search_roots":["src/","nonexistent/"],"depends_on":[]}]}`)
	errs := ValidateReverseEngineeringQueue(data, dir, []string{"optimizer"})
	// Should have an error for nonexistent/ but not for src/
	found := false
	for _, e := range errs {
		if strings.Contains(e, "nonexistent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for nonexistent directory, got %v", errs)
	}
}
