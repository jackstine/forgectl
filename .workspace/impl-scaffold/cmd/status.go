package cmd

import (
	"fmt"
	"strings"

	"impl-scaffold/state"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print full session state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !state.Exists(dir) {
			return fmt.Errorf("no state file found. Run init first")
		}

		s, err := state.Load(dir)
		if err != nil {
			return err
		}

		fmt.Printf("=== Session Config ===\n")
		fmt.Printf("Min Rounds:  %d\n", s.MinRounds)
		fmt.Printf("Max Rounds:  %d\n", s.MaxRounds)
		fmt.Printf("Sub-Agents:  %d\n", s.SubAgents)
		fmt.Printf("User Guided: %v\n", s.UserGuided)
		fmt.Printf("State:       %s\n", s.State)
		fmt.Println()

		if s.CurrentPlan != nil {
			p := s.CurrentPlan
			fmt.Printf("=== Current Plan ===\n")
			fmt.Printf("ID:      %d\n", p.ID)
			fmt.Printf("Name:    %s\n", p.Name)
			fmt.Printf("Domain:  %s\n", p.Domain)
			fmt.Printf("Topic:   %s\n", p.Topic)
			fmt.Printf("File:    %s\n", p.File)
			if len(p.Specs) > 0 {
				fmt.Printf("Specs:   %s\n", strings.Join(p.Specs, ", "))
			}
			if len(p.CodeSearchRoots) > 0 {
				fmt.Printf("Roots:   %s\n", strings.Join(p.CodeSearchRoots, ", "))
			}
			fmt.Printf("Round:   %d\n", p.Round)
			fmt.Println()

			if p.Study.SpecsNotes != "" || p.Study.CodeNotes != "" || p.Study.PackagesNotes != "" {
				fmt.Printf("--- Study Notes ---\n")
				if p.Study.SpecsNotes != "" {
					fmt.Printf("Specs:    %s\n", p.Study.SpecsNotes)
				}
				if p.Study.CodeNotes != "" {
					fmt.Printf("Code:     %s\n", p.Study.CodeNotes)
				}
				if p.Study.PackagesNotes != "" {
					fmt.Printf("Packages: %s\n", p.Study.PackagesNotes)
				}
				fmt.Println()
			}

			if len(p.Evals) > 0 {
				fmt.Printf("--- Eval History ---\n")
				for _, e := range p.Evals {
					fmt.Printf("Round %d: %s", e.Round, e.Verdict)
					if len(e.Deficiencies) > 0 {
						fmt.Printf(" [%s]", strings.Join(e.Deficiencies, ", "))
					}
					if e.Fixed != "" {
						fmt.Printf(" → Fixed: %s", e.Fixed)
					}
					fmt.Println()
				}
				fmt.Println()
			}
		}

		if len(s.Queue) > 0 {
			fmt.Printf("=== Queue (%d) ===\n", len(s.Queue))
			for _, q := range s.Queue {
				fmt.Printf("  [%d] %s (%s)\n", q.ID, q.Name, q.Domain)
			}
			fmt.Println()
		}

		if len(s.Completed) > 0 {
			fmt.Printf("=== Completed (%d) ===\n", len(s.Completed))
			for _, c := range s.Completed {
				fmt.Printf("  [%d] %s (%s) — %d round(s)\n", c.ID, c.Name, c.Domain, c.RoundsTaken)
				for _, e := range c.Evals {
					fmt.Printf("       Round %d: %s", e.Round, e.Verdict)
					if len(e.Deficiencies) > 0 {
						fmt.Printf(" [%s]", strings.Join(e.Deficiencies, ", "))
					}
					if e.Fixed != "" {
						fmt.Printf(" → Fixed: %s", e.Fixed)
					}
					fmt.Println()
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
