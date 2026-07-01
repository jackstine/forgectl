package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var setCommitHashesDomain string

var setCommitHashesCmd = &cobra.Command{
	Use:   "set-commit-hashes [--domain <domain>] <hash> [<hash>...]",
	Short: "Set commit hashes for a domain's completed specs",
	RunE:  runSetCommitHashes,
}

func init() {
	setCommitHashesCmd.Flags().StringVar(&setCommitHashesDomain, "domain", "", "Domain name (required at DONE, inferred elsewhere)")
	rootCmd.AddCommand(setCommitHashesCmd)
}

func runSetCommitHashes(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("set-commit-hashes is only valid in the specifying phase (current phase: %s)", s.Phase)
	}

	// State check.
	validStates := map[state.StateName]bool{
		state.StateCrossReferenceReview: true,
		state.StateDone:                 true,
	}
	if !validStates[s.State] {
		return fmt.Errorf("set-commit-hashes is not valid in state %s", s.State)
	}

	// Require at least one hash argument.
	if len(args) == 0 {
		return fmt.Errorf("set-commit-hashes requires at least one hash argument")
	}

	// Domain inference.
	domain := setCommitHashesDomain
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

	// Validate domain has at least one completed spec, and set hashes on every completed spec in it.
	hasDomain := false
	for i := range s.Specifying.Completed {
		if s.Specifying.Completed[i].Domain == domain {
			hasDomain = true
			s.Specifying.Completed[i].CommitHashes = args
		}
	}
	if !hasDomain {
		return fmt.Errorf("domain %q has no completed specs; set-commit-hashes requires at least one completed spec for the domain", domain)
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set commit hashes for domain %q: %v\n", domain, args)
	return nil
}
