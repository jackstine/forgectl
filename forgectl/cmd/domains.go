package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "List configured domains",
	Long:  "List all domains declared in .forgectl/config. Use this to confirm domain setup before initializing a session.",
	Args:  cobra.NoArgs,
	RunE:  runDomains,
}

func init() {
	rootCmd.AddCommand(domainsCmd)
}

func runDomains(cmd *cobra.Command, args []string) error {
	projectRoot, _, cfg, err := resolveSession()
	if err != nil {
		return err
	}
	_ = projectRoot

	out := cmd.OutOrStdout()

	if len(cfg.Domains) == 0 {
		fmt.Fprintf(out, "No domains configured in .forgectl/config.\n")
		fmt.Fprintf(out, "\n")
		fmt.Fprintf(out, "Without explicit domains, forgectl derives groupings from spec file paths at runtime.\n")
		fmt.Fprintf(out, "For multi-subsystem projects, declare domains explicitly:\n")
		fmt.Fprintf(out, "\n")
		fmt.Fprintf(out, "  [[domains]]\n")
		fmt.Fprintf(out, "  name = \"my-domain\"\n")
		fmt.Fprintf(out, "  path = \"my-domain\"\n")
		return nil
	}

	fmt.Fprintf(out, "Configured domains (%d):\n\n", len(cfg.Domains))
	for _, d := range cfg.Domains {
		fmt.Fprintf(out, "  %-24s  %s/specs/\n", d.Name, d.Path)
	}

	// Check for an active session and show RE domains if present.
	stateDir := state.StateDir(projectRoot, cfg)
	if state.Exists(stateDir) {
		s, err := state.Load(stateDir)
		if err == nil && s.Phase == state.PhaseReverseEngineering && s.ReverseEngineering != nil {
			reDomains := s.ReverseEngineering.Domains
			if len(reDomains) > 0 {
				fmt.Fprintf(out, "\nActive reverse_engineering session domains (%d):\n\n", len(reDomains))
				for _, d := range reDomains {
					fmt.Fprintf(out, "  %s\n", d)
				}
			}
		}
	}

	return nil
}
