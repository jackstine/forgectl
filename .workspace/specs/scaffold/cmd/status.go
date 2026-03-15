package cmd

import (
	"fmt"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show full session state",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	// Session config.
	fmt.Fprintln(out, "=== Session ===")
	fmt.Fprintf(out, "Evaluation rounds: %d\n", s.EvaluationRounds)
	fmt.Fprintf(out, "User-guided:       %v\n", s.UserGuided)
	fmt.Fprintf(out, "State:             %s\n", s.State)
	fmt.Fprintln(out)

	// Current spec.
	if s.CurrentSpec != nil {
		fmt.Fprintln(out, "=== Current Spec ===")
		fmt.Fprintf(out, "Name:    %s\n", s.CurrentSpec.Name)
		fmt.Fprintf(out, "Domain:  %s\n", s.CurrentSpec.Domain)
		fmt.Fprintf(out, "Topic:   %s\n", s.CurrentSpec.Topic)
		fmt.Fprintf(out, "File:    %s\n", s.CurrentSpec.File)
		if s.State == state.PhaseEvaluate || s.State == state.PhaseRefine {
			fmt.Fprintf(out, "Round:   %d/%d\n", s.CurrentSpec.Round, s.EvaluationRounds)
		}
		fmt.Fprintln(out)
	}

	// Queue grouped by domain.
	if len(s.Queue) > 0 {
		fmt.Fprintln(out, "=== Queue ===")
		domainOrder := []string{}
		grouped := map[string][]state.QueueSpec{}
		for _, q := range s.Queue {
			if _, ok := grouped[q.Domain]; !ok {
				domainOrder = append(domainOrder, q.Domain)
			}
			grouped[q.Domain] = append(grouped[q.Domain], q)
		}
		for _, domain := range domainOrder {
			fmt.Fprintf(out, "[%s]\n", domain)
			for _, q := range grouped[domain] {
				fmt.Fprintf(out, "  - %s\n", q.Name)
			}
		}
		fmt.Fprintln(out)
	}

	// Completed.
	if len(s.Completed) > 0 {
		fmt.Fprintln(out, "=== Completed ===")
		for _, c := range s.Completed {
			fmt.Fprintf(out, "  ✓ %s (%s) — %d rounds\n", c.Name, c.Domain, c.RoundsTaken)
		}
		fmt.Fprintln(out)
	}

	return nil
}
