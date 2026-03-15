package cmd

import (
	"fmt"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show the current state and what to do next",
	RunE:  runNext,
}

func init() {
	rootCmd.AddCommand(nextCmd)
}

func runNext(cmd *cobra.Command, args []string) error {
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "State:   %s\n", s.State)

	if s.CurrentSpec != nil {
		fmt.Fprintf(out, "Spec:    %s\n", s.CurrentSpec.Name)
		fmt.Fprintf(out, "Domain:  %s\n", s.CurrentSpec.Domain)
		fmt.Fprintf(out, "File:    %s\n", s.CurrentSpec.File)
		if s.State == state.PhaseEvaluate || s.State == state.PhaseRefine {
			fmt.Fprintf(out, "Round:   %d/%d\n", s.CurrentSpec.Round, s.EvaluationRounds)
		}
	}

	fmt.Fprintf(out, "Action:  %s\n", state.ActionDescription(s))

	return nil
}
