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
	initCmd.Flags().StringVar(&initFrom, "from", "", "Path to input file (required for session init)")
	initCmd.Flags().StringVar(&initPhase, "phase", "specifying", "Starting phase: specifying, planning, implementing, ui_implementing, reverse_engineering")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	fromSet := initFrom != ""
	phaseSet := cmd.Flags().Changed("phase")

	// --phase without --from has no meaning.
	if phaseSet && !fromSet {
		return fmt.Errorf("--from is required when --phase is set.")
	}

	out := cmd.OutOrStdout()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	scaffold, err := state.Scaffold(cwd)
	if err != nil {
		return err
	}
	if scaffold.CreatedConfig {
		fmt.Fprintln(out, "Created default .forgectl/config. Domain-aware spec placement is not enforced until [[domains]] entries are added to it.")
	}

	// Scaffold-only mode: no --from provided, scaffolding is the entire action.
	if !fromSet {
		return nil
	}

	// Reject generate_planning_queue as an explicit --phase value.
	if initPhase == string(state.PhaseGeneratePlanningQueue) {
		return fmt.Errorf("generate_planning_queue requires a completed specifying phase. Use --phase specifying instead.")
	}

	validPhases := map[string]bool{"specifying": true, "planning": true, "implementing": true, "ui_implementing": true, "reverse_engineering": true}
	if !validPhases[initPhase] {
		return fmt.Errorf("--phase must be specifying, planning, implementing, ui_implementing, or reverse_engineering")
	}

	// Load and validate config. Scaffolding guarantees the file exists; any read
	// failure here is a permission/IO error, and a parse failure means an existing
	// user-authored config is malformed.
	projectRoot := scaffold.ProjectRoot
	cfg, err := state.LoadConfig(projectRoot)
	if err != nil {
		return err
	}
	stateDir := state.StateDir(projectRoot, cfg)

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
		s.Implementing.CurrentPlanFile = initFrom
		s.Implementing.CurrentPlanDomain = plan.Context.Domain
		s.Planning = &state.PlanningState{
			CurrentPlan: &state.ActivePlan{
				Name:   plan.Context.Module,
				Domain: plan.Context.Domain,
				File:   initFrom,
			},
		}

	case state.PhaseUIImplementing:
		// The QA and e2e loops require a launchable app and a test runner, so the
		// required UI config keys must be present and non-empty before init.
		var missing []string
		if cfg.UIImplementing.App.LaunchCommand == "" {
			missing = append(missing, "ui_implementing.app.launch_command")
		}
		if cfg.UIImplementing.App.URL == "" {
			missing = append(missing, "ui_implementing.app.url")
		}
		if cfg.UIImplementing.E2E.TestCommand == "" {
			missing = append(missing, "ui_implementing.e2e.test_command")
		}
		if cfg.UIImplementing.E2E.TestDir == "" {
			missing = append(missing, "ui_implementing.e2e.test_dir")
		}
		if len(missing) > 0 {
			for _, k := range missing {
				fmt.Fprintf(out, "missing required UI config key: %s\n", k)
			}
			return fmt.Errorf("ui_implementing requires non-empty config keys: %v", missing)
		}

		// Otherwise identical to the implementing case: validate plan.json,
		// reset tracking fields, and build the ui_implementing state.
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

		s.UIImplementing = state.NewUIImplementingState()
		s.UIImplementing.CurrentPlanFile = initFrom
		s.UIImplementing.CurrentPlanDomain = plan.Context.Domain
		s.Planning = &state.PlanningState{
			CurrentPlan: &state.ActivePlan{
				Name:   plan.Context.Module,
				Domain: plan.Context.Domain,
				File:   initFrom,
			},
		}

	case state.PhaseReverseEngineering:
		validationErrs := state.ValidateReverseEngineeringInput(data)
		if len(validationErrs) > 0 {
			printValidationErrors(out, validationErrs)
			fmt.Fprintln(out, "\nExpected schema:")
			fmt.Fprintln(out, state.ReverseEngineeringInitSchema())
			return fmt.Errorf("input validation failed")
		}
		var input state.ReverseEngineeringInitInput
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}
		reState := state.NewReverseEngineeringState(input.Concept, input.Domains)
		reState.ColleagueReview = cfg.ReverseEngineering.Reconcile.ColleagueReview
		s.ReverseEngineering = reState
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
	case state.PhaseUIImplementing:
		return cfg.UIImplementing.Batch, cfg.UIImplementing.Eval.MinRounds, cfg.UIImplementing.Eval.MaxRounds
	case state.PhaseReverseEngineering:
		// No batching in reverse engineering; rounds reflect the reconcile loop.
		return 0, cfg.ReverseEngineering.Reconcile.MinRounds, cfg.ReverseEngineering.Reconcile.MaxRounds
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
