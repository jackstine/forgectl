package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"scaffold/state"

	"github.com/spf13/cobra"
)

var (
	initRounds    int
	initFrom      string
	initUserGuided bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a spec generation session",
	Long:  "Creates a scaffold state file from a validated queue input file.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().IntVar(&initRounds, "rounds", 0, "Number of evaluation rounds per spec (required)")
	initCmd.Flags().StringVar(&initFrom, "from", "", "Path to queue input JSON file (required)")
	initCmd.Flags().BoolVar(&initUserGuided, "user-guided", false, "Enable user discussion during SELECT phase")
	_ = initCmd.MarkFlagRequired("rounds")
	_ = initCmd.MarkFlagRequired("from")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if initRounds < 1 {
		return fmt.Errorf("--rounds must be a positive integer, got %d", initRounds)
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

	// Validate against schema.
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

	// Parse the validated input.
	var input state.QueueInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parsing queue file: %w", err)
	}

	// Check dependency warnings.
	warnings := state.CheckDependencies(input.Specs)
	for _, w := range warnings {
		fmt.Fprintln(cmd.OutOrStdout(), w)
	}

	// Create and save state.
	s := state.NewState(initRounds, initUserGuided, input.Specs)
	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized with %d specs, %d evaluation rounds per spec", len(input.Specs), initRounds)
	if initUserGuided {
		fmt.Fprint(cmd.OutOrStdout(), ", user-guided mode")
	}
	fmt.Fprintln(cmd.OutOrStdout(), ".")
	return nil
}
