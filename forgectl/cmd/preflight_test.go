package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"forgectl/state"
)

// preflight is the query an operator runs *before* committing to a planning
// cycle, so two properties matter above the verdict itself: it must work in a
// project that has not been initialized (that is when it is most useful), and
// it must not write anything (it is a question, not a step).

// runPreflight_ invokes the command with a fresh flag value and captures stdout
// separately from the returned error, which is what Execute() sends to stderr.
func runPreflightCmd(t *testing.T, from string) (string, error) {
	t.Helper()
	preflightFrom = from
	t.Cleanup(func() { preflightFrom = "" })

	var out bytes.Buffer
	preflightCmd.SetOut(&out)
	t.Cleanup(func() { preflightCmd.SetOut(nil) })

	err := runPreflight(preflightCmd, nil)
	return out.String(), err
}

func writePreflightQueue(t *testing.T, path string, domains ...string) {
	t.Helper()
	var plans []state.PlanQueueEntry
	for i, d := range domains {
		plans = append(plans, state.PlanQueueEntry{
			Name:            fmt.Sprintf("Plan %d", i+1),
			Domain:          d,
			File:            d + "/plan.json",
			Specs:           []string{},
			SpecCommits:     []string{},
			CodeSearchRoots: []string{d + "/"},
		})
	}
	data, err := json.Marshal(state.PlanQueueInput{Plans: plans})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// saveSession writes a state file, creating the state directory first — Save
// does not create it, and these tests build a session by hand rather than
// through init.
func saveSession(t *testing.T, root string, s *state.ForgeState) {
	t.Helper()
	if err := os.MkdirAll(resolvedStateDir(root), 0755); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(resolvedStateDir(root), s); err != nil {
		t.Fatal(err)
	}
}

func writeWorkspaceFile(t *testing.T, root, domain, name string) {
	t.Helper()
	p := filepath.Join(root, domain, ".forge_workspace", name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPreflightFromAllCleanIsReady(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core", "api")

	out, err := runPreflightCmd(t, queue)

	if err != nil {
		t.Fatalf("expected READY (exit 0), got error: %v", err)
	}
	if !strings.Contains(out, "Planning readiness: READY") {
		t.Errorf("stdout missing READY verdict:\n%s", out)
	}
	if !strings.Contains(out, "Inspected 2 domain workspaces — all clean.") {
		t.Errorf("stdout missing the inspected count:\n%s", out)
	}
}

func TestPreflightFromDirtyDomainIsBlocked(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core", "api")
	writeWorkspaceFile(t, dir, "api", "plan.json")

	out, err := runPreflightCmd(t, queue)

	if err == nil {
		t.Fatal("a blocked verdict must exit non-zero")
	}
	// The verdict text travels on the error so Execute() routes it to stderr.
	if !strings.Contains(err.Error(), "Planning readiness: BLOCKED") {
		t.Errorf("error missing BLOCKED verdict:\n%s", err)
	}
	if !strings.Contains(err.Error(), "api/.forge_workspace/") {
		t.Errorf("error does not name the dirty workspace path:\n%s", err)
	}
	if strings.Contains(err.Error(), "core") {
		t.Errorf("error names the clean domain:\n%s", err)
	}
	if !strings.Contains(err.Error(), "then re-run preflight.") {
		t.Errorf("error missing the close-out remediation:\n%s", err)
	}
	if out != "" {
		t.Errorf("blocked verdict wrote to stdout instead of stderr:\n%s", out)
	}
}

func TestPreflightWithoutFromUsesSessionPlanQueue(t *testing.T) {
	dir := setupProjectDir(t)
	writePreflightQueue(t, filepath.Join(dir, "generated-queue.json"), "core")
	writeWorkspaceFile(t, dir, "core", "plan.json")

	// A session whose generate_planning_queue phase produced a queue.
	s := &state.ForgeState{
		Phase:                 state.PhaseGeneratePlanningQueue,
		State:                 state.StateOrient,
		StartedAtPhase:        state.PhaseGeneratePlanningQueue,
		Config:                state.DefaultForgeConfig(),
		GeneratePlanningQueue: &state.GeneratePlanningQueueState{PlanQueueFile: "generated-queue.json"},
	}
	saveSession(t, dir, s)

	_, err := runPreflightCmd(t, "")

	if err == nil {
		t.Fatal("expected BLOCKED — the session's queue names a dirty domain")
	}
	if !strings.Contains(err.Error(), "core/.forge_workspace/") {
		t.Errorf("session-resolved queue was not used:\n%s", err)
	}
}

func TestPreflightFromWorksWithoutASession(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core")

	// No state file exists — this is the pre-init case preflight is for.
	if state.Exists(resolvedStateDir(dir)) {
		t.Fatal("fixture unexpectedly has a session")
	}

	out, err := runPreflightCmd(t, queue)

	if err != nil {
		t.Fatalf("preflight --from must not require a session: %v", err)
	}
	if !strings.Contains(out, "READY") {
		t.Errorf("expected READY, got:\n%s", out)
	}
}

func TestPreflightFromNonexistentFileErrors(t *testing.T) {
	dir := setupProjectDir(t)
	missing := filepath.Join(dir, "nope.json")

	_, err := runPreflightCmd(t, missing)

	if err == nil {
		t.Fatal("expected an error for a nonexistent --from file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error does not name the path: %v", err)
	}
}

func TestPreflightFromInvalidQueueErrors(t *testing.T) {
	dir := setupProjectDir(t)
	bad := filepath.Join(dir, "bad.json")
	// Valid JSON, but not a plan queue.
	os.WriteFile(bad, []byte(`{"specs": []}`), 0644)

	_, err := runPreflightCmd(t, bad)

	if err == nil {
		t.Fatal("expected an error for an invalid plan queue")
	}
	if !strings.Contains(err.Error(), bad) {
		t.Errorf("error does not name the path: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid plan queue") {
		t.Errorf("error lacks the validation detail: %v", err)
	}
}

func TestPreflightWithoutFromAndNoSessionErrors(t *testing.T) {
	setupProjectDir(t)

	_, err := runPreflightCmd(t, "")

	if err == nil {
		t.Fatal("expected an error with no --from and no session")
	}
	if err.Error() != noPlanQueueMessage {
		t.Errorf("message = %q,\nwant %q", err.Error(), noPlanQueueMessage)
	}
}

func TestPreflightWithoutFromAndNoPendingQueueErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*state.ForgeState)
	}{
		{"generate_planning_queue state absent", func(s *state.ForgeState) {
			s.GeneratePlanningQueue = nil
		}},
		{"plan queue file path empty", func(s *state.ForgeState) {
			s.GeneratePlanningQueue = &state.GeneratePlanningQueueState{PlanQueueFile: ""}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupProjectDir(t)
			s := &state.ForgeState{
				Phase:          state.PhaseGeneratePlanningQueue,
				State:          state.StateOrient,
				StartedAtPhase: state.PhaseGeneratePlanningQueue,
				Config:         state.DefaultForgeConfig(),
			}
			tc.setup(s)
			saveSession(t, dir, s)

			_, err := runPreflightCmd(t, "")

			if err == nil {
				t.Fatal("expected an error with no pending plan queue")
			}
			if err.Error() != noPlanQueueMessage {
				t.Errorf("message = %q,\nwant %q", err.Error(), noPlanQueueMessage)
			}
		})
	}
}

func TestPreflightErrorPathsWriteNothingToStdout(t *testing.T) {
	dir := setupProjectDir(t)
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`not json`), 0644)

	// Errors are returned, never printed — Execute() is what puts them on
	// stderr with a non-zero exit. Anything written to stdout here would show
	// up on the success channel of a failed command.
	for _, from := range []string{bad, filepath.Join(dir, "missing.json"), ""} {
		out, err := runPreflightCmd(t, from)
		if err == nil {
			t.Fatalf("--from %q: expected an error", from)
		}
		if out != "" {
			t.Errorf("--from %q wrote to stdout on an error path:\n%s", from, out)
		}
	}
}

func TestPreflightLeavesFilesystemUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dirty bool
	}{
		{"ready path", false},
		{"blocked path", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupProjectDir(t)
			queue := filepath.Join(dir, "plan-queue.json")
			writePreflightQueue(t, queue, "core", "api")
			if tc.dirty {
				writeWorkspaceFile(t, dir, "api", "plan.json")
			}

			before := snapshotDir(t, dir)
			runPreflightCmd(t, queue)
			after := snapshotDir(t, dir)

			if len(before) != len(after) {
				t.Fatalf("tree changed: %d entries → %d\nbefore: %v\nafter:  %v",
					len(before), len(after), before, after)
			}
			for i := range before {
				if before[i] != after[i] {
					t.Errorf("entry changed:\n before: %s\n after:  %s", before[i], after[i])
				}
			}
			if state.Exists(resolvedStateDir(dir)) {
				t.Error("preflight created a state file")
			}
		})
	}
}

// snapshotDir records every path under root with its mode and size, so any
// creation, deletion, or modification shows up as a diff.
func snapshotDir(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		size := int64(0)
		if info.Mode().IsRegular() {
			size = info.Size()
		}
		entries = append(entries, fmt.Sprintf("%s mode=%s size=%d", rel, info.Mode(), size))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot walk failed: %v", err)
	}
	sort.Strings(entries)
	return entries
}
