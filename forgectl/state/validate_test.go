package state

import (
	"encoding/json"
	"os"
	"path/filepath"
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
