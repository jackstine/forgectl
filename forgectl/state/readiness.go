package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DomainWorkspaceStatus is the per-domain result of a workspace inspection.
//
// It is pure inspection: producing one carries no verdict, prints nothing, and
// writes nothing. The readiness gate aggregates these into a verdict; the
// inspection itself only reports what is on disk.
type DomainWorkspaceStatus struct {
	// Domain is the domain name as it appears in the plan queue entry.
	Domain string `json:"domain"`
	// WorkspacePath is the project-root-relative path that was inspected,
	// rendered as "<domain-path>/<workspace_dir>/" — with the trailing
	// separator, because that is the form the gate's blocked output prints and
	// the form the spec's data model defines.
	WorkspacePath string `json:"workspace_path"`
	// Clean is true when the workspace directory is absent or contains no
	// regular files at any depth.
	Clean bool `json:"clean"`
	// Error carries the OS error detail when the directory exists but could not
	// be traversed. When set, Clean is false — an uninspectable workspace must
	// never pass silently as clean.
	Error string `json:"error,omitempty"`
}

// DefaultWorkspaceDirName is the workspace directory name used when config
// carries no explicit paths.workspace_dir value.
const DefaultWorkspaceDirName = ".forge_workspace"

// DomainPath resolves a domain name to its project-root-relative directory.
//
// A domain declared under [[domains]] resolves to its configured path. Domains
// are optional, so an undeclared name falls back to the name itself — the
// convention the rest of the scaffold already relies on. A name may contain a
// path separator ("protocols/ws1"), which resolves to that nested directory
// rather than to a single flat component.
func DomainPath(cfg ForgeConfig, domain string) string {
	for _, d := range cfg.Domains {
		if d.Name == domain && d.Path != "" {
			return filepath.Clean(d.Path)
		}
	}
	return filepath.Clean(domain)
}

// DomainWorkspacePath returns the project-root-relative workspace path for a
// domain: "<domain-path>/<workspace_dir>/", with a trailing separator.
//
// The workspace directory name comes from paths.workspace_dir rather than a
// hardcoded literal, so a project that renames its workspace directory is
// inspected at the location it actually uses.
func DomainWorkspacePath(cfg ForgeConfig, domain string) string {
	workspaceDir := cfg.Paths.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = DefaultWorkspaceDirName
	}
	return filepath.Join(DomainPath(cfg, domain), workspaceDir) + string(filepath.Separator)
}

// InspectDomainWorkspace classifies a domain's workspace directory as clean or
// dirty. It performs no writes of any kind — no directory creation, no file
// creation, no deletion.
//
// Clean: the directory does not exist, or it exists and contains no regular
// files at any depth (a tree of empty subdirectories is clean).
//
// Dirty: any regular file at any depth, including dotfiles and placeholders —
// a lone .gitkeep is a prior-cycle artifact like any other. Also dirty when the
// directory exists but cannot be traversed, with the OS error retained: the
// ambiguity must block rather than silently pass.
//
// The walk stops at the first regular file found rather than traversing the
// whole tree; a workspace holding thousands of files answers as fast as one
// holding a single file.
func InspectDomainWorkspace(projectRoot string, cfg ForgeConfig, domain string) DomainWorkspaceStatus {
	relPath := DomainWorkspacePath(cfg, domain)
	status := DomainWorkspaceStatus{
		Domain:        domain,
		WorkspacePath: relPath,
		Clean:         true,
	}

	absPath := relPath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(projectRoot, relPath)
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Absent workspace — nothing was ever written, or close-out
			// removed it. Clean.
			return status
		}
		status.Clean = false
		status.Error = err.Error()
		return status
	}
	if !info.IsDir() {
		// Something occupies the workspace path but is not a directory, so it
		// cannot be traversed. Report it rather than guessing.
		status.Clean = false
		status.Error = absPath + " is not a directory"
		return status
	}

	walkErr := filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A directory that cannot be read surfaces here rather than
			// aborting the walk, which is what lets a permission failure be
			// reported as dirty-with-detail instead of a false clean.
			status.Clean = false
			status.Error = err.Error()
			return filepath.SkipAll
		}
		// WalkDir does not follow symlinks and a symlink is not a regular
		// file, so a dangling link cannot fabricate an error or a false dirty.
		if d.Type().IsRegular() {
			status.Clean = false
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil && status.Error == "" {
		status.Clean = false
		status.Error = walkErr.Error()
	}

	return status
}

// ReadinessVerdict is the aggregate result of a readiness evaluation across an
// incoming plan queue's domain set.
//
// The verdict is stateless: it is derived entirely from the filesystem and the
// supplied domain list. It consults no marker file and records nothing, so
// evaluating twice over an unchanged tree gives an identical answer. The
// consequence is deliberate — the gate guarantees a workspace is clean, not
// that its prior contents were ever archived. Archival discipline belongs to
// the close-out procedure, which the gate can direct an operator to but cannot
// verify.
type ReadinessVerdict struct {
	// Ready is true iff every inspected workspace is clean.
	Ready bool `json:"ready"`
	// Statuses is every domain inspected, in queue order. Retained in full so
	// callers can log a per-domain result, not only the failures.
	Statuses []DomainWorkspaceStatus `json:"statuses"`
	// DirtyDomains is the subset with Clean false; empty when Ready.
	DirtyDomains []DomainWorkspaceStatus `json:"dirty_domains"`
}

// QueueDomains returns the distinct domains named by a plan queue, in queue
// order. A domain named by several entries is inspected — and reported — once.
func QueueDomains(queue PlanQueueInput) []string {
	var domains []string
	seen := make(map[string]bool, len(queue.Plans))
	for _, p := range queue.Plans {
		if p.Domain == "" || seen[p.Domain] {
			continue
		}
		seen[p.Domain] = true
		domains = append(domains, p.Domain)
	}
	return domains
}

// EvaluateReadiness inspects each domain's workspace and assembles the verdict.
// Only the domains passed in are inspected — a dirty workspace belonging to a
// domain outside the incoming queue is never looked at and never blocks.
func EvaluateReadiness(projectRoot string, cfg ForgeConfig, domains []string) ReadinessVerdict {
	verdict := ReadinessVerdict{Ready: true}
	for _, d := range domains {
		status := InspectDomainWorkspace(projectRoot, cfg, d)
		verdict.Statuses = append(verdict.Statuses, status)
		if !status.Clean {
			verdict.Ready = false
			verdict.DirtyDomains = append(verdict.DirtyDomains, status)
		}
	}
	return verdict
}

// Render returns the operator-facing verdict text, without a trailing newline.
//
// The same blocked rendering is used by preflight and by all three cold-start
// enforcement points, so an operator sees one message wherever the gate fires.
func (v ReadinessVerdict) Render() string {
	if v.Ready {
		return "Planning readiness: READY\n" +
			fmt.Sprintf("Inspected %d %s — all clean.", len(v.Statuses), pluralWorkspaces(len(v.Statuses)))
	}

	var b strings.Builder
	b.WriteString("Planning readiness: BLOCKED\n")
	b.WriteString("The following domain workspaces contain prior-cycle artifacts:\n")

	// Pad the domain column so the paths line up and the list stays scannable
	// when domain names differ in length.
	width := 0
	for _, d := range v.DirtyDomains {
		if len(d.Domain) > width {
			width = len(d.Domain)
		}
	}
	for _, d := range v.DirtyDomains {
		b.WriteString(fmt.Sprintf("  - %-*s   %s", width, d.Domain, d.WorkspacePath))
		if d.Error != "" {
			// A workspace that could not be traversed is dirty for a different
			// reason than one holding files; without the detail the operator
			// has no way to tell those apart or to fix the second kind.
			b.WriteString(fmt.Sprintf("   (could not inspect: %s)", d.Error))
		}
		b.WriteString("\n")
	}

	b.WriteString("Run the workspace close-out procedure for each domain to archive and clear it,\n")
	b.WriteString("then re-run preflight.")
	return b.String()
}

// Log level names carried in a gate log entry's Detail. LogEntry has no level
// field of its own — the activity log is one JSONL stream shared by every
// command — so the level travels in Detail rather than in a second log file or
// a changed record shape.
const (
	logLevelInfo  = "INFO"
	logLevelError = "ERROR"
	logLevelDebug = "DEBUG"
)

// LogReadinessGate records a gate evaluation to the activity log.
//
// Three levels, matching the spec's observability table: one INFO entry with the
// inspected-domain count and the verdict, one ERROR entry naming the dirty
// domains when entry is blocked, and one DEBUG entry per inspected domain with
// its workspace path and result. The per-domain DEBUG entries are what make a
// blocked run diagnosable after the fact — the INFO line says how many were
// looked at, but only DEBUG says which paths those were.
//
// Callers pass the logger they already built for their entry point. A logger
// constructed with an empty session id is a no-op (see NewLogger), which is what
// keeps a session-less preflight from writing a log file without any special
// case here.
func LogReadinessGate(logger *Logger, cmd string, phase PhaseName, stateName string, v ReadinessVerdict) {
	if logger == nil || !logger.Enabled() {
		return
	}

	verdict := "ready"
	if !v.Ready {
		verdict = "blocked"
	}
	entry := func(level string, detail map[string]interface{}) LogEntry {
		detail["level"] = level
		detail["gate"] = "planning_readiness"
		return LogEntry{
			TS:     LogNow(),
			Cmd:    cmd,
			Phase:  string(phase),
			State:  stateName,
			Detail: detail,
		}
	}

	logger.Write(entry(logLevelInfo, map[string]interface{}{
		"domains_inspected": len(v.Statuses),
		"verdict":           verdict,
	}))

	if !v.Ready {
		dirty := make([]map[string]interface{}, 0, len(v.DirtyDomains))
		for _, d := range v.DirtyDomains {
			dirty = append(dirty, map[string]interface{}{
				"domain":         d.Domain,
				"workspace_path": d.WorkspacePath,
			})
		}
		logger.Write(entry(logLevelError, map[string]interface{}{
			"message":       "planning entry blocked by the readiness gate",
			"dirty_domains": dirty,
		}))
	}

	for _, st := range v.Statuses {
		logger.Write(entry(logLevelDebug, map[string]interface{}{
			"domain":         st.Domain,
			"workspace_path": st.WorkspacePath,
			"clean":          st.Clean,
		}))
	}
}

// ReadinessError is the failure a cold-start planning entry returns when the
// gate blocks it.
//
// Its message is the verdict's rendered text verbatim, so an operator sees the
// identical dirty-domain list and close-out remediation whether they asked with
// preflight or tripped the gate at init or a phase shift. Carrying the verdict
// rather than a flattened string also lets a caller inspect which domains were
// dirty without parsing the message back apart.
type ReadinessError struct {
	Verdict ReadinessVerdict
}

func (e *ReadinessError) Error() string { return e.Verdict.Render() }

// pluralWorkspaces keeps the READY line grammatical for a single-domain queue,
// which is the common case for a one-domain project.
func pluralWorkspaces(n int) string {
	if n == 1 {
		return "domain workspace"
	}
	return "domain workspaces"
}
