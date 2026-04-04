package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var addDomainCmd = &cobra.Command{
	Use:   "add-domain <domain>",
	Short: "Add a domain during reverse engineering QUEUE state",
	Args:  cobra.ExactArgs(1),
	RunE:  runAddDomain,
}

func init() {
	rootCmd.AddCommand(addDomainCmd)
}

func runAddDomain(cmd *cobra.Command, args []string) error {
	_, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// State gate: only available during reverse_engineering QUEUE.
	if s.Phase != state.PhaseReverseEngineering || s.State != state.StateQueue {
		return fmt.Errorf("forgectl add-domain is only available during the QUEUE state.")
	}

	domain := args[0]
	re := s.ReverseEngineering

	// Reject duplicates.
	for _, d := range re.Domains {
		if d == domain {
			return fmt.Errorf("domain %q already exists", domain)
		}
	}

	re.Domains = append(re.Domains, domain)
	re.TotalDomains++

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added domain %q. Total domains: %d.\n", domain, re.TotalDomains)
	return nil
}
