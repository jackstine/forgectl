package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindProjectRoot walks up from cwd until .forgectl/ is found.
// Returns the directory containing .forgectl/ or an error.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".forgectl")); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("No .forgectl directory found.")
		}
		dir = parent
	}
}
