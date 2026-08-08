package state

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Workspace inspection is the foundation the planning readiness gate stands on:
// a false "clean" lets a cold start bury a prior cycle's plan.json under a new
// one, and a false "dirty" blocks every multi-domain run. These tests pin both
// directions, including the cases that are easy to get subtly wrong — empty
// subdirectory trees, dotfiles, and an unreadable directory.

func cfgWithWorkspaceDir(dir string, domains ...DomainConfig) ForgeConfig {
	return ForgeConfig{
		Domains: domains,
		Paths:   PathsConfig{WorkspaceDir: dir},
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestInspectDomainWorkspaceAbsentDirectoryIsClean(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	got := InspectDomainWorkspace(root, cfg, "protocols")

	if !got.Clean {
		t.Errorf("absent workspace reported dirty: %+v", got)
	}
	if got.Error != "" {
		t.Errorf("absent workspace carried an error: %q", got.Error)
	}
	if got.Domain != "protocols" {
		t.Errorf("Domain = %q, want %q", got.Domain, "protocols")
	}
}

func TestInspectDomainWorkspaceTopLevelFileIsDirty(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/plan.json"), "{}")

	got := InspectDomainWorkspace(root, cfg, "protocols")

	if got.Clean {
		t.Errorf("workspace with a top-level file reported clean: %+v", got)
	}
	if got.Error != "" {
		t.Errorf("a readable dirty workspace must carry no error, got %q", got.Error)
	}
}

func TestInspectDomainWorkspaceNestedFileIsDirty(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t,
		filepath.Join(root, "protocols/.forge_workspace/implementation_plan/notes/deep/a.md"),
		"notes")

	got := InspectDomainWorkspace(root, cfg, "protocols")

	if got.Clean {
		t.Errorf("workspace with a deeply nested file reported clean: %+v", got)
	}
}

func TestInspectDomainWorkspaceHonorsConfiguredWorkspaceDir(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_scratch")

	// The artifact lives under the *configured* workspace dir. A hardcoded
	// ".forge_workspace" would miss it entirely and report clean.
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_scratch/plan.json"), "{}")
	// A default-named directory that is empty must not influence the result.
	if err := os.MkdirAll(filepath.Join(root, "protocols/.forge_workspace"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := InspectDomainWorkspace(root, cfg, "protocols")

	wantPath := "protocols/.forge_scratch" + string(filepath.Separator)
	if got.WorkspacePath != wantPath {
		t.Errorf("WorkspacePath = %q, want %q", got.WorkspacePath, wantPath)
	}
	if filepath.IsAbs(got.WorkspacePath) {
		t.Errorf("WorkspacePath must be project-root-relative, got %q", got.WorkspacePath)
	}
	if got.Clean {
		t.Errorf("configured workspace dir holding a file reported clean: %+v", got)
	}
}

func TestInspectDomainWorkspaceResolvesConfiguredDomainPath(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace",
		DomainConfig{Name: "portal", Path: "services/portal"})

	mustWriteFile(t, filepath.Join(root, "services/portal/.forge_workspace/plan.json"), "{}")

	got := InspectDomainWorkspace(root, cfg, "portal")

	wantPath := filepath.Join("services/portal", ".forge_workspace") + string(filepath.Separator)
	if got.WorkspacePath != wantPath {
		t.Errorf("WorkspacePath = %q, want %q", got.WorkspacePath, wantPath)
	}
	if got.Clean {
		t.Errorf("configured domain path holding a file reported clean: %+v", got)
	}

	// With no matching [[domains]] entry the name itself is the path.
	unconfigured := InspectDomainWorkspace(root, cfg, "launcher")
	wantFallback := filepath.Join("launcher", ".forge_workspace") + string(filepath.Separator)
	if unconfigured.WorkspacePath != wantFallback {
		t.Errorf("unconfigured domain WorkspacePath = %q, want %q",
			unconfigured.WorkspacePath, wantFallback)
	}
	if !unconfigured.Clean {
		t.Errorf("unconfigured domain with no workspace on disk reported dirty: %+v", unconfigured)
	}
}

func TestInspectDomainWorkspaceDomainNameWithSeparator(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	// "protocols/ws1" is a nested domain, not a flat directory component.
	mustWriteFile(t, filepath.Join(root, "protocols/ws1/.forge_workspace/plan.json"), "{}")
	// A sibling workspace one level up must not be the one inspected.
	if err := os.MkdirAll(filepath.Join(root, "protocols/.forge_workspace"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := InspectDomainWorkspace(root, cfg, "protocols/ws1")

	wantPath := filepath.Join("protocols/ws1", ".forge_workspace") + string(filepath.Separator)
	if got.WorkspacePath != wantPath {
		t.Errorf("WorkspacePath = %q, want %q", got.WorkspacePath, wantPath)
	}
	if got.Clean {
		t.Errorf("nested domain workspace holding a file reported clean: %+v", got)
	}
}

func TestInspectDomainWorkspaceEmptySubdirectoriesAreClean(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	// A close-out that removed files but left the directory skeleton behind is
	// still clean — the skeleton carries no prior-cycle content.
	dirs := []string{
		"protocols/.forge_workspace",
		"protocols/.forge_workspace/implementation_plan",
		"protocols/.forge_workspace/implementation_plan/notes",
		"protocols/.forge_workspace/implementation_plan/evals/nested/deeper",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := InspectDomainWorkspace(root, cfg, "protocols")

	if !got.Clean {
		t.Errorf("empty subdirectory tree reported dirty: %+v", got)
	}
	if got.Error != "" {
		t.Errorf("empty subdirectory tree carried an error: %q", got.Error)
	}
}

func TestInspectDomainWorkspaceDotfileOnlyIsDirty(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	// A placeholder is still a regular file. Treating .gitkeep as clean would
	// let a workspace that git deliberately preserves slip through the gate.
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/nested/.gitkeep"), "")

	got := InspectDomainWorkspace(root, cfg, "protocols")

	if got.Clean {
		t.Errorf("dotfile-only workspace reported clean: %+v", got)
	}
}

func TestInspectDomainWorkspaceUntraversableIsDirtyWithError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission bits")
	}

	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	ws := filepath.Join(root, "protocols/.forge_workspace")
	locked := filepath.Join(ws, "locked")
	mustWriteFile(t, filepath.Join(locked, "plan.json"), "{}")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	// Restore permissions so t.TempDir cleanup can remove the tree.
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	got := InspectDomainWorkspace(root, cfg, "protocols")

	if got.Clean {
		t.Errorf("untraversable workspace reported clean: %+v", got)
	}
	if got.Error == "" {
		t.Error("untraversable workspace lost the OS error detail")
	}
	if !strings.Contains(got.Error, "locked") {
		t.Errorf("error detail does not name the offending path: %q", got.Error)
	}
}

func TestInspectDomainWorkspaceWritesNothing(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/plan.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/notes/a.md"), "note")
	if err := os.MkdirAll(filepath.Join(root, "launcher/.forge_workspace/empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	before := snapshotTree(t, root)

	// Inspect a dirty domain, a clean-but-present domain, and an absent one.
	// None of the three may touch the filesystem — preflight is documented as
	// non-mutating and the gate runs before init decides anything.
	InspectDomainWorkspace(root, cfg, "protocols")
	InspectDomainWorkspace(root, cfg, "launcher")
	InspectDomainWorkspace(root, cfg, "never-existed")

	after := snapshotTree(t, root)

	if len(before) != len(after) {
		t.Fatalf("tree changed: %d entries before, %d after\nbefore: %v\nafter:  %v",
			len(before), len(after), before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("entry %d changed:\n before: %s\n after:  %s", i, before[i], after[i])
		}
	}
}

// snapshotTree records every path under root with its mode and content size, so
// a creation, deletion, or modification anywhere in the tree shows up as a diff.
func snapshotTree(t *testing.T, root string) []string {
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

// --- Readiness verdict and its rendering -------------------------------------
//
// The verdict is what three separate cold-start entry points and the preflight
// command all consult, so the rendering is pinned exactly: an operator who hits
// the gate at init and again at a phase shift must see the same message, and
// that message is the only place the close-out remediation is stated.

func queueOf(domains ...string) PlanQueueInput {
	q := PlanQueueInput{}
	for i, d := range domains {
		q.Plans = append(q.Plans, PlanQueueEntry{
			Name:   fmt.Sprintf("Plan %d", i+1),
			Domain: d,
			File:   d + "/plan.json",
		})
	}
	return q
}

func TestEvaluateReadinessAllCleanIsReady(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	v := EvaluateReadiness(root, cfg, QueueDomains(queueOf("protocols", "launcher", "portal")))

	if !v.Ready {
		t.Fatalf("expected READY, got dirty domains: %+v", v.DirtyDomains)
	}
	if len(v.DirtyDomains) != 0 {
		t.Errorf("ready verdict listed %d dirty domains", len(v.DirtyDomains))
	}
	if len(v.Statuses) != 3 {
		t.Errorf("inspected %d domains, want 3", len(v.Statuses))
	}

	want := "Planning readiness: READY\nInspected 3 domain workspaces — all clean."
	if got := v.Render(); got != want {
		t.Errorf("READY output =\n%s\nwant:\n%s", got, want)
	}
}

func TestEvaluateReadinessTwoDirtyDomainsBlockWithRemediation(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/plan.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "launcher/.forge_workspace/notes/a.md"), "note")

	v := EvaluateReadiness(root, cfg, QueueDomains(queueOf("protocols", "launcher")))

	if v.Ready {
		t.Fatal("expected BLOCKED with two dirty workspaces")
	}
	if len(v.DirtyDomains) != 2 {
		t.Fatalf("dirty domains = %d, want 2", len(v.DirtyDomains))
	}

	got := v.Render()
	want := "Planning readiness: BLOCKED\n" +
		"The following domain workspaces contain prior-cycle artifacts:\n" +
		"  - protocols   protocols/.forge_workspace/\n" +
		"  - launcher    launcher/.forge_workspace/\n" +
		"Run the workspace close-out procedure for each domain to archive and clear it,\n" +
		"then re-run preflight."
	if got != want {
		t.Errorf("BLOCKED output =\n%s\n\nwant:\n%s", got, want)
	}
}

func TestEvaluateReadinessBlockedOutputOmitsCleanDomains(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	// Only the middle domain is dirty.
	mustWriteFile(t, filepath.Join(root, "launcher/.forge_workspace/plan.json"), "{}")

	v := EvaluateReadiness(root, cfg, QueueDomains(queueOf("protocols", "launcher", "portal")))

	if v.Ready {
		t.Fatal("one dirty domain among clean ones must block the verdict")
	}
	if len(v.DirtyDomains) != 1 || v.DirtyDomains[0].Domain != "launcher" {
		t.Fatalf("dirty domains = %+v, want only launcher", v.DirtyDomains)
	}

	got := v.Render()
	if !strings.Contains(got, "launcher") {
		t.Errorf("BLOCKED output does not name the dirty domain:\n%s", got)
	}
	for _, clean := range []string{"protocols", "portal"} {
		if strings.Contains(got, clean) {
			t.Errorf("BLOCKED output names clean domain %q:\n%s", clean, got)
		}
	}
}

func TestQueueDomainsDeduplicatesPreservingOrder(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/plan.json"), "{}")

	// Three entries, two of which name the same domain.
	domains := QueueDomains(queueOf("protocols", "launcher", "protocols"))
	want := []string{"protocols", "launcher"}
	if len(domains) != len(want) {
		t.Fatalf("domains = %v, want %v", domains, want)
	}
	for i := range want {
		if domains[i] != want[i] {
			t.Fatalf("domains = %v, want %v (queue order preserved)", domains, want)
		}
	}

	v := EvaluateReadiness(root, cfg, domains)
	if len(v.Statuses) != 2 {
		t.Errorf("inspected %d workspaces, want 2 — a repeated domain is inspected once", len(v.Statuses))
	}
	if n := strings.Count(v.Render(), "- protocols"); n != 1 {
		t.Errorf("dirty domain listed %d times, want 1", n)
	}
}

func TestEvaluateReadinessUntraversableDomainReportsErrorDetail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission bits")
	}

	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	locked := filepath.Join(root, "protocols/.forge_workspace/locked")
	mustWriteFile(t, filepath.Join(locked, "plan.json"), "{}")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	v := EvaluateReadiness(root, cfg, []string{"protocols"})

	if v.Ready {
		t.Fatal("an uninspectable workspace must block rather than pass")
	}
	got := v.Render()
	if !strings.Contains(got, "could not inspect:") {
		t.Errorf("BLOCKED output lost the traversal-failure detail:\n%s", got)
	}
	if !strings.Contains(got, "protocols") {
		t.Errorf("BLOCKED output does not name the domain:\n%s", got)
	}
}

func TestEvaluateReadinessIgnoresDomainsOutsideTheQueue(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")

	// Domain "a" is filthy, but the queue names only "b".
	mustWriteFile(t, filepath.Join(root, "a/.forge_workspace/plan.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "a/.forge_workspace/notes/x.md"), "x")

	v := EvaluateReadiness(root, cfg, QueueDomains(queueOf("b")))

	if !v.Ready {
		t.Fatalf("a dirty workspace outside the incoming queue must not block: %+v", v.DirtyDomains)
	}
	for _, s := range v.Statuses {
		if s.Domain == "a" {
			t.Errorf("domain %q was inspected despite not being in the queue", s.Domain)
		}
	}
	if got, want := v.Render(), "Planning readiness: READY\nInspected 1 domain workspace — all clean."; got != want {
		t.Errorf("READY output =\n%s\nwant:\n%s", got, want)
	}
}

func TestEvaluateReadinessIsStatelessAcrossCalls(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t, filepath.Join(root, "launcher/.forge_workspace/plan.json"), "{}")

	domains := QueueDomains(queueOf("protocols", "launcher"))

	first := EvaluateReadiness(root, cfg, domains)
	second := EvaluateReadiness(root, cfg, domains)

	// No marker file, no memo, no "already checked" shortcut: the same tree
	// must produce the same verdict every time it is asked.
	if first.Ready != second.Ready {
		t.Errorf("verdict changed between calls: %v then %v", first.Ready, second.Ready)
	}
	if first.Render() != second.Render() {
		t.Errorf("rendering changed between calls:\n%s\n\nthen:\n%s", first.Render(), second.Render())
	}
	if len(first.DirtyDomains) != len(second.DirtyDomains) {
		t.Errorf("dirty domain count changed: %d then %d", len(first.DirtyDomains), len(second.DirtyDomains))
	}
}

// --- Gate logging ------------------------------------------------------------
//
// The gate's log entries are the only record of why a planning cycle was — or
// was not — allowed to start. A blocked run leaves no state file and no
// workspace change, so without these entries the refusal is invisible after the
// terminal scrollback is gone.

// readGateLog returns the entries a logger wrote, decoded.
func readGateLog(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log %s: %v", path, err)
	}
	var entries []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e map[string]interface{}
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

// entriesAtLevel filters decoded entries by the level recorded in Detail.
func entriesAtLevel(entries []map[string]interface{}, level string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, e := range entries {
		detail, _ := e["detail"].(map[string]interface{})
		if detail != nil && detail["level"] == level {
			out = append(out, e)
		}
	}
	return out
}

func TestLogReadinessGateReadyRecordsInfoWithCount(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "planning-abcd1234.jsonl")
	logger := &Logger{enabled: true, path: logPath}

	v := EvaluateReadiness(root, cfgWithWorkspaceDir(".forge_workspace"),
		QueueDomains(queueOf("protocols", "launcher", "portal")))
	LogReadinessGate(logger, "init", PhasePlanning, "ORIENT", v)

	entries := readGateLog(t, logPath)
	info := entriesAtLevel(entries, "INFO")
	if len(info) != 1 {
		t.Fatalf("expected exactly 1 INFO entry, got %d", len(info))
	}
	detail := info[0]["detail"].(map[string]interface{})
	if got := detail["domains_inspected"]; got != float64(3) {
		t.Errorf("domains_inspected = %v, want 3", got)
	}
	if got := detail["verdict"]; got != "ready" {
		t.Errorf("verdict = %v, want ready", got)
	}
	// A ready verdict is not an error, so nothing should be logged as one.
	if n := len(entriesAtLevel(entries, "ERROR")); n != 0 {
		t.Errorf("a ready verdict must log no ERROR entry, got %d", n)
	}
}

func TestLogReadinessGateBlockedRecordsErrorAlongsideInfo(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t, filepath.Join(root, "protocols/.forge_workspace/plan.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "launcher/.forge_workspace/notes/n.md"), "notes")

	logPath := filepath.Join(t.TempDir(), "planning-abcd1234.jsonl")
	logger := &Logger{enabled: true, path: logPath}

	v := EvaluateReadiness(root, cfg, QueueDomains(queueOf("protocols", "launcher", "portal")))
	LogReadinessGate(logger, "advance", PhasePlanning, "ORIENT", v)

	entries := readGateLog(t, logPath)
	info := entriesAtLevel(entries, "INFO")
	if len(info) != 1 {
		t.Fatalf("the INFO entry must still be written when blocked, got %d", len(info))
	}
	if got := info[0]["detail"].(map[string]interface{})["verdict"]; got != "blocked" {
		t.Errorf("verdict = %v, want blocked", got)
	}

	errs := entriesAtLevel(entries, "ERROR")
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 ERROR entry, got %d", len(errs))
	}
	dirty, ok := errs[0]["detail"].(map[string]interface{})["dirty_domains"].([]interface{})
	if !ok {
		t.Fatalf("ERROR detail has no dirty_domains list: %v", errs[0]["detail"])
	}
	if len(dirty) != 2 {
		t.Fatalf("expected 2 dirty domains, got %d", len(dirty))
	}
	var names, paths []string
	for _, d := range dirty {
		m := d.(map[string]interface{})
		names = append(names, m["domain"].(string))
		paths = append(paths, m["workspace_path"].(string))
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "launcher,protocols" {
		t.Errorf("dirty domains = %v, want launcher and protocols", names)
	}
	for _, p := range paths {
		if !strings.Contains(p, ".forge_workspace") {
			t.Errorf("dirty entry missing its workspace path: %q", p)
		}
	}
}

func TestLogReadinessGateRecordsOneDebugEntryPerDomain(t *testing.T) {
	root := t.TempDir()
	cfg := cfgWithWorkspaceDir(".forge_workspace")
	mustWriteFile(t, filepath.Join(root, "launcher/.forge_workspace/plan.json"), "{}")

	logPath := filepath.Join(t.TempDir(), "planning-abcd1234.jsonl")
	logger := &Logger{enabled: true, path: logPath}

	v := EvaluateReadiness(root, cfg, QueueDomains(queueOf("protocols", "launcher", "portal")))
	LogReadinessGate(logger, "init", PhasePlanning, "ORIENT", v)

	debug := entriesAtLevel(readGateLog(t, logPath), "DEBUG")
	if len(debug) != 3 {
		t.Fatalf("expected one DEBUG entry per inspected domain (3), got %d", len(debug))
	}
	// Every domain must appear with its path and its own result — the count in
	// the INFO line is useless for diagnosis without these.
	seen := map[string]bool{}
	for _, e := range debug {
		d := e["detail"].(map[string]interface{})
		name := d["domain"].(string)
		seen[name] = true
		if path, _ := d["workspace_path"].(string); !strings.HasPrefix(path, name) {
			t.Errorf("domain %q logged workspace_path %q", name, path)
		}
		wantClean := name != "launcher"
		if got := d["clean"]; got != wantClean {
			t.Errorf("domain %q clean = %v, want %v", name, got, wantClean)
		}
	}
	for _, name := range []string{"protocols", "launcher", "portal"} {
		if !seen[name] {
			t.Errorf("no DEBUG entry for domain %q", name)
		}
	}
}

// Edge case: outside a session there is no session id, NewLogger returns a
// disabled logger, and nothing is written. This is why the gate needs no
// special case for preflight — the no-op falls out of the existing logger.
func TestLogReadinessGateWritesNothingWithoutASession(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("HOME", logDir)

	logger := NewLogger(LogsConfig{Enabled: true}, PhasePlanning, "")
	if logger.Enabled() {
		t.Fatal("a logger with no session id must be disabled")
	}

	v := EvaluateReadiness(t.TempDir(), cfgWithWorkspaceDir(".forge_workspace"),
		QueueDomains(queueOf("protocols")))
	LogReadinessGate(logger, "preflight", PhasePlanning, "ORIENT", v)

	files, err := filepath.Glob(filepath.Join(logDir, ".forgectl", "logs", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("no log file should exist outside a session, found %v", files)
	}
}
