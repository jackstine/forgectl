package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var addDomainCmd = &cobra.Command{
	Use:   "add-domain <domain>",
	Short: "Add a domain to the reverse_engineering domain list (QUEUE state only)",
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

	// State-gated: only available during the reverse_engineering QUEUE state.
	if s.Phase != state.PhaseReverseEngineering || s.State != state.StateQueue {
		return fmt.Errorf("forgectl add-domain is only available during the QUEUE state.")
	}

	domain := args[0]
	re := s.ReverseEngineering
	if re == nil {
		return fmt.Errorf("reverse_engineering state is not initialized")
	}

	// Reject a domain already in the list.
	for _, d := range re.Domains {
		if d == domain {
			return fmt.Errorf("domain %q already exists", domain)
		}
	}

	// Append to the end of the domain order and update the count. The
	// SURVEY→GAP_ANALYSIS→DECOMPOSE→QUEUE loop does not re-run for the added
	// domain — the user is responsible for having performed that analysis.
	re.Domains = append(re.Domains, domain)
	re.DomainCount = len(re.Domains)

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added domain %q. Domain list: %v\n", domain, re.Domains)
	return nil
}
