package cmd

import (
	"fmt"
	"path/filepath"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var reconcileHash string

var reconcileCommitCmd = &cobra.Command{
	Use:   "reconcile-commit",
	Short: "Auto-register a commit to all specs it touched",
	RunE:  runReconcileCommit,
}

func init() {
	reconcileCommitCmd.Flags().StringVar(&reconcileHash, "hash", "", "Commit hash (required)")
	_ = reconcileCommitCmd.MarkFlagRequired("hash")
	rootCmd.AddCommand(reconcileCommitCmd)
}

func runReconcileCommit(cmd *cobra.Command, args []string) error {
	_, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	absDir, _ := filepath.Abs(stateDir)
	repoRoot, err := state.GitRepoRoot(absDir)
	if err != nil {
		return fmt.Errorf("cannot find git repo: %w", err)
	}

	if err := state.GitHashExists(repoRoot, reconcileHash); err != nil {
		return fmt.Errorf("commit does not exist in the repository")
	}

	updated, err := state.ReconcileCommit(s, repoRoot, reconcileHash)
	if err != nil {
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Activity logging.
	logger := state.NewLogger(s.Config.Logs, s.StartedAtPhase, s.SessionID)
	matchedSpecs := make([]map[string]interface{}, len(updated))
	for i, ms := range updated {
		matchedSpecs[i] = map[string]interface{}{"id": ms.ID, "name": ms.Name}
	}
	logger.Write(state.LogEntry{
		TS:    state.LogNow(),
		Cmd:   "reconcile-commit",
		Phase: string(s.Phase),
		State: string(s.State),
		Detail: map[string]interface{}{
			"hash":          reconcileHash,
			"matched_specs": matchedSpecs,
		},
	})

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Registered %s to %d specs:\n", reconcileHash, len(updated))
	for _, ms := range updated {
		fmt.Fprintf(out, "  - %s\n", ms.Name)
	}

	return nil
}
