package state

import (
	"io/fs"
	"os"
	"path/filepath"
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
