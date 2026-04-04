package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var setRootsDomain string

var setRootsCmd = &cobra.Command{
	Use:   "set-roots [--domain <domain>] <path> [<path>...]",
	Short: "Set code search roots for a domain",
	RunE:  runSetRoots,
}

func init() {
	setRootsCmd.Flags().StringVar(&setRootsDomain, "domain", "", "Domain name (required at DONE, inferred elsewhere)")
	rootCmd.AddCommand(setRootsCmd)
}

func runSetRoots(cmd *cobra.Command, args []string) error {
	_, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// Phase check.
	if s.Phase != state.PhaseSpecifying {
		return fmt.Errorf("set-roots is only valid in the specifying phase (current phase: %s)", s.Phase)
	}

	// State check.
	validStates := map[state.StateName]bool{
		state.StateCrossReferenceReview: true,
		state.StateDone:                 true,
	}
	if !validStates[s.State] {
		return fmt.Errorf("set-roots is not valid in state %s", s.State)
	}

	// Require at least one path argument.
	if len(args) == 0 {
		return fmt.Errorf("set-roots requires at least one path argument")
	}

	// Domain inference.
	domain := setRootsDomain
	if domain == "" {
		switch s.State {
		case state.StateDone:
			return fmt.Errorf("--domain is required at state DONE")
		case state.StateCrossReferenceReview:
			domain = s.Specifying.CurrentDomain
		}
	}
	if domain == "" {
		return fmt.Errorf("--domain is required (cannot infer domain from current state)")
	}

	// Validate domain has at least one completed spec.
	hasDomain := false
	for _, c := range s.Specifying.Completed {
		if c.Domain == domain {
			hasDomain = true
			break
		}
	}
	if !hasDomain {
		return fmt.Errorf("domain %q has no completed specs; set-roots requires at least one completed spec for the domain", domain)
	}

	// Store roots.
	if s.Specifying.Domains == nil {
		s.Specifying.Domains = make(map[string]state.DomainMeta)
	}
	s.Specifying.Domains[domain] = state.DomainMeta{CodeSearchRoots: args}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set code search roots for domain %q: %v\n", domain, args)
	return nil
}
