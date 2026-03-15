package cmd

import (
	"fmt"
	"path/filepath"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var (
	addCommitID   int
	addCommitHash string
)

var addCommitCmd = &cobra.Command{
	Use:   "add-commit",
	Short: "Add a commit hash to a completed spec",
	Long:  "Associates an additional commit hash with a completed spec by ID. Use when eval fixes or reconciliation changes produce commits after the initial acceptance.",
	RunE:  runAddCommit,
}

func init() {
	addCommitCmd.Flags().IntVar(&addCommitID, "id", 0, "Spec ID to add commit to (required)")
	addCommitCmd.Flags().StringVar(&addCommitHash, "hash", "", "Commit hash to add (required)")
	_ = addCommitCmd.MarkFlagRequired("id")
	_ = addCommitCmd.MarkFlagRequired("hash")
	rootCmd.AddCommand(addCommitCmd)
}

func runAddCommit(cmd *cobra.Command, args []string) error {
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// Validate hash exists in git.
	absDir, _ := filepath.Abs(stateDir)
	repoRoot, err := state.GitRepoRoot(absDir)
	if err != nil {
		return fmt.Errorf("cannot find git repo: %w", err)
	}
	if err := state.GitHashExists(repoRoot, addCommitHash); err != nil {
		return err
	}

	if err := state.AddCommitToSpec(s, addCommitID, addCommitHash); err != nil {
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	out := cmd.OutOrStdout()

	// Find the spec to show confirmation.
	for _, c := range s.Completed {
		if c.ID == addCommitID {
			fmt.Fprintf(out, "Added %s to spec %d (%s). Total commits: %d.\n",
				addCommitHash, c.ID, c.Name, len(c.CommitHashes))
			break
		}
	}

	return nil
}
