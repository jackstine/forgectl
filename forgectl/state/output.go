package state

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"forgectl/evaluators"
)

// phaseEvalConfig returns (batch, minRounds, maxRounds) for the current phase.
func phaseEvalConfig(s *ForgeState) (int, int, int) {
	switch s.Phase {
	case PhaseSpecifying:
		c := s.Config.Specifying
		return c.Batch, c.Eval.MinRounds, c.Eval.MaxRounds
	case PhasePlanning:
		c := s.Config.Planning
		return c.Batch, c.Eval.MinRounds, c.Eval.MaxRounds
	default:
		c := s.Config.Implementing
		return c.Batch, c.Eval.MinRounds, c.Eval.MaxRounds
	}
}

// PrintAdvanceOutput prints the action description for the new state after advance.
func PrintAdvanceOutput(w io.Writer, s *ForgeState, dir string) {
	switch s.Phase {
	case PhaseSpecifying:
		printSpecifyingOutput(w, s, dir)
	case PhasePlanning:
		printPlanningOutput(w, s, dir)
	case PhaseImplementing:
		printImplementingOutput(w, s, dir)
	}

	// Phase shift output is printed regardless of phase.
	if s.State == StatePhaseShift && s.PhaseShift != nil {
		printPhaseShiftOutput(w, s)
	}
}

// --- Specifying ---

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
		evalFile := batchEvalFile(cs.Domain, spec.BatchNumber, spec.BatchRound)
		fmt.Fprintf(w, "State:   EVALUATE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(cs.Domain))
		fmt.Fprintf(w, "Batch:   %d specs\n", len(spec.CurrentSpecs))
		fmt.Fprintf(w, "Round:   %d/%d\n", spec.BatchRound, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Specs:\n")
		for i, bcs := range spec.CurrentSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, bcs.File)
		}
		fmt.Fprintf(w, "Action:  Please spawn 1 %s sub-agent to evaluate the spec batch.\n", s.Config.Specifying.Eval.Type)
		fmt.Fprintf(w, "         Eval output: %s\n", evalFile)
		if s.Config.General.EnableCommits {
			fmt.Fprintf(w, "         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>\n")
			fmt.Fprintf(w, "           (--message <commit msg> required with PASS)\n")
		} else {
			fmt.Fprintf(w, "         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>\n")
		}

	case StateRefine:
		evalFile := batchEvalFile(cs.Domain, spec.BatchNumber, spec.BatchRound)
		fmt.Fprintf(w, "State:   REFINE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(cs.Domain))
		fmt.Fprintf(w, "Batch:   %d specs\n", len(spec.CurrentSpecs))
		fmt.Fprintf(w, "Round:   %d/%d\n", spec.BatchRound, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Specs:\n")
		for i, bcs := range spec.CurrentSpecs {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, bcs.File)
		}
		fmt.Fprintf(w, "Action:  Read the eval report and address any findings in the spec files\n")
		fmt.Fprintf(w, "         using the spec skill.\n")
		fmt.Fprintf(w, "         Eval report: %s\n", evalFile)
		fmt.Fprintf(w, "         Format:      references/spec-format.md\n")
		fmt.Fprintf(w, "         Process:     references/spec-generation-skill.md\n")
		fmt.Fprintf(w, "         Scoping:     references/topic-of-concern.md\n")
		fmt.Fprintf(w, "         After completion of the above, advance to continue evaluation.\n")

	case StateAccept:
		fmt.Fprintf(w, "State:   ACCEPT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", spec.CurrentDomain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(spec.CurrentDomain))
		if cs != nil {
			fmt.Fprintf(w, "Batch:   %d specs accepted\n", len(spec.CurrentSpecs))
			fmt.Fprintf(w, "Round:   %d/%d\n", spec.BatchRound, s.Config.Specifying.Eval.MaxRounds)
		}
		fmt.Fprintf(w, "Action:  Batch accepted.\n")
		fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")

	case StateCrossReference:
		currentDomain := spec.CurrentDomain
		fmt.Fprintf(w, "State:   CROSS_REFERENCE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Domain:  %s\n", currentDomain)
		fmt.Fprintf(w, "Path:    %s\n", domainPath(currentDomain))
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
		fmt.Fprintf(w, "Action:  Please spawn 1 %s sub-agent to evaluate cross-reference consistency.\n", evalAgentType)
		fmt.Fprintf(w, "         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>\n")

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
		maxRounds := s.Config.Specifying.Reconciliation.MaxRounds
		fmt.Fprintf(w, "State:   RECONCILE_EVAL\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Round:   %d/%d\n", spec.Reconcile.Round, maxRounds)
		fmt.Fprintf(w, "Action:  Please spawn 1 opus sub-agent to evaluate cross-domain reconciliation.\n")
		fmt.Fprintf(w, "         The sub-agent should run: forgectl eval\n")
		fmt.Fprintf(w, "         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>\n")
		if s.Config.General.EnableCommits {
			fmt.Fprintf(w, "           (--message <commit msg> required with PASS)\n")
		}

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
		fmt.Fprintf(w, "Action:  Run evaluation sub-agent against the plan (round %d/%d).\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		fmt.Fprintf(w, "         Sub-agent: forgectl eval\n")
		if s.Config.General.EnableEvalOutput {
			fmt.Fprintf(w, "         Advance with --verdict PASS|FAIL --eval-report <path>.\n")
		} else {
			fmt.Fprintf(w, "         Advance with --verdict PASS|FAIL.\n")
		}

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
		if s.Config.General.EnableEvalOutput {
			if lastEval.Verdict == "FAIL" {
				fmt.Fprintf(w, "Action:  Study the eval file %q\n", evalFile)
				fmt.Fprintf(w, "         and implement any corrections as needed.\n")
			} else {
				fmt.Fprintf(w, "Action:  Minimum evaluation rounds not met. Spawn a sub-agent to re-evaluate the plan.\n")
				fmt.Fprintf(w, "         Eval report: %s\n", evalFile)
			}
			fmt.Fprintf(w, "         Apply \"fresh\" eyes and a tightened lens when reviewing the work,\n")
			fmt.Fprintf(w, "         then apply corrections as needed.\n")
			fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
		} else {
			fmt.Fprintf(w, "Action:  Make corrections based off communication with the evaluator.\n")
			fmt.Fprintf(w, "         Implement any corrections as needed.\n")
			fmt.Fprintf(w, "         Apply \"fresh\" eyes and a tightened lens when reviewing the work,\n")
			fmt.Fprintf(w, "         then apply corrections as needed.\n")
			fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
		}

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

			fmt.Fprintf(w, "Action:   STOP please review and discuss with user before continuing.\n")
			fmt.Fprintf(w, "          After completion of the above, %s\n", actionContinue)
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
		if len(item.Specs) > 0 {
			for i, spec := range item.Specs {
				if i == 0 {
					fmt.Fprintf(w, "Specs:   %s\n", spec)
				} else {
					fmt.Fprintf(w, "         %s\n", spec)
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
			if s.Config.General.EnableEvalOutput {
				fmt.Fprintf(w, "Action:  Study the eval file %q\n", evalFile)
				fmt.Fprintf(w, "         and implement any corrections as needed.\n")
				fmt.Fprintf(w, "         Apply \"fresh\" eyes and a tightened lens when reviewing the work,\n")
				fmt.Fprintf(w, "         then apply corrections as needed.\n")
				fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
			} else {
				fmt.Fprintf(w, "Action:  Make corrections based off communication with the evaluator.\n")
				fmt.Fprintf(w, "         Implement any corrections as needed.\n")
				fmt.Fprintf(w, "         Apply \"fresh\" eyes and a tightened lens when reviewing the work,\n")
				fmt.Fprintf(w, "         then apply corrections as needed.\n")
				fmt.Fprintf(w, "         After completion of the above, advance to continue.\n")
			}
		} else {
			fmt.Fprintf(w, "Action:  Implement this item.\n")
			fmt.Fprintf(w, "         When complete, run: forgectl advance --message <commit msg>\n")
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

		fmt.Fprintf(w, "Action:   Ask the evaluation sub-agent to verify batch items against their tests.\n")
		fmt.Fprintf(w, "          The sub-agent should run: forgectl eval\n")
		fmt.Fprintf(w, "          After reviewing the eval report, run:\n")
		if s.Config.General.EnableEvalOutput {
			fmt.Fprintf(w, "            forgectl advance --eval-report <path> --verdict PASS|FAIL\n")
		} else {
			fmt.Fprintf(w, "            forgectl advance --verdict PASS|FAIL\n")
		}

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
			fmt.Fprintf(w, "Action:  Advance to continue.\n")
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

	if ps.From == PhasePlanning && ps.To == PhaseImplementing {
		if s.Planning != nil && s.Planning.CurrentPlan != nil {
			fmt.Fprintf(w, "Plan:    %s\n", s.Planning.CurrentPlan.Name)
			fmt.Fprintf(w, "Domain:  %s\n", s.Planning.CurrentPlan.Domain)
			fmt.Fprintf(w, "File:    %s\n", s.Planning.CurrentPlan.File)
		}
	}

	switch {
	case ps.From == PhaseSpecifying && ps.To == PhaseGeneratePlanningQueue:
		fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
		fmt.Fprintf(w, "When ready:\n")
		fmt.Fprintf(w, "  forgectl advance                            # generate plan queue from completed specs\n")
		fmt.Fprintf(w, "  forgectl advance --from <plan-queue.json>   # OR provide a plan queue (skips generation)\n")
	case ps.From == PhaseGeneratePlanningQueue && ps.To == PhasePlanning:
		fmt.Fprintf(w, "\nAdvance to continue.\n")
	default:
		fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
		fmt.Fprintf(w, "When ready, run: forgectl advance\n")
	}
}

// --- Status ---

// PrintStatus prints the session status. When verbose is true, appends detailed
// queue, completed, and implementing sections after the standard output.
func PrintStatus(w io.Writer, s *ForgeState, dir string, verbose bool) {
	// Header (always shown).
	fmt.Fprintf(w, "Session: forgectl-state.json\n")
	fmt.Fprintf(w, "Phase:   %s", s.Phase)
	if s.StartedAtPhase != "" && s.StartedAtPhase == s.Phase && s.StartedAtPhase != PhaseSpecifying {
		fmt.Fprintf(w, " (started here)")
	}
	fmt.Fprintln(w)
	batch, min, max := phaseEvalConfig(s)
	fmt.Fprintf(w, "Config:  batch=%d, rounds=%d-%d, guided=%v\n", batch, min, max, s.Config.General.UserGuided)
	fmt.Fprintln(w)

	// Current state (always shown).
	fmt.Fprintf(w, "--- Current ---\n\n")
	PrintAdvanceOutput(w, s, dir)
	fmt.Fprintln(w)

	if !verbose {
		return
	}

	// Verbose: Specifying section.
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

	// Previous evaluations.
	if len(plan.Evals) > 0 {
		fmt.Fprintf(w, "\n--- PREVIOUS EVALUATIONS ---\n\n")
		for _, e := range plan.Evals {
			fmt.Fprintf(w, "Round %d: %s", e.Round, e.Verdict)
			if e.EvalReport != "" {
				fmt.Fprintf(w, " — %s", e.EvalReport)
			}
			fmt.Fprintln(w)
		}
	}

	if s.Config.General.EnableEvalOutput {
		evalDir := filepath.Join(filepath.Dir(plan.CurrentPlan.File), "evals")
		reportFile := filepath.Join(evalDir, fmt.Sprintf("round-%d.md", plan.Round))
		fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
		fmt.Fprintf(w, "Write your evaluation report to:\n")
		fmt.Fprintf(w, "  %s\n", reportFile)
	}

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

	for i, id := range batch.Items {
		item := findItem(plan, id)
		if item == nil {
			continue
		}

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
		fmt.Fprintln(w)
	}

	// Previous evaluations.
	if len(batch.Evals) > 0 {
		fmt.Fprintf(w, "--- PREVIOUS EVALUATIONS ---\n\n")
		for _, e := range batch.Evals {
			fmt.Fprintf(w, "Round %d: %s", e.Round, e.Verdict)
			if e.EvalReport != "" {
				fmt.Fprintf(w, " — %s", e.EvalReport)
			}
			fmt.Fprintln(w)
		}
	}

	if s.Config.General.EnableEvalOutput {
		evalDir := filepath.Join(currentPlanDir(s), "evals")
		reportFile := filepath.Join(evalDir, fmt.Sprintf("batch-%d-round-%d.md", impl.BatchNumber, evalRound))
		fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
		fmt.Fprintf(w, "Write your evaluation report to:\n")
		fmt.Fprintf(w, "  %s\n", reportFile)
	}

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

	if s.Config.General.EnableEvalOutput && len(spec.Completed) > 0 {
		specDir := filepath.Dir(spec.Completed[0].File)
		reportFile := filepath.Join(specDir, ".eval", fmt.Sprintf("reconciliation-r%d.md", round))
		fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
		fmt.Fprintf(w, "Write your evaluation report to:\n")
		fmt.Fprintf(w, "  %s\n", reportFile)
	}

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

	if s.Config.General.EnableEvalOutput && len(domainSpecs) > 0 {
		specDir := filepath.Dir(domainSpecs[0].File)
		reportFile := filepath.Join(specDir, ".eval", fmt.Sprintf("cross-reference-r%d.md", round))
		fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
		fmt.Fprintf(w, "Write your evaluation report to:\n")
		fmt.Fprintf(w, "  %s\n", reportFile)
	}

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
