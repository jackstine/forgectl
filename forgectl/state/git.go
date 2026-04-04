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

// GitShowFiles returns the files changed in a commit.
func GitShowFiles(workDir string, hash string) ([]string, error) {
	cmd := exec.Command("git", "show", "--name-only", "--pretty=format:", hash)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// AddCommitToSpec appends a commit hash to a completed spec by ID.
func AddCommitToSpec(s *ForgeState, specID int, hash string) error {
	if s.Specifying == nil {
		return fmt.Errorf("no specifying phase data")
	}

	for i := range s.Specifying.Completed {
		if s.Specifying.Completed[i].ID == specID {
			// Check duplicate.
			for _, h := range s.Specifying.Completed[i].CommitHashes {
				if h == hash {
					return fmt.Errorf("commit %s already registered to spec %d", hash, specID)
				}
			}
			s.Specifying.Completed[i].CommitHashes = append(s.Specifying.Completed[i].CommitHashes, hash)
			return nil
		}
	}

	// Check if active.
	for _, cs := range s.Specifying.CurrentSpecs {
		if cs != nil && cs.ID == specID {
			return fmt.Errorf("spec %d is still active", specID)
		}
	}

	return fmt.Errorf("no completed spec with ID %d", specID)
}

// MatchedSpec identifies a completed spec matched by ReconcileCommit.
type MatchedSpec struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ReconcileCommit matches a commit's changed files against completed spec file paths.
// Returns the matched specs (ID + Name) for each spec that was updated.
func ReconcileCommit(s *ForgeState, workDir string, hash string) ([]MatchedSpec, error) {
	if s.Specifying == nil {
		return nil, fmt.Errorf("no specifying phase data")
	}

	files, err := GitShowFiles(workDir, hash)
	if err != nil {
		return nil, err
	}

	var updated []MatchedSpec
	for i := range s.Specifying.Completed {
		for _, f := range files {
			if f == s.Specifying.Completed[i].File {
				// Check duplicate.
				isDup := false
				for _, h := range s.Specifying.Completed[i].CommitHashes {
					if h == hash {
						isDup = true
						break
					}
				}
				if !isDup {
					s.Specifying.Completed[i].CommitHashes = append(s.Specifying.Completed[i].CommitHashes, hash)
					updated = append(updated, MatchedSpec{
						ID:   s.Specifying.Completed[i].ID,
						Name: s.Specifying.Completed[i].Name,
					})
				}
				break
			}
		}
	}

	return updated, nil
}
