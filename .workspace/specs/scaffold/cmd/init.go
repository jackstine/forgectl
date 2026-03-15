package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var (
	initMinRounds  int
	initMaxRounds  int
	initFrom       string
	initUserGuided bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a spec generation session",
	Long:  "Creates a scaffold state file from a validated queue input file.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().IntVar(&initMinRounds, "min-rounds", 1, "Minimum evaluation rounds before review (default 1)")
	initCmd.Flags().IntVar(&initMaxRounds, "max-rounds", 0, "Maximum evaluation rounds per spec (required)")
	initCmd.Flags().StringVar(&initFrom, "from", "", "Path to queue input JSON file (required)")
	initCmd.Flags().BoolVar(&initUserGuided, "user-guided", false, "Enable user discussion during SELECT phase")
	_ = initCmd.MarkFlagRequired("max-rounds")
	_ = initCmd.MarkFlagRequired("from")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if initMaxRounds < 1 {
		return fmt.Errorf("--max-rounds must be a positive integer, got %d", initMaxRounds)
	}
	if initMinRounds < 1 {
		return fmt.Errorf("--min-rounds must be a positive integer, got %d", initMinRounds)
	}
	if initMinRounds > initMaxRounds {
		return fmt.Errorf("--min-rounds (%d) cannot exceed --max-rounds (%d)", initMinRounds, initMaxRounds)
	}

	if state.Exists(stateDir) {
		return fmt.Errorf("state file already exists. Delete it to reinitialize")
	}

	data, err := os.ReadFile(initFrom)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", initFrom)
		}
		return fmt.Errorf("reading file: %w", err)
	}

	validationErrs := state.ValidateQueueInput(data)
	if len(validationErrs) > 0 {
		fmt.Fprintln(cmd.OutOrStderr(), "Validation errors:")
		for _, e := range validationErrs {
			fmt.Fprintf(cmd.OutOrStderr(), "  - %s\n", e)
		}
		fmt.Fprintln(cmd.OutOrStderr())
		fmt.Fprintln(cmd.OutOrStderr(), "Expected schema:")
		fmt.Fprintln(cmd.OutOrStderr(), state.ValidSchema())
		return fmt.Errorf("queue file validation failed")
	}

	var input state.QueueInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parsing queue file: %w", err)
	}

	warnings := state.CheckDependencies(input.Specs)
	for _, w := range warnings {
		fmt.Fprintln(cmd.OutOrStdout(), w)
	}

	s := state.NewState(initMinRounds, initMaxRounds, initUserGuided, input.Specs)
	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized with %d specs, %d-%d evaluation rounds per spec",
		len(input.Specs), initMinRounds, initMaxRounds)
	if initUserGuided {
		fmt.Fprint(cmd.OutOrStdout(), ", user-guided mode")
	}
	fmt.Fprintln(cmd.OutOrStdout(), ".")
	return nil
}
