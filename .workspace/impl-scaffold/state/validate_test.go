package state

import (
	"strings"
	"testing"
)

func validQueueJSON() string {
	return `{
  "plans": [
    {
      "name": "Protocol Implementation",
      "domain": "protocols",
      "topic": "WS1 and WS2 message contracts",
      "file": "protocols/.workspace/implementation_plan/PLAN.md",
      "specs": ["protocols/ws1/specs/ws1.md"],
      "code_search_roots": ["api/", "optimizer/"]
    }
  ]
}`
}

func TestValidateValidInput(t *testing.T) {
	errs := ValidateQueueInput([]byte(validQueueJSON()))
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidateInvalidJSON(t *testing.T) {
	errs := ValidateQueueInput([]byte("{bad json"))
	if len(errs) != 1 || !strings.Contains(errs[0], "invalid JSON") {
		t.Fatalf("expected JSON parse error, got: %v", errs)
	}
}

func TestValidateMissingPlans(t *testing.T) {
	errs := ValidateQueueInput([]byte(`{}`))
	if len(errs) == 0 {
		t.Fatal("expected error for missing plans")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "plans") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error mentioning plans, got: %v", errs)
	}
}

func TestValidateEmptyPlans(t *testing.T) {
	errs := ValidateQueueInput([]byte(`{"plans": []}`))
	if len(errs) == 0 {
		t.Fatal("expected error for empty plans")
	}
}

func TestValidateMissingRequiredField(t *testing.T) {
	// Missing "specs" field
	input := `{
  "plans": [
    {
      "name": "Test",
      "domain": "test",
      "topic": "Test topic",
      "file": "test.md",
      "code_search_roots": ["src/"]
    }
  ]
}`
	errs := ValidateQueueInput([]byte(input))
	if len(errs) == 0 {
		t.Fatal("expected error for missing specs field")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "specs") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error mentioning specs, got: %v", errs)
	}
}

func TestValidateExtraField(t *testing.T) {
	input := `{
  "plans": [
    {
      "name": "Test",
      "domain": "test",
      "topic": "Test topic",
      "file": "test.md",
      "specs": [],
      "code_search_roots": [],
      "extra_field": "bad"
    }
  ]
}`
	errs := ValidateQueueInput([]byte(input))
	if len(errs) == 0 {
		t.Fatal("expected error for extra field")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "extra_field") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error mentioning extra_field, got: %v", errs)
	}
}

func TestValidateExtraTopLevelField(t *testing.T) {
	input := `{"plans": [{"name":"A","domain":"d","topic":"t","file":"f","specs":[],"code_search_roots":[]}], "extra": true}`
	errs := ValidateQueueInput([]byte(input))
	if len(errs) == 0 {
		t.Fatal("expected error for extra top-level field")
	}
}

func TestValidateEmptyStringField(t *testing.T) {
	input := `{
  "plans": [
    {
      "name": "",
      "domain": "test",
      "topic": "Test topic",
      "file": "test.md",
      "specs": [],
      "code_search_roots": []
    }
  ]
}`
	errs := ValidateQueueInput([]byte(input))
	if len(errs) == 0 {
		t.Fatal("expected error for empty name")
	}
}

func TestValidateWrongFieldType(t *testing.T) {
	input := `{
  "plans": [
    {
      "name": 123,
      "domain": "test",
      "topic": "Test topic",
      "file": "test.md",
      "specs": [],
      "code_search_roots": []
    }
  ]
}`
	errs := ValidateQueueInput([]byte(input))
	if len(errs) == 0 {
		t.Fatal("expected error for wrong type")
	}
}

func TestParseQueue(t *testing.T) {
	plans, err := ParseQueue([]byte(validQueueJSON()))
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if plans[0].ID != 1 {
		t.Fatalf("expected ID 1, got %d", plans[0].ID)
	}
	if plans[0].Name != "Protocol Implementation" {
		t.Fatalf("expected Protocol Implementation, got %s", plans[0].Name)
	}
}

func TestParseQueueAssignsSequentialIDs(t *testing.T) {
	input := `{
  "plans": [
    {"name":"A","domain":"d","topic":"t","file":"a.md","specs":[],"code_search_roots":[]},
    {"name":"B","domain":"d","topic":"t","file":"b.md","specs":[],"code_search_roots":[]},
    {"name":"C","domain":"d","topic":"t","file":"c.md","specs":[],"code_search_roots":[]}
  ]
}`
	plans, err := ParseQueue([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range plans {
		if p.ID != i+1 {
			t.Fatalf("plan %d: expected ID %d, got %d", i, i+1, p.ID)
		}
	}
}
