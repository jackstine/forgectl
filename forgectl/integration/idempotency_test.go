//go:build integration

package integration

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"forgectl/state"
)

// idempotencyConfig is the standard specifying config used by the idempotency
// tests: unguided, commits disabled so COMPLETE needs no --message.
const idempotencyConfig = `
[specifying]
batch = 1
[specifying.eval]
min_rounds = 1
max_rounds = 3
[general]
user_guided = false
enable_commits = false
`

// readStateBytes reads the state file bytes directly from disk, failing the
// test if the file is missing or unreadable.
func readStateBytes(t *testing.T, p *Project) []byte {
	t.Helper()
	path := p.Path(".forgectl/state/forgectl-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}
	return data
}

// assertStateUnchanged fails unless the current state file bytes are
// byte-identical to snapshot.
func assertStateUnchanged(t *testing.T, p *Project, snapshot []byte, context string) {
	t.Helper()
	after := readStateBytes(t, p)
	if !bytes.Equal(snapshot, after) {
		t.Errorf("%s: state file was mutated (expected byte-identical snapshot)", context)
	}
}

// initSpecifyingForIdempotency drives a fresh project to specifying ORIENT
// using the idempotency config (commits disabled, unguided).
func initSpecifyingForIdempotency(t *testing.T) *Project {
	t.Helper()
	p := NewProject(t)
	p.WriteConfig(idempotencyConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)
	return p
}

// TestL1FailedAdvanceStateUnchanged covers L1: a failed advance must never
// half-mutate state. For each error path the state file must be byte-identical
// before and after the failing call.
func TestL1FailedAdvanceStateUnchanged(t *testing.T) {
	// Case 1: ORIENT + --verdict PASS.
	// --verdict is only valid in eval states; validateAdvanceFlags returns an
	// error before state.Advance is ever called, so no mutation is possible.
	t.Run("orient_verdict_invalid", func(t *testing.T) {
		p := initSpecifyingForIdempotency(t)
		p.AssertAt(state.PhaseSpecifying, state.StateOrient)

		before := readStateBytes(t, p)
		res := p.forge("advance", "--verdict", "PASS")
		if res.Exit == 0 {
			t.Fatal("advance --verdict PASS at ORIENT succeeded; expected non-zero exit")
		}
		assertStateUnchanged(t, p, before, "ORIENT + --verdict PASS")
	})

	// Case 2: EVALUATE + advance (no --verdict).
	// state.Advance returns an error ("--verdict is required in EVALUATE state")
	// which is not a *ValidationError, so state is not saved.
	t.Run("evaluate_missing_verdict", func(t *testing.T) {
		p := initSpecifyingForIdempotency(t)

		// ORIENT → SELECT → DRAFT → EVALUATE.
		p.mustForge("advance") // → SELECT
		p.mustForge("advance") // → DRAFT
		p.WriteFile("specs/a.md", "# Spec A\n")
		p.mustForge("advance") // → EVALUATE
		p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

		before := readStateBytes(t, p)
		res := p.forge("advance") // missing --verdict
		if res.Exit == 0 {
			t.Fatal("advance without --verdict at EVALUATE succeeded; expected non-zero exit")
		}
		if !strings.Contains(res.Out(), "verdict") {
			t.Errorf("error output did not mention 'verdict':\n%s", res.Out())
		}
		assertStateUnchanged(t, p, before, "EVALUATE + missing --verdict")
	})

	// Case 3: specifying DRAFT + --verdict PASS.
	// --verdict is rejected by validateAdvanceFlags at DRAFT (only valid in eval
	// states), so state.Advance is never called and state stays unchanged.
	t.Run("draft_verdict_invalid", func(t *testing.T) {
		p := initSpecifyingForIdempotency(t)

		// ORIENT → SELECT → DRAFT.
		p.mustForge("advance") // → SELECT
		p.mustForge("advance") // → DRAFT
		p.AssertAt(state.PhaseSpecifying, state.StateDraft)

		before := readStateBytes(t, p)
		res := p.forge("advance", "--verdict", "PASS")
		if res.Exit == 0 {
			t.Fatal("advance --verdict PASS at specifying DRAFT succeeded; expected non-zero exit")
		}
		assertStateUnchanged(t, p, before, "specifying DRAFT + --verdict PASS")
	})
}

// TestL2InitOverExistingStateRefuses covers L2: a second `init --phase
// specifying --from <file>` when a state file already exists must exit
// non-zero, print an explanatory message, and leave the existing state file
// byte-identical.
func TestL2InitOverExistingStateRefuses(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(idempotencyConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	// First init: must succeed.
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	if !p.StateExists() {
		t.Fatal("first init did not create a state file")
	}
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	before := readStateBytes(t, p)

	// Second init over the existing state: must fail.
	res := p.forge("init", "--phase", "specifying", "--from", "spec-queue.json")
	if res.Exit == 0 {
		t.Fatal("second init succeeded; expected non-zero exit when state file already exists")
	}

	// The error message must guide the user toward resolution. The binary emits
	// "State file already exists. Delete it to reinitialize." so any of the
	// key substrings is sufficient.
	combined := res.Out()
	if !strings.Contains(combined, "already") && !strings.Contains(combined, "reinitialize") && !strings.Contains(combined, "exists") {
		t.Errorf("second init error did not mention the conflict (want 'already', 'exists', or 'reinitialize'):\n%s", combined)
	}

	// The state file must be byte-identical to what it was before the second init.
	assertStateUnchanged(t, p, before, "init over existing state")
}

// TestL3FlagContextGuards covers L3: flag-context guards reject misplaced flags
// before mutating state. Each sub-case snapshots state, triggers the rejection,
// and asserts non-zero exit plus an unchanged state file.
func TestL3FlagContextGuards(t *testing.T) {
	// --file outside specifying DRAFT (tested at ORIENT).
	t.Run("file_at_orient", func(t *testing.T) {
		p := initSpecifyingForIdempotency(t)
		p.AssertAt(state.PhaseSpecifying, state.StateOrient)

		before := readStateBytes(t, p)
		res := p.forge("advance", "--file", "foo.md")
		if res.Exit == 0 {
			t.Fatal("advance --file at ORIENT succeeded; expected non-zero exit")
		}
		if !strings.Contains(res.Out(), "--file") {
			t.Errorf("error output did not mention '--file':\n%s", res.Out())
		}
		assertStateUnchanged(t, p, before, "--file at ORIENT")
	})

	// --verdict outside an eval state (tested at ORIENT).
	t.Run("verdict_at_orient", func(t *testing.T) {
		p := initSpecifyingForIdempotency(t)
		p.AssertAt(state.PhaseSpecifying, state.StateOrient)

		before := readStateBytes(t, p)
		res := p.forge("advance", "--verdict", "PASS")
		if res.Exit == 0 {
			t.Fatal("advance --verdict at ORIENT succeeded; expected non-zero exit")
		}
		if !strings.Contains(res.Out(), "--verdict") {
			t.Errorf("error output did not mention '--verdict':\n%s", res.Out())
		}
		assertStateUnchanged(t, p, before, "--verdict at ORIENT")
	})

	// --file at SELECT state.
	t.Run("file_at_select", func(t *testing.T) {
		p := initSpecifyingForIdempotency(t)

		// ORIENT → SELECT.
		p.mustForge("advance")
		p.AssertAt(state.PhaseSpecifying, state.StateSelect)

		before := readStateBytes(t, p)
		res := p.forge("advance", "--file", "foo.md")
		if res.Exit == 0 {
			t.Fatal("advance --file at SELECT succeeded; expected non-zero exit")
		}
		if !strings.Contains(res.Out(), "--file") {
			t.Errorf("error output did not mention '--file':\n%s", res.Out())
		}
		assertStateUnchanged(t, p, before, "--file at SELECT")
	})
}
