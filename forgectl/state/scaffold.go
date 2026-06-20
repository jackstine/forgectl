package state

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// defaultConfigTemplate is the canonical commented configuration written to
// .forgectl/config when none exists. It is a verbatim copy of the reference at
// docs/default-config.toml; TestEmbeddedTemplateMatchesDocs guards against drift.
//
//go:embed default-config.toml
var defaultConfigTemplate string

// DefaultConfigTemplate returns the embedded default config TOML — the content
// scaffolding writes for a fresh project. Parsing it yields a configuration
// byte-for-byte equivalent in effect to DefaultForgeConfig().
func DefaultConfigTemplate() string {
	return defaultConfigTemplate
}

// ScaffoldResult reports what configuration scaffolding established.
type ScaffoldResult struct {
	// ProjectRoot is the directory containing the .forgectl/ the session will use.
	ProjectRoot string
	// CreatedDir is true when .forgectl/ was created during this invocation.
	CreatedDir bool
	// CreatedConfig is true when .forgectl/config was written during this invocation.
	CreatedConfig bool
}

// Scaffold guarantees that .forgectl/ and .forgectl/config exist, creating only
// what is missing. It walks up from cwd looking for an ancestor .forgectl/
// directory; if found, that ancestor is the project root and no new directory is
// created. If none is found anywhere in the hierarchy, .forgectl/ is created at
// cwd, which becomes the project root. The default config template is written
// (atomically) only when no config file already exists at the project root.
//
// Scaffolding is non-destructive: an existing config is never read, modified, or
// overwritten. Directory and config creation are independent idempotent steps, so
// a half-completed bootstrap is safely resumable on a later invocation.
func Scaffold(cwd string) (ScaffoldResult, error) {
	var res ScaffoldResult

	// Step 1-3: resolve the project root, creating .forgectl/ only if no
	// ancestor directory-form .forgectl/ exists.
	root, err := FindProjectRoot(cwd)
	if err != nil {
		root = cwd
		dir := filepath.Join(root, ".forgectl")
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return res, fmt.Errorf("creating .forgectl directory: %w", mkErr)
		}
		res.CreatedDir = true
	}
	res.ProjectRoot = root

	// Step 4-6: write the default config only when no config file is present.
	configPath := filepath.Join(root, ".forgectl", "config")
	if info, statErr := os.Stat(configPath); statErr == nil && info.Mode().IsRegular() {
		// An existing regular config file is authoritative; leave it untouched.
		return res, nil
	}

	if err := atomicWriteFile(configPath, []byte(defaultConfigTemplate), 0644); err != nil {
		return res, fmt.Errorf("writing default .forgectl/config: %w", err)
	}
	res.CreatedConfig = true
	return res, nil
}

// atomicWriteFile writes data to path via a temp file then rename, so a failure
// never leaves partial or truncated content at path. The temp file is removed if
// the rename fails (e.g., the target is occupied by a directory).
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
