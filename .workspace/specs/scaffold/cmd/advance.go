package cmd

import (
	"fmt"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var (
	advanceFile    string
	advanceVerdict string
)

var advanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Transition from current state to next",
	RunE:  runAdvance,
}

func init() {
	advanceCmd.Flags().StringVar(&advanceFile, "file", "", "Spec file path (required in DRAFT state)")
	advanceCmd.Flags().StringVar(&advanceVerdict, "verdict", "", "PASS or FAIL (required in EVALUATE state)")
	rootCmd.AddCommand(advanceCmd)
}

func runAdvance(cmd *cobra.Command, args []string) error {
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	prevState := s.State

	if err := state.Advance(s, advanceFile, advanceVerdict); err != nil {
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s → %s\n", prevState, s.State)

	if s.CurrentSpec != nil {
		fmt.Fprintf(out, "Spec:    %s\n", s.CurrentSpec.Name)
	}

	fmt.Fprintf(out, "Action:  %s\n", state.ActionDescription(s))

	return nil
}
