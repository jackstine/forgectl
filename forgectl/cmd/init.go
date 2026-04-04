package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var (
	initBatchSize int
	initMinRounds int
	initMaxRounds int
	initFrom      string
	initPhase     string
	initGuided    bool
	initNoGuided  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a scaffold session",
	Long:  "Creates a state file from a validated input file.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().IntVar(&initBatchSize, "batch-size", 0, "Max items per batch (required)")
	initCmd.Flags().IntVar(&initMinRounds, "min-rounds", 1, "Minimum evaluation rounds (default 1)")
	initCmd.Flags().IntVar(&initMaxRounds, "max-rounds", 0, "Maximum evaluation rounds (required)")
	initCmd.Flags().StringVar(&initFrom, "from", "", "Path to input file (required)")
	initCmd.Flags().StringVar(&initPhase, "phase", "specifying", "Starting phase: specifying, planning, implementing")
	initCmd.Flags().BoolVar(&initGuided, "guided", false, "Enable guided mode (default)")
	initCmd.Flags().BoolVar(&initNoGuided, "no-guided", false, "Disable guided mode")
	_ = initCmd.MarkFlagRequired("from")
	_ = initCmd.MarkFlagRequired("batch-size")
	_ = initCmd.MarkFlagRequired("max-rounds")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if initBatchSize < 1 {
		return fmt.Errorf("--batch-size must be at least 1")
	}
	if initMinRounds < 1 {
		return fmt.Errorf("--min-rounds must be at least 1")
	}
	if initMinRounds > initMaxRounds {
		return fmt.Errorf("--min-rounds cannot exceed --max-rounds")
	}

	validPhases := map[string]bool{"specifying": true, "planning": true, "implementing": true}
	if !validPhases[initPhase] {
		return fmt.Errorf("--phase must be specifying, planning, or implementing")
	}

	if state.Exists(stateDir) {
		return fmt.Errorf("State file already exists. Delete it to reinitialize.")
	}

	data, err := os.ReadFile(initFrom)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", initFrom)
		}
		return fmt.Errorf("reading file: %w", err)
	}

	// Determine guided mode.
	userGuided := true // default
	if initNoGuided {
		userGuided = false
	}
	if initGuided {
		userGuided = true
	}

	phase := state.PhaseName(initPhase)
	out := cmd.OutOrStdout()

	cfg := state.DefaultForgeConfig()
	cfg.General.UserGuided = userGuided
	// Populate phase-specific batch/eval settings from CLI flags.
	// These will be superseded by TOML config loading in a future overhaul.
	cfg.Specifying.Batch = initBatchSize
	cfg.Specifying.Eval.MinRounds = initMinRounds
	cfg.Specifying.Eval.MaxRounds = initMaxRounds
	cfg.Planning.Batch = initBatchSize
	cfg.Planning.Eval.MinRounds = initMinRounds
	cfg.Planning.Eval.MaxRounds = initMaxRounds
	cfg.Implementing.Batch = initBatchSize
	cfg.Implementing.Eval.MinRounds = initMinRounds
	cfg.Implementing.Eval.MaxRounds = initMaxRounds

	s := &state.ForgeState{
		Phase:          phase,
		State:          state.StateOrient,
		Config:         cfg,
		StartedAtPhase: phase,
	}

	switch phase {
	case state.PhaseSpecifying:
		validationErrs := state.ValidateSpecQueue(data)
		if len(validationErrs) > 0 {
			printValidationErrors(out, validationErrs)
			fmt.Fprintln(out, "\nExpected schema:")
			fmt.Fprintln(out, state.SpecQueueSchema())
			return fmt.Errorf("input validation failed")
		}
		var input state.SpecQueueInput
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}
		s.Specifying = state.NewSpecifyingState(input.Specs)

	case state.PhasePlanning:
		validationErrs := state.ValidatePlanQueue(data)
		if len(validationErrs) > 0 {
			printValidationErrors(out, validationErrs)
			fmt.Fprintln(out, "\nExpected schema:")
			fmt.Fprintln(out, state.PlanQueueSchema())
			return fmt.Errorf("input validation failed")
		}
		var input state.PlanQueueInput
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}
		s.Planning = state.NewPlanningState(input.Plans)
		if len(s.Planning.Queue) > 0 {
			entry := s.Planning.Queue[0]
			s.Planning.Queue = s.Planning.Queue[1:]
			s.Planning.CurrentPlan = &state.ActivePlan{
				ID:              1,
				Name:            entry.Name,
				Domain:          entry.Domain,
				Topic:           entry.Topic,
				File:            entry.File,
				Specs:           entry.Specs,
				CodeSearchRoots: entry.CodeSearchRoots,
			}
		}

	case state.PhaseImplementing:
		// Validate as plan.json.
		validationErrs := state.ValidatePlanJSON(data, stateDir)
		if len(validationErrs) > 0 {
			printValidationErrors(out, validationErrs)
			return fmt.Errorf("plan validation failed")
		}
		var plan state.PlanJSON
		if err := json.Unmarshal(data, &plan); err != nil {
			return fmt.Errorf("parsing plan: %w", err)
		}

		// Add passes and rounds to items.
		for i := range plan.Items {
			plan.Items[i].Passes = "pending"
			plan.Items[i].Rounds = 0
		}

		// Write updated plan back.
		planData, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling plan: %w", err)
		}
		if err := os.WriteFile(initFrom, planData, 0644); err != nil {
			return fmt.Errorf("writing plan: %w", err)
		}

		s.Implementing = state.NewImplementingState()
		// We need a Planning reference for the plan file path.
		// Populate name and domain from plan.json context.
		s.Planning = &state.PlanningState{
			CurrentPlan: &state.ActivePlan{
				Name:   plan.Context.Module,
				Domain: plan.Context.Domain,
				File:   initFrom,
			},
		}
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	state.PrintAdvanceOutput(out, s, stateDir)

	return nil
}

func printValidationErrors(w interface{ Write([]byte) (int, error) }, errs []string) {
	fmt.Fprintln(w, "Validation errors:")
	for _, e := range errs {
		fmt.Fprintf(w, "  %s\n", e)
	}
}
