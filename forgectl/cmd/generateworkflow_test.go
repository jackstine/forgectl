package cmd

import (
	"bytes"
	"os"
	"path/filepath"
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
