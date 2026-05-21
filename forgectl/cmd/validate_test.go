package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgectl/state"
)

func runValidateCmd(t *testing.T, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = "" // reset flag
	if len(args) > 1 {
		for i, a := range args {
			if a == "--type" && i+1 < len(args) {
				validateType = args[i+1]
			}
		}
	}
	// Find the file path arg (last non-flag arg).
	filePath := args[len(args)-1]
	err := runValidate(validateCmd, []string{filePath})
	return buf.String(), err
}

func writeSpecQueue(t *testing.T, dir string, specs []state.SpecQueueEntry) string {
	t.Helper()
	input := state.SpecQueueInput{Specs: specs}
	data, _ := json.Marshal(input)
	p := filepath.Join(dir, "spec-queue.json")
	os.WriteFile(p, data, 0644)
	return p
}

func writePlanQueue(t *testing.T, dir string) string {
	t.Helper()
	input := state.PlanQueueInput{Plans: []state.PlanQueueEntry{
		{Name: "Plan A", Domain: "test", File: "plan.json", Specs: []string{}, SpecCommits: []string{"abc1234"}, CodeSearchRoots: []string{}},
	}}
	data, _ := json.Marshal(input)
	p := filepath.Join(dir, "plan-queue.json")
	os.WriteFile(p, data, 0644)
	return p
}

func TestValidateAutoDetectsSpecQueue(t *testing.T) {
	dir := t.TempDir()
	p := writeSpecQueue(t, dir, []state.SpecQueueEntry{
		{Name: "A", Domain: "d", Topic: "t", File: "a.md", PlanningSources: []string{}, DependsOn: []string{}},
	})

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "spec-queue") {
		t.Errorf("output should mention spec-queue, got: %q", out)
	}
	if !strings.Contains(out, "1 entries") {
		t.Errorf("output should show 1 entries, got: %q", out)
	}
}

func TestValidateAutoDetectsPlanQueue(t *testing.T) {
	dir := t.TempDir()
	p := writePlanQueue(t, dir)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "plan-queue") {
		t.Errorf("output should mention plan-queue, got: %q", out)
	}
	if !strings.Contains(out, "no errors") {
		t.Errorf("output should say no errors, got: %q", out)
	}
}

func TestValidateAutoDetectsPlan(t *testing.T) {
	dir := t.TempDir()
	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "test", Module: "mod"},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"item1"}}},
		Items: []state.PlanItem{
			{ID: "item1", Name: "Item One", Description: "desc", DependsOn: []string{}, Tests: []state.PlanTest{}},
		},
	}
	data, _ := json.Marshal(plan)
	p := filepath.Join(dir, "plan.json")
	os.WriteFile(p, data, 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "plan") {
		t.Errorf("output should mention plan, got: %q", out)
	}
	if !strings.Contains(out, "no errors") {
		t.Errorf("output should say no errors, got: %q", out)
	}
}

func TestValidateTypeFlagPlanQueue(t *testing.T) {
	dir := t.TempDir()
	p := writePlanQueue(t, dir)

	validateType = "plan-queue"
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success with --type plan-queue, got: %v", err)
	}
}

func TestValidateUnrecognizedTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "unknown.json")
	os.WriteFile(p, []byte(`{"widgets": []}`), 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err == nil {
		t.Fatal("expected error for unrecognized top-level key")
	}
	out := buf.String()
	if !strings.Contains(out, "cannot detect file type") {
		t.Errorf("output should say cannot detect file type, got: %q", out)
	}
}

func TestValidateTypeMismatch(t *testing.T) {
	dir := t.TempDir()
	p := writePlanQueue(t, dir)

	validateType = "spec-queue"
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
	out := buf.String()
	if !strings.Contains(out, "spec-queue") && !strings.Contains(out, "plans") {
		t.Errorf("output should mention the mismatch, got: %q", out)
	}
}

func TestValidateInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.json")
	os.WriteFile(p, []byte(`{"bad": `), 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("error should mention invalid JSON, got: %v", err)
	}
}

func TestValidateWithValidationErrors(t *testing.T) {
	dir := t.TempDir()
	// Missing required field 'topic' in spec.
	bad := `{"specs":[{"name":"A","domain":"d","file":"a.md"}]}`
	p := filepath.Join(dir, "bad-queue.json")
	os.WriteFile(p, []byte(bad), 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err == nil {
		t.Fatal("expected validation error")
	}
	out := buf.String()
	if !strings.Contains(out, "1.") {
		t.Errorf("output should have numbered errors starting with 1., got: %q", out)
	}
}

func TestValidateAutoDetectsREInit(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"concept": "auth refactor", "domains": ["optimizer", "api"]}`)
	p := filepath.Join(dir, "re-init.json")
	os.WriteFile(p, data, 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success, got: %v — output: %s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "re-init") {
		t.Errorf("output should mention re-init, got: %q", out)
	}
}

func TestValidateAutoDetectsREQueue(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"specs": [{"name":"Auth Spec","domain":"optimizer","topic":"auth","file":"specs/auth.md","action":"create","code_search_roots":["src/"],"depends_on":[]}]}`)
	p := filepath.Join(dir, "re-queue.json")
	os.WriteFile(p, data, 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success, got: %v — output: %s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "re-queue") {
		t.Errorf("output should mention re-queue, got: %q", out)
	}
}

func TestValidateSpecQueueNotMistakenForREQueue(t *testing.T) {
	dir := t.TempDir()
	// Spec queue entries have "planning_sources", not "action".
	p := writeSpecQueue(t, dir, []state.SpecQueueEntry{
		{Name: "A", Domain: "d", Topic: "t", File: "a.md", PlanningSources: []string{}, DependsOn: []string{}},
	})

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := buf.String()
	// Should be detected as spec-queue, not re-queue.
	if !strings.Contains(out, "spec-queue") {
		t.Errorf("output should mention spec-queue, got: %q", out)
	}
	if strings.Contains(out, "re-queue") {
		t.Errorf("should not mention re-queue for spec queue files, got: %q", out)
	}
}

func TestValidatePlanResolvesRefsRelativeToFile(t *testing.T) {
	dir := t.TempDir()
	// Create a ref file at a known path relative to the plan file.
	refDir := filepath.Join(dir, "notes")
	os.MkdirAll(refDir, 0755)
	os.WriteFile(filepath.Join(refDir, "item1.md"), []byte("# notes"), 0644)

	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "test", Module: "mod"},
		Refs:    []state.PlanRef{{ID: "r1", Path: "notes/item1.md"}},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"item1"}}},
		Items: []state.PlanItem{
			{ID: "item1", Name: "Item One", Description: "desc", Refs: []string{"notes/item1.md"}, DependsOn: []string{}, Tests: []state.PlanTest{}},
		},
	}
	data, _ := json.Marshal(plan)
	p := filepath.Join(dir, "plan.json")
	os.WriteFile(p, data, 0644)

	validateType = ""
	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	err := runValidate(validateCmd, []string{p})
	if err != nil {
		t.Fatalf("expected success with valid ref path, got: %v — output: %s", err, buf.String())
	}
}
