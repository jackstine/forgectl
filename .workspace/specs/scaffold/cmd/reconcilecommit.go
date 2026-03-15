package cmd

import (
	"fmt"
	"path/filepath"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var reconcileCommitHash string

var reconcileCommitCmd = &cobra.Command{
	Use:   "reconcile-commit",
	Short: "Register a commit to all specs it touched",
	Long:  "Determines which spec files were modified in a commit and adds the hash to each matching completed spec automatically.",
	RunE:  runReconcileCommit,
}

func init() {
	reconcileCommitCmd.Flags().StringVar(&reconcileCommitHash, "hash", "", "Commit hash to register (required)")
	_ = reconcileCommitCmd.MarkFlagRequired("hash")
	rootCmd.AddCommand(reconcileCommitCmd)
}

func runReconcileCommit(cmd *cobra.Command, args []string) error {
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	absDir, _ := filepath.Abs(stateDir)
	repoRoot, err := state.GitRepoRoot(absDir)
	if err != nil {
		return fmt.Errorf("cannot find git repo: %w", err)
	}

	// Validate hash exists in git.
	if err := state.GitHashExists(repoRoot, reconcileCommitHash); err != nil {
		return err
	}

	updated, err := state.ReconcileCommit(s, repoRoot, reconcileCommitHash)
	if err != nil {
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Registered %s to %d specs:\n", reconcileCommitHash, len(updated))
	for _, name := range updated {
		fmt.Fprintf(out, "  - %s\n", name)
	}

	return nil
}
