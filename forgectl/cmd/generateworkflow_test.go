package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"forgectl/state"
)

// item is a small helper for building plan items in tests.
func item(id string, deps ...string) state.PlanItem {
	return state.PlanItem{ID: id, Name: id, Description: id, DependsOn: deps}
}

// planWith assembles a PlanJSON from layers (each a slice of item ids) and the
// item records, so tests can describe a plan shape compactly.
func planWith(layers [][]string, items ...state.PlanItem) state.PlanJSON {
	p := state.PlanJSON{
		Context: state.PlanContext{Domain: "d", Module: "m"},
		Items:   items,
	}
	for i, l := range layers {
		p.Layers = append(p.Layers, state.PlanLayerDef{
			ID:    "L" + string(rune('0'+i)),
			Items: l,
		})
	}
	return p
}

// collectIDs flattens the ids across all batches in run order.
func collectIDs(batches []Batch) []string {
	var out []string
	for _, b := range batches {
		for _, it := range b.Items {
			out = append(out, it.ID)
		}
	}
	return out
}

// batchIndexOf returns the 1-based batch index containing id, or 0 if absent.
func batchIndexOf(batches []Batch, id string) int {
	for _, b := range batches {
		for _, it := range b.Items {
			if it.ID == id {
				return b.Index
			}
		}
	}
	return 0
}

// Functional: every plan item appears in exactly one batch across all layers.
func TestComputeBatches_EveryItemExactlyOnce(t *testing.T) {
	plan := planWith(
		[][]string{{"a", "b"}, {"c", "d", "e"}},
		item("a"), item("b"), item("c"), item("d", "c"), item("e"),
	)
	batches := computeBatches(plan, 2)

	seen := map[string]int{}
	for _, id := range collectIDs(batches) {
		seen[id]++
	}
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if seen[id] != 1 {
			t.Errorf("item %q appears %d times, want exactly 1", id, seen[id])
		}
	}
	if len(seen) != 5 {
		t.Errorf("got %d distinct items across batches, want 5", len(seen))
	}
}

// Functional: for any item, all its depends_on appear in the same or an
// earlier-numbered batch.
func TestComputeBatches_DependenciesNotLater(t *testing.T) {
	// Layer L1 has A,B,C,D,E where D depends on A and E depends on D. With
	// batch size 2 the deps must still land in an equal-or-earlier batch.
	plan := planWith(
		[][]string{{"a", "b", "c", "d", "e"}},
		item("a"), item("b"), item("c"), item("d", "a"), item("e", "d"),
	)
	batches := computeBatches(plan, 2)

	for _, b := range batches {
		for _, it := range b.Items {
			for _, dep := range it.DependsOn {
				depIdx := batchIndexOf(batches, dep)
				if depIdx == 0 {
					t.Fatalf("dependency %q of %q missing from all batches", dep, it.ID)
				}
				if depIdx > b.Index {
					t.Errorf("item %q in batch %d depends on %q in later batch %d",
						it.ID, b.Index, dep, depIdx)
				}
			}
		}
	}
}

// Functional: no batch exceeds the configured size, and every batch is
// non-empty.
func TestComputeBatches_SizeBounds(t *testing.T) {
	plan := planWith(
		[][]string{{"a", "b", "c"}, {"d", "e", "f", "g"}},
		item("a"), item("b"), item("c"), item("d"), item("e"), item("f"), item("g"),
	)
	batches := computeBatches(plan, 2)

	for _, b := range batches {
		if len(b.Items) == 0 {
			t.Errorf("batch %d is empty", b.Index)
		}
		if len(b.Items) > 2 {
			t.Errorf("batch %d has %d items, exceeds size 2", b.Index, len(b.Items))
		}
	}
}

// Functional: batches are numbered sequentially from 1, continuing across layer
// boundaries without resetting.
func TestComputeBatches_SequentialNumberingAcrossLayers(t *testing.T) {
	// L0: 2 items (size 2 -> 1 batch). L1: 3 items (size 2 -> 2 batches).
	// Expect batch indices 1, 2, 3 in order.
	plan := planWith(
		[][]string{{"a", "b"}, {"c", "d", "e"}},
		item("a"), item("b"), item("c"), item("d"), item("e"),
	)
	batches := computeBatches(plan, 2)

	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	for i, b := range batches {
		if b.Index != i+1 {
			t.Errorf("batch at position %d has index %d, want %d", i, b.Index, i+1)
		}
	}
	// The second layer's first batch must not reset to 1.
	if batches[1].Index != 2 {
		t.Errorf("layer boundary reset numbering: got %d, want 2", batches[1].Index)
	}
}

// Edge case: when batch size exceeds a layer's item count, that layer produces a
// single batch containing all its items.
func TestComputeBatches_BatchLargerThanLayer(t *testing.T) {
	plan := planWith(
		[][]string{{"a", "b"}},
		item("a"), item("b"),
	)
	batches := computeBatches(plan, 5)

	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}
	if len(batches[0].Items) != 2 {
		t.Errorf("single batch has %d items, want 2", len(batches[0].Items))
	}
}

// Edge case: a plan with a single item produces exactly one batch of one item.
func TestComputeBatches_SingleItem(t *testing.T) {
	plan := planWith(
		[][]string{{"only"}},
		item("only"),
	)
	batches := computeBatches(plan, 2)

	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}
	if len(batches[0].Items) != 1 || batches[0].Items[0].ID != "only" {
		t.Errorf("want one batch of [only], got %+v", batches[0].Items)
	}
	if batches[0].Index != 1 {
		t.Errorf("single batch index = %d, want 1", batches[0].Index)
	}
}

// Stable topological ordering: a dependent is ordered after its in-layer
// dependency even when declared earlier.
func TestTopoSortLayer_DependencyBeforeDependent(t *testing.T) {
	byID := map[string]state.PlanItem{
		"a": item("a", "c"), // a depends on c
		"b": item("b"),
		"c": item("c"),
	}
	got := topoSortLayer([]string{"a", "b", "c"}, byID)

	pos := map[string]int{}
	for i, id := range got {
		pos[id] = i
	}
	if pos["c"] > pos["a"] {
		t.Errorf("c (dep) ordered after a (dependent): %v", got)
	}
}

// Functional: domain/module values with uppercase and non-alphanumeric
// characters sanitize to lowercase, hyphen-separated output.
func TestDeriveOutputName_Sanitized(t *testing.T) {
	cases := []struct {
		domain, module, want string
	}{
		{"forgectl", "forgectl", "forgectl-forgectl-impl.js"},
		{"My_Domain", "Core.App", "my-domain-core-app-impl.js"},
		{"A  B", "C__D", "a-b-c-d-impl.js"},
		{"-lead-", "trail-", "lead-trail-impl.js"},
		{"Weird!!!Name", "v1.2", "weird-name-v1-2-impl.js"},
	}
	for _, tc := range cases {
		got := deriveOutputName(state.PlanContext{Domain: tc.domain, Module: tc.module})
		if got != tc.want {
			t.Errorf("deriveOutputName(%q,%q) = %q, want %q", tc.domain, tc.module, got, tc.want)
		}
		// The name (minus extension) must be pure slash-command charset.
		base := strings.TrimSuffix(got, ".js")
		for _, r := range base {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				t.Errorf("sanitized name %q contains illegal char %q", got, r)
			}
		}
	}
}

// Edge case: when the derived name already exists, a new prefixed name is chosen
// and the original file is left byte-for-byte unchanged.
func TestResolveWorkflowPath_CollisionChoosesFreshName(t *testing.T) {
	dir := t.TempDir()
	intended := "d-m-impl.js"
	original := []byte("// pre-existing workflow — must not be touched\n")
	if err := os.WriteFile(filepath.Join(dir, intended), original, 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	got, err := resolveWorkflowPath(dir, intended, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) == intended {
		t.Fatalf("collision not resolved: got the intended name %q", intended)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		// The chosen path must be fresh (nonexistent) so the later write creates it.
		t.Errorf("chosen path %q should not already exist", got)
	}
	// Original file bytes unchanged.
	after, err := os.ReadFile(filepath.Join(dir, intended))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Errorf("original file was modified: got %q want %q", after, original)
	}
}

// Functional: a WARN line is printed on collision, naming both the intended and
// the chosen filename.
func TestResolveWorkflowPath_CollisionWarns(t *testing.T) {
	dir := t.TempDir()
	intended := "d-m-impl.js"
	if err := os.WriteFile(filepath.Join(dir, intended), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	got, err := resolveWorkflowPath(dir, intended, &warn)
	if err != nil {
		t.Fatal(err)
	}
	line := warn.String()
	if !strings.Contains(line, "WARN") {
		t.Errorf("expected a WARN line, got %q", line)
	}
	if !strings.Contains(line, intended) {
		t.Errorf("WARN should name the intended filename %q, got %q", intended, line)
	}
	if !strings.Contains(line, filepath.Base(got)) {
		t.Errorf("WARN should name the chosen filename %q, got %q", filepath.Base(got), line)
	}
}

// No collision: the intended path is returned and nothing is logged.
func TestResolveWorkflowPath_NoCollision(t *testing.T) {
	dir := t.TempDir()
	intended := "d-m-impl.js"

	var warn bytes.Buffer
	got, err := resolveWorkflowPath(dir, intended, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != intended {
		t.Errorf("got %q, want intended name %q", filepath.Base(got), intended)
	}
	if warn.Len() != 0 {
		t.Errorf("expected no WARN with no collision, got %q", warn.String())
	}
}

// --- L2: script rendering ---

// renderCfg builds a ForgeConfig with the implementing values a render test needs.
func renderCfg(im, it, em, et string, minR, maxR int, commits bool) state.ForgeConfig {
	var cfg state.ForgeConfig
	cfg.Implementing.Implement = state.AgentConfig{Model: im, Type: it, Count: 1}
	cfg.Implementing.Eval.Model = em
	cfg.Implementing.Eval.Type = et
	cfg.Implementing.Eval.Count = 1
	cfg.Implementing.Eval.MinRounds = minR
	cfg.Implementing.Eval.MaxRounds = maxR
	cfg.General.EnableCommits = commits
	return cfg
}

// sampleItem builds an item whose fields are individually recognizable so a
// test can assert the item's description/files/specs/tests appear in a prompt.
func sampleItem(id string) state.PlanItem {
	return state.PlanItem{
		ID:          id,
		Name:        "Name-" + id,
		Description: "Desc-" + id,
		Files:       []string{"file_" + id + ".go"},
		Specs:       []string{"spec_" + id + ".md#anchor"},
		Tests:       []state.PlanTest{{Category: "functional", Description: "crit-" + id}},
	}
}

func ctxDM() state.PlanContext { return state.PlanContext{Domain: "d", Module: "m"} }

// stripJSStringLiterals removes double-quoted string literals so a test can grep
// the residual *code* for forbidden APIs without matching baked prompt text.
// The emitted prompts are JSON-encoded single-line strings, so no literal
// newlines appear inside a quoted span and this simple regex is sufficient.
func stripJSStringLiterals(s string) string {
	re := regexp.MustCompile(`"(\\.|[^"\\])*"`)
	return re.ReplaceAllString(s, `""`)
}

func countLabelPrefix(labels []string, prefix string) int {
	n := 0
	for _, l := range labels {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

type scriptRun struct {
	Log    []string       `json:"log"`
	Labels []string       `json:"labels"`
	Meta   map[string]any `json:"meta"`
}

func (r scriptRun) joinedLog() string { return strings.Join(r.Log, "\n") }

// runEmittedScript executes the emitted workflow with node, stubbing agent()/
// log()/phase(). detectorResults feeds the change-detector one token per round;
// primaryReturn/evalReturn are the raw JS the primary/eval stubs return ("null"
// or a quoted string). It skips when node is unavailable — the loop control-flow
// contract can only be verified by executing the script.
func runEmittedScript(t *testing.T, script string, detectorResults []string, primaryReturn, evalReturn string) scriptRun {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available; skipping emitted-script execution test")
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "wf.mjs")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	detJSON, _ := json.Marshal(detectorResults)
	runner := "import { pathToFileURL } from 'node:url'\n" +
		"globalThis.__log = []\n" +
		"globalThis.__labels = []\n" +
		"const detectorResults = " + string(detJSON) + "\n" +
		"let detectIdx = 0\n" +
		"const primaryReturn = " + primaryReturn + "\n" +
		"const evalReturn = " + evalReturn + "\n" +
		"globalThis.log = (m) => { globalThis.__log.push(String(m)) }\n" +
		"globalThis.phase = () => {}\n" +
		"globalThis.agent = async (prompt, opts) => {\n" +
		"  const label = (opts && opts.label) || \"\"\n" +
		"  globalThis.__labels.push(label)\n" +
		"  if (label.indexOf(\"detect:\") === 0) {\n" +
		"    const r = detectIdx < detectorResults.length ? detectorResults[detectIdx] : \"CLEAN\"\n" +
		"    detectIdx++\n" +
		"    return r\n" +
		"  }\n" +
		"  if (label.indexOf(\"primary:\") === 0) return primaryReturn\n" +
		"  if (label.indexOf(\"eval:\") === 0) return evalReturn\n" +
		"  return \"STAGED\"\n" +
		"}\n" +
		"const mod = await import(pathToFileURL(process.argv[2]).href)\n" +
		"process.stdout.write(JSON.stringify({ log: globalThis.__log, labels: globalThis.__labels, meta: mod.meta }))\n"
	runnerPath := filepath.Join(dir, "runner.mjs")
	if err := os.WriteFile(runnerPath, []byte(runner), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", runnerPath, scriptPath).Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("node execution failed: %v\nstderr:\n%s\nscript:\n%s", err, stderr, script)
	}
	var run scriptRun
	if err := json.Unmarshal(out, &run); err != nil {
		t.Fatalf("parsing runner output: %v\nraw: %s", err, out)
	}
	return run
}

// Functional: pure-literal meta with name "<domain>-<module>-impl", a one-line
// description, and a phases array of one entry per batch plus an evaluate phase.
func TestRender_MetaLiteral(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}, {Index: 2, Items: []state.PlanItem{sampleItem("b")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	run := runEmittedScript(t, script, []string{"CLEAN"}, `"ok"`, `"ok"`)

	if run.Meta["name"] != "d-m-impl" {
		t.Errorf("meta.name = %v, want d-m-impl", run.Meta["name"])
	}
	if desc, _ := run.Meta["description"].(string); strings.TrimSpace(desc) == "" || strings.Contains(desc, "\n") {
		t.Errorf("meta.description must be a non-empty one-liner, got %q", desc)
	}
	phases, ok := run.Meta["phases"].([]any)
	if !ok {
		t.Fatalf("meta.phases is not an array: %T", run.Meta["phases"])
	}
	if len(phases) != 3 {
		t.Fatalf("meta.phases length = %d, want 3 (2 batches + evaluate)", len(phases))
	}
	last := phases[2].(map[string]any)
	if last["title"] != "Evaluate" {
		t.Errorf("last phase title = %v, want Evaluate", last["title"])
	}
}

// Functional: no forbidden APIs appear as code (they may appear inside baked
// prompt strings — e.g. the item's own test text — so we strip string literals
// before checking).
func TestRender_NoForbiddenAPIsAsCode(t *testing.T) {
	// Give an item whose text deliberately contains the forbidden tokens to prove
	// the check tolerates them inside prompt strings.
	it := sampleItem("a")
	it.Description = "must contain no require, import, Date.now(), Math.random(), new Date(), fs API, or calls to forgectl"
	batches := []Batch{{Index: 1, Items: []state.PlanItem{it}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))

	// Sanity: the raw script does contain the tokens (inside the prompt string).
	if !strings.Contains(script, "Math.random()") {
		t.Fatal("expected the baked prompt to carry the item's test text verbatim")
	}
	residual := stripJSStringLiterals(script)
	for _, bad := range []string{"require(", "import ", "import(", "Date.now", "Math.random", "new Date", "fs.", "forgectl"} {
		if strings.Contains(residual, bad) {
			t.Errorf("forbidden token %q appears as code (after stripping strings):\n%s", bad, residual)
		}
	}
}

// Functional: the primary agent() call uses the baked implement model/type.
func TestRender_PrimaryUsesImplementModelType(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("opus", "explore", "sonnet", "general-purpose", 1, 3, false))
	if !strings.Contains(script, `"primary:batch-1", model: "opus", agentType: "explore"`) {
		t.Errorf("primary agent() call missing baked implement model/type; script:\n%s", script)
	}
}

// Functional: the evaluator agent() call uses the baked eval model/type, and
// those differ from the primary's when configured differently.
func TestRender_EvaluatorUsesEvalModelType_Distinct(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "opus", "eval", 1, 3, false))
	if !strings.Contains(script, `model: "opus", agentType: "eval", phase: "Evaluate"`) {
		t.Errorf("evaluator agent() call missing baked eval model/type; script:\n%s", script)
	}
	if !strings.Contains(script, `"primary:batch-1", model: "sonnet", agentType: "general-purpose"`) {
		t.Errorf("primary agent() should carry the (distinct) implement model/type; script:\n%s", script)
	}
}

// Functional: the change-detector uses a haiku model with a Bash-capable agent
// type (claude).
func TestRender_ChangeDetectorHaiku(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	if !strings.Contains(script, `model: "haiku", agentType: "claude"`) {
		t.Errorf("change-detector agent() should use haiku/claude; script:\n%s", script)
	}
}

// Functional: when enable_commits is false, no commit instruction appears in any
// baked prompt or log line.
func TestRender_CommitsGatedOff(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	if strings.Contains(strings.ToLower(script), "commit") {
		t.Errorf("enable_commits=false but script mentions commit:\n%s", script)
	}
}

// Functional: when enable_commits is true, commit instructions appear in the
// primary and evaluator prompts and the change-detector commits.
func TestRender_CommitsGatedOn(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, true))
	if !strings.Contains(script, "git add -A && git commit") {
		t.Errorf("enable_commits=true but no commit instruction found:\n%s", script)
	}
	if !strings.Contains(script, "batch 1 committed") {
		t.Errorf("enable_commits=true should log a batch-committed INFO line")
	}
}

// Functional: the evaluator prompt bakes the full GauntletEval instructions.
func TestRender_GauntletEvalEmbedded(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	// A distinctive phrase from evaluators/gauntlet-eval.md.
	if !strings.Contains(script, "Mutate, do not report") {
		t.Errorf("evaluator prompt missing the embedded GauntletEval instructions")
	}
}

// Functional: a two-item batch carries both items' description/files/specs/tests
// in both the primary and the evaluator prompt (so each field appears >= 2x).
func TestRender_WholeBatchInBothPrompts(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a"), sampleItem("b")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	for _, token := range []string{"Desc-a", "Desc-b", "file_a.go", "file_b.go", "spec_a.md", "spec_b.md", "crit-a", "crit-b"} {
		if got := strings.Count(script, token); got < 2 {
			t.Errorf("token %q appears %d times, want >= 2 (primary + evaluator prompts)", token, got)
		}
	}
}

// Functional: the evaluator fan-out is independent of eval.count — scripts
// generated with count=1 and count=4 are byte-identical.
func TestRender_CountIgnored(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	c1 := renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false)
	c1.Implementing.Eval.Count = 1
	c4 := renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false)
	c4.Implementing.Eval.Count = 4
	if renderWorkflowScript(ctxDM(), batches, c1) != renderWorkflowScript(ctxDM(), batches, c4) {
		t.Error("scripts differ when only eval.count differs; count must not affect rendering")
	}
}

// Functional (executed): primary runs once and the evaluator runs exactly
// max_rounds times when every round changes code; a force-accept WARN is logged.
func TestRender_PrimaryOnce_ForceAcceptCeiling(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	run := runEmittedScript(t, script, []string{"CHANGED", "CHANGED", "CHANGED"}, `"ok"`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "primary:"); n != 1 {
		t.Errorf("primary invocations = %d, want 1", n)
	}
	if n := countLabelPrefix(run.Labels, "eval:"); n != 3 {
		t.Errorf("evaluator invocations = %d, want 3", n)
	}
	if !strings.Contains(run.joinedLog(), "force-accepted") {
		t.Errorf("expected a force-accept WARN; log:\n%s", run.joinedLog())
	}
}

// Functional (executed): convergence ends the loop at the first clean round at
// or beyond min_rounds.
func TestRender_ConvergenceEndsLoop(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 5, false))
	run := runEmittedScript(t, script, []string{"CHANGED", "CLEAN"}, `"ok"`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "eval:"); n != 2 {
		t.Errorf("evaluator invocations = %d, want 2", n)
	}
	if !strings.Contains(run.joinedLog(), "converged at round 2") {
		t.Errorf("expected 'converged at round 2'; log:\n%s", run.joinedLog())
	}
}

// Edge case (executed): the min_rounds floor forces a second round even when
// round one is clean.
func TestRender_MinRoundsFloorEnforced(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 2, 5, false))
	run := runEmittedScript(t, script, []string{"CLEAN", "CLEAN"}, `"ok"`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "eval:"); n != 2 {
		t.Errorf("evaluator invocations = %d, want 2 (min_rounds floor)", n)
	}
}

// Edge case (executed): min_rounds=0 lets the first clean round end the loop.
func TestRender_MinRoundsZero(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 0, 3, false))
	run := runEmittedScript(t, script, []string{"CLEAN"}, `"ok"`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "eval:"); n != 1 {
		t.Errorf("evaluator invocations = %d, want 1 (no floor)", n)
	}
}

// Edge case (executed): max_rounds=0 runs no evaluator round; the batch is
// force-accepted after the primary.
func TestRender_MaxRoundsZero(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 0, 0, false))
	run := runEmittedScript(t, script, nil, `"ok"`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "primary:"); n != 1 {
		t.Errorf("primary invocations = %d, want 1", n)
	}
	if n := countLabelPrefix(run.Labels, "eval:"); n != 0 {
		t.Errorf("evaluator invocations = %d, want 0 (max_rounds=0)", n)
	}
	if !strings.Contains(run.joinedLog(), "force-accepted") {
		t.Errorf("expected a force-accept log line; log:\n%s", run.joinedLog())
	}
}

// Edge case (executed): a git failure (GIT_ERROR) is treated as changed and
// surfaced at ERROR, preventing false convergence.
func TestRender_GitErrorTreatedAsChanged(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 2, false))
	run := runEmittedScript(t, script, []string{"GIT_ERROR", "GIT_ERROR"}, `"ok"`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "eval:"); n != 2 {
		t.Errorf("evaluator invocations = %d, want 2 (git error never converges, runs to ceiling)", n)
	}
	log := run.joinedLog()
	if !strings.Contains(log, "could not run git") {
		t.Errorf("expected an ERROR about git failure; log:\n%s", log)
	}
	if !strings.Contains(log, "force-accepted") {
		t.Errorf("git-error rounds should force-accept at the ceiling; log:\n%s", log)
	}
}

// Edge case (executed): an evaluator that returns null at/beyond min_rounds is
// surfaced at ERROR, and since the detector observes no change the batch
// converges optimistically.
func TestRender_EvaluatorNullConvergesWithError(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	run := runEmittedScript(t, script, []string{"CLEAN"}, `"ok"`, `null`)

	if n := countLabelPrefix(run.Labels, "eval:"); n != 1 {
		t.Errorf("evaluator invocations = %d, want 1 (converges on clean detector)", n)
	}
	if !strings.Contains(run.joinedLog(), "evaluator agent returned null") {
		t.Errorf("expected an ERROR about the null evaluator; log:\n%s", run.joinedLog())
	}
}

// Edge case (executed): a null primary does not abort the batch; the evaluator
// loop still runs and an ERROR is surfaced.
func TestRender_PrimaryNullDoesNotAbort(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "sonnet", "general-purpose", 1, 3, false))
	run := runEmittedScript(t, script, []string{"CHANGED", "CLEAN"}, `null`, `"ok"`)

	if n := countLabelPrefix(run.Labels, "eval:"); n < 1 {
		t.Errorf("evaluator should still run after a null primary; got %d eval calls", n)
	}
	if !strings.Contains(run.joinedLog(), "primary agent returned null") {
		t.Errorf("expected an ERROR about the null primary; log:\n%s", run.joinedLog())
	}
}

// Functional (executed): the required gauntlet log lines are emitted — batch
// started with ids, per-round outcome, converged, and the DEBUG bounds/models.
func TestRender_RequiredLogLines(t *testing.T) {
	batches := []Batch{{Index: 1, Items: []state.PlanItem{sampleItem("a"), sampleItem("b")}}}
	script := renderWorkflowScript(ctxDM(), batches, renderCfg("sonnet", "general-purpose", "opus", "eval", 1, 3, false))
	run := runEmittedScript(t, script, []string{"CHANGED", "CLEAN"}, `"ok"`, `"ok"`)
	log := run.joinedLog()

	wants := []string{
		"INFO: batch 1 started: [a, b]",
		"INFO: batch 1 round 1: changed",
		"INFO: batch 1 round 2: clean",
		"INFO: batch 1 converged at round 2",
		"DEBUG: batch 1 agents",
		"min_rounds=1 max_rounds=3",
	}
	for _, w := range wants {
		if !strings.Contains(log, w) {
			t.Errorf("missing log line %q; full log:\n%s", w, log)
		}
	}
}

// --- L3: generate-workflow command ---

// writeValidPlan writes a plan.json that passes ValidatePlanJSON: context d/m,
// one layer of two items, each with a present depends_on and a valid test.
func writeValidPlan(t *testing.T, dir string) string {
	t.Helper()
	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "d", Module: "m"},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Core", Items: []string{"a", "b"}}},
		Items: []state.PlanItem{
			{ID: "a", Name: "Item A", Description: "Do A", DependsOn: []string{}, Tests: []state.PlanTest{{Category: "functional", Description: "a works"}}},
			{ID: "b", Name: "Item B", Description: "Do B", DependsOn: []string{"a"}, Tests: []state.PlanTest{{Category: "functional", Description: "b works"}}},
		},
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runGen(t *testing.T, planPath string, verbose bool) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	generateWorkflowCmd.SetOut(&buf)
	generateWorkflowVerbose = verbose
	defer func() { generateWorkflowVerbose = false }()
	err := runGenerateWorkflow(generateWorkflowCmd, []string{planPath})
	return buf.String(), err
}

func workflowFiles(t *testing.T, projectRoot string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(projectRoot, ".claude", "workflows"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// setupBareDir creates a temp dir with no .forgectl and chdirs into it.
func setupBareDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return dir
}

// Functional: a valid plan + resolvable config writes exactly one new file and
// exits zero, printing the output path and slash-command name plus the INFO
// lines for started/validated/batch-count.
func TestGenerateWorkflow_Success(t *testing.T) {
	dir := setupProjectDir(t)
	plan := writeValidPlan(t, dir)

	out, err := runGen(t, plan, false)
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	files := workflowFiles(t, dir)
	if len(files) != 1 {
		t.Fatalf("expected exactly one workflow file, got %v", files)
	}
	if !strings.HasSuffix(files[0], ".js") {
		t.Errorf("workflow file %q should end in .js", files[0])
	}
	for _, want := range []string{
		"INFO: generating workflow from plan",
		"INFO: plan validated: plan.json",
		"INFO: computed 1 batch(es) across 2 item(s)",
		"slash command: /d-m-impl",
		filepath.Join(".claude", "workflows"),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; full output:\n%s", want, out)
		}
	}
}

// Rejection: a nonexistent plan path exits non-zero naming the path; no file is
// written.
func TestGenerateWorkflow_MissingPlan(t *testing.T) {
	dir := setupProjectDir(t)
	out, err := runGen(t, filepath.Join(dir, "nope.json"), false)
	if err == nil {
		t.Fatal("expected an error for a missing plan")
	}
	if !strings.Contains(out, "nope.json") {
		t.Errorf("error output should name the missing path; got:\n%s", out)
	}
	if files := workflowFiles(t, dir); len(files) != 0 {
		t.Errorf("no file should be written on failure, got %v", files)
	}
}

// Rejection: a malformed JSON plan exits non-zero with a located parse error,
// distinct from a semantic validation failure; no file is written.
func TestGenerateWorkflow_MalformedJSON(t *testing.T) {
	dir := setupProjectDir(t)
	p := filepath.Join(dir, "plan.json")
	os.WriteFile(p, []byte("{ \"context\": { \"domain\": \"d\" "), 0o644)

	out, err := runGen(t, p, false)
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	if !strings.Contains(out, "invalid JSON") || !strings.Contains(out, "line") {
		t.Errorf("expected a located JSON parse error; got:\n%s", out)
	}
	if strings.Contains(out, "validation failed") {
		t.Errorf("malformed JSON must be distinct from a validation failure; got:\n%s", out)
	}
	if files := workflowFiles(t, dir); len(files) != 0 {
		t.Errorf("no file should be written on failure, got %v", files)
	}
}

// Rejection: a structurally valid plan that fails ValidatePlanJSON exits
// non-zero with the validator's diagnostics; no file is written.
func TestGenerateWorkflow_InvalidPlan(t *testing.T) {
	dir := setupProjectDir(t)
	// Valid JSON, but context.domain is empty — a validation failure.
	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "", Module: "m"},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Core", Items: []string{"a"}}},
		Items:   []state.PlanItem{{ID: "a", Name: "A", Description: "Do A", DependsOn: []string{}, Tests: []state.PlanTest{{Category: "functional", Description: "x"}}}},
	}
	data, _ := json.MarshalIndent(plan, "", "  ")
	p := filepath.Join(dir, "plan.json")
	os.WriteFile(p, data, 0o644)

	out, err := runGen(t, p, false)
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(out, "validation failed") {
		t.Errorf("expected validator diagnostics; got:\n%s", out)
	}
	if files := workflowFiles(t, dir); len(files) != 0 {
		t.Errorf("no file should be written on failure, got %v", files)
	}
}

// Rejection: with no resolvable .forgectl/config the command exits non-zero
// stating config is required; no file is written.
func TestGenerateWorkflow_NoConfig(t *testing.T) {
	dir := setupBareDir(t)
	plan := writeValidPlan(t, dir)

	out, err := runGen(t, plan, false)
	if err == nil {
		t.Fatal("expected an error when no config is resolvable")
	}
	if !strings.Contains(strings.ToLower(out), "config") {
		t.Errorf("error should state config is required; got:\n%s", out)
	}
	if files := workflowFiles(t, dir); len(files) != 0 {
		t.Errorf("no file should be written on failure, got %v", files)
	}
}

// Rejection: a resolved config missing a required implementing field is rejected
// with an error naming the missing block.
func TestGenerateWorkflow_IncompleteConfig(t *testing.T) {
	var cfg state.ForgeConfig // zero value: all implementing fields blank
	err := validateGenerationConfig(cfg)
	if err == nil {
		t.Fatal("expected an error for a config missing implementing fields")
	}
	for _, want := range []string{"implementing.implement.model", "implementing.eval.model"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name the missing block %q; got: %v", want, err)
		}
	}
	// A complete config passes.
	if err := validateGenerationConfig(state.DefaultForgeConfig()); err != nil {
		t.Errorf("default config should be complete, got: %v", err)
	}
}

// Functional: two runs differing only in implementing.eval.count produce
// byte-identical scripts — count never changes evaluator fan-out.
func TestGenerateWorkflow_CountIgnoredEndToEnd(t *testing.T) {
	read := func(cfgTOML string) []byte {
		d := t.TempDir()
		os.MkdirAll(filepath.Join(d, ".forgectl"), 0o755)
		os.WriteFile(filepath.Join(d, ".forgectl", "config"), []byte(cfgTOML), 0o644)
		orig, _ := os.Getwd()
		os.Chdir(d)
		defer os.Chdir(orig)
		plan := writeValidPlan(t, d)
		if _, err := runGen(t, plan, false); err != nil {
			t.Fatalf("generation failed: %v", err)
		}
		files := workflowFiles(t, d)
		if len(files) != 1 {
			t.Fatalf("expected one workflow, got %v", files)
		}
		b, err := os.ReadFile(filepath.Join(d, ".claude", "workflows", files[0]))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	one := read("[implementing.eval]\ncount = 1\n")
	four := read("[implementing.eval]\ncount = 4\n")
	if !bytes.Equal(one, four) {
		t.Error("scripts differ when only eval.count differs")
	}
}

// Functional: a successful generation leaves plan.json and .forgectl/config
// byte-for-byte unchanged.
func TestGenerateWorkflow_InputsUnchanged(t *testing.T) {
	dir := setupProjectDir(t)
	plan := writeValidPlan(t, dir)

	planBefore, _ := os.ReadFile(plan)
	cfgPath := filepath.Join(dir, ".forgectl", "config")
	cfgBefore, _ := os.ReadFile(cfgPath)

	if _, err := runGen(t, plan, false); err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	planAfter, _ := os.ReadFile(plan)
	cfgAfter, _ := os.ReadFile(cfgPath)
	if !bytes.Equal(planBefore, planAfter) {
		t.Error("plan.json was modified by generation")
	}
	if !bytes.Equal(cfgBefore, cfgAfter) {
		t.Error(".forgectl/config was modified by generation")
	}
}

// Edge case: a mid-write filesystem failure leaves no file under
// .claude/workflows/ and exits non-zero.
func TestGenerateWorkflow_WriteFailureNoPartialFile(t *testing.T) {
	dir := setupProjectDir(t)
	plan := writeValidPlan(t, dir)
	wfDir := filepath.Join(dir, ".claude", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the target directory unwritable so the atomic temp-file create fails.
	if err := os.Chmod(wfDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(wfDir, 0o755) })

	out, err := runGen(t, plan, false)
	if err == nil {
		t.Fatal("expected a write failure")
	}
	if !strings.Contains(out, "ERROR") {
		t.Errorf("expected an ERROR diagnostic; got:\n%s", out)
	}
	if files := workflowFiles(t, dir); len(files) != 0 {
		t.Errorf("no partial file should remain, got %v", files)
	}
}

// Functional: at verbose (DEBUG) verbosity the command logs the resolved config
// and the per-batch item ids in run order.
func TestGenerateWorkflow_DebugVerbosity(t *testing.T) {
	dir := setupProjectDir(t)
	plan := writeValidPlan(t, dir)

	out, err := runGen(t, plan, true)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if !strings.Contains(out, "DEBUG: baked config") {
		t.Errorf("expected a DEBUG config line; got:\n%s", out)
	}
	if !strings.Contains(out, "DEBUG: batch 1 items: [a, b]") {
		t.Errorf("expected a DEBUG per-batch item-ids line; got:\n%s", out)
	}
}
