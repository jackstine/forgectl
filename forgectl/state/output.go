package state

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"forgectl/evaluators"
)

// CurrentEvalMode returns the effective eval mode for the phase the session is
// currently in, selecting that phase's eval block and resolving it through
// EvalModeFor (which honors the legacy enable_eval_output back-compat). Phases
// without a standard eval block fall back to the implementing block.
func (s *ForgeState) CurrentEvalMode() string {
	switch s.Phase {
	case PhaseSpecifying:
		return EvalModeFor(s.Config.Specifying.Eval, s.Config.General)
	case PhasePlanning:
		return EvalModeFor(s.Config.Planning.Eval, s.Config.General)
	default:
		return EvalModeFor(s.Config.Implementing.Eval, s.Config.General)
	}
}

// PrintAdvanceOutput prints the action description for the new state after advance.
func PrintAdvanceOutput(w io.Writer, s *ForgeState, dir string) {
	switch s.Phase {
	case PhaseSpecifying:
		printSpecifyingOutput(w, s, dir)
	case PhaseGeneratePlanningQueue:
		printGeneratePlanningQueueOutput(w, s)
	case PhasePlanning:
		printPlanningOutput(w, s, dir)
	case PhaseImplementing:
		printImplementingOutput(w, s, dir)
	case PhaseReverseEngineering:
		printReverseEngineeringOutput(w, s, dir)
	}

	// Phase shift output is printed regardless of phase.
	if s.State == StatePhaseShift && s.PhaseShift != nil {
		printPhaseShiftOutput(w, s)
	}
}

// --- Specifying ---

// evalEntryAction carries the per-state wording used to render the Action body
// of a state that enters evaluation (EVALUATE, CROSS_REFERENCE_EVAL,
// RECONCILE_EVAL). The body is rendered differently per eval_mode:
//   - report:         spawn to evaluate, run forgectl eval, advance with --eval-report.
//   - direct:         spawn to evaluate and correct, files staged, advance without report.
//   - conversational: spawn to evaluate, run forgectl eval, advance without report.
type evalEntryAction struct {
	label        string // line prefix incl. trailing spaces, e.g. "Action:  "
	indent       string // continuation indent aligned under label
	spawnEval    string // report/conversational spawn line (verb "evaluate ...")
	spawnCorrect string // direct spawn line (verb "evaluate and correct ...")
	runEval      string // report/conversational middle line, e.g. "The sub-agent should run: forgectl eval"
	stagedNote   string // direct staged-files note
	reportTail   string // report-mode advance tail, e.g. "advance with --verdict PASS|FAIL --eval-report <path>"
}

// writeEvalEntryAction renders the Action body for an eval-entry state according
// to the resolved eval_mode. The header lines (State/Phase/Round/...) are written
// by the caller; this writes only the "Action:" block.
func writeEvalEntryAction(w io.Writer, mode string, a evalEntryAction) {
	const noReport = "advance with --verdict PASS|FAIL"
	switch mode {
	case "direct":
		fmt.Fprintf(w, "%s%s\n", a.label, a.spawnCorrect)
		fmt.Fprintf(w, "%sSub-agent runs: forgectl eval\n", a.indent)
		fmt.Fprintf(w, "%s%s\n", a.indent, a.stagedNote)
		fmt.Fprintf(w, "%sAfter completion of the above, %s\n", a.indent, noReport)
	case "report":
		fmt.Fprintf(w, "%s%s\n", a.label, a.spawnEval)
		fmt.Fprintf(w, "%s%s\n", a.indent, a.runEval)
		fmt.Fprintf(w, "%sAfter completion of the above, %s\n", a.indent, a.reportTail)
	default: // conversational
		fmt.Fprintf(w, "%s%s\n", a.label, a.spawnEval)
		fmt.Fprintf(w, "%s%s\n", a.indent, a.runEval)
		fmt.Fprintf(w, "%sAfter completion of the above, %s\n", a.indent, noReport)
	}
}

// writeRefineBody writes the per-mode correction-guidance lines shared by REFINE
// and the implementing IMPLEMENT after-eval re-entry: a mode-specific two-line
// lead-in followed by the shared "fresh eyes" lines. firstLabel prefixes the
// first emitted line (e.g. "Action:  ", or the continuation indent when a
// preamble such as "Minimum evaluation rounds not met." was already printed on
// the Action label). indent prefixes every subsequent line. Per eval_mode:
//   - report:         "Study the eval file <file>" / "and implement any corrections as needed."
//   - direct:         "Review unstaged changes from the evaluator (git diff)." / "Accept, revise, or revert corrections as needed."
//   - conversational: "Make corrections based off communication with the evaluator." / "Implement any corrections as needed."
func writeRefineBody(w io.Writer, mode, evalFile, firstLabel, indent string) {
	switch mode {
	case "direct":
		fmt.Fprintf(w, "%sReview unstaged changes from the evaluator (git diff).\n", firstLabel)
		fmt.Fprintf(w, "%sAccept, revise, or revert corrections as needed.\n", indent)
	case "report":
		fmt.Fprintf(w, "%sStudy the eval file %q\n", firstLabel, evalFile)
		fmt.Fprintf(w, "%sand implement any corrections as needed.\n", indent)
	default: // conversational
		fmt.Fprintf(w, "%sMake corrections based off communication with the evaluator.\n", firstLabel)
		fmt.Fprintf(w, "%sImplement any corrections as needed.\n", indent)
	}
	fmt.Fprintf(w, "%sApply \"fresh\" eyes and a tightened lens when reviewing the work,\n", indent)
	fmt.Fprintf(w, "%sthen apply corrections as needed.\n", indent)
}

// writeImplementReviewReminders writes the spec-review (and optional
// reference-review) reminders shared by every IMPLEMENT action — first round
// and every subsequent round. The item context is re-presented on each round,
// so both lines are phrased "if you have not already done so." When the plan
// has spec_commits the spec reminder points at the per-spec `Read:` git
// command; when it does not, no such command was rendered, so the reminder
// tells the engineer to read the spec files directly. The reference reminder is
// emitted only when the item has Refs.
func writeImplementReviewReminders(w io.Writer, indent string, hasSpecCommits, hasRefs bool) {
	if hasSpecCommits {
		fmt.Fprintf(w, "%sPlease review the specification(s) above if you have not already done so —\n", indent)
		fmt.Fprintf(w, "%srun the git command shown under each spec to read its definition.\n", indent)
	} else {
		fmt.Fprintf(w, "%sPlease review the specification(s) above if you have not already done so —\n", indent)
		fmt.Fprintf(w, "%sread the spec file(s) listed above.\n", indent)
	}
	if hasRefs {
		fmt.Fprintf(w, "%sPlease review the reference file(s) under Refs if you have not already done so.\n", indent)
	}
}

// writeEvalTrailingSections renders the per-mode --- PREVIOUS EVALUATIONS ---
// and --- REPORT OUTPUT --- sections shared by every eval-context output
// function. It emits a single leading blank line before the first section it
// writes (and one between the two), so callers should NOT pre-emit a separator.
//   - report:         lists prior rounds as "Round n: VERDICT — <report path>";
//                     REPORT OUTPUT names the report file to write.
//   - direct:         lists prior rounds as "Round n: VERDICT — (direct corrections)";
//                     REPORT OUTPUT instructs direct corrections to the <directNoun>
//                     files — UNLESS directShowsReport is false (reconciliation),
//                     where the REPORT OUTPUT section is omitted entirely.
//   - conversational: both sections omitted.
func writeEvalTrailingSections(w io.Writer, mode string, evals []EvalRecord, reportFile, directNoun string, directShowsReport bool) {
	if mode != "report" && mode != "direct" {
		return // conversational (or unknown) — omit both sections.
	}
	if len(evals) > 0 {
		fmt.Fprintf(w, "\n--- PREVIOUS EVALUATIONS ---\n\n")
		for _, e := range evals {
			fmt.Fprintf(w, "Round %d: %s", e.Round, e.Verdict)
			if mode == "direct" {
				fmt.Fprintf(w, " — (direct corrections)")
			} else if e.EvalReport != "" {
				fmt.Fprintf(w, " — %s", e.EvalReport)
			}
			fmt.Fprintln(w)
		}
	}
	if mode == "direct" {
		if directShowsReport {
			fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
			fmt.Fprintf(w, "Make corrections directly to the %s files.\n", directNoun)
		}
		return
	}
	fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
	fmt.Fprintf(w, "Write your evaluation report to:\n")
	fmt.Fprintf(w, "  %s\n", reportFile)
}

func printSpecifyingOutput(w io.Writer, s *ForgeState, dir string) {
	spec := s.Specifying
	var cs *ActiveSpec
	if len(spec.CurrentSpecs) > 0 {
		cs = spec.CurrentSpecs[0]
	}

	// domainPath returns "<domain>/" for output.
	domainPath := func(domain string) string {
		return domain + "/"
	}

	// batchEvalFile returns the eval file path for the current batch and round.
	batchEvalFile := func(domain string, batchNum, round int) string {
		return filepath.Join(domain, "specs", ".eval", fmt.Sprintf("batch-%d-r%d.md", batchNum, round))
	}

	switch s.State {
	case StateSelect:
		fmt.Fprintf(w, "State:   SELECT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(cs.Domain))
		fmt.Fprintf(w, "Batch:   %d specs\n", len(spec.CurrentSpecs))
		fmt.Fprintf(w, "Specs:\n")
		for i, bcs := range spec.CurrentSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, bcs.Name)
			fmt.Fprintf(w, "      File:    %s\n", bcs.File)
			fmt.Fprintf(w, "      Topic:   %s\n", bcs.Topic)
			if len(bcs.PlanningSources) > 0 {
				fmt.Fprintf(w, "      Sources: %s\n", strings.Join(bcs.PlanningSources, ", "))
			}
		}
		fmt.Fprintf(w, "Action:  Study each planning source.\n")
		fmt.Fprintf(w, "         Study each spec doc that exists.\n")
		if s.Config.General.UserGuided {
			fmt.Fprintf(w, "         STOP please review and discuss with user before continuing.\n")
		}
		fmt.Fprintf(w, "         After completion of the above, advance to begin drafting.\n")

	case StateDraft:
		fmt.Fprintf(w, "State:   DRAFT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(cs.Domain))
		fmt.Fprintf(w, "Batch:   %d specs\n", len(spec.CurrentSpecs))
		fmt.Fprintf(w, "Specs:\n")
		for i, bcs := range spec.CurrentSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, bcs.File)
			if len(bcs.PlanningSources) > 0 {
				fmt.Fprintf(w, "      Sources: %s\n", strings.Join(bcs.PlanningSources, ", "))
			}
		}
		fmt.Fprintf(w, "Action:  Draft all specs in the batch using the spec skill.\n")
		fmt.Fprintf(w, "         Format:    references/spec-format.md\n")
		fmt.Fprintf(w, "         Process:   references/spec-generation-skill.md\n")
		fmt.Fprintf(w, "         Scoping:   references/topic-of-concern.md\n")
		fmt.Fprintf(w, "         If a topic needs splitting or a missing spec is identified,\n")
		fmt.Fprintf(w, "         write the new spec file, then register it:\n")
		fmt.Fprintf(w, "           forgectl add-queue-item --name <name> --topic <topic> --file <file> [--source <path>...]\n")
		fmt.Fprintf(w, "         After completion of the above, advance to begin evaluation.\n")

	case StateEvaluate:
		fmt.Fprintf(w, "State:   EVALUATE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(cs.Domain))
		fmt.Fprintf(w, "Batch:   %d specs\n", len(spec.CurrentSpecs))
		fmt.Fprintf(w, "Round:   %d/%d\n", cs.Round, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Specs:\n")
		for i, bcs := range spec.CurrentSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, bcs.File)
		}
		specEvalType := s.Config.Specifying.Eval.Type
		writeEvalEntryAction(w, EvalModeFor(s.Config.Specifying.Eval, s.Config.General), evalEntryAction{
			label:        "Action:  ",
			indent:       "         ",
			spawnEval:    fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate the spec batch.", specEvalType),
			spawnCorrect: fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate and correct the spec.", specEvalType),
			runEval:      "The sub-agent should run: forgectl eval",
			stagedNote:   "Spec files have been staged. Sub-agent makes corrections directly.",
			reportTail:   "advance with --verdict PASS|FAIL --eval-report <path>",
		})

	case StateRefine:
		evalFile := batchEvalFile(cs.Domain, spec.BatchNumber, cs.Round)
		fmt.Fprintf(w, "State:   REFINE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(cs.Domain))
		fmt.Fprintf(w, "Batch:   %d specs\n", len(spec.CurrentSpecs))
		fmt.Fprintf(w, "Round:   %d/%d\n", cs.Round, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Specs:\n")
		for i, bcs := range spec.CurrentSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, bcs.File)
		}
		specRefineMode := EvalModeFor(s.Config.Specifying.Eval, s.Config.General)
		writeRefineBody(w, specRefineMode, evalFile, "Action:  ", "         ")
		if specRefineMode == "direct" {
			fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
		} else {
			fmt.Fprintf(w, "         Format:      references/spec-format.md\n")
			fmt.Fprintf(w, "         Process:     references/spec-generation-skill.md\n")
			fmt.Fprintf(w, "         Scoping:     references/topic-of-concern.md\n")
			fmt.Fprintf(w, "         After completion of the above, advance to continue evaluation.\n")
		}

	case StateAccept:
		fmt.Fprintf(w, "State:   ACCEPT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", spec.CurrentDomain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(spec.CurrentDomain))
		if cs != nil {
			fmt.Fprintf(w, "Batch:   %d specs accepted\n", len(spec.CurrentSpecs))
			fmt.Fprintf(w, "Round:   %d/%d\n", cs.Round, s.Config.Specifying.Eval.MaxRounds)
		}
		fmt.Fprintf(w, "Action:  Batch accepted.\n")
		fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")

	case StateCrossReference:
		currentDomain := spec.CurrentDomain
		cr := spec.CrossReference[currentDomain]
		fmt.Fprintf(w, "State:   CROSS_REFERENCE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", currentDomain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(currentDomain))
		fmt.Fprintf(w, "Round:   %d/%d\n", cr.Round, s.Config.Specifying.CrossReference.MaxRounds)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Specs in domain:\n")

		// Session-completed specs for this domain.
		var sessionSpecs []CompletedSpec
		for _, c := range spec.Completed {
			if c.Domain == currentDomain {
				sessionSpecs = append(sessionSpecs, c)
			}
		}
		if len(sessionSpecs) > 0 {
			fmt.Fprintf(w, "  [session — completed]\n")
			for _, c := range sessionSpecs {
				fmt.Fprintf(w, "    %s (batch %d)\n", filepath.Base(c.File), c.BatchNumber)
			}
		}

		// Existing specs not in session.
		existingSpecs := findExistingSpecs(dir, currentDomain, spec)
		if len(existingSpecs) > 0 {
			fmt.Fprintf(w, "  [existing — not in queue]\n")
			for _, f := range existingSpecs {
				fmt.Fprintf(w, "    %s\n", f)
			}
		}

		fmt.Fprintln(w)
		agentCount := s.Config.Specifying.CrossReference.Count
		agentType := s.Config.Specifying.CrossReference.Type
		fmt.Fprintf(w, "Action:  Please spawn %d %s sub-agent(s) to cross-reference ALL specs in this domain.\n", agentCount, agentType)
		fmt.Fprintf(w, "         Assign each sub-agent a subset of specs to review against the others.\n")
		fmt.Fprintf(w, "         Fix any findings.\n")
		fmt.Fprintf(w, "         After completion of the above, advance to begin evaluation.\n")

	case StateCrossReferenceEval:
		currentDomain := spec.CurrentDomain
		cr := spec.CrossReference[currentDomain]
		evalFile := filepath.Join(currentDomain, "specs", ".eval", fmt.Sprintf("cross-reference-r%d.md", cr.Round))
		evalAgentType := s.Config.Specifying.CrossReference.Eval.Type
		if evalAgentType == "" {
			evalAgentType = "opus"
		}
		fmt.Fprintf(w, "State:   CROSS_REFERENCE_EVAL\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", currentDomain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(currentDomain))
		fmt.Fprintf(w, "Round:   %d/%d\n", cr.Round, s.Config.Specifying.CrossReference.MaxRounds)
		fmt.Fprintf(w, "Eval:    %s\n", evalFile)
		fmt.Fprintln(w)
		writeEvalEntryAction(w, EvalModeFor(s.Config.Specifying.Eval, s.Config.General), evalEntryAction{
			label:        "Action:  ",
			indent:       "         ",
			spawnEval:    fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate cross-reference consistency.", evalAgentType),
			spawnCorrect: fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate and correct cross-references.", evalAgentType),
			runEval:      "The sub-agent should run: forgectl eval",
			stagedNote:   "Spec files have been staged. Sub-agent makes corrections directly.",
			reportTail:   "advance with --verdict PASS|FAIL --eval-report <path>",
		})

	case StateCrossReferenceReview:
		currentDomain := spec.CurrentDomain
		cr := spec.CrossReference[currentDomain]
		var lastEval EvalRecord
		if len(cr.Evals) > 0 {
			lastEval = cr.Evals[len(cr.Evals)-1]
		}
		evalFile := filepath.Join(currentDomain, "specs", ".eval", fmt.Sprintf("cross-reference-r%d.md", cr.Round))
		fmt.Fprintf(w, "State:   CROSS_REFERENCE_REVIEW\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", currentDomain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(currentDomain))
		fmt.Fprintf(w, "Round:   %d/%d\n", cr.Round, s.Config.Specifying.CrossReference.MaxRounds)
		fmt.Fprintf(w, "Verdict: %s\n", lastEval.Verdict)
		fmt.Fprintf(w, "Eval:    %s\n", evalFile)
		fmt.Fprintln(w)
		if s.Config.Specifying.CrossReference.UserReview {
			fmt.Fprintf(w, "Action:  STOP please review and discuss with user before continuing.\n")
		} else {
			fmt.Fprintf(w, "Action:  Domain cross-reference complete.\n")
		}
		fmt.Fprintf(w, "         If additional specs are needed for this domain,\n")
		fmt.Fprintf(w, "         write the new spec file, then register it:\n")
		fmt.Fprintf(w, "           forgectl add-queue-item --name <name> --topic <topic> --file <file> [--source <path>...]\n")
		fmt.Fprintf(w, "         Set code search roots for this domain (used in planning phase):\n")
		fmt.Fprintf(w, "           forgectl set-roots <path> [<path>...]\n")
		fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")

	case StateDone:
		fmt.Fprintf(w, "State:   DONE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Specs:   %d completed\n", len(spec.Completed))
		fmt.Fprintf(w, "Action:  All individual specs complete.\n")
		fmt.Fprintf(w, "         If additional specs are needed,\n")
		fmt.Fprintf(w, "         write the new spec file, then register it:\n")
		fmt.Fprintf(w, "           forgectl add-queue-item --name <name> --domain <domain> --topic <topic> --file <file> [--source <path>...]\n")
		fmt.Fprintf(w, "           Adding specs here re-enters ORIENT for the new items before reconciliation.\n")
		fmt.Fprintf(w, "         Set code search roots for any domain not yet configured (used in planning phase):\n")
		fmt.Fprintf(w, "           forgectl set-roots --domain <domain> <path> [<path>...]\n")
		fmt.Fprintf(w, "         When ready, advance to begin reconciliation.\n")

	case StateReconcile:
		domains := uniqueDomains(spec.Completed)
		fmt.Fprintf(w, "State:   RECONCILE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Specs:   %d completed across %d domains\n", len(spec.Completed), len(domains))
		fmt.Fprintf(w, "Action:  Cross-validate all specs across domains: verify Depends On entries,\n")
		fmt.Fprintf(w, "         Integration Points symmetry, naming consistency. Stage changes with git add.\n")
		fmt.Fprintf(w, "         After completion of the above, advance to begin evaluation.\n")

	case StateReconcileEval:
		domains := uniqueDomains(spec.Completed)
		maxRounds := s.Config.Specifying.Reconciliation.MaxRounds
		fmt.Fprintf(w, "State:   RECONCILE_EVAL\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Round:   %d/%d\n", spec.Reconcile.Round, maxRounds)
		fmt.Fprintf(w, "Specs:   %d completed across %d domains\n", len(spec.Completed), len(domains))
		writeEvalEntryAction(w, EvalModeFor(s.Config.Specifying.Eval, s.Config.General), evalEntryAction{
			label:        "Action:  ",
			indent:       "         ",
			spawnEval:    "Please spawn 1 opus sub-agent to evaluate cross-domain reconciliation.",
			spawnCorrect: "Please spawn 1 opus sub-agent to evaluate and correct the reconciliation.",
			runEval:      "The sub-agent should run: forgectl eval",
			stagedNote:   "Spec files have been staged. Sub-agent makes corrections directly.",
			reportTail:   "advance with --verdict PASS|FAIL --eval-report <path>",
		})

	case StateReconcileReview:
		domains := uniqueDomains(spec.Completed)
		maxRounds := s.Config.Specifying.Reconciliation.MaxRounds
		var lastVerdict string
		if len(spec.Reconcile.Evals) > 0 {
			lastVerdict = spec.Reconcile.Evals[len(spec.Reconcile.Evals)-1].Verdict
		}
		fmt.Fprintf(w, "State:   RECONCILE_REVIEW\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Specs:   %d completed across %d domains\n", len(spec.Completed), len(domains))
		fmt.Fprintf(w, "Round:   %d/%d\n", spec.Reconcile.Round, maxRounds)
		fmt.Fprintf(w, "Verdict: %s\n", lastVerdict)
		fmt.Fprintln(w)
		if s.Config.Specifying.Reconciliation.UserReview {
			fmt.Fprintf(w, "Action:  STOP please review and discuss with user before continuing.\n")
		} else {
			fmt.Fprintf(w, "Action:  Reconciliation review complete.\n")
		}
		fmt.Fprintf(w, "         If additional specs are needed,\n")
		fmt.Fprintf(w, "         write the new spec file, then register it:\n")
		fmt.Fprintf(w, "           forgectl add-queue-item --name <name> --domain <domain> --topic <topic> --file <file> [--source <path>...]\n")
		fmt.Fprintf(w, "           Adding specs here re-enters DONE for the new items before reconciliation restarts.\n")
		fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")

	case StateComplete:
		fmt.Fprintf(w, "State:   COMPLETE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Specs:   %d completed, reconciled\n", len(spec.Completed))
		fmt.Fprintf(w, "Action:  Specifying phase complete. Advance to continue.\n")
	}
}

// findExistingSpecs returns spec file basenames in <dir>/<domain>/specs/ that
// are not already tracked in the session (not in completed, queue, or currentSpecs).
func findExistingSpecs(dir, domain string, spec *SpecifyingState) []string {
	specsDir := filepath.Join(dir, domain, "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil
	}

	// Build set of tracked files (by basename).
	tracked := make(map[string]bool)
	for _, c := range spec.Completed {
		tracked[filepath.Base(c.File)] = true
	}
	for _, q := range spec.Queue {
		tracked[filepath.Base(q.File)] = true
	}
	for _, cs := range spec.CurrentSpecs {
		tracked[filepath.Base(cs.File)] = true
	}

	var existing []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		if !tracked[e.Name()] {
			existing = append(existing, e.Name())
		}
	}
	return existing
}

// uniqueDomains returns the set of unique domain names from completed specs.
func uniqueDomains(completed []CompletedSpec) []string {
	seen := make(map[string]bool)
	var domains []string
	for _, c := range completed {
		if !seen[c.Domain] {
			seen[c.Domain] = true
			domains = append(domains, c.Domain)
		}
	}
	return domains
}

// --- Generate Planning Queue ---

func printGeneratePlanningQueueOutput(w io.Writer, s *ForgeState) {
	switch s.State {
	case StateOrient:
		planQueueFile := ""
		if s.GeneratePlanningQueue != nil {
			planQueueFile = s.GeneratePlanningQueue.PlanQueueFile
		}
		fmt.Fprintf(w, "State:   ORIENT\n")
		fmt.Fprintf(w, "Phase:   generate_planning_queue\n")
		fmt.Fprintf(w, "\nGenerated: %s\n", planQueueFile)
		fmt.Fprintf(w, "\nAdvance to continue.\n")

	case StateRefine:
		planQueueFile := ""
		if s.GeneratePlanningQueue != nil {
			planQueueFile = s.GeneratePlanningQueue.PlanQueueFile
		}
		fmt.Fprintf(w, "State:   REFINE\n")
		fmt.Fprintf(w, "Phase:   generate_planning_queue\n")
		fmt.Fprintf(w, "\nStop and review the generated plan queue %s. Reorder and edit as needed.\n", planQueueFile)
		fmt.Fprintf(w, "\nAdvance when ready.\n")
	}
}

// --- Planning ---

func printPlanningOutput(w io.Writer, s *ForgeState, dir string) {
	plan := s.Planning
	cp := plan.CurrentPlan

	switch s.State {
	case StateOrient:
		fmt.Fprintf(w, "State:   ORIENT\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Action:  Advance to begin studying specs.\n")

	case StateStudySpecs:
		fmt.Fprintf(w, "State:   STUDY_SPECS\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Specs:   %s\n", strings.Join(cp.Specs, ", "))
		fmt.Fprintf(w, "Roots:   %s\n", strings.Join(cp.CodeSearchRoots, ", "))
		fmt.Fprintf(w, "Action:  Study the specs: %s\n", strings.Join(cp.Specs, ", "))
		fmt.Fprintf(w, "         Review git diffs for spec commits. Advance when done.\n")

	case StateStudyCode:
		fmt.Fprintf(w, "State:   STUDY_CODE\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Roots:   %s\n", strings.Join(cp.CodeSearchRoots, ", "))
		for i, spec := range cp.Specs {
			if i == 0 {
				fmt.Fprintf(w, "Specs:   %s\n", spec)
			} else {
				fmt.Fprintf(w, "         %s\n", spec)
			}
		}
		fmt.Fprintf(w, "Action:  Explore the codebase in relation to the specs under study.\n")
		fmt.Fprintf(w, "         Sub-agents: 3. Search roots: %s.\n", strings.Join(cp.CodeSearchRoots, ", "))
		fmt.Fprintf(w, "         Focus: find code relevant to the specs listed above.\n")
		fmt.Fprintf(w, "         Advance when done.\n")

	case StateStudyPackages:
		fmt.Fprintf(w, "State:   STUDY_PACKAGES\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Action:  Study the project's technical stack: package manifests, library docs, CLAUDE.md references.\n")
		fmt.Fprintf(w, "         Advance when done.\n")

	case StateReview:
		fmt.Fprintf(w, "State:   REVIEW\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Action:  Review study findings before drafting.\n")
		fmt.Fprintf(w, "         Plan format: PLAN_FORMAT.md\n")
		if s.Config.General.UserGuided {
			fmt.Fprintf(w, "         Stop and review and discuss with user before continuing.\n")
		}
		fmt.Fprintf(w, "         Advance to begin drafting.\n")

	case StateDraft:
		planDir := filepath.Dir(cp.File)
		fmt.Fprintf(w, "State:   DRAFT\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Action:  Draft the implementation plan.\n")
		fmt.Fprintf(w, "         Output: plan.json + notes/ at %s\n", planDir)
		fmt.Fprintf(w, "         Format: PLAN_FORMAT.md\n")
		fmt.Fprintf(w, "         Advance when plan and notes are ready.\n")

	case StateValidate:
		fmt.Fprintf(w, "State:   VALIDATE\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Action:  Plan validation failed. Fix the plan and advance to re-validate.\n")
		fmt.Fprintf(w, "         Format: PLAN_FORMAT.md\n")

	case StateSelfReview:
		notesDir := filepath.Join(filepath.Dir(cp.File), "notes") + "/"
		fmt.Fprintf(w, "State:   SELF_REVIEW\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		for i, spec := range cp.Specs {
			if i == 0 {
				fmt.Fprintf(w, "Specs:   %s\n", spec)
			} else {
				fmt.Fprintf(w, "         %s\n", spec)
			}
		}
		fmt.Fprintf(w, "Notes:   %s\n", notesDir)
		fmt.Fprintf(w, "Action:  Review your plan against the specs and your study notes.\n")
		fmt.Fprintf(w, "         Verify coverage, dependency ordering, and layer structure.\n")
		fmt.Fprintf(w, "         Revise plan.json and notes as needed before evaluation.\n")
		fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")

	case StateEvaluate:
		fmt.Fprintf(w, "State:   EVALUATE\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		planEvalType := s.Config.Planning.Eval.Type
		writeEvalEntryAction(w, EvalModeFor(s.Config.Planning.Eval, s.Config.General), evalEntryAction{
			label:        "Action:  ",
			indent:       "         ",
			spawnEval:    fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate the plan.", planEvalType),
			spawnCorrect: fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate and correct the plan.", planEvalType),
			runEval:      "Sub-agent runs: forgectl eval",
			stagedNote:   "Plan files have been staged. Sub-agent makes corrections directly.",
			reportTail:   "advance with --verdict PASS|FAIL --eval-report <path>",
		})

	case StateRefine:
		evalDir := filepath.Join(filepath.Dir(cp.File), "evals")
		evalFile := filepath.Join(evalDir, fmt.Sprintf("round-%d.md", plan.Round))

		lastEval := plan.Evals[len(plan.Evals)-1]
		fmt.Fprintf(w, "State:   REFINE\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		planRefineMode := EvalModeFor(s.Config.Planning.Eval, s.Config.General)
		// PASS below min_rounds prints a preamble line; FAIL does not.
		firstLabel := "Action:  "
		if lastEval.Verdict != "FAIL" {
			fmt.Fprintf(w, "Action:  Minimum evaluation rounds not met.\n")
			firstLabel = "         "
		}
		writeRefineBody(w, planRefineMode, evalFile, firstLabel, "         ")
		fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")

	case StateAccept:
		fmt.Fprintf(w, "State:   ACCEPT\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		lastEval := plan.Evals[len(plan.Evals)-1]
		maxReached := lastEval.Verdict == "FAIL" && plan.Round >= s.Config.Planning.Eval.MaxRounds
		if s.Config.General.EnableCommits {
			if maxReached {
				fmt.Fprintf(w, "Action:  Plan accepted (max rounds reached). Advance with --message \"your commit message\" to commit and continue.\n")
			} else {
				fmt.Fprintf(w, "Action:  Plan accepted. Advance with --message \"your commit message\" to commit and continue.\n")
			}
		} else {
			if maxReached {
				fmt.Fprintf(w, "Action:  Plan accepted (max rounds reached). Advance to continue.\n")
			} else {
				fmt.Fprintf(w, "Action:  Plan accepted. Advance to continue.\n")
			}
		}
	}
}

// --- Implementing ---

func printImplementingOutput(w io.Writer, s *ForgeState, dir string) {
	impl := s.Implementing

	switch s.State {
	case StateOrient:
		plan, err := loadPlan(s, dir)
		if err != nil {
			fmt.Fprintf(w, "State:   ORIENT\n")
			fmt.Fprintf(w, "Phase:   implementing\n")
			fmt.Fprintf(w, "Error:   %s\n", err)
			return
		}

		if impl.CurrentLayer == nil {
			// Initial orient — show init summary.
			fmt.Fprintf(w, "State:   ORIENT\n")
			fmt.Fprintf(w, "Phase:   implementing\n")
			fmt.Fprintf(w, "Plan:    %s\n", s.Planning.CurrentPlan.Name)
			fmt.Fprintf(w, "Domain:  %s\n", s.Planning.CurrentPlan.Domain)
			fmt.Fprintf(w, "File:    %s\n", s.Planning.CurrentPlan.File)
			fmt.Fprintf(w, "Config:  batch=%d, rounds=%d-%d\n", s.Config.Implementing.Batch, s.Config.Implementing.Eval.MinRounds, s.Config.Implementing.Eval.MaxRounds)
			fmt.Fprintf(w, "\nInitialized plan.json for implementation:\n")
			fmt.Fprintf(w, "  Items:  %d (passes: pending, rounds: 0)\n", len(plan.Items))
			fmt.Fprintf(w, "  Layers: %d", len(plan.Layers))
			for i, l := range plan.Layers {
				count := len(l.Items)
				if i == 0 {
					fmt.Fprintf(w, " (%s %s: %d items", l.ID, l.Name, count)
				} else {
					fmt.Fprintf(w, ", %s %s: %d items", l.ID, l.Name, count)
				}
			}
			fmt.Fprintf(w, ")\n")
		} else {
			fmt.Fprintf(w, "State:    ORIENT\n")
			fmt.Fprintf(w, "Phase:    implementing\n")
			fmt.Fprintf(w, "Layer:    %s %s\n", impl.CurrentLayer.ID, impl.CurrentLayer.Name)

			// Check for force-accepted (failed) items in current layer.
			layer := findLayer(plan, impl.CurrentLayer.ID)
			if layer != nil {
				var failedItems []*PlanItem
				for _, id := range layer.Items {
					item := findItem(plan, id)
					if item != nil && item.Passes == "failed" {
						failedItems = append(failedItems, item)
					}
				}
				if len(failedItems) > 0 {
					fmt.Fprintf(w, "          FORCE ACCEPT: %d items marked failed (max rounds %d/%d reached)\n", len(failedItems), s.Config.Implementing.Eval.MaxRounds, s.Config.Implementing.Eval.MaxRounds)
					for _, item := range failedItems {
						fmt.Fprintf(w, "          - [%s] %s\n", item.ID, item.Name)
					}
				}

				// Count progress.
				terminal := 0
				passed := 0
				failed := 0
				total := len(layer.Items)
				for _, id := range layer.Items {
					item := findItem(plan, id)
					if item != nil {
						if item.Passes == "passed" {
							terminal++
							passed++
						} else if item.Passes == "failed" {
							terminal++
							failed++
						}
					}
				}
				if failed > 0 {
					fmt.Fprintf(w, "Progress: %d/%d items terminal (%d passed, %d failed)", terminal, total, passed, failed)
				} else {
					fmt.Fprintf(w, "Progress: %d/%d items passed", terminal, total)
				}
				layerComplete := terminal == total
				if layerComplete {
					// Check if this is the final layer.
					isFinalLayer := false
					for i, l := range plan.Layers {
						if l.ID == impl.CurrentLayer.ID && i == len(plan.Layers)-1 {
							isFinalLayer = true
							break
						}
					}
					if isFinalLayer {
						fmt.Fprintf(w, " — layer complete (final layer)")
					} else {
						fmt.Fprintf(w, " — layer complete")
					}
				}
				fmt.Fprintln(w)
			}
		}

		if impl.CurrentLayer == nil {
			// Initial orient uses narrower alignment (matching State:   ).
			if s.Config.General.UserGuided {
				fmt.Fprintf(w, "Action:  Stop and review and discuss with user before continuing.\n")
				fmt.Fprintf(w, "         Selecting first batch. Run: forgectl advance\n")
			} else {
				fmt.Fprintf(w, "Action:  Selecting first batch. Run: forgectl advance\n")
			}
		} else {
			// Non-initial orient uses wider alignment (matching State:    ).
			layerDef := findLayer(plan, impl.CurrentLayer.ID)
			layerComplete := layerDef != nil && allLayerItemsTerminal(plan, *layerDef)

			// Determine Next: line.
			if layerComplete {
				// Find the next layer.
				nextLayer := (*PlanLayerDef)(nil)
				for i, l := range plan.Layers {
					if l.ID == impl.CurrentLayer.ID && i+1 < len(plan.Layers) {
						nextLayer = &plan.Layers[i+1]
						break
					}
				}
				if nextLayer != nil {
					var ids []string
					for _, id := range nextLayer.Items {
						ids = append(ids, fmt.Sprintf("[%s]", id))
					}
					fmt.Fprintf(w, "Next:     %s %s — %d items: %s\n", nextLayer.ID, nextLayer.Name, len(nextLayer.Items), strings.Join(ids, ", "))
				}
			} else if layerDef != nil {
				// Count pending items in current layer for next batch.
				pending := 0
				for _, id := range layerDef.Items {
					item := findItem(plan, id)
					if item != nil && item.Passes == "pending" {
						pending++
					}
				}
				nextBatchSize := pending
				if nextBatchSize > s.Config.Implementing.Batch {
					nextBatchSize = s.Config.Implementing.Batch
				}
				if nextBatchSize > 0 {
					fmt.Fprintf(w, "Next:     %d unblocked items in next batch\n", nextBatchSize)
				}
			}

			// Determine action text.
			var actionContinue string
			if layerComplete {
				nextExists := false
				for i, l := range plan.Layers {
					if l.ID == impl.CurrentLayer.ID && i+1 < len(plan.Layers) {
						nextExists = true
						break
					}
				}
				if nextExists {
					actionContinue = "advance to next layer."
				} else {
					actionContinue = "advance to continue."
				}
			} else {
				actionContinue = "advance to select next batch."
			}

			if s.Config.General.UserGuided {
				fmt.Fprintf(w, "Action:   STOP please review and discuss with user before continuing.\n")
				fmt.Fprintf(w, "          After completion of the above, %s\n", actionContinue)
			} else {
				fmt.Fprintf(w, "Action:   After completion of the above, %s\n", actionContinue)
			}
		}

	case StateImplement:
		batch := impl.CurrentBatch
		itemID := batch.Items[batch.CurrentItemIndex]

		plan, err := loadPlan(s, dir)
		if err != nil {
			fmt.Fprintf(w, "Error: %s\n", err)
			return
		}

		item := findItem(plan, itemID)
		if item == nil {
			fmt.Fprintf(w, "Error: item %q not found in plan\n", itemID)
			return
		}

		fmt.Fprintf(w, "State:   IMPLEMENT\n")
		fmt.Fprintf(w, "Phase:   implementing\n")
		fmt.Fprintf(w, "Layer:   %s %s\n", impl.CurrentLayer.ID, impl.CurrentLayer.Name)
		fmt.Fprintf(w, "Batch:   %d/%d\n", impl.BatchNumber, countTotalBatches(plan, s.Config.Implementing.Batch))

		if batch.EvalRound > 0 {
			fmt.Fprintf(w, "Round:   %d/%d\n", batch.EvalRound, s.Config.Implementing.Eval.MaxRounds)
			if len(batch.Evals) > 0 {
				lastEval := batch.Evals[len(batch.Evals)-1]
				evalDir := filepath.Join(currentPlanDir(s), "evals")
				evalFile := filepath.Join(evalDir, fmt.Sprintf("batch-%d-round-%d.md", impl.BatchNumber, lastEval.Round))
				fmt.Fprintf(w, "Eval:    %s\n", evalFile)
				note := fmt.Sprintf("%s recorded for round %d.", lastEval.Verdict, lastEval.Round)
				if lastEval.Verdict == "PASS" && batch.EvalRound < s.Config.Implementing.Eval.MinRounds {
					note += fmt.Sprintf(" Minimum rounds not yet met (%d/%d).", batch.EvalRound, s.Config.Implementing.Eval.MinRounds)
				}
				fmt.Fprintf(w, "Note:    %s\n", note)
			}
		}

		fmt.Fprintf(w, "Item:    [%s] %s\n", item.ID, item.Name)
		fmt.Fprintf(w, "         %s\n", item.Description)
		fmt.Fprintf(w, "         (%d of %d in batch)\n", batch.CurrentItemIndex+1, len(batch.Items))

		if len(item.Steps) > 0 {
			fmt.Fprintf(w, "Steps:\n")
			for i, step := range item.Steps {
				fmt.Fprintf(w, "  %d. %s\n", i+1, step)
			}
		}
		if len(item.Files) > 0 {
			fmt.Fprintf(w, "Files:   %s\n", strings.Join(item.Files, ", "))
		}
		specCommits := s.Planning.CurrentPlan.SpecCommits
		if len(item.Specs) > 0 {
			for i, spec := range item.Specs {
				if i == 0 {
					fmt.Fprintf(w, "Specs:   %s\n", spec)
				} else {
					fmt.Fprintf(w, "         %s\n", spec)
				}
				// Each spec entry carries a copy-pasteable git command bounded to
				// the plan's spec_commits. git show (not git log -p) keeps the
				// output to exactly the named commits; the '**/<file>' pathspec
				// self-filters that list to the commits that touched the spec.
				// Spec entries are display-only names with an optional #anchor, so
				// strip the anchor and glob the basename rather than treating it
				// as a validated on-disk path. Omitted when spec_commits is empty.
				if len(specCommits) > 0 {
					file, _, _ := strings.Cut(spec, "#")
					fmt.Fprintf(w, "         Read: git show %s -- '**/%s'\n", strings.Join(specCommits, " "), file)
				}
			}
		}
		if len(item.Refs) > 0 {
			for i, ref := range item.Refs {
				if i == 0 {
					fmt.Fprintf(w, "Refs:    %s\n", ref)
				} else {
					fmt.Fprintf(w, "         %s\n", ref)
				}
			}
		}

		// Test summary.
		testCounts := map[string]int{}
		for _, t := range item.Tests {
			testCounts[t.Category]++
		}
		var testParts []string
		for _, cat := range []string{"functional", "rejection", "edge_case"} {
			if c, ok := testCounts[cat]; ok {
				testParts = append(testParts, fmt.Sprintf("%d %s", c, cat))
			}
		}
		if len(testParts) > 0 {
			fmt.Fprintf(w, "Tests:   %s\n", strings.Join(testParts, ", "))
		}

		if batch.EvalRound > 0 {
			planDir := currentPlanDir(s)
			evalDir := filepath.Join(planDir, "evals")
			lastEval := batch.Evals[len(batch.Evals)-1]
			evalFile := filepath.Join(evalDir, fmt.Sprintf("batch-%d-round-%d.md", impl.BatchNumber, lastEval.Round))
			implRefineMode := EvalModeFor(s.Config.Implementing.Eval, s.Config.General)
			writeRefineBody(w, implRefineMode, evalFile, "Action:  ", "         ")
			writeImplementReviewReminders(w, "         ", len(specCommits) > 0, len(item.Refs) > 0)
			fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
		} else {
			fmt.Fprintf(w, "Action:  Implement this item.\n")
			writeImplementReviewReminders(w, "         ", len(specCommits) > 0, len(item.Refs) > 0)
			fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
		}

	case StateEvaluate:
		batch := impl.CurrentBatch
		plan, _ := loadPlan(s, dir)
		totalBatches := 0
		if plan != nil {
			totalBatches = countTotalBatches(plan, s.Config.Implementing.Batch)
		}
		fmt.Fprintf(w, "State:    EVALUATE\n")
		fmt.Fprintf(w, "Phase:    implementing\n")
		fmt.Fprintf(w, "Layer:    %s %s\n", impl.CurrentLayer.ID, impl.CurrentLayer.Name)
		fmt.Fprintf(w, "Batch:    %d/%d\n", impl.BatchNumber, totalBatches)
		fmt.Fprintf(w, "Round:    %d/%d\n", batch.EvalRound+1, s.Config.Implementing.Eval.MaxRounds)
		fmt.Fprintf(w, "Items:\n")

		if plan != nil {
			for _, id := range batch.Items {
				item := findItem(plan, id)
				if item != nil {
					fmt.Fprintf(w, "  - [%s] %s\n", item.ID, item.Name)
				}
			}
		}

		implEvalType := s.Config.Implementing.Eval.Type
		writeEvalEntryAction(w, EvalModeFor(s.Config.Implementing.Eval, s.Config.General), evalEntryAction{
			label:        "Action:   ",
			indent:       "          ",
			spawnEval:    fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate the implementation batch.", implEvalType),
			spawnCorrect: fmt.Sprintf("Please spawn 1 %s sub-agent to evaluate and correct the batch.", implEvalType),
			runEval:      "The sub-agent should run: forgectl eval",
			stagedNote:   "Batch files have been staged. Sub-agent makes corrections directly.",
			reportTail:   "advance with --eval-report <path> --verdict PASS|FAIL",
		})

	case StateCommit:
		batch := impl.CurrentBatch
		plan, _ := loadPlan(s, dir)
		totalBatches := 0
		if plan != nil {
			totalBatches = countTotalBatches(plan, s.Config.Implementing.Batch)
		}
		fmt.Fprintf(w, "State:   COMMIT\n")
		fmt.Fprintf(w, "Phase:   implementing\n")
		fmt.Fprintf(w, "Layer:   %s %s\n", impl.CurrentLayer.ID, impl.CurrentLayer.Name)
		fmt.Fprintf(w, "Batch:   %d/%d\n", impl.BatchNumber, totalBatches)
		fmt.Fprintf(w, "Items:\n")

		if plan != nil && batch != nil {
			for _, id := range batch.Items {
				item := findItem(plan, id)
				if item != nil {
					status := item.Passes
					if item.Passes == "failed" {
						status = fmt.Sprintf("failed (force-accept, %d/%d rounds)", item.Rounds, s.Config.Implementing.Eval.MaxRounds)
					}
					fmt.Fprintf(w, "  - [%s] %s\n", item.ID, status)
				}
			}
		}

		if s.Config.General.EnableCommits {
			fmt.Fprintf(w, "Action:  Advance with --message \"your commit message\" to commit and continue.\n")
		} else {
			fmt.Fprintf(w, "Action:  Commit your changes before continuing.\n")
			fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
		}

	case StateDone:
		plan, _ := loadPlan(s, dir)
		fmt.Fprintf(w, "State:   DONE\n")
		fmt.Fprintf(w, "Phase:   implementing\n")

		// Check if more domains remain.
		moreDomains := (s.Planning != nil && len(s.Planning.Queue) > 0) ||
			(s.Implementing != nil && len(s.Implementing.PlanQueue) > 0)

		fmt.Fprintf(w, "Summary:\n")
		if plan != nil {
			totalItems := 0
			totalPassed := 0
			totalRounds := 0
			totalBatches := 0

			for _, layer := range plan.Layers {
				passed := 0
				total := len(layer.Items)
				for _, id := range layer.Items {
					item := findItem(plan, id)
					if item != nil && item.Passes == "passed" {
						passed++
					}
				}
				fmt.Fprintf(w, "  %s %s:  %d/%d passed\n", layer.ID, layer.Name, passed, total)
				totalItems += total
				totalPassed += passed
			}

			for _, lh := range impl.LayerHistory {
				for _, bh := range lh.Batches {
					totalBatches++
					totalRounds += bh.EvalRounds
				}
			}

			fmt.Fprintf(w, "  Total:          %d/%d items passed\n", totalPassed, totalItems)
			fmt.Fprintf(w, "  Eval rounds:    %d across %d batches\n", totalRounds, totalBatches)
		}

		if moreDomains {
			fmt.Fprintf(w, "Action:  Domain complete. Advance to continue to next domain.\n")
		} else {
			fmt.Fprintf(w, "Action:  All items complete. Session done.\n")
		}
	}
}

// --- Phase Shift ---

func printPhaseShiftOutput(w io.Writer, s *ForgeState) {
	ps := s.PhaseShift

	fmt.Fprintf(w, "State:   PHASE_SHIFT\n")
	fmt.Fprintf(w, "From:    %s → %s\n", ps.From, ps.To)

	if ps.From == PhaseSpecifying && ps.To == PhasePlanning {
		if s.Specifying != nil {
			// Collect domain order and per-domain roots.
			var domainOrder []string
			domainSeen := make(map[string]bool)
			for _, cs := range s.Specifying.Completed {
				if !domainSeen[cs.Domain] {
					domainSeen[cs.Domain] = true
					domainOrder = append(domainOrder, cs.Domain)
				}
			}
			fmt.Fprintf(w, "\nDomains:  %d (%s)\n", len(domainOrder), strings.Join(domainOrder, ", "))
			fmt.Fprintf(w, "Specs:    %d completed\n", len(s.Specifying.Completed))
			for i, domain := range domainOrder {
				var roots []string
				isDefault := false
				if meta, ok := s.Specifying.Domains[domain]; ok && len(meta.CodeSearchRoots) > 0 {
					roots = meta.CodeSearchRoots
				} else {
					roots = []string{domain + "/"}
					isDefault = true
				}
				rootStr := strings.Join(roots, ", ")
				if isDefault {
					rootStr += " (default)"
				}
				if i == 0 {
					fmt.Fprintf(w, "Roots:    %s → %s\n", domain, rootStr)
				} else {
					fmt.Fprintf(w, "          %s → %s\n", domain, rootStr)
				}
			}
		}
		fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
		fmt.Fprintf(w, "When ready, run:\n")
		fmt.Fprintf(w, "  forgectl advance                          # auto-generate plan queue from completed specs\n")
		fmt.Fprintf(w, "  forgectl advance --from <plan-queue.json> # OR provide a custom plan queue\n")
	} else if ps.From == PhasePlanning && ps.To == PhaseImplementing {
		if s.Planning != nil && s.Planning.CurrentPlan != nil {
			fmt.Fprintf(w, "Plan:    %s\n", s.Planning.CurrentPlan.Name)
			fmt.Fprintf(w, "Domain:  %s\n", s.Planning.CurrentPlan.Domain)
			fmt.Fprintf(w, "File:    %s\n", s.Planning.CurrentPlan.File)
		}
		fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
		fmt.Fprintf(w, "When ready, run: forgectl advance\n")
	} else if ps.From == PhaseSpecifying && ps.To == PhaseGeneratePlanningQueue {
		fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
		fmt.Fprintf(w, "When ready:\n")
		fmt.Fprintf(w, "  forgectl advance                            # generate plan queue from completed specs\n")
		fmt.Fprintf(w, "  forgectl advance --from <plan-queue.json>   # OR provide a plan queue (skips generation)\n")
	} else if ps.From == PhaseGeneratePlanningQueue && ps.To == PhasePlanning {
		fmt.Fprintf(w, "\nAdvance to continue.\n")
	} else {
		fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
		fmt.Fprintf(w, "When ready, run: forgectl advance\n")
	}
}

// --- Reverse Engineering ---

func printReverseEngineeringOutput(w io.Writer, s *ForgeState, dir string) {
	re := s.ReverseEngineering
	if re == nil {
		return
	}
	cfg := s.Config.ReverseEngineering

	n := re.DomainCount
	domain := ""
	if re.DomainIndex >= 1 && re.DomainIndex <= len(re.Domains) {
		domain = re.Domains[re.DomainIndex-1]
	}

	// spawnLine renders the configured sub-agent spawn instruction for a block.
	spawn := func(ac AgentConfig) string {
		return fmt.Sprintf("Spawn %d %s %s sub-agents", ac.Count, ac.Model, ac.Type)
	}
	// topicRules emits the shared topic-of-concern formatting rules.
	topicRules := func() {
		fmt.Fprintf(w, "    - Must be a single topic that fits in one sentence\n")
		fmt.Fprintf(w, "    - Must not contain \"and\" conjoining unrelated capabilities\n")
		fmt.Fprintf(w, "    - Must describe an activity, not a vague statement\n")
		fmt.Fprintf(w, "    - Valid:   \"The optimizer validates repository URLs before cloning\"\n")
		fmt.Fprintf(w, "    - Invalid: \"The optimizer handles repos, validation, and caching\"\n")
	}

	fmt.Fprintf(w, "Phase: reverse_engineering\n")

	switch s.State {
	case StateOrient:
		fmt.Fprintf(w, "State: ORIENT\n")
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		parts := make([]string, len(re.Domains))
		for i, d := range re.Domains {
			parts[i] = fmt.Sprintf("%s (%d/%d)", d, i+1, n)
		}
		fmt.Fprintf(w, "Domains: %s\n", strings.Join(parts, ", "))
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  Prepare for reverse engineering across %d domains.\n", n)
		fmt.Fprintf(w, "  Domain order: %s\n", strings.Join(re.Domains, " → "))
		fmt.Fprintf(w, "\n  Requirements before advancing:\n")
		fmt.Fprintf(w, "  - Confirm you are familiar with the work concept scope\n")
		fmt.Fprintf(w, "  - Confirm domain ordering is correct\n")
		fmt.Fprintf(w, "    (SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE runs per domain in this order)\n")
		if len(re.Domains) > 0 {
			fmt.Fprintf(w, "\n  Advance to begin SURVEY on domain: %s\n", re.Domains[0])
		}

	case StateSurvey:
		fmt.Fprintf(w, "State: SURVEY\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  Survey existing specifications in %s/specs/.\n", domain)
		// Note the absence of a specs/ directory: the user proceeds with an
		// empty spec inventory for this domain (GAP_ANALYSIS treats all
		// behavior as unspecified).
		if domain != "" {
			if info, err := os.Stat(filepath.Join(dir, domain, "specs")); err != nil || !info.IsDir() {
				fmt.Fprintf(w, "\n  NOTE: %s/specs/ does not exist. Proceed with an empty spec\n", domain)
				fmt.Fprintf(w, "  inventory for this domain — all behavior is unspecified.\n")
			}
		}
		fmt.Fprintf(w, "\n  %s\n", spawn(cfg.Survey))
		fmt.Fprintf(w, "  scoped to %s/specs/.\n", domain)
		fmt.Fprintf(w, "\n  Read all spec files in the directory to understand what is specified.\n")
		fmt.Fprintf(w, "  Identify which specs pertain to the concept.\n")
		fmt.Fprintf(w, "\n  For each spec, extract:\n")
		fmt.Fprintf(w, "    - Spec file name\n")
		fmt.Fprintf(w, "    - Topic of concern\n")
		fmt.Fprintf(w, "    - Behaviors defined\n")
		fmt.Fprintf(w, "    - Integration points\n")
		fmt.Fprintf(w, "    - Dependencies\n")
		fmt.Fprintf(w, "    - Relevance: whether this spec pertains to the concept\n")
		fmt.Fprintf(w, "\n  Disregard specs that do not pertain to the concept.\n")
		fmt.Fprintf(w, "\n  Advance when complete.\n")

	case StateGapAnalysis:
		fmt.Fprintf(w, "State: GAP_ANALYSIS\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  Identify unspecified behavior in the %s source code\n", domain)
		fmt.Fprintf(w, "  that pertains to the concept.\n")
		fmt.Fprintf(w, "\n  %s scoped to the %s source code.\n", spawn(cfg.GapAnalysis), domain)
		fmt.Fprintf(w, "\n  For each behavior found in code that is not covered by an existing spec:\n")
		fmt.Fprintf(w, "    - Describe what the behavior does\n")
		fmt.Fprintf(w, "    - Identify a topic of concern for it:\n")
		topicRules()
		fmt.Fprintf(w, "    - Note where in the code it is implemented\n")
		fmt.Fprintf(w, "    - Note if an existing spec partially covers it (and what the gap is)\n")
		fmt.Fprintf(w, "\n  Advance when complete.\n")
		fmt.Fprintf(w, "  Next: DECOMPOSE for domain %s\n", domain)

	case StateDecompose:
		fmt.Fprintf(w, "State: DECOMPOSE\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  Synthesize findings from domain %s.\n", domain)
		fmt.Fprintf(w, "\n  From the SURVEY and GAP_ANALYSIS results for this domain,\n")
		fmt.Fprintf(w, "  determine which specifications need to be created or updated.\n")
		fmt.Fprintf(w, "\n  For each spec, define:\n")
		fmt.Fprintf(w, "    - Name (display name)\n")
		fmt.Fprintf(w, "    - Domain: %s\n", domain)
		fmt.Fprintf(w, "    - Topic of concern:\n")
		topicRules()
		fmt.Fprintf(w, "    - File: target path relative to domain root (specs/<kebab-case-name>.md)\n")
		fmt.Fprintf(w, "    - Action: \"create\" for new specs, \"update\" for existing specs with gaps\n")
		fmt.Fprintf(w, "    - Code search roots:\n")
		fmt.Fprintf(w, "        - The directory (or directories) forming the root of the core\n")
		fmt.Fprintf(w, "          code implementing this topic of concern.\n")
		fmt.Fprintf(w, "        - Go as deep as needed: the root is the deepest directory that\n")
		fmt.Fprintf(w, "          still contains ALL the files for that one capability — often a\n")
		fmt.Fprintf(w, "          nested package several levels down (e.g. net/http/internal/\n")
		fmt.Fprintf(w, "          httpcommon/), not a broad top-level directory.\n")
		fmt.Fprintf(w, "        - A topic may span multiple such directories; list each one.\n")
		fmt.Fprintf(w, "        - Directories, not single files; relative to the domain root.\n")
		fmt.Fprintf(w, "        - Must be non-empty for every spec.\n")
		fmt.Fprintf(w, "    - Dependencies on other specs\n")
		fmt.Fprintf(w, "\n  Decide:\n")
		fmt.Fprintf(w, "    - Which gaps warrant new specs vs. updates to existing specs\n")
		fmt.Fprintf(w, "    - How to group related behaviors into single-topic specs\n")
		fmt.Fprintf(w, "\n  Advance when the spec list for this domain is finalized.\n")

	case StateQueue:
		queuePath := filepath.Join(s.Config.Paths.StateDir, "reverse-engineering-queue.json")
		fmt.Fprintf(w, "State: QUEUE\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "Queue file: %s\n", queuePath)
		fmt.Fprintf(w, "\nAction:\n")
		if re.QueueContentHash == "" {
			// First advance — the file has not been read yet.
			fmt.Fprintf(w, "  Write the reverse engineering queue file with entries for domain %s\n", domain)
			fmt.Fprintf(w, "  at: %s\n", queuePath)
			fmt.Fprintf(w, "\n  Requirements:\n")
			fmt.Fprintf(w, "    - All paths relative to domain root (<project_root>/%s/)\n", domain)
			fmt.Fprintf(w, "    - Order entries by dependency: specs with no dependencies first\n")
			fmt.Fprintf(w, "    - code_search_roots must be non-empty for every entry\n")
			fmt.Fprintf(w, "    - No circular dependencies\n")
		} else {
			// Subsequent advance — accumulate this domain's entries.
			fmt.Fprintf(w, "  Add entries for domain %s to the existing queue file.\n", domain)
			fmt.Fprintf(w, "\n  Update the queue file at: %s\n", queuePath)
			fmt.Fprintf(w, "  Add new entries for this domain alongside existing entries.\n")
		}
		fmt.Fprintf(w, "\n  Advance when the file is written:\n")
		fmt.Fprintf(w, "    forgectl advance\n")

	case StateExecuteReverseEngineer:
		m := len(re.Queue)
		var item *REQueueEntry
		if re.ExecuteItemIndex >= 1 && re.ExecuteItemIndex <= m {
			item = &re.Queue[re.ExecuteItemIndex-1]
		}
		fmt.Fprintf(w, "State: EXECUTE_REVERSE_ENGINEER\n")
		if item == nil {
			fmt.Fprintf(w, "\nAction:\n  No item to execute.\n")
			break
		}
		fmt.Fprintf(w, "Item: %d/%d\n", re.ExecuteItemIndex, m)
		fmt.Fprintf(w, "Domain: %s\n", item.Domain)
		fmt.Fprintf(w, "Spec: %s  (%s)\n", item.Name, item.Action)
		fmt.Fprintf(w, "Target file: %s/%s\n", item.Domain, item.File)
		fmt.Fprintf(w, "Topic of concern: %q\n", item.Topic)
		fmt.Fprintf(w, "\ncode_search_roots:\n")
		for _, root := range item.CodeSearchRoots {
			fmt.Fprintf(w, "  - %s/%s\n", item.Domain, root)
		}
		fmt.Fprintf(w, "\n  code_search_roots are the directories forming the root of the core\n")
		fmt.Fprintf(w, "  code that implements this spec's topic of concern. A topic of concern\n")
		fmt.Fprintf(w, "  may span multiple directories within a module — every listed root, and\n")
		fmt.Fprintf(w, "  every file recursively beneath it, is in scope. These are directories,\n")
		fmt.Fprintf(w, "  not single files, relative to the domain root. Read the core code in\n")
		fmt.Fprintf(w, "  full before writing.\n")
		verb := "Create"
		if item.Action == "update" {
			verb = "Update"
		}
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  %s the specification\n", verb)
		fmt.Fprintf(w, "  at %s/%s from the code under the search roots above.\n", item.Domain, item.File)
		fmt.Fprintf(w, "\n  %s\n", spawn(cfg.Execute))
		fmt.Fprintf(w, "  scoped to the search roots to examine the implementation.\n")
		fmt.Fprintf(w, "\n  Write the specification in the standard spec format, capturing the\n")
		fmt.Fprintf(w, "  contracts, behaviors, invariants, edge cases, and testing criteria\n")
		fmt.Fprintf(w, "  that the code currently implements for this topic of concern.\n")
		if item.Action == "update" {
			fmt.Fprintf(w, "  The file already exists — preserve content that is still correct and\n")
			fmt.Fprintf(w, "  revise the rest to match the current implementation.\n")
		}
		fmt.Fprintf(w, "\n  Advance when the specification file is written:\n")
		fmt.Fprintf(w, "    forgectl advance\n")

	case StatePostReverseEngineer:
		m := len(re.Queue)
		var item *REQueueEntry
		if re.ExecuteItemIndex >= 1 && re.ExecuteItemIndex <= m {
			item = &re.Queue[re.ExecuteItemIndex-1]
		}
		fmt.Fprintf(w, "State: POST_REVERSE_ENGINEER\n")
		if item != nil {
			fmt.Fprintf(w, "Item: %d/%d\n", re.ExecuteItemIndex, m)
			fmt.Fprintf(w, "Spec: %s\n", item.Name)
			fmt.Fprintf(w, "Target file: %s/%s\n", item.Domain, item.File)
		}
		fmt.Fprintf(w, "\nSTOP ensure you have created the specification that you need,\n")
		fmt.Fprintf(w, "please tell your user to clear your context window for the next iteration.\n")
		if re.ExecuteItemIndex < m {
			fmt.Fprintf(w, "\nAdvance to continue with item %d/%d:\n", re.ExecuteItemIndex+1, m)
		} else {
			fmt.Fprintf(w, "\nAdvance to proceed to RECONCILE:\n")
		}
		fmt.Fprintf(w, "  forgectl advance\n")

	case StateReconcile:
		fmt.Fprintf(w, "State: RECONCILE\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "Round: %d\n", re.ReconcileRound)
		fmt.Fprintf(w, "\nSpecs created or updated for this domain:\n")
		printREDomainSpecs(w, re, domain)
		// Surface any expected-but-missing spec files for this domain.
		if gaps := ReverseEngineeringDomainGaps(re, dir); len(gaps) > 0 {
			fmt.Fprintf(w, "\nMissing spec files (report the gap; do not fabricate):\n")
			for _, g := range gaps {
				fmt.Fprintf(w, "  - %s/%s\n", domain, g)
			}
		}
		fmt.Fprintf(w, "\nAction:\n")
		if re.ReconcileRound > 1 {
			fmt.Fprintf(w, "  Reconciliation evaluation failed on the previous round.\n")
			fmt.Fprintf(w, "  Address the findings from the evaluation report and re-reconcile.\n\n")
		}
		fmt.Fprintf(w, "  Cross-reference specifications for domain %s.\n", domain)
		fmt.Fprintf(w, "\n  For every spec that was created or updated, use its depends_on\n")
		fmt.Fprintf(w, "  to add cross-references to the corresponding specs.\n")
		fmt.Fprintf(w, "  Update both the new/updated spec and the spec it references:\n")
		fmt.Fprintf(w, "    - Add Depends On entries in the new/updated spec\n")
		fmt.Fprintf(w, "    - Add Integration Points in both directions\n")
		fmt.Fprintf(w, "      (if A depends on B, both A and B reference each other)\n")
		fmt.Fprintf(w, "\n  Verify consistency:\n")
		fmt.Fprintf(w, "    - Every Depends On reference points to a spec that exists\n")
		fmt.Fprintf(w, "    - Every Depends On has a corresponding Integration Points row\n")
		fmt.Fprintf(w, "      in the referenced spec\n")
		fmt.Fprintf(w, "    - Integration Points are symmetric (A ↔ B)\n")
		fmt.Fprintf(w, "    - Spec names are consistent across all references\n")
		fmt.Fprintf(w, "    - No circular dependencies in the Depends On graph\n")
		fmt.Fprintf(w, "\n  Stage all changes:\n")
		fmt.Fprintf(w, "    git add the modified spec files.\n")
		fmt.Fprintf(w, "\n  Advance when reconciliation is complete and changes are staged.\n")

	case StateReconcileEval:
		maxRounds := cfg.Reconcile.MaxRounds
		fmt.Fprintf(w, "State: RECONCILE_EVAL\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "Round: %d/%d\n", re.ReconcileRound, maxRounds)
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  Evaluate cross-spec consistency for domain %s.\n", domain)
		fmt.Fprintf(w, "\n  %s to evaluate the reconciliation.\n", spawn(cfg.Reconcile.Eval))
		fmt.Fprintf(w, "\n  Instruct your sub-agents to run:\n")
		fmt.Fprintf(w, "    forgectl eval\n")
		fmt.Fprintf(w, "\n  This outputs the evaluation prompt with the full spec files\n")
		fmt.Fprintf(w, "  and consistency checklist for the sub-agents to review.\n")
		fmt.Fprintf(w, "\n  After the sub-agents complete their evaluation, advance with the verdict:\n")
		fmt.Fprintf(w, "    forgectl advance --verdict PASS --eval-report <path>\n")
		fmt.Fprintf(w, "    forgectl advance --verdict FAIL --eval-report <path>\n")
		reportFile := filepath.Join(domain, "specs", ".eval", fmt.Sprintf("reconciliation-r%d.md", re.ReconcileRound))
		fmt.Fprintf(w, "\n  Eval reports are written to: %s\n", reportFile)

	case StateColleagueReview:
		fmt.Fprintf(w, "State: COLLEAGUE_REVIEW\n")
		fmt.Fprintf(w, "Domain: %s (%d/%d)\n", domain, re.DomainIndex, n)
		fmt.Fprintf(w, "Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "\nAction:\n")
		fmt.Fprintf(w, "  STOP and review the specifications with your colleague.\n")
		fmt.Fprintf(w, "\n  Advance when the review is complete:\n")
		fmt.Fprintf(w, "    forgectl advance\n")

	case StateReconcileAdvance:
		fmt.Fprintf(w, "State: RECONCILE_ADVANCE\n")
		if re.DomainIndex < re.DomainCount {
			next := ""
			if re.DomainIndex < len(re.Domains) {
				next = re.Domains[re.DomainIndex]
			}
			fmt.Fprintf(w, "Domain: %s (%d/%d) → %s\n", domain, re.DomainIndex, n, next)
			fmt.Fprintf(w, "\nAction:\n")
			fmt.Fprintf(w, "  Domain %s reconciliation complete.\n", domain)
			fmt.Fprintf(w, "\n  Next: RECONCILE for domain %s (%d/%d)\n", next, re.DomainIndex+1, n)
		} else {
			fmt.Fprintf(w, "Domain: %s (%d/%d) → DONE\n", domain, re.DomainIndex, n)
			fmt.Fprintf(w, "\nAction:\n")
			fmt.Fprintf(w, "  Domain %s reconciliation complete.\n", domain)
			fmt.Fprintf(w, "\n  All domains reconciled. Advancing to DONE.\n")
		}
		fmt.Fprintf(w, "\n  Advance to proceed.\n")

	case StateDone:
		fmt.Fprintf(w, "State: DONE\n")
		fmt.Fprintf(w, "\nThe reverse engineering workflow is complete. All spec files have been\n")
		fmt.Fprintf(w, "produced, verified, and reconciled across all domains.\n")
	}
}

// printREDomainSpecs lists the queue entries for a domain with their action and
// depends_on, used by the RECONCILE action output.
func printREDomainSpecs(w io.Writer, re *ReverseEngineeringState, domain string) {
	for _, entry := range re.Queue {
		if entry.Domain != domain {
			continue
		}
		fmt.Fprintf(w, "  - %s/%s  (%s)\n", entry.Domain, entry.File, entry.Action)
		fmt.Fprintf(w, "    depends_on: %v\n", entry.DependsOn)
	}
}

// --- Status ---

// PrintStatus prints the session status. When verbose is true, full phase
// sections are appended after the progress line.
func PrintStatus(w io.Writer, s *ForgeState, dir string, verbose bool) {
	// Session path: prefer relative (cfg.Paths.StateDir/forgectl-state.json).
	sessionLabel := filepath.Join(s.Config.Paths.StateDir, "forgectl-state.json")
	fmt.Fprintf(w, "Session: %s\n", sessionLabel)

	fmt.Fprintf(w, "Phase:   %s", s.Phase)
	if s.StartedAtPhase != "" && s.StartedAtPhase == s.Phase && s.StartedAtPhase != PhaseSpecifying {
		fmt.Fprintf(w, " (started here)")
	}
	fmt.Fprintln(w)

	// Phase-appropriate config values.
	batch, minRounds, maxRounds := phaseConfig(s)
	fmt.Fprintf(w, "Config:  batch=%d, rounds=%d-%d, guided=%v\n", batch, minRounds, maxRounds, s.Config.General.UserGuided)
	fmt.Fprintln(w)

	// Current state + action.
	fmt.Fprintf(w, "--- Current ---\n\n")
	PrintAdvanceOutput(w, s, dir)
	fmt.Fprintln(w)

	// One-line progress summary.
	printProgressLine(w, s, dir)
	fmt.Fprintln(w)


	if !verbose {
		return
	}

	// --- Verbose sections ---

	// Specifying section.
	if s.Specifying != nil {
		fmt.Fprintf(w, "--- Specifying ---\n\n")
		spec := s.Specifying
		if len(spec.Completed) > 0 && len(spec.Queue) == 0 && len(spec.CurrentSpecs) == 0 {
			if spec.Reconcile != nil && len(spec.Reconcile.Evals) > 0 {
				fmt.Fprintf(w, "  Complete (%d specs, reconciled)\n", len(spec.Completed))
			} else {
				fmt.Fprintf(w, "  Complete (%d specs)\n", len(spec.Completed))
			}
		}

		if len(spec.Queue) > 0 {
			fmt.Fprintf(w, "\n--- Queue ---\n\n")
			for i, q := range spec.Queue {
				fmt.Fprintf(w, "  [%d] %s (%s)\n", len(spec.Completed)+i+2, q.Name, q.Domain)
			}
		}

		if len(spec.Completed) > 0 {
			fmt.Fprintf(w, "\n--- Completed ---\n\n")
			for _, c := range spec.Completed {
				roundLabel := "rounds"
				if c.RoundsTaken == 1 {
					roundLabel = "round"
				}
				fmt.Fprintf(w, "  [%d] %s (%s)  — %d %s", c.ID, c.Name, c.Domain, c.RoundsTaken, roundLabel)
				if len(c.CommitHashes) > 0 {
					fmt.Fprintf(w, ", commit %s", strings.Join(c.CommitHashes, ", "))
				}
				fmt.Fprintln(w)
				for _, e := range c.Evals {
					fmt.Fprintf(w, "       Round %d: %s", e.Round, e.Verdict)
					if e.EvalReport != "" {
						fmt.Fprintf(w, " — %s", e.EvalReport)
					}
					fmt.Fprintln(w)
				}
			}
		}
		fmt.Fprintln(w)
	}

	// Verbose: Planning section.
	if s.Planning != nil {
		fmt.Fprintf(w, "--- Planning ---\n\n")
		plan := s.Planning
		if len(plan.Evals) > 0 {
			lastEval := plan.Evals[len(plan.Evals)-1]
			if lastEval.Verdict == "PASS" && plan.Round >= s.Config.Planning.Eval.MinRounds {
				acceptLabel := "rounds"
				if plan.Round == 1 {
					acceptLabel = "round"
				}
				fmt.Fprintf(w, "  Accepted (%d %s)\n", plan.Round, acceptLabel)
			}
			for _, e := range plan.Evals {
				fmt.Fprintf(w, "    Round %d: %s", e.Round, e.Verdict)
				if e.EvalReport != "" {
					fmt.Fprintf(w, " — %s", e.EvalReport)
				}
				fmt.Fprintln(w)
			}
		} else {
			fmt.Fprintf(w, "  Evals: (none yet)\n")
		}

		fmt.Fprintf(w, "\n--- Queue ---\n\n")
		if len(plan.Queue) > 0 {
			for _, q := range plan.Queue {
				fmt.Fprintf(w, "  %s (%s)\n", q.Name, q.Domain)
			}
		} else {
			fmt.Fprintf(w, "  empty\n")
		}
		fmt.Fprintln(w)
	}

	// Verbose: Implementing section.
	if s.Implementing != nil {
		fmt.Fprintf(w, "--- Implementing ---\n\n")
		plan, _ := loadPlan(s, dir)
		if plan != nil {
			for _, layer := range plan.Layers {
				fmt.Fprintf(w, "  Layer %s (%s):", layer.ID, layer.Name)
				if allLayerItemsTerminal(plan, layer) {
					fmt.Fprintf(w, " complete\n")
				} else {
					fmt.Fprintf(w, " in progress\n")
				}
				for _, id := range layer.Items {
					item := findItem(plan, id)
					if item != nil {
						roundLabel := "rounds"
						if item.Rounds == 1 {
							roundLabel = "round"
						}
						fmt.Fprintf(w, "    [%s]  %s  (%d %s)\n", id, item.Passes, item.Rounds, roundLabel)
					}
				}
			}
		}
		fmt.Fprintln(w)
	}

	// Verbose: Reverse Engineering section.
	if s.ReverseEngineering != nil {
		re := s.ReverseEngineering
		fmt.Fprintf(w, "--- Reverse Engineering ---\n\n")
		fmt.Fprintf(w, "  Concept: %q\n", re.Concept)
		fmt.Fprintf(w, "  Domains (%d):\n", re.DomainCount)
		for i, d := range re.Domains {
			marker := "  "
			if i+1 == re.DomainIndex {
				marker = "→ "
			}
			fmt.Fprintf(w, "    %s[%d] %s\n", marker, i+1, d)
		}

		if len(re.Queue) > 0 {
			fmt.Fprintf(w, "\n--- Queue ---\n\n")
			for i, q := range re.Queue {
				fmt.Fprintf(w, "  [%d] %s (%s)  %s/%s  (%s)\n", i+1, q.Name, q.Domain, q.Domain, q.File, q.Action)
			}
		}

		if len(re.DomainReconcile) > 0 {
			fmt.Fprintf(w, "\n--- Reconcile ---\n\n")
			for _, d := range re.Domains {
				rec, ok := re.DomainReconcile[d]
				if !ok {
					continue
				}
				fmt.Fprintf(w, "  %s: round %d\n", d, rec.Round)
				for _, e := range rec.Evals {
					fmt.Fprintf(w, "    Round %d: %s", e.Round, e.Verdict)
					if e.EvalReport != "" {
						fmt.Fprintf(w, " — %s", e.EvalReport)
					}
					fmt.Fprintln(w)
				}
			}
		}
		fmt.Fprintln(w)
	}
}

// phaseConfig returns the batch size and round bounds for the current phase.
func phaseConfig(s *ForgeState) (batch, minRounds, maxRounds int) {
	switch s.Phase {
	case PhaseSpecifying:
		return s.Config.Specifying.Batch, s.Config.Specifying.Eval.MinRounds, s.Config.Specifying.Eval.MaxRounds
	case PhasePlanning:
		return s.Config.Planning.Batch, s.Config.Planning.Eval.MinRounds, s.Config.Planning.Eval.MaxRounds
	case PhaseReverseEngineering:
		// Reverse engineering has no batch; report the reconcile round bounds.
		return 0, s.Config.ReverseEngineering.Reconcile.MinRounds, s.Config.ReverseEngineering.Reconcile.MaxRounds
	default: // implementing
		return s.Config.Implementing.Batch, s.Config.Implementing.Eval.MinRounds, s.Config.Implementing.Eval.MaxRounds
	}
}

// printProgressLine writes a one-line progress summary for the current phase.
func printProgressLine(w io.Writer, s *ForgeState, dir string) {
	switch s.Phase {
	case PhaseSpecifying:
		if s.Specifying == nil {
			return
		}
		spec := s.Specifying
		total := len(spec.Completed) + len(spec.Queue) + len(spec.CurrentSpecs)
		fmt.Fprintf(w, "Progress: %d/%d specs completed, %d queued\n", len(spec.Completed), total, len(spec.Queue))

	case PhasePlanning:
		if s.Planning == nil {
			return
		}
		fmt.Fprintf(w, "Progress: round %d of %d\n", s.Planning.Round, s.Config.Planning.Eval.MaxRounds)

	case PhaseImplementing:
		plan, _ := loadPlan(s, dir)
		if plan == nil {
			return
		}
		var passed, failed, remaining int
		for i := range plan.Items {
			switch plan.Items[i].Passes {
			case "passed":
				passed++
			case "failed":
				failed++
			default:
				remaining++
			}
		}
		total := passed + failed + remaining
		fmt.Fprintf(w, "Progress: %d/%d passed, %d failed, %d remaining\n", passed, total, failed, remaining)

	case PhaseReverseEngineering:
		if s.ReverseEngineering == nil {
			return
		}
		re := s.ReverseEngineering
		fmt.Fprintf(w, "Progress: domain %d/%d, %d queued specs\n", re.DomainIndex, re.DomainCount, len(re.Queue))
	}
}

// --- Eval command ---

// PrintEvalOutput prints the evaluation context for the sub-agent.
func PrintEvalOutput(w io.Writer, s *ForgeState, dir string) error {
	switch s.Phase {
	case PhasePlanning:
		return printPlanningEval(w, s)
	case PhaseImplementing:
		return printImplementingEval(w, s, dir)
	default:
		return fmt.Errorf("eval is only valid in planning or implementing EVALUATE state (current: %s %s)", s.Phase, s.State)
	}
}

func printPlanningEval(w io.Writer, s *ForgeState) error {
	if s.State != StateEvaluate {
		return fmt.Errorf("eval is only valid in EVALUATE state (current: %s)", s.State)
	}

	plan := s.Planning

	fmt.Fprintf(w, "=== PLAN EVALUATION ROUND %d/%d ===\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
	fmt.Fprintf(w, "Plan:   %s\n", plan.CurrentPlan.Name)
	fmt.Fprintf(w, "Domain: %s\n", plan.CurrentPlan.Domain)
	fmt.Fprintf(w, "File:   %s\n", plan.CurrentPlan.File)

	// Evaluator instructions.
	fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
	fmt.Fprintf(w, "%s\n", evaluators.PlanEval)

	// Plan references.
	fmt.Fprintf(w, "\n--- PLAN REFERENCES ---\n\n")
	fmt.Fprintf(w, "Plan:    %s\n", plan.CurrentPlan.File)
	fmt.Fprintf(w, "Format:  PLAN_FORMAT.md\n")
	fmt.Fprintf(w, "Specs:\n")
	for _, spec := range plan.CurrentPlan.Specs {
		fmt.Fprintf(w, "  - %s\n", spec)
	}

	// Previous evaluations + report output, per eval_mode.
	evalDir := filepath.Join(filepath.Dir(plan.CurrentPlan.File), "evals")
	reportFile := filepath.Join(evalDir, fmt.Sprintf("round-%d.md", plan.Round))
	writeEvalTrailingSections(w, EvalModeFor(s.Config.Planning.Eval, s.Config.General), plan.Evals, reportFile, "plan", true)

	return nil
}

func printImplementingEval(w io.Writer, s *ForgeState, dir string) error {
	if s.State != StateEvaluate {
		return fmt.Errorf("eval is only valid in EVALUATE state (current: %s)", s.State)
	}

	impl := s.Implementing
	batch := impl.CurrentBatch

	evalRound := batch.EvalRound + 1

	fmt.Fprintf(w, "=== IMPLEMENTATION EVALUATION ROUND %d/%d ===\n", evalRound, s.Config.Implementing.Eval.MaxRounds)
	fmt.Fprintf(w, "Layer: %s %s\n", impl.CurrentLayer.ID, impl.CurrentLayer.Name)

	plan, planErr := loadPlan(s, dir)
	totalBatches := 0
	if planErr == nil && plan != nil {
		totalBatches = countTotalBatches(plan, s.Config.Implementing.Batch)
	}
	fmt.Fprintf(w, "Batch: %d/%d\n", impl.BatchNumber, totalBatches)

	// Evaluator instructions.
	fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
	fmt.Fprintf(w, "%s\n", evaluators.ImplEval)

	// Items to evaluate.
	fmt.Fprintf(w, "\n--- ITEMS TO EVALUATE ---\n\n")
	if planErr != nil {
		return planErr
	}

	rendered := 0
	for i, id := range batch.Items {
		item := findItem(plan, id)
		if item == nil {
			continue
		}
		if rendered > 0 {
			fmt.Fprintln(w)
		}
		rendered++

		fmt.Fprintf(w, "[%d] %s — %s\n", i+1, item.ID, item.Name)
		fmt.Fprintf(w, "    Description: %s\n", item.Description)
		if len(item.Specs) > 0 {
			for i, spec := range item.Specs {
				if i == 0 {
					fmt.Fprintf(w, "    Specs:       %s\n", spec)
				} else {
					fmt.Fprintf(w, "                 %s\n", spec)
				}
			}
		}
		if len(item.Refs) > 0 {
			for i, ref := range item.Refs {
				if i == 0 {
					fmt.Fprintf(w, "    Refs:        %s\n", ref)
				} else {
					fmt.Fprintf(w, "                 %s\n", ref)
				}
			}
		}
		if len(item.Files) > 0 {
			fmt.Fprintf(w, "    Files:       %s\n", strings.Join(item.Files, ", "))
		}
		if len(item.Steps) > 0 {
			fmt.Fprintf(w, "    Steps:\n")
			for j, step := range item.Steps {
				fmt.Fprintf(w, "      %d. %s\n", j+1, step)
			}
		}
		if len(item.Tests) > 0 {
			fmt.Fprintf(w, "    Tests:\n")
			for _, t := range item.Tests {
				fmt.Fprintf(w, "      [%s] %s\n", t.Category, t.Description)
			}
		}
	}

	// Previous evaluations + report output, per eval_mode.
	evalDir := filepath.Join(currentPlanDir(s), "evals")
	reportFile := filepath.Join(evalDir, fmt.Sprintf("batch-%d-round-%d.md", impl.BatchNumber, evalRound))
	writeEvalTrailingSections(w, EvalModeFor(s.Config.Implementing.Eval, s.Config.General), batch.Evals, reportFile, "batch", true)

	return nil
}

// PrintSpecEvalOutput prints the per-batch spec evaluation context for the
// sub-agent. Valid in specifying EVALUATE state. It embeds the spec evaluator
// prompt, lists the batch specs (filename, topic, full path), and renders the
// per-mode --- PREVIOUS EVALUATIONS --- and --- REPORT OUTPUT --- sections.
func PrintSpecEvalOutput(w io.Writer, s *ForgeState, projectRoot string) error {
	if s.Phase != PhaseSpecifying || s.State != StateEvaluate {
		return fmt.Errorf("eval is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
	}

	spec := s.Specifying
	if spec == nil || len(spec.CurrentSpecs) == 0 {
		return fmt.Errorf("no spec batch is currently being evaluated")
	}
	cs := spec.CurrentSpecs[0]
	maxRounds := s.Config.Specifying.Eval.MaxRounds

	fmt.Fprintf(w, "=== SPEC EVALUATION ROUND %d/%d ===\n", cs.Round, maxRounds)
	fmt.Fprintf(w, "Domain: %s\n", cs.Domain)
	fmt.Fprintf(w, "Batch:  %d\n", spec.BatchNumber)
	fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
	fmt.Fprintf(w, "%s\n", evaluators.SpecEval)

	fmt.Fprintf(w, "\n--- SPECS TO EVALUATE ---\n\n")
	for i, bcs := range spec.CurrentSpecs {
		fmt.Fprintf(w, "[%d] %s\n", i+1, filepath.Base(bcs.File))
		fmt.Fprintf(w, "    Topic: %s\n", bcs.Topic)
		fmt.Fprintf(w, "    File:  %s\n", bcs.File)
		if i < len(spec.CurrentSpecs)-1 {
			fmt.Fprintln(w)
		}
	}

	reportFile := filepath.Join(cs.Domain, "specs", ".eval", fmt.Sprintf("batch-%d-r%d.md", spec.BatchNumber, cs.Round))
	writeEvalTrailingSections(w, EvalModeFor(s.Config.Specifying.Eval, s.Config.General), cs.Evals, reportFile, "spec", true)

	return nil
}

// PrintReconcileEvalOutput prints the reconciliation evaluation context for the sub-agent.
// Valid in specifying RECONCILE_EVAL state.
func PrintReconcileEvalOutput(w io.Writer, s *ForgeState) error {
	if s.Phase != PhaseSpecifying || s.State != StateReconcileEval {
		return fmt.Errorf("eval is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
	}

	spec := s.Specifying
	round := 0
	maxRounds := s.Config.Specifying.Reconciliation.MaxRounds
	if spec.Reconcile != nil {
		round = spec.Reconcile.Round
	}

	fmt.Fprintf(w, "=== RECONCILIATION EVALUATION ROUND %d/%d ===\n", round, maxRounds)
	fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
	fmt.Fprintf(w, "%s\n", evaluators.ReconcileEval)

	// Domain counts from completed specs.
	domainCounts := map[string]int{}
	var domainOrder []string
	for _, c := range spec.Completed {
		if _, seen := domainCounts[c.Domain]; !seen {
			domainOrder = append(domainOrder, c.Domain)
		}
		domainCounts[c.Domain]++
	}
	fmt.Fprintf(w, "\n--- DOMAINS ---\n\n")
	for _, d := range domainOrder {
		fmt.Fprintf(w, "%s: %d specs\n", d, domainCounts[d])
	}

	fmt.Fprintf(w, "\n--- RECONCILIATION CONTEXT ---\n\n")
	fmt.Fprintf(w, "Run: git diff --staged\n")

	reportFile := ""
	if len(spec.Completed) > 0 {
		reportFile = filepath.Join(filepath.Dir(spec.Completed[0].File), ".eval", fmt.Sprintf("reconciliation-r%d.md", round))
	}
	var prevEvals []EvalRecord
	if spec.Reconcile != nil {
		prevEvals = spec.Reconcile.Evals
	}
	// Reconciliation operates on staged cross-domain changes (git diff --staged);
	// direct/conversational modes omit the REPORT OUTPUT section.
	writeEvalTrailingSections(w, EvalModeFor(s.Config.Specifying.Eval, s.Config.General), prevEvals, reportFile, "spec", false)

	return nil
}

// PrintCrossRefEvalOutput prints the cross-reference evaluation context for the sub-agent.
// Valid in specifying CROSS_REFERENCE_EVAL state.
func PrintCrossRefEvalOutput(w io.Writer, s *ForgeState) error {
	if s.Phase != PhaseSpecifying || s.State != StateCrossReferenceEval {
		return fmt.Errorf("eval is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
	}

	spec := s.Specifying
	domain := spec.CurrentDomain
	round := 0
	maxRounds := s.Config.Specifying.CrossReference.MaxRounds
	if spec.CrossReference != nil {
		if cr, ok := spec.CrossReference[domain]; ok {
			round = cr.Round
		}
	}

	fmt.Fprintf(w, "=== CROSS-REFERENCE EVALUATION ROUND %d/%d ===\n", round, maxRounds)
	fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
	fmt.Fprintf(w, "%s\n", evaluators.CrossRefEval)

	// Domain and its specs from completed.
	var domainSpecs []CompletedSpec
	for _, c := range spec.Completed {
		if c.Domain == domain {
			domainSpecs = append(domainSpecs, c)
		}
	}
	fmt.Fprintf(w, "\n--- DOMAIN ---\n\n")
	fmt.Fprintf(w, "%s: %d specs\n", domain, len(domainSpecs))

	if len(domainSpecs) > 0 {
		fmt.Fprintf(w, "\n--- SPECS ---\n\n")
		for i, c := range domainSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, c.File)
		}
	}

	reportFile := ""
	if len(domainSpecs) > 0 {
		reportFile = filepath.Join(filepath.Dir(domainSpecs[0].File), ".eval", fmt.Sprintf("cross-reference-r%d.md", round))
	}
	var prevEvals []EvalRecord
	if spec.CrossReference != nil {
		if cr, ok := spec.CrossReference[domain]; ok {
			prevEvals = cr.Evals
		}
	}
	writeEvalTrailingSections(w, EvalModeFor(s.Config.Specifying.Eval, s.Config.General), prevEvals, reportFile, "spec", true)

	return nil
}

// PrintReverseEngineeringEvalOutput prints the reconciliation evaluation context
// for the sub-agent during the reverse_engineering RECONCILE_EVAL state. It
// populates the embedded reconcile evaluator prompt with the current domain's
// spec list (each with its depends_on), the current round, and the report path.
func PrintReverseEngineeringEvalOutput(w io.Writer, s *ForgeState) error {
	if s.Phase != PhaseReverseEngineering || s.State != StateReconcileEval {
		return fmt.Errorf("forgectl eval is only available during RECONCILE_EVAL.")
	}
	re := s.ReverseEngineering
	if re == nil {
		return fmt.Errorf("reverse_engineering state is not initialized")
	}

	round := re.ReconcileRound
	maxRounds := s.Config.ReverseEngineering.Reconcile.MaxRounds

	domain := ""
	if re.DomainIndex >= 1 && re.DomainIndex <= len(re.Domains) {
		domain = re.Domains[re.DomainIndex-1]
	}

	fmt.Fprintf(w, "=== RECONCILIATION EVALUATION ROUND %d/%d ===\n", round, maxRounds)
	fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
	fmt.Fprintf(w, "%s\n", evaluators.ReconcileEval)

	// Specs created or updated for the current domain, with their depends_on.
	fmt.Fprintf(w, "\n--- DOMAIN ---\n\n")
	fmt.Fprintf(w, "%s (%d/%d)\n", domain, re.DomainIndex, re.DomainCount)

	fmt.Fprintf(w, "\n--- SPECS ---\n\n")
	count := 0
	for _, entry := range re.Queue {
		if entry.Domain != domain {
			continue
		}
		count++
		fmt.Fprintf(w, "  [%d] %s/%s  (%s)\n", count, entry.Domain, entry.File, entry.Action)
		fmt.Fprintf(w, "      depends_on: %v\n", entry.DependsOn)
	}
	if count == 0 {
		fmt.Fprintf(w, "  (no specs queued for this domain)\n")
	}

	reportFile := filepath.Join(domain, "specs", ".eval", fmt.Sprintf("reconciliation-r%d.md", round))
	fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
	fmt.Fprintf(w, "Write your evaluation report to:\n")
	fmt.Fprintf(w, "  %s\n", reportFile)

	return nil
}

// --- Helpers ---

// currentPlanDir returns the directory containing the active plan file.
func currentPlanDir(s *ForgeState) string {
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		return filepath.Dir(s.Planning.CurrentPlan.File)
	}
	return "."
}

func findLayer(plan *PlanJSON, id string) *PlanLayerDef {
	for i := range plan.Layers {
		if plan.Layers[i].ID == id {
			return &plan.Layers[i]
		}
	}
	return nil
}

func countTotalBatches(plan *PlanJSON, batchSize int) int {
	total := 0
	for _, layer := range plan.Layers {
		items := len(layer.Items)
		batches := (items + batchSize - 1) / batchSize
		total += batches
	}
	return total
}
