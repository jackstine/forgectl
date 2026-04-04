package state

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// AutoCommit stages files per the given strategy and commits with message.
// Returns the full commit hash on success.
func AutoCommit(projectRoot string, strategy string, stageTargets []string, message string) (string, error) {
	var addArgs []string
	switch strategy {
	case "strict", "all-specs", "scoped":
		// Stage specific paths passed in stageTargets.
		addArgs = append([]string{"-C", projectRoot, "add"}, stageTargets...)
	case "tracked":
		addArgs = []string{"-C", projectRoot, "add", "-u"}
	case "all":
		addArgs = []string{"-C", projectRoot, "add", "-A"}
	default:
		return "", fmt.Errorf("unknown commit strategy %q", strategy)
	}

	addCmd := exec.Command("git", addArgs...)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %s", strings.TrimSpace(string(out)))
	}

	commitCmd := exec.Command("git", "-C", projectRoot, "commit", "-m", message)
	commitOut, commitErr := commitCmd.CombinedOutput()
	if commitErr != nil {
		outStr := strings.TrimSpace(string(commitOut))
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "nothing added to commit") {
			fmt.Fprintf(os.Stderr, "notice: nothing to commit, skipping\n")
			return "", nil
		}
		return "", fmt.Errorf("git commit failed: %s", outStr)
	}

	hashCmd := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD")
	out, err := hashCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitHashExists checks if a commit hash exists in the repository.
func GitHashExists(workDir string, hash string) error {
	cmd := exec.Command("git", "cat-file", "-t", hash)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("commit %q does not exist in the repository", hash)
	}
	objType := strings.TrimSpace(string(out))
	if objType != "commit" {
		return fmt.Errorf("%q is a %s, not a commit", hash, objType)
	}
	return nil
}

// GitRepoRoot returns the root directory of the git repository containing workDir.
func GitRepoRoot(workDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", err)
	}
	return strings.TrimSpace(string(out)), nil
}

