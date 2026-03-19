package cmd

import (
	"fmt"
	"os"

	"impl-scaffold/state"

	"github.com/spf13/cobra"
)

var (
	fromFlag      string
	minRoundsFlag int
	maxRoundsFlag int
	subAgentsFlag int
	userGuidedFlag bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new implementation plan session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if state.Exists(dir) {
			return fmt.Errorf("state file already exists. Delete it to reinitialize")
		}

		if maxRoundsFlag == 0 {
			return fmt.Errorf("--max-rounds is required")
		}
		if minRoundsFlag > maxRoundsFlag {
			return fmt.Errorf("--min-rounds cannot exceed --max-rounds")
		}
		if subAgentsFlag < 1 {
			return fmt.Errorf("--sub-agents must be at least 1")
		}

		data, err := os.ReadFile(fromFlag)
		if err != nil {
			return fmt.Errorf("cannot read queue file: %w", err)
		}

		errs := state.ValidateQueueInput(data)
		if len(errs) > 0 {
			cmd.PrintErrln("Validation errors:")
			for _, e := range errs {
				cmd.PrintErrf("  - %s\n", e)
			}
			cmd.PrintErrln("\nExpected schema:")
			cmd.PrintErrln(state.ValidSchema())
			return fmt.Errorf("queue validation failed")
		}

		plans, err := state.ParseQueue(data)
		if err != nil {
			return fmt.Errorf("cannot parse queue: %w", err)
		}

		s := state.NewState(minRoundsFlag, maxRoundsFlag, subAgentsFlag, userGuidedFlag, plans)
		if err := state.Save(dir, s); err != nil {
			return err
		}

		cmd.Println(state.FormatState(s))
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&fromFlag, "from", "", "Path to queue input JSON file (required)")
	initCmd.Flags().IntVar(&minRoundsFlag, "min-rounds", 1, "Minimum evaluation rounds")
	initCmd.Flags().IntVar(&maxRoundsFlag, "max-rounds", 0, "Maximum evaluation rounds (required)")
	initCmd.Flags().IntVar(&subAgentsFlag, "sub-agents", 3, "Number of sub-agents for STUDY_CODE")
	initCmd.Flags().BoolVar(&userGuidedFlag, "user-guided", false, "Pause at SELECT for discussion")
	initCmd.MarkFlagRequired("from")
	initCmd.MarkFlagRequired("max-rounds")
	rootCmd.AddCommand(initCmd)
}
