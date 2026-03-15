package state

import (
	"strings"
	"testing"
)

func TestValidateQueueInput_Valid(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "Repository Loading",
				"domain": "optimizer",
				"topic": "The optimizer clones or locates a repository",
				"file": "optimizer/specs/repository-loading.md",
				"planning_sources": [".workspace/planning/optimizer/repo.md"],
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateQueueInput_MultipleSpecs(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "Spec A",
				"domain": "api",
				"topic": "Topic A",
				"file": "api/specs/a.md",
				"planning_sources": [],
				"depends_on": []
			},
			{
				"name": "Spec B",
				"domain": "optimizer",
				"topic": "Topic B",
				"file": "optimizer/specs/b.md",
				"planning_sources": ["plan.md"],
				"depends_on": ["Spec A"]
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateQueueInput_InvalidJSON(t *testing.T) {
	data := []byte(`{not json}`)
	errs := ValidateQueueInput(data)
	if len(errs) != 1 || !strings.Contains(errs[0], "invalid JSON") {
		t.Errorf("expected invalid JSON error, got: %v", errs)
	}
}

func TestValidateQueueInput_MissingSpecs(t *testing.T) {
	data := []byte(`{}`)
	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, `missing required top-level field: "specs"`)
}

func TestValidateQueueInput_EmptySpecsArray(t *testing.T) {
	data := []byte(`{"specs": []}`)
	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, `"specs" array must be non-empty`)
}

func TestValidateQueueInput_ExtraTopLevelField(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "A",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [],
				"depends_on": []
			}
		],
		"version": "1.0"
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, `extra top-level field: "version"`)
}

func TestValidateQueueInput_ExtraSpecField(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "A",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [],
				"depends_on": [],
				"priority": "high"
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, `extra field: "priority"`)
}

func TestValidateQueueInput_MissingRequiredField(t *testing.T) {
	// Missing "domain" and "file"
	data := []byte(`{
		"specs": [
			{
				"name": "A",
				"topic": "T",
				"planning_sources": [],
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, `missing required field: "domain"`)
	assertContainsError(t, errs, `missing required field: "file"`)
}

func TestValidateQueueInput_WrongType_StringField(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": 123,
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [],
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, "specs[0].name: expected string, got number")
}

func TestValidateQueueInput_WrongType_ArrayField(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "A",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": "not-an-array",
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, "specs[0].planning_sources: expected array of strings, got string")
}

func TestValidateQueueInput_EmptyStringField(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [],
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, "specs[0].name: must be a non-empty string")
}

func TestValidateQueueInput_WhitespaceOnlyStringField(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "   ",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [],
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, "specs[0].name: must be a non-empty string")
}

func TestValidateQueueInput_NonStringInArray(t *testing.T) {
	data := []byte(`{
		"specs": [
			{
				"name": "A",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [123],
				"depends_on": []
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, "specs[0].planning_sources[0]: expected string, got number")
}

func TestValidateQueueInput_SpecsNotArray(t *testing.T) {
	data := []byte(`{"specs": "not-array"}`)
	errs := ValidateQueueInput(data)
	assertContainsError(t, errs, `"specs" must be an array`)
}

func TestValidateQueueInput_MultipleErrorsAtOnce(t *testing.T) {
	// Extra field + missing field + wrong type — all reported together
	data := []byte(`{
		"specs": [
			{
				"name": "A",
				"topic": 42,
				"file": "f.md",
				"planning_sources": [],
				"depends_on": [],
				"extra": true
			}
		]
	}`)

	errs := ValidateQueueInput(data)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
	assertContainsError(t, errs, `extra field: "extra"`)
	assertContainsError(t, errs, `missing required field: "domain"`)
	assertContainsError(t, errs, "expected string, got number")
}

func TestCheckDependencies_AllPresent(t *testing.T) {
	specs := []QueueSpec{
		{Name: "A", DependsOn: []string{}},
		{Name: "B", DependsOn: []string{"A"}},
	}
	warnings := CheckDependencies(specs)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestCheckDependencies_MissingDep(t *testing.T) {
	specs := []QueueSpec{
		{Name: "B", DependsOn: []string{"A"}},
	}
	warnings := CheckDependencies(specs)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], `"A"`) {
		t.Errorf("expected warning about A, got: %s", warnings[0])
	}
}

func assertContainsError(t *testing.T, errs []string, substr string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e, substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got: %v", substr, errs)
}
