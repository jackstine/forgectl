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

	fmt.Fprintln(out, "=== Session ===")
	fmt.Fprintf(out, "Evaluation rounds: %d-%d\n", s.MinRounds, s.MaxRounds)
	fmt.Fprintf(out, "User-guided:       %v\n", s.UserGuided)
	fmt.Fprintf(out, "State:             %s\n", s.State)
	fmt.Fprintln(out)

	if s.CurrentSpec != nil {
		fmt.Fprintln(out, "=== Current Spec ===")
		fmt.Fprintf(out, "ID:      %d\n", s.CurrentSpec.ID)
		fmt.Fprintf(out, "Name:    %s\n", s.CurrentSpec.Name)
		fmt.Fprintf(out, "Domain:  %s\n", s.CurrentSpec.Domain)
		fmt.Fprintf(out, "Topic:   %s\n", s.CurrentSpec.Topic)
		fmt.Fprintf(out, "File:    %s\n", s.CurrentSpec.File)
		if s.State == state.PhaseEvaluate || s.State == state.PhaseRefine || s.State == state.PhaseReview {
			fmt.Fprintf(out, "Round:   %d/%d\n", s.CurrentSpec.Round, s.MaxRounds)
		}
		if len(s.CurrentSpec.Evals) > 0 {
			fmt.Fprintln(out, "Eval history:")
			for _, e := range s.CurrentSpec.Evals {
				fmt.Fprintf(out, "  Round %d: %s", e.Round, e.Verdict)
				if len(e.Deficiencies) > 0 {
					fmt.Fprintf(out, " — %v", e.Deficiencies)
				}
				if e.Fixed != "" {
					fmt.Fprintf(out, " → Fixed: %s", e.Fixed)
				}
				fmt.Fprintln(out)
			}
		}
		fmt.Fprintln(out)
	}

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
				fmt.Fprintf(out, "  %d. %s\n", q.ID, q.Name)
			}
		}
		fmt.Fprintln(out)
	}

	if len(s.Completed) > 0 {
		fmt.Fprintln(out, "=== Completed ===")
		for _, c := range s.Completed {
			rounds := fmt.Sprintf("%d rounds", c.RoundsTaken)
			if c.CommitHash != "" {
				rounds += fmt.Sprintf(" [%s]", c.CommitHash)
			}
			verdict := "PASS"
			if len(c.Evals) > 0 {
				last := c.Evals[len(c.Evals)-1]
				verdict = last.Verdict
			}
			fmt.Fprintf(out, "  ✓ %d. %s (%s) — %s, last: %s\n", c.ID, c.Name, c.Domain, rounds, verdict)
		}
		fmt.Fprintln(out)
	}

	return nil
}
