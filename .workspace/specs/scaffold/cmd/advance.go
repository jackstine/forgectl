package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var (
	advanceFile         string
	advanceVerdict      string
	advanceMessage      string
	advanceDeficiencies string
	advanceFixed        string
)

var advanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Transition from current state to next",
	RunE:  runAdvance,
}

func init() {
	advanceCmd.Flags().StringVar(&advanceFile, "file", "", "Spec file path (optional in DRAFT state — overrides queue value)")
	advanceCmd.Flags().StringVar(&advanceVerdict, "verdict", "", "PASS or FAIL (required in EVALUATE state, optional in REVIEW)")
	advanceCmd.Flags().StringVar(&advanceMessage, "message", "", "Git commit message (required with --verdict PASS)")
	advanceCmd.Flags().StringVar(&advanceDeficiencies, "deficiencies", "", "Comma-separated failed dimensions (with --verdict FAIL)")
	advanceCmd.Flags().StringVar(&advanceFixed, "fixed", "", "Description of what was fixed (in REFINE state)")
	rootCmd.AddCommand(advanceCmd)
}

func runAdvance(cmd *cobra.Command, args []string) error {
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// Validate --message is required when verdict is PASS in EVALUATE.
	if s.State == state.PhaseEvaluate && advanceVerdict == "PASS" && advanceMessage == "" {
		return fmt.Errorf("--message is required when --verdict is PASS. Provide a commit message for the accepted spec")
	}

	prevState := s.State
	needsCommit := s.State == state.PhaseEvaluate && advanceVerdict == "PASS"

	// Capture spec file before advance mutates state.
	var specFile string
	if s.CurrentSpec != nil {
		specFile = s.CurrentSpec.File
	}

	// Parse deficiencies.
	var deficiencies []string
	if advanceDeficiencies != "" {
		for _, d := range strings.Split(advanceDeficiencies, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				deficiencies = append(deficiencies, d)
			}
		}
	}

	in := state.AdvanceInput{
		File:         advanceFile,
		Verdict:      advanceVerdict,
		Deficiencies: deficiencies,
		Fixed:        advanceFixed,
	}

	if err := state.Advance(s, in); err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	// Auto-commit on EVALUATE(PASS) → ACCEPT.
	if needsCommit && s.State == state.PhaseAccept {
		absDir, _ := filepath.Abs(stateDir)
		repoRoot, err := state.GitRepoRoot(absDir)
		if err != nil {
			fmt.Fprintf(out, "Warning: cannot find git repo: %v\n", err)
			fmt.Fprintf(out, "Please commit manually: %s\n", specFile)
		} else {
			if err := state.Save(stateDir, s); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}

			absSpecPath := filepath.Join(repoRoot, specFile)

			hash, err := state.GitCommit(repoRoot, []string{absSpecPath}, advanceMessage)
			if err != nil {
				fmt.Fprintf(out, "Warning: auto-commit failed: %v\n", err)
				fmt.Fprintf(out, "Please commit manually: %s\n", specFile)
			} else {
				s.LastCommitHash = hash
				fmt.Fprintf(out, "Committed: %s\n", hash)
			}
		}
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(out, "%s → %s\n", prevState, s.State)

	if s.CurrentSpec != nil {
		fmt.Fprintf(out, "Spec:    %s\n", s.CurrentSpec.Name)
	}

	fmt.Fprintf(out, "Action:  %s\n", state.ActionDescription(s))

	return nil
}
