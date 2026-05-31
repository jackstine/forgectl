package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var (
	advanceVerdict    string
	advanceEvalReport string
	advanceMessage    string
	advanceFile       string
	advanceFrom       string
	advanceGuided     bool
	advanceNoGuided   bool
)

var advanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Transition from current state to next",
	RunE:  runAdvance,
}

func init() {
	advanceCmd.Flags().StringVar(&advanceVerdict, "verdict", "", "PASS or FAIL")
	advanceCmd.Flags().StringVar(&advanceEvalReport, "eval-report", "", "Path to evaluation report")
	advanceCmd.Flags().StringVar(&advanceMessage, "message", "", "Commit message or acceptance message")
	advanceCmd.Flags().StringVar(&advanceFile, "file", "", "Override file path (specifying DRAFT)")
	advanceCmd.Flags().StringVar(&advanceFrom, "from", "", "Path to queue input file (phase shift)")
	advanceCmd.Flags().BoolVar(&advanceGuided, "guided", false, "Enable guided mode")
	advanceCmd.Flags().BoolVar(&advanceNoGuided, "no-guided", false, "Disable guided mode")
	rootCmd.AddCommand(advanceCmd)
}

func runAdvance(cmd *cobra.Command, args []string) error {
	projectRoot, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	// Validate context-dependent flag constraints and print warnings.
	if err2 := validateAdvanceFlags(s); err2 != nil {
		return err2
	}
	printAdvanceWarnings(out, s)

	// Build input.
	var guided *bool
	if cmd.Flags().Changed("guided") {
		g := true
		guided = &g
	}
	if cmd.Flags().Changed("no-guided") {
		g := false
		guided = &g
	}

	in := state.AdvanceInput{
		Verdict:    advanceVerdict,
		EvalReport: advanceEvalReport,
		Message:    advanceMessage,
		File:       advanceFile,
		From:       advanceFrom,
		Guided:     guided,
	}

	// Snapshot state before transition for logging. The reverse_engineering
	// domain and reconcile round are captured here because the transition
	// mutates them in place (e.g. RECONCILE_ADVANCE increments the domain).
	prevState := string(s.State)
	prevPhase := string(s.Phase)
	reCtx := captureRELogContext(s)

	err = state.Advance(s, in, projectRoot)
	if err != nil {
		// Check if it's a validation error — still save state if VALIDATE was entered.
		if ve, ok := err.(*state.ValidationError); ok {
			if err2 := state.Save(stateDir, s); err2 != nil {
				return fmt.Errorf("saving state: %w", err2)
			}
			fmt.Fprintln(out)
			state.PrintAdvanceOutput(out, s, projectRoot)
			fmt.Fprintln(out)
			fmt.Fprintf(out, "FAIL: %d errors in plan.json\n\n", len(ve.Errors))
			for _, e := range ve.Errors {
				fmt.Fprintf(out, "  %s\n", e)
			}
			return fmt.Errorf("validation failed")
		}
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Activity logging.
	detail := buildAdvanceDetail(in, reCtx)
	logger := state.NewLogger(s.Config.Logs, s.StartedAtPhase, s.SessionID)
	logger.Write(state.LogEntry{
		TS:        state.LogNow(),
		Cmd:       "advance",
		Phase:     prevPhase,
		PrevState: prevState,
		State:     string(s.State),
		Detail:    detail,
	})

	// Archive session at terminal states.
	if isTerminalState(s) {
		domain := sessionDomain(s)
		if archErr := state.ArchiveSession(stateDir, domain, s); archErr != nil {
			fmt.Fprintf(out, "Warning: failed to archive session: %v\n", archErr)
		}
	}

	state.PrintAdvanceOutput(out, s, projectRoot)

	return nil
}

// reLogContext carries the reverse_engineering domain/round context captured
// before a transition mutates the state. It is nil for other phases.
type reLogContext struct {
	domain          string
	round           int
	inReconcileEval bool
}

// captureRELogContext snapshots the current reverse_engineering domain and
// reconcile round before an advance transition mutates them. It returns nil
// when the session is not in the reverse_engineering phase, so the detail
// builder leaves the domain/round fields off for other phases.
func captureRELogContext(s *state.ForgeState) *reLogContext {
	if s.Phase != state.PhaseReverseEngineering || s.ReverseEngineering == nil {
		return nil
	}
	re := s.ReverseEngineering
	ctx := &reLogContext{
		round:           re.ReconcileRound,
		inReconcileEval: s.State == state.StateReconcileEval,
	}
	// DomainIndex is 1-based; guard the bounds so an uninitialised or
	// out-of-range index simply omits the domain rather than panicking.
	if re.DomainIndex >= 1 && re.DomainIndex <= len(re.Domains) {
		ctx.domain = re.Domains[re.DomainIndex-1]
	}
	return ctx
}

// buildAdvanceDetail builds the log detail map from advance flags and the
// pre-advance state context. For reverse_engineering advances it adds the
// current domain, and the reconcile round when advancing from RECONCILE_EVAL
// (where a verdict is also present).
func buildAdvanceDetail(in state.AdvanceInput, reCtx *reLogContext) map[string]interface{} {
	detail := map[string]interface{}{}
	if in.Verdict != "" {
		detail["verdict"] = in.Verdict
	}
	if in.EvalReport != "" {
		detail["eval_report"] = in.EvalReport
	}
	if reCtx != nil {
		if reCtx.domain != "" {
			detail["domain"] = reCtx.domain
		}
		if reCtx.inReconcileEval {
			detail["round"] = reCtx.round
		}
	}
	return detail
}

// isTerminalState returns true when the session has reached a terminal point
// that warrants archiving.
func isTerminalState(s *state.ForgeState) bool {
	// Implementing phase complete.
	if s.Phase == state.PhaseImplementing && s.State == state.StateDone {
		return true
	}
	// Specifying phase complete (phase shifting to planning, started at specifying).
	if s.State == state.StatePhaseShift &&
		s.PhaseShift != nil &&
		s.PhaseShift.From == state.PhaseSpecifying &&
		s.StartedAtPhase == state.PhaseSpecifying {
		return true
	}
	return false
}

// sessionDomain returns the domain name for archive file naming.
func sessionDomain(s *state.ForgeState) string {
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		return s.Planning.CurrentPlan.Domain
	}
	if s.Specifying != nil && len(s.Specifying.Completed) > 0 {
		return s.Specifying.Completed[0].Domain
	}
	return "unknown"
}

func validateAdvanceFlags(s *state.ForgeState) error {
	// --file only valid in specifying DRAFT.
	if advanceFile != "" && !(s.Phase == state.PhaseSpecifying && s.State == state.StateDraft) {
		return fmt.Errorf("--file is only valid in specifying DRAFT state (current: %s %s)", s.Phase, s.State)
	}

	// --verdict only valid in eval states.
	if advanceVerdict != "" {
		validStates := map[state.StateName]bool{
			state.StateEvaluate:           true,
			state.StateReconcileEval:      true,
			state.StateCrossReferenceEval: true,
		}
		if !validStates[s.State] {
			return fmt.Errorf("--verdict is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
		}
	}

	return nil
}

// printAdvanceWarnings prints warnings about flags that will be ignored due to config settings.
func printAdvanceWarnings(w interface{ Write([]byte) (int, error) }, s *state.ForgeState) {
	evalStates := map[state.StateName]bool{
		state.StateEvaluate:           true,
		state.StateReconcileEval:      true,
		state.StateCrossReferenceEval: true,
	}

	// Warn if --eval-report provided but the current phase's eval_mode is not "report".
	// This is the single user-facing emission of the warning; the state transition
	// layer does not re-print it, so the warning appears exactly once.
	if advanceEvalReport != "" && evalStates[s.State] {
		var ec state.EvalConfig
		switch s.Phase {
		case state.PhaseSpecifying:
			ec = s.Config.Specifying.Eval
		case state.PhasePlanning:
			ec = s.Config.Planning.Eval
		case state.PhaseImplementing:
			ec = s.Config.Implementing.Eval
		}
		if state.EvalModeFor(ec, s.Config.General) != "report" {
			fmt.Fprintf(w, "warning: --eval-report is ignored, --eval-report is only used in report mode\n")
		}
	}

	// Warn if --message provided but commits are disabled.
	if advanceMessage != "" && !s.Config.General.EnableCommits {
		commitStates := map[state.StateName]bool{
			state.StateComplete:  true,
			state.StateAccept:    true,
			state.StateImplement: true,
			state.StateCommit:    true,
		}
		if commitStates[s.State] {
			fmt.Fprintf(w, "warning: --message is ignored, commits are not enabled\n")
		}
	}
}
