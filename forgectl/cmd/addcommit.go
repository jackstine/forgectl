package cmd

import (
	"fmt"
	"path/filepath"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var (
	addCommitID   int
	addCommitHash string
)

var addCommitCmd = &cobra.Command{
	Use:   "add-commit",
	Short: "Register a commit hash to a completed spec",
	RunE:  runAddCommit,
}

func init() {
	addCommitCmd.Flags().IntVar(&addCommitID, "id", 0, "Spec ID (required)")
	addCommitCmd.Flags().StringVar(&addCommitHash, "hash", "", "Commit hash (required)")
	_ = addCommitCmd.MarkFlagRequired("id")
	_ = addCommitCmd.MarkFlagRequired("hash")
	rootCmd.AddCommand(addCommitCmd)
}

func runAddCommit(cmd *cobra.Command, args []string) error {
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
	if err := state.GitHashExists(repoRoot, addCommitHash); err != nil {
		return fmt.Errorf("commit does not exist in the repository")
	}

	if err := state.AddCommitToSpec(s, addCommitID, addCommitHash); err != nil {
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Activity logging.
	specName := ""
	if s.Specifying != nil {
		for _, cs := range s.Specifying.Completed {
			if cs.ID == addCommitID {
				specName = cs.Name
				break
			}
		}
	}
	logger := state.NewLogger(s.Config.Logs, s.StartedAtPhase, s.SessionID)
	logger.Write(state.LogEntry{
		TS:    state.LogNow(),
		Cmd:   "add-commit",
		Phase: string(s.Phase),
		State: string(s.State),
		Detail: map[string]interface{}{
			"spec_id":   addCommitID,
			"spec_name": specName,
			"hash":      addCommitHash,
		},
	})

	fmt.Fprintf(cmd.OutOrStdout(), "Added %s to spec %d.\n", addCommitHash, addCommitID)
	return nil
}
