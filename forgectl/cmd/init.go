package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var (
	initFrom  string
	initPhase string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a scaffold session",
	Long:  "Creates a state file from a validated input file and project config.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initFrom, "from", "", "Path to input file (required)")
	initCmd.Flags().StringVar(&initPhase, "phase", "specifying", "Starting phase: specifying, planning, implementing")
	_ = initCmd.MarkFlagRequired("from")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Reject generate_planning_queue as an explicit --phase value.
	if initPhase == string(state.PhaseGeneratePlanningQueue) {
		return fmt.Errorf("generate_planning_queue requires a completed specifying phase. Use --phase specifying instead.")
	}

	validPhases := map[string]bool{"specifying": true, "planning": true, "implementing": true}
	if !validPhases[initPhase] {
		return fmt.Errorf("--phase must be specifying, planning, or implementing")
	}

	// Discover project root, load and validate config.
	projectRoot, stateDir, cfg, err := resolveSession()
	if err != nil {
		return err
	}

	violations := state.ValidateConfig(cfg)
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(cmd.OutOrStdout(), v)
		}
		return fmt.Errorf("config validation failed")
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

	sessionID := state.GenerateSessionID()
	phase := state.PhaseName(initPhase)
	out := cmd.OutOrStdout()

	s := &state.ForgeState{
		Phase:          phase,
		State:          state.StateOrient,
		Config:         cfg,
		SessionID:      sessionID,
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

		// Validate domain config against spec queue when domains are configured.
		if len(cfg.Domains) > 0 {
			domainPaths := map[string]string{}
			for _, d := range cfg.Domains {
				domainPaths[d.Name] = d.Path
			}
			for i, spec := range input.Specs {
				if _, ok := domainPaths[spec.Domain]; !ok {
					return fmt.Errorf("specs[%d]: domain %q not found in config domains", i, spec.Domain)
				}
				expectedPrefix := domainPaths[spec.Domain] + "/specs/"
				if len(spec.File) < len(expectedPrefix) || spec.File[:len(expectedPrefix)] != expectedPrefix {
					return fmt.Errorf("specs[%d]: file %q must start with %s", i, spec.File, expectedPrefix)
				}
			}
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
				File:            entry.File,
				Specs:           entry.Specs,
				SpecCommits:     entry.SpecCommits,
				CodeSearchRoots: entry.CodeSearchRoots,
			}
		}

	case state.PhaseImplementing:
		validationErrs := state.ValidatePlanJSON(data, stateDir)
		if len(validationErrs) > 0 {
			printValidationErrors(out, validationErrs)
			return fmt.Errorf("plan validation failed")
		}
		var plan state.PlanJSON
		if err := json.Unmarshal(data, &plan); err != nil {
			return fmt.Errorf("parsing plan: %w", err)
		}

		for i := range plan.Items {
			plan.Items[i].Passes = "pending"
			plan.Items[i].Rounds = 0
		}

		planData, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling plan: %w", err)
		}
		if err := os.WriteFile(initFrom, planData, 0644); err != nil {
			return fmt.Errorf("writing plan: %w", err)
		}

		s.Implementing = state.NewImplementingState()
		s.Planning = &state.PlanningState{
			CurrentPlan: &state.ActivePlan{
				Name:   plan.Context.Module,
				Domain: plan.Context.Domain,
				File:   initFrom,
			},
		}
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Activity logging — prune first, then create file, then write init entry.
	state.PruneLogs(cfg.Logs)
	logger := state.NewLogger(cfg.Logs, phase, sessionID)
	batchSize, minRounds, maxRounds := phaseRoundConfig(cfg, phase)
	logger.Write(state.LogEntry{
		TS:    state.LogNow(),
		Cmd:   "init",
		Phase: string(phase),
		State: string(s.State),
		Detail: map[string]interface{}{
			"from":       initFrom,
			"batch_size": batchSize,
			"rounds":     fmt.Sprintf("%d-%d", minRounds, maxRounds),
			"guided":     cfg.General.UserGuided,
		},
	})

	state.PrintAdvanceOutput(out, s, projectRoot)

	return nil
}

// phaseRoundConfig returns batch size and min/max rounds for the given phase.
func phaseRoundConfig(cfg state.ForgeConfig, phase state.PhaseName) (batchSize, minRounds, maxRounds int) {
	switch phase {
	case state.PhaseSpecifying:
		return cfg.Specifying.Batch, cfg.Specifying.Eval.MinRounds, cfg.Specifying.Eval.MaxRounds
	case state.PhasePlanning:
		return cfg.Planning.Batch, cfg.Planning.Eval.MinRounds, cfg.Planning.Eval.MaxRounds
	case state.PhaseImplementing:
		return cfg.Implementing.Batch, cfg.Implementing.Eval.MinRounds, cfg.Implementing.Eval.MaxRounds
	default:
		return 0, 0, 0
	}
}

func printValidationErrors(w interface{ Write([]byte) (int, error) }, errs []string) {
	fmt.Fprintln(w, "Validation errors:")
	for _, e := range errs {
		fmt.Fprintf(w, "  %s\n", e)
	}
}
