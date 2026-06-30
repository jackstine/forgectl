package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Advance transitions the state machine forward based on current state and input.
func Advance(s *ForgeState, in AdvanceInput, dir string) error {
	// Update guided setting if provided.
	if in.Guided != nil {
		s.Config.General.UserGuided = *in.Guided
	}

	// Phase shift is handled before phase dispatch — it can occur
	// while the phase field still reads the source phase.
	if s.State == StatePhaseShift {
		return advancePhaseShift(s, in, dir)
	}

	switch s.Phase {
	case PhaseSpecifying:
		return advanceSpecifying(s, in, dir)
	case PhaseGeneratePlanningQueue:
		return advanceGeneratePlanningQueue(s, in, dir)
	case PhasePlanning:
		return advancePlanning(s, in, dir)
	case PhaseImplementing:
		return advanceImplementing(s, in, dir)
	case PhaseUIImplementing:
		return advanceUIImplementing(s, in, dir)
	case PhaseReverseEngineering:
		return advanceReverseEngineering(s, in, dir)
	default:
		return fmt.Errorf("unknown phase %q", s.Phase)
	}
}

// --- Specifying Phase ---

func advanceSpecifying(s *ForgeState, in AdvanceInput, dir string) error {
	spec := s.Specifying

	switch s.State {
	case StateOrient:
		// Select batch: take up to BatchSize contiguous specs from the first domain.
		if len(spec.Queue) == 0 {
			return fmt.Errorf("queue is empty")
		}
		firstDomain := spec.Queue[0].Domain
		batchSize := s.Config.Specifying.Batch
		if batchSize < 1 {
			batchSize = 1
		}
		var taken int
		for taken < len(spec.Queue) && taken < batchSize && spec.Queue[taken].Domain == firstDomain {
			taken++
		}
		spec.BatchNumber++
		spec.CurrentDomain = firstDomain

		batch := make([]*ActiveSpec, taken)
		for i, entry := range spec.Queue[:taken] {
			batch[i] = &ActiveSpec{
				ID:              len(spec.Completed) + i + 1,
				Name:            entry.Name,
				Domain:          entry.Domain,
				Topic:           entry.Topic,
				File:            entry.File,
				PlanningSources: entry.PlanningSources,
				DependsOn:       entry.DependsOn,
			}
		}
		spec.Queue = spec.Queue[taken:]
		spec.CurrentSpecs = batch
		s.State = StateSelect

	case StateSelect:
		s.State = StateDraft

	case StateDraft:
		for _, cs := range spec.CurrentSpecs {
			cs.Round = 1
		}
		s.State = StateEvaluate

	case StateEvaluate:
		cs := spec.CurrentSpecs[0]
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in EVALUATE state")
		}
		// Per spec-lifecycle.md, EVALUATE only accepts --verdict and --eval-report.
		// --eval-report is required only in report mode. The ignore warning for
		// non-report modes is emitted once at the cmd layer (printAdvanceWarnings).
		if EvalModeFor(s.Config.Specifying.Eval, s.Config.General) == "report" && in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in EVALUATE state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if in.EvalReport != "" {
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}

		eval := EvalRecord{
			Round:      cs.Round,
			Verdict:    in.Verdict,
			EvalReport: in.EvalReport,
		}
		for _, bcs := range spec.CurrentSpecs {
			bcs.Evals = append(bcs.Evals, eval)
		}

		minRounds := s.Config.Specifying.Eval.MinRounds
		maxRounds := s.Config.Specifying.Eval.MaxRounds

		if in.Verdict == "PASS" {
			if cs.Round >= minRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		} else {
			if cs.Round >= maxRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		}

	case StateRefine:
		for _, cs := range spec.CurrentSpecs {
			cs.Round++
		}
		s.State = StateEvaluate

	case StateAccept:
		currentDomain := spec.CurrentDomain
		for _, cs := range spec.CurrentSpecs {
			completed := CompletedSpec{
				ID:          cs.ID,
				Name:        cs.Name,
				Domain:      cs.Domain,
				File:        cs.File,
				BatchNumber: spec.BatchNumber,
				RoundsTaken: cs.Round,
				Evals:       cs.Evals,
			}
			spec.Completed = append(spec.Completed, completed)
		}
		spec.CurrentSpecs = nil

		// Check if the same domain has more queued specs.
		hasSameDomain := false
		for _, q := range spec.Queue {
			if q.Domain == currentDomain {
				hasSameDomain = true
				break
			}
		}

		if hasSameDomain {
			s.State = StateOrient
		} else {
			// Domain exhausted — start cross-reference.
			if spec.CrossReference == nil {
				spec.CrossReference = make(map[string]*CrossReferenceState)
			}
			spec.CrossReference[currentDomain] = &CrossReferenceState{Domain: currentDomain}
			s.State = StateCrossReference
		}

	case StateCrossReference:
		currentDomain := spec.CurrentDomain
		spec.CrossReference[currentDomain].Round++
		s.State = StateCrossReferenceEval

	case StateCrossReferenceEval:
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in CROSS_REFERENCE_EVAL state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		// --eval-report is required only in report mode; the ignore warning for
		// non-report modes is emitted once at the cmd layer.
		if EvalModeFor(s.Config.Specifying.Eval, s.Config.General) == "report" && in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in CROSS_REFERENCE_EVAL state")
		}
		if in.EvalReport != "" {
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}

		currentDomain := spec.CurrentDomain
		cr := spec.CrossReference[currentDomain]
		eval := EvalRecord{
			Round:      cr.Round,
			Verdict:    in.Verdict,
			EvalReport: in.EvalReport,
		}
		cr.Evals = append(cr.Evals, eval)

		minRounds := s.Config.Specifying.CrossReference.MinRounds
		maxRounds := s.Config.Specifying.CrossReference.MaxRounds

		forced := in.Verdict == "FAIL" && cr.Round >= maxRounds
		passed := in.Verdict == "PASS" && cr.Round >= minRounds
		if passed || forced {
			// CROSS_REFERENCE_REVIEW fires once — only on the first passing eval (round==1).
			// Subsequent passing evals skip review and go directly to next domain or DONE.
			if cr.Round == 1 {
				s.State = StateCrossReferenceReview
			} else {
				specCrossRefNextOrDone(s)
			}
		} else {
			s.State = StateCrossReference
		}

	case StateCrossReferenceReview:
		specCrossRefNextOrDone(s)

	case StateDone:
		spec.Reconcile = &ReconcileState{Round: 0}
		s.State = StateReconcile

	case StateReconcile:
		spec.Reconcile.Round++
		s.State = StateReconcileEval

	case StateReconcileEval:
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in RECONCILE_EVAL state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		// --eval-report is required only in report mode; the ignore warning for
		// non-report modes is emitted once at the cmd layer.
		if EvalModeFor(s.Config.Specifying.Eval, s.Config.General) == "report" && in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in RECONCILE_EVAL state")
		}
		if in.EvalReport != "" {
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}

		eval := EvalRecord{
			Round:      spec.Reconcile.Round,
			Verdict:    in.Verdict,
			EvalReport: in.EvalReport,
		}
		spec.Reconcile.Evals = append(spec.Reconcile.Evals, eval)

		minRounds := s.Config.Specifying.Reconciliation.MinRounds
		maxRounds := s.Config.Specifying.Reconciliation.MaxRounds

		forced := in.Verdict == "FAIL" && spec.Reconcile.Round >= maxRounds
		passed := in.Verdict == "PASS" && spec.Reconcile.Round >= minRounds
		if passed || forced {
			// RECONCILE_REVIEW fires once — only on the first passing (or forced) eval (round==1).
			if spec.Reconcile.Round == 1 {
				s.State = StateReconcileReview
			} else {
				s.State = StateComplete
			}
		} else {
			s.State = StateReconcile
		}

	case StateReconcileReview:
		// No flags — transition is queue-based only.
		// Non-empty queue re-enters DONE so new specs can be drafted before reconciliation restarts.
		if len(spec.Queue) > 0 {
			s.State = StateDone
		} else {
			s.State = StateComplete
		}

	case StateComplete:
		if s.Config.General.EnableCommits {
			if in.Message == "" {
				return fmt.Errorf("--message is required in COMPLETE state when enable_commits is true")
			}
			// Collect spec files from all completed specs.
			var stageTargets []string
			for _, cs := range spec.Completed {
				if cs.File != "" {
					stageTargets = append(stageTargets, cs.File)
				}
			}
			hash, err := AutoCommit(dir, s.Config.Specifying.CommitStrategy, stageTargets, in.Message)
			if err != nil {
				return err
			}
			if hash != "" {
				for i := range spec.Completed {
					spec.Completed[i].CommitHashes = append(spec.Completed[i].CommitHashes, hash)
				}
			}
		}
		s.State = StatePhaseShift
		s.PhaseShift = &PhaseShiftInfo{From: PhaseSpecifying, To: PhaseGeneratePlanningQueue}

	default:
		return fmt.Errorf("cannot advance from state %q in specifying phase", s.State)
	}

	return nil
}

// specCrossRefNextOrDone moves to ORIENT if more domains remain in the queue,
// or to DONE when all queue items are exhausted.
func specCrossRefNextOrDone(s *ForgeState) {
	spec := s.Specifying
	if len(spec.Queue) > 0 {
		// More specs in other domains — continue with ORIENT.
		s.State = StateOrient
	} else {
		s.State = StateDone
	}
}

// --- Generate Planning Queue Phase ---

func advanceGeneratePlanningQueue(s *ForgeState, in AdvanceInput, dir string) error {
	switch s.State {
	case StateOrient:
		s.State = StateRefine

	case StateRefine:
		// Validate the plan-queue.json file.
		planQueueFile := s.GeneratePlanningQueue.PlanQueueFile
		if dir != "" {
			planQueueFile = filepath.Join(dir, planQueueFile)
		}
		data, err := os.ReadFile(planQueueFile)
		if err != nil {
			return fmt.Errorf("reading plan queue %q: %w", planQueueFile, err)
		}
		validationErrs := ValidatePlanQueue(data)
		if len(validationErrs) > 0 {
			return &ValidationError{Errors: validationErrs}
		}
		s.State = StatePhaseShift
		s.PhaseShift = &PhaseShiftInfo{From: PhaseGeneratePlanningQueue, To: PhasePlanning}

	default:
		return fmt.Errorf("cannot advance from state %q in generate_planning_queue phase", s.State)
	}
	return nil
}

// populatePlanningFromQueue pulls the first entry from the planning queue into CurrentPlan.
func populatePlanningFromQueue(s *ForgeState) {
	if len(s.Planning.Queue) > 0 {
		entry := s.Planning.Queue[0]
		s.Planning.Queue = s.Planning.Queue[1:]
		s.Planning.CurrentPlan = &ActivePlan{
			ID:              1,
			Name:            entry.Name,
			Domain:          entry.Domain,
			File:            entry.File,
			Specs:           entry.Specs,
			SpecCommits:     entry.SpecCommits,
			CodeSearchRoots: entry.CodeSearchRoots,
			Kind:            entry.Kind,
		}
	}
}

// --- Planning Phase ---

func advancePlanning(s *ForgeState, in AdvanceInput, dir string) error {
	switch s.State {
	case StateOrient:
		s.State = StateStudySpecs

	case StateStudySpecs:
		s.State = StateStudyCode

	case StateStudyCode:
		s.State = StateStudyPackages

	case StateStudyPackages:
		s.State = StateReview

	case StateReview:
		s.State = StateDraft

	case StateDraft:
		return advancePlanningFromDraftOrRefine(s, dir)

	case StateValidate:
		return advancePlanningFromValidate(s, dir)

	case StateEvaluate:
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in EVALUATE state")
		}
		// --eval-report is required only in report mode; the ignore warning for
		// non-report modes is emitted once at the cmd layer.
		if EvalModeFor(s.Config.Planning.Eval, s.Config.General) == "report" && in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in EVALUATE state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if in.EvalReport != "" {
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}

		eval := EvalRecord{
			Round:      s.Planning.Round,
			Verdict:    in.Verdict,
			EvalReport: in.EvalReport,
		}
		s.Planning.Evals = append(s.Planning.Evals, eval)

		minRounds := s.Config.Planning.Eval.MinRounds
		maxRounds := s.Config.Planning.Eval.MaxRounds

		if in.Verdict == "PASS" {
			if s.Planning.Round >= minRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		} else {
			if s.Planning.Round >= maxRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		}

	case StateRefine:
		s.Planning.Round++
		return advancePlanningFromDraftOrRefine(s, dir)

	case StateSelfReview:
		return advancePlanningFromSelfReview(s, dir)

	case StateAccept:
		if s.Config.General.EnableCommits && in.Message == "" {
			return fmt.Errorf("--message is required in planning ACCEPT state when enable_commits is true")
		}
		if s.Config.General.EnableCommits {
			strategy := effectivePlanStrategy(s)
			stageTargets := planScopeTargets(s, strategy)
			if _, err := AutoCommit(dir, strategy, stageTargets, in.Message); err != nil {
				return fmt.Errorf("Error: STOP there was a failure with auto committing in forgectl, please tell the user: %s", err)
			}
		}
		// Add current plan to completed.
		if s.Planning.CurrentPlan != nil {
			s.Planning.Completed = append(s.Planning.Completed, CompletedPlan{
				ID:     s.Planning.CurrentPlan.ID,
				Name:   s.Planning.CurrentPlan.Name,
				Domain: s.Planning.CurrentPlan.Domain,
				File:   s.Planning.CurrentPlan.File,
				Kind:   s.Planning.CurrentPlan.Kind,
			})
		}
		if s.Config.Planning.PlanAllBeforeImplementing && len(s.Planning.Queue) > 0 {
			// More plans to plan first: pop next from queue.
			entry := s.Planning.Queue[0]
			s.Planning.Queue = s.Planning.Queue[1:]
			s.Planning.Round = 0
			s.Planning.Evals = nil
			s.Planning.CurrentPlan = &ActivePlan{
				ID:              s.Planning.CurrentPlan.ID + 1,
				Name:            entry.Name,
				Domain:          entry.Domain,
				File:            entry.File,
				Specs:           entry.Specs,
				SpecCommits:     entry.SpecCommits,
				CodeSearchRoots: entry.CodeSearchRoots,
				Kind:            entry.Kind,
			}
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhasePlanning}
		} else if s.Config.Planning.PlanAllBeforeImplementing {
			// Queue empty — all plans accepted; enter DONE to build the implementing queue.
			s.State = StateDone
		} else {
			// Interleaved: shift immediately to this plan's implementation phase.
			to := PhaseImplementing
			if s.Planning.CurrentPlan != nil && s.Planning.CurrentPlan.Kind == "ui" {
				to = PhaseUIImplementing
			}
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: to}
		}

	case StateDone:
		if in.Verdict != "" || in.EvalReport != "" || in.Message != "" {
			return fmt.Errorf("DONE is a pass-through state. No flags accepted.")
		}
		// plan_all_before_implementing: set the first completed plan as current and
		// transition to the Planning → Implementing PHASE_SHIFT, which will initialize
		// plan.json, route by kind, and populate the implementing queue from remaining
		// completed plans.
		if len(s.Planning.Completed) > 0 {
			first := s.Planning.Completed[0]
			to := PhaseImplementing
			if first.Kind == "ui" {
				to = PhaseUIImplementing
			}
			s.Planning.CurrentPlan = &ActivePlan{
				ID:     first.ID,
				Name:   first.Name,
				Domain: first.Domain,
				File:   first.File,
				Kind:   first.Kind,
			}
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: to}
		} else {
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhaseImplementing}
		}

	default:
		return fmt.Errorf("cannot advance from state %q in planning phase", s.State)
	}

	return nil
}

func advancePlanningFromDraftOrRefine(s *ForgeState, dir string) error {
	fromDraft := s.State == StateDraft

	planPath := s.Planning.CurrentPlan.File
	fullPath := filepath.Join(dir, planPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		s.State = StateValidate
		if fromDraft {
			s.Planning.Round = 1
		}
		return fmt.Errorf("cannot read plan file %q: %w", planPath, err)
	}

	baseDir := filepath.Dir(fullPath)
	validationErrs := ValidatePlanJSON(data, baseDir)
	if len(validationErrs) > 0 {
		s.State = StateValidate
		if fromDraft {
			s.Planning.Round = 1
		}
		return &ValidationError{Errors: validationErrs}
	}

	if fromDraft {
		s.Planning.Round = 1
	}
	if fromDraft && s.Config.Planning.SelfReview {
		s.State = StateSelfReview
	} else {
		s.State = StateEvaluate
	}
	return nil
}

func advancePlanningFromValidate(s *ForgeState, dir string) error {
	planPath := s.Planning.CurrentPlan.File
	fullPath := filepath.Join(dir, planPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("cannot read plan file %q: %w", planPath, err)
	}

	baseDir := filepath.Dir(fullPath)
	validationErrs := ValidatePlanJSON(data, baseDir)
	if len(validationErrs) > 0 {
		return &ValidationError{Errors: validationErrs}
	}

	if s.Config.Planning.SelfReview {
		s.State = StateSelfReview
	} else {
		s.State = StateEvaluate
	}
	return nil
}

func advancePlanningFromSelfReview(s *ForgeState, dir string) error {
	planPath := s.Planning.CurrentPlan.File
	fullPath := filepath.Join(dir, planPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("cannot read plan file %q: %w", planPath, err)
	}

	baseDir := filepath.Dir(fullPath)
	validationErrs := ValidatePlanJSON(data, baseDir)
	if len(validationErrs) > 0 {
		s.State = StateValidate
		return &ValidationError{Errors: validationErrs}
	}

	s.State = StateEvaluate
	return nil
}

// --- Implementing Phase ---

func advanceImplementing(s *ForgeState, in AdvanceInput, dir string) error {
	impl := s.Implementing

	// Derive CurrentPlanDomain when missing — handles state files created before
	// the field was added to the phase-shift path. Persists via the normal Save
	// call at the end of the advance loop.
	if impl.CurrentPlanDomain == "" {
		if s.Planning != nil && s.Planning.CurrentPlan != nil && s.Planning.CurrentPlan.Domain != "" {
			impl.CurrentPlanDomain = s.Planning.CurrentPlan.Domain
		} else if impl.CurrentPlanFile != "" {
			parts := strings.SplitN(impl.CurrentPlanFile, "/", 3)
			if len(parts) >= 2 {
				impl.CurrentPlanDomain = filepath.Join(parts[0], parts[1])
			}
		}
	}

	switch s.State {
	case StateOrient:
		return advanceImplFromOrient(s, dir)

	case StateImplement:
		return advanceImplFromImplement(s, in, dir)

	case StateEvaluate:
		return advanceImplFromEvaluate(s, in, dir)

	case StateCommit:
		if s.Config.General.EnableCommits && in.Message == "" {
			return fmt.Errorf("--message is required in COMMIT state when enable_commits is true")
		}
		if s.Config.General.EnableCommits {
			strategy := effectiveImplStrategy(s)
			stageTargets := implScopeTargets(impl, nil, strategy)
			if _, err := AutoCommit(dir, strategy, stageTargets, in.Message); err != nil {
				return fmt.Errorf("Error: STOP there was a failure with auto committing in forgectl, please tell the user: %s", err)
			}
		}
		// Archive batch to history.
		archiveBatch(s)

		// Check if all layers complete.
		plan, err := loadPlan(s, dir)
		if err != nil {
			return err
		}
		if allLayersComplete(plan) {
			s.State = StateDone
		} else {
			s.State = StateOrient
		}

	case StateDone:
		// Check for remaining plans.
		if s.Planning != nil && len(s.Planning.Queue) > 0 {
			// Interleaved mode: return to planning for next plan.
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhaseImplementing, To: PhasePlanning}
		} else if impl != nil && len(impl.PlanQueue) > 0 {
			// All-first mode: implement next plan from queue.
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhaseImplementing, To: PhaseImplementing}
		} else {
			return fmt.Errorf("session complete.")
		}

	default:
		return fmt.Errorf("cannot advance from state %q in implementing phase", s.State)
	}

	return nil
}

func advanceImplFromOrient(s *ForgeState, dir string) error {
	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}

	impl := s.Implementing

	// Find current layer or advance to next.
	for _, layer := range plan.Layers {
		if impl.CurrentLayer != nil && impl.CurrentLayer.ID == layer.ID {
			// Check if all items in this layer are terminal.
			if allLayerItemsTerminal(plan, layer) {
				continue
			}
		}

		// Check if all prior layers are complete.
		allPriorComplete := true
		for _, priorLayer := range plan.Layers {
			if priorLayer.ID == layer.ID {
				break
			}
			if !allLayerItemsTerminal(plan, priorLayer) {
				allPriorComplete = false
				break
			}
		}
		if !allPriorComplete {
			continue
		}

		// Check for unblocked items.
		batch := selectBatch(plan, layer, s.Config.Implementing.Batch)
		if len(batch) == 0 {
			continue
		}

		impl.CurrentLayer = &LayerRef{ID: layer.ID, Name: layer.Name}
		impl.BatchNumber++
		impl.CurrentBatch = &BatchState{
			Items:            batch,
			CurrentItemIndex: 0,
			EvalRound:        0,
		}
		s.State = StateImplement
		return nil
	}

	// All layers complete.
	s.State = StateDone
	return nil
}

func advanceImplFromImplement(s *ForgeState, in AdvanceInput, dir string) error {
	impl := s.Implementing
	batch := impl.CurrentBatch

	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}

	// First round requires --message when enable_commits is true.
	if batch.EvalRound == 0 && s.Config.General.EnableCommits && in.Message == "" {
		return fmt.Errorf("--message is required for first-round implementation when enable_commits is true")
	}

	// Mark current item as done — saved after commit succeeds to keep
	// plan.json and state consistent on commit failure.
	itemID := batch.Items[batch.CurrentItemIndex]
	setItemPasses(plan, itemID, "done")

	// First-round auto-commit: one commit per item for crash safety.
	// Save plan only after commit succeeds so plan.json stays consistent with
	// the state file if the commit fails.
	if batch.EvalRound == 0 && s.Config.General.EnableCommits {
		strategy := effectiveImplStrategy(s)
		item := findItem(plan, itemID)
		stageTargets := implScopeTargets(impl, item, strategy)
		if _, err := AutoCommit(dir, strategy, stageTargets, in.Message); err != nil {
			return fmt.Errorf("Error: STOP there was a failure with auto committing in forgectl, please tell the user: %s", err)
		}
	}

	if err := savePlan(s, dir, plan); err != nil {
		return err
	}

	if batch.CurrentItemIndex < len(batch.Items)-1 {
		// More items in batch.
		batch.CurrentItemIndex++
		s.State = StateImplement
	} else {
		// Last item — increment rounds on all batch items.
		for _, id := range batch.Items {
			incrementItemRounds(plan, id)
		}
		if err := savePlan(s, dir, plan); err != nil {
			return err
		}
		batch.EvalRound++
		s.State = StateEvaluate
	}

	return nil
}

func advanceImplFromEvaluate(s *ForgeState, in AdvanceInput, dir string) error {
	if in.Verdict == "" {
		return fmt.Errorf("--verdict is required in EVALUATE state")
	}
	// --eval-report is required only in report mode; the ignore warning for
	// non-report modes is emitted once at the cmd layer.
	if EvalModeFor(s.Config.Implementing.Eval, s.Config.General) == "report" && in.EvalReport == "" {
		return fmt.Errorf("--eval-report is required in EVALUATE state")
	}
	if in.Verdict != "PASS" && in.Verdict != "FAIL" {
		return fmt.Errorf("--verdict must be PASS or FAIL")
	}
	if in.EvalReport != "" {
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
		}
	}

	impl := s.Implementing
	batch := impl.CurrentBatch

	eval := EvalRecord{
		Round:      batch.EvalRound,
		Verdict:    in.Verdict,
		EvalReport: in.EvalReport,
	}
	batch.Evals = append(batch.Evals, eval)

	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}

	minRounds := s.Config.Implementing.Eval.MinRounds
	maxRounds := s.Config.Implementing.Eval.MaxRounds

	if in.Verdict == "PASS" {
		if batch.EvalRound >= minRounds {
			// Mark items passed.
			for _, id := range batch.Items {
				setItemPasses(plan, id, "passed")
			}
			if err := savePlan(s, dir, plan); err != nil {
				return err
			}
			s.State = StateCommit
		} else {
			// Min rounds not met — re-implement.
			batch.CurrentItemIndex = 0
			s.State = StateImplement
		}
	} else {
		if batch.EvalRound >= maxRounds {
			// Force accept — mark items failed.
			for _, id := range batch.Items {
				setItemPasses(plan, id, "failed")
			}
			if err := savePlan(s, dir, plan); err != nil {
				return err
			}
			s.State = StateCommit
		} else {
			// Re-implement.
			batch.CurrentItemIndex = 0
			s.State = StateImplement
		}
	}

	return nil
}

// --- UI Implementing Phase ---
//
// The ui_implementing phase runs each batch through three sequential loops —
// code eval, QA, then e2e — before COMMIT. Each loop carries its own round
// counter (EvalRound/QARound/E2ERound) and history. A loop's counter is
// incremented when the batch ENTERS that loop's evaluator state (code:
// IMPLEMENT→EVALUATE; qa: →QA_TEST; e2e: →E2E_VERIFY), so each evaluator state
// records its verdict against the counter directly and the output renders the
// counter without an offset.

func advanceUIImplementing(s *ForgeState, in AdvanceInput, dir string) error {
	switch s.State {
	case StateOrient:
		return advanceUIFromOrient(s, dir)
	case StateImplement:
		return advanceUIFromImplement(s, in, dir)
	case StateEvaluate:
		return advanceUIFromEvaluate(s, in, dir)
	case StateQATest:
		return advanceUIFromQATest(s, in, dir)
	case StateUIRefine:
		return advanceUIFromUIRefine(s)
	case StateE2EAuthor:
		return advanceUIFromE2EAuthor(s)
	case StateE2EVerify:
		return advanceUIFromE2EVerify(s, in, dir)
	case StateE2ERemediate:
		return advanceUIFromE2ERemediate(s)
	case StateCommit:
		return advanceUIFromCommit(s, in, dir)
	case StateDone:
		return advanceUIFromDone(s)
	default:
		return fmt.Errorf("cannot advance from state %q in ui_implementing phase", s.State)
	}
}

func advanceUIFromOrient(s *ForgeState, dir string) error {
	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}
	ui := s.UIImplementing

	for _, layer := range plan.Layers {
		if ui.CurrentLayer != nil && ui.CurrentLayer.ID == layer.ID {
			if allLayerItemsTerminal(plan, layer) {
				continue
			}
		}
		// All prior layers must be complete.
		allPriorComplete := true
		for _, priorLayer := range plan.Layers {
			if priorLayer.ID == layer.ID {
				break
			}
			if !allLayerItemsTerminal(plan, priorLayer) {
				allPriorComplete = false
				break
			}
		}
		if !allPriorComplete {
			continue
		}
		batch := selectBatch(plan, layer, s.Config.UIImplementing.Batch)
		if len(batch) == 0 {
			continue
		}
		ui.CurrentLayer = &LayerRef{ID: layer.ID, Name: layer.Name}
		ui.BatchNumber++
		// Reset all three loop counters for the new batch.
		ui.CurrentBatch = &UIBatchState{
			Items:            batch,
			CurrentItemIndex: 0,
			EvalRound:        0,
			QARound:          0,
			E2ERound:         0,
		}
		s.State = StateImplement
		return nil
	}

	s.State = StateDone
	return nil
}

func advanceUIFromImplement(s *ForgeState, in AdvanceInput, dir string) error {
	ui := s.UIImplementing
	batch := ui.CurrentBatch

	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}

	// First round requires --message when enable_commits is true (EvalRound is
	// still 0 until the batch's first code evaluation).
	if batch.EvalRound == 0 && s.Config.General.EnableCommits && in.Message == "" {
		return fmt.Errorf("--message is required for first-round implementation when enable_commits is true")
	}

	itemID := batch.Items[batch.CurrentItemIndex]
	setItemPasses(plan, itemID, "done")

	// First-round auto-commit: one commit per item for crash safety.
	if batch.EvalRound == 0 && s.Config.General.EnableCommits {
		strategy := effectiveUIStrategy(s)
		item := findItem(plan, itemID)
		stageTargets := uiScopeTargets(ui, item, strategy)
		if _, err := AutoCommit(dir, strategy, stageTargets, in.Message); err != nil {
			return fmt.Errorf("Error: STOP there was a failure with auto committing in forgectl, please tell the user: %s", err)
		}
	}

	if err := savePlan(s, dir, plan); err != nil {
		return err
	}

	if batch.CurrentItemIndex < len(batch.Items)-1 {
		batch.CurrentItemIndex++
		s.State = StateImplement
		return nil
	}

	// Last item — increment item rounds and enter the code EVALUATE loop.
	for _, id := range batch.Items {
		incrementItemRounds(plan, id)
	}
	if err := savePlan(s, dir, plan); err != nil {
		return err
	}
	batch.EvalRound++ // entering the code-eval loop
	s.State = StateEvaluate
	return nil
}

func advanceUIFromEvaluate(s *ForgeState, in AdvanceInput, dir string) error {
	cfg := s.Config.UIImplementing
	if err := requireVerdict(in, EvalModeFor(cfg.Eval, s.Config.General)); err != nil {
		return err
	}

	ui := s.UIImplementing
	batch := ui.CurrentBatch

	// EvalRound was incremented on entry to EVALUATE; record against it directly.
	batch.Evals = append(batch.Evals, EvalRecord{
		Round:      batch.EvalRound,
		Verdict:    in.Verdict,
		EvalReport: in.EvalReport,
	})

	toQA := (in.Verdict == "PASS" && batch.EvalRound >= cfg.Eval.MinRounds) ||
		(in.Verdict == "FAIL" && batch.EvalRound >= cfg.Eval.MaxRounds)
	if toQA {
		if in.Verdict == "FAIL" {
			batch.CodeForceAccepted = true
		}
		batch.QARound++ // entering the QA loop
		s.State = StateQATest
		return nil
	}
	// Below min (PASS) or below max (FAIL) — re-implement the batch.
	batch.CurrentItemIndex = 0
	s.State = StateImplement
	return nil
}

func advanceUIFromQATest(s *ForgeState, in AdvanceInput, dir string) error {
	cfg := s.Config.UIImplementing
	if err := requireVerdict(in, EvalModeFor(cfg.QA, s.Config.General)); err != nil {
		return err
	}

	ui := s.UIImplementing
	batch := ui.CurrentBatch

	// QARound was incremented on entry to QA_TEST; record against it directly.
	// Record only after the step-list invariant is satisfied for the exit paths.
	toE2E := (in.Verdict == "PASS" && batch.QARound >= cfg.QA.MinRounds) ||
		(in.Verdict == "FAIL" && batch.QARound >= cfg.QA.MaxRounds)

	if toE2E {
		// Invariant 6: the QA loop must produce the e2e step list on every exit
		// toward E2E_AUTHOR. Confirm presence (and count scenarios) before exiting.
		count, exists := countQAScenarios(s, dir, ui.BatchNumber)
		if !exists {
			return fmt.Errorf("QA step list %q does not exist; the QA evaluator must write it before advancing toward E2E_AUTHOR", qaStepListPath(s, ui.BatchNumber))
		}
		if count == 0 {
			fmt.Fprintf(os.Stderr, "WARN: QA step list %q has 0 scenarios; the e2e loop will pass vacuously.\n", qaStepListPath(s, ui.BatchNumber))
		}
		batch.QAEvals = append(batch.QAEvals, EvalRecord{
			Round:      batch.QARound,
			Verdict:    in.Verdict,
			EvalReport: in.EvalReport,
		})
		if in.Verdict == "FAIL" {
			batch.QAForceAccepted = true
		}
		s.State = StateE2EAuthor
		return nil
	}

	// Below min (PASS) or below max (FAIL) — iterate on the UI.
	batch.QAEvals = append(batch.QAEvals, EvalRecord{
		Round:      batch.QARound,
		Verdict:    in.Verdict,
		EvalReport: in.EvalReport,
	})
	s.State = StateUIRefine
	return nil
}

func advanceUIFromUIRefine(s *ForgeState) error {
	batch := s.UIImplementing.CurrentBatch
	batch.QARound++ // re-entering the QA loop
	s.State = StateQATest
	return nil
}

func advanceUIFromE2EAuthor(s *ForgeState) error {
	// Bridging state: no verdict. Always enter the e2e verification loop. A
	// zero-scenario step list still advances (it passes vacuously in E2E_VERIFY).
	batch := s.UIImplementing.CurrentBatch
	batch.E2ERound++ // entering the e2e loop
	s.State = StateE2EVerify
	return nil
}

func advanceUIFromE2EVerify(s *ForgeState, in AdvanceInput, dir string) error {
	cfg := s.Config.UIImplementing
	if err := requireVerdict(in, EvalModeFor(cfg.E2E.EvalConfig, s.Config.General)); err != nil {
		return err
	}

	batch := s.UIImplementing.CurrentBatch

	// E2ERound was incremented on entry to E2E_VERIFY; record against it directly.
	batch.E2EEvals = append(batch.E2EEvals, EvalRecord{
		Round:      batch.E2ERound,
		Verdict:    in.Verdict,
		EvalReport: in.EvalReport,
	})

	toCommit := (in.Verdict == "PASS" && batch.E2ERound >= cfg.E2E.MinRounds) ||
		(in.Verdict == "FAIL" && batch.E2ERound >= cfg.E2E.MaxRounds)
	if toCommit {
		if in.Verdict == "FAIL" {
			batch.E2EForceAccepted = true
		}
		s.State = StateCommit
		return nil
	}
	// Below min (PASS) or below max (FAIL) — remediate and re-verify.
	s.State = StateE2ERemediate
	return nil
}

func advanceUIFromE2ERemediate(s *ForgeState) error {
	batch := s.UIImplementing.CurrentBatch
	batch.E2ERound++ // re-entering the e2e loop
	s.State = StateE2EVerify
	return nil
}

func advanceUIFromCommit(s *ForgeState, in AdvanceInput, dir string) error {
	if s.Config.General.EnableCommits && in.Message == "" {
		return fmt.Errorf("--message is required in COMMIT state when enable_commits is true")
	}

	// Batch commit: stage per ui_implementing.commit_strategy and commit.
	if s.Config.General.EnableCommits {
		strategy := effectiveUIStrategy(s)
		stageTargets := uiScopeTargets(s.UIImplementing, nil, strategy)
		if _, err := AutoCommit(dir, strategy, stageTargets, in.Message); err != nil {
			return fmt.Errorf("Error: STOP there was a failure with auto committing in forgectl, please tell the user: %s", err)
		}
	}

	ui := s.UIImplementing
	batch := ui.CurrentBatch

	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}

	// Items are marked terminal here, after all three loops have run: passed
	// when every loop stayed within budget, failed when any loop force-accepted.
	forced := batch.CodeForceAccepted || batch.QAForceAccepted || batch.E2EForceAccepted
	status := "passed"
	if forced {
		status = "failed"
	}
	for _, id := range batch.Items {
		setItemPasses(plan, id, status)
	}
	if err := savePlan(s, dir, plan); err != nil {
		return err
	}

	archiveUIBatch(s)

	if allLayersComplete(plan) {
		s.State = StateDone
	} else {
		s.State = StateOrient
	}
	return nil
}

func advanceUIFromDone(s *ForgeState) error {
	ui := s.UIImplementing
	if s.Planning != nil && len(s.Planning.Queue) > 0 {
		// Interleaved mode: return to planning for the next domain.
		s.State = StatePhaseShift
		s.PhaseShift = &PhaseShiftInfo{From: PhaseUIImplementing, To: PhasePlanning}
	} else if ui != nil && len(ui.PlanQueue) > 0 {
		// All-first mode: implement the next plan from the queue (domain boundary).
		s.State = StatePhaseShift
		s.PhaseShift = &PhaseShiftInfo{From: PhaseUIImplementing, To: PhaseImplementing}
	} else {
		return fmt.Errorf("session complete.")
	}
	return nil
}

// requireVerdict validates the shared --verdict / --eval-report rules for the
// three ui_implementing evaluator states. --eval-report is required only in
// report mode.
func requireVerdict(in AdvanceInput, mode string) error {
	if in.Verdict == "" {
		return fmt.Errorf("--verdict is required in this state")
	}
	if in.Verdict != "PASS" && in.Verdict != "FAIL" {
		return fmt.Errorf("--verdict must be PASS or FAIL")
	}
	if mode == "report" && in.EvalReport == "" {
		return fmt.Errorf("--eval-report is required in report mode")
	}
	if in.EvalReport != "" {
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
		}
	}
	return nil
}

// archiveUIBatch records the current batch's three loop counters and histories
// into UILayerHistory and clears the current batch.
func archiveUIBatch(s *ForgeState) {
	ui := s.UIImplementing
	batch := ui.CurrentBatch

	history := UIBatchHistory{
		BatchNumber: ui.BatchNumber,
		Items:       batch.Items,
		EvalRounds:  batch.EvalRound,
		QARounds:    batch.QARound,
		E2ERounds:   batch.E2ERound,
		Evals:       batch.Evals,
		QAEvals:     batch.QAEvals,
		E2EEvals:    batch.E2EEvals,
	}

	found := false
	for i := range ui.LayerHistory {
		if ui.LayerHistory[i].LayerID == ui.CurrentLayer.ID {
			ui.LayerHistory[i].Batches = append(ui.LayerHistory[i].Batches, history)
			found = true
			break
		}
	}
	if !found {
		ui.LayerHistory = append(ui.LayerHistory, UILayerHistory{
			LayerID: ui.CurrentLayer.ID,
			Batches: []UIBatchHistory{history},
		})
	}

	ui.CurrentBatch = nil
}

// ValidateUIConfigKeys returns one error per required ui_implementing config key
// that is empty. These keys are validated at the phase boundary (init / phase
// shift), not in ValidateConfig (which runs for every phase).
func ValidateUIConfigKeys(cfg UIImplementingConfig) []string {
	var errs []string
	if cfg.App.LaunchCommand == "" {
		errs = append(errs, "ui_implementing.app.launch_command is required but empty")
	}
	if cfg.App.URL == "" {
		errs = append(errs, "ui_implementing.app.url is required but empty")
	}
	if cfg.E2E.TestCommand == "" {
		errs = append(errs, "ui_implementing.e2e.test_command is required but empty")
	}
	if cfg.E2E.TestDir == "" {
		errs = append(errs, "ui_implementing.e2e.test_dir is required but empty")
	}
	return errs
}

// routeImplementationDomainBoundary sets up the implementation phase for the next
// plan at an all-first domain boundary, routing by the entry's kind: "ui" enters
// ui_implementing (validating the required UI config keys), "code"/absent enters
// implementing. The remaining queue is carried into the destination phase's state.
func routeImplementationDomainBoundary(s *ForgeState, entry PlanQueueEntry, remaining []PlanQueueEntry) error {
	if entry.Kind == "ui" {
		if keyErrs := ValidateUIConfigKeys(s.Config.UIImplementing); len(keyErrs) > 0 {
			return &ValidationError{Errors: keyErrs}
		}
		ui := NewUIImplementingState()
		ui.CurrentPlanFile = entry.File
		ui.CurrentPlanDomain = entry.Domain
		ui.PlanQueue = remaining
		s.UIImplementing = ui
		s.Implementing = nil
		s.Phase = PhaseUIImplementing
	} else {
		impl := NewImplementingState()
		impl.CurrentPlanFile = entry.File
		impl.PlanQueue = remaining
		s.Implementing = impl
		s.UIImplementing = nil
		s.Phase = PhaseImplementing
	}
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		s.Planning.CurrentPlan = &ActivePlan{
			ID:              s.Planning.CurrentPlan.ID + 1,
			Name:            entry.Name,
			Domain:          entry.Domain,
			File:            entry.File,
			Specs:           entry.Specs,
			SpecCommits:     entry.SpecCommits,
			CodeSearchRoots: entry.CodeSearchRoots,
			Kind:            entry.Kind,
		}
	}
	return nil
}

// --- Phase Shift ---

func advancePhaseShift(s *ForgeState, in AdvanceInput, dir string) error {
	if s.PhaseShift == nil {
		return fmt.Errorf("no phase shift info")
	}

	switch {
	case s.PhaseShift.From == PhaseSpecifying && s.PhaseShift.To == PhaseGeneratePlanningQueue:
		if in.From != "" {
			// --from provided: skip generate_planning_queue phase, go directly to planning.
			data, err := os.ReadFile(in.From)
			if err != nil {
				return fmt.Errorf("reading plan queue: %w", err)
			}
			validationErrs := ValidatePlanQueue(data)
			if len(validationErrs) > 0 {
				return &ValidationError{Errors: validationErrs}
			}
			var input PlanQueueInput
			if err := json.Unmarshal(data, &input); err != nil {
				return fmt.Errorf("parsing plan queue: %w", err)
			}
			s.Planning = NewPlanningState(input.Plans)
			populatePlanningFromQueue(s)
			s.Phase = PhasePlanning
			s.State = StateOrient
			s.PhaseShift = nil
		} else {
			// Auto-generate from completed specs, write to file, enter generate_planning_queue.
			relPath, err := autoGeneratePlanQueue(s, dir)
			if err != nil {
				return fmt.Errorf("auto-generating plan queue: %w", err)
			}
			s.GeneratePlanningQueue = &GeneratePlanningQueueState{PlanQueueFile: relPath}
			s.Phase = PhaseGeneratePlanningQueue
			s.State = StateOrient
			s.PhaseShift = nil
		}

	case s.PhaseShift.From == PhaseGeneratePlanningQueue && s.PhaseShift.To == PhasePlanning:
		planQueueFile := s.GeneratePlanningQueue.PlanQueueFile
		if dir != "" {
			planQueueFile = filepath.Join(dir, planQueueFile)
		}
		data, err := os.ReadFile(planQueueFile)
		if err != nil {
			return fmt.Errorf("reading plan queue: %w", err)
		}

		var input PlanQueueInput
		if inFile := in.From; inFile != "" {
			// Override with provided file.
			data, err = os.ReadFile(inFile)
			if err != nil {
				return fmt.Errorf("reading plan queue override: %w", err)
			}
		}
		validationErrs := ValidatePlanQueue(data)
		if len(validationErrs) > 0 {
			return &ValidationError{Errors: validationErrs}
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("parsing plan queue: %w", err)
		}
		s.Planning = NewPlanningState(input.Plans)
		populatePlanningFromQueue(s)
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhasePlanning &&
		(s.PhaseShift.To == PhaseImplementing || s.PhaseShift.To == PhaseUIImplementing):
		planPath := s.Planning.CurrentPlan.File
		fullPath := filepath.Join(dir, planPath)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("reading plan.json: %w", err)
		}

		baseDir := filepath.Dir(fullPath)
		validationErrs := ValidatePlanJSON(data, baseDir)
		if len(validationErrs) > 0 {
			return &ValidationError{Errors: validationErrs}
		}

		var plan PlanJSON
		if err := json.Unmarshal(data, &plan); err != nil {
			return fmt.Errorf("parsing plan.json: %w", err)
		}

		// Add passes and rounds to items.
		for i := range plan.Items {
			plan.Items[i].Passes = "pending"
			plan.Items[i].Rounds = 0
		}

		// Write updated plan.
		planData, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling plan: %w", err)
		}
		if err := os.WriteFile(fullPath, planData, 0644); err != nil {
			return fmt.Errorf("writing plan: %w", err)
		}

		// In plan_all_before_implementing mode the DONE handler set CurrentPlan to
		// s.Planning.Completed[0] and all plans are in Completed. Build the
		// implementing queue from the remaining completed plans so subsequent domain
		// boundaries chain correctly. In interleaved mode Completed holds only the
		// plans already implemented and the queue must stay empty.
		var implQueue []PlanQueueEntry
		if s.Config.Planning.PlanAllBeforeImplementing && len(s.Planning.Completed) > 1 {
			for _, cp := range s.Planning.Completed[1:] {
				implQueue = append(implQueue, PlanQueueEntry{
					Name:   cp.Name,
					Domain: cp.Domain,
					File:   cp.File,
					Kind:   cp.Kind,
				})
			}
		}

		// Route by the active plan's kind: "ui" → ui_implementing, else implementing.
		if s.Planning.CurrentPlan.Kind == "ui" {
			if keyErrs := ValidateUIConfigKeys(s.Config.UIImplementing); len(keyErrs) > 0 {
				// Remain at PHASE_SHIFT so the operator can fix the config.
				return &ValidationError{Errors: keyErrs}
			}
			s.UIImplementing = NewUIImplementingState()
			s.UIImplementing.CurrentPlanFile = planPath
			s.UIImplementing.CurrentPlanDomain = s.Planning.CurrentPlan.Domain
			s.UIImplementing.PlanQueue = implQueue
			s.Implementing = nil
			s.Phase = PhaseUIImplementing
		} else {
			s.Implementing = NewImplementingState()
			s.Implementing.CurrentPlanFile = planPath
			s.Implementing.PlanQueue = implQueue
			s.UIImplementing = nil
			s.Phase = PhaseImplementing
		}
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhasePlanning && s.PhaseShift.To == PhasePlanning:
		// PlanAllBeforeImplementing: advance to next plan in queue.
		s.Planning.Round = 0
		s.Planning.Evals = nil
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhaseImplementing && s.PhaseShift.To == PhaseImplementing:
		// All-first domain boundary: pop next plan from Implementing.PlanQueue and
		// route by its kind (code → implementing, ui → ui_implementing).
		if len(s.Implementing.PlanQueue) > 0 {
			entry := s.Implementing.PlanQueue[0]
			remaining := s.Implementing.PlanQueue[1:]
			if err := routeImplementationDomainBoundary(s, entry, remaining); err != nil {
				return err
			}
			// Initialize plan.json for the incoming domain (add passes/rounds).
			if err := initPlanForImplementing(s, dir); err != nil {
				return err
			}
		} else {
			s.Phase = PhaseImplementing
		}
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhaseUIImplementing && s.PhaseShift.To == PhaseImplementing:
		// All-first domain boundary from a ui plan: pop next plan from
		// UIImplementing.PlanQueue and route by its kind.
		if s.UIImplementing != nil && len(s.UIImplementing.PlanQueue) > 0 {
			entry := s.UIImplementing.PlanQueue[0]
			remaining := s.UIImplementing.PlanQueue[1:]
			if err := routeImplementationDomainBoundary(s, entry, remaining); err != nil {
				return err
			}
			// Initialize plan.json for the incoming domain (add passes/rounds).
			if err := initPlanForImplementing(s, dir); err != nil {
				return err
			}
		} else {
			s.Phase = PhaseImplementing
		}
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhaseImplementing && s.PhaseShift.To == PhasePlanning,
		s.PhaseShift.From == PhaseUIImplementing && s.PhaseShift.To == PhasePlanning:
		// Interleaved mode: return to planning for the next plan from Planning.Queue.
		if len(s.Planning.Queue) > 0 {
			entry := s.Planning.Queue[0]
			s.Planning.Queue = s.Planning.Queue[1:]
			s.Planning.Round = 0
			s.Planning.Evals = nil
			s.Planning.CurrentPlan = &ActivePlan{
				ID:              s.Planning.CurrentPlan.ID + 1,
				Name:            entry.Name,
				Domain:          entry.Domain,
				File:            entry.File,
				Specs:           entry.Specs,
				SpecCommits:     entry.SpecCommits,
				CodeSearchRoots: entry.CodeSearchRoots,
				Kind:            entry.Kind,
			}
		}
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	default:
		return fmt.Errorf("unknown phase shift: %s → %s", s.PhaseShift.From, s.PhaseShift.To)
	}

	return nil
}

// autoGeneratePlanQueue builds a plan queue from completed specifying-phase specs,
// writes it to disk, and returns the relative path and any error.
// One PlanQueueEntry is produced per domain, preserving domain order of first appearance.
func autoGeneratePlanQueue(s *ForgeState, dir string) (string, error) {
	spec := s.Specifying

	// Group specs by domain, preserving order of first appearance.
	type domainGroup struct {
		specs []CompletedSpec
	}
	groupMap := make(map[string]*domainGroup)
	var domainOrder []string
	for _, cs := range spec.Completed {
		if _, seen := groupMap[cs.Domain]; !seen {
			groupMap[cs.Domain] = &domainGroup{}
			domainOrder = append(domainOrder, cs.Domain)
		}
		groupMap[cs.Domain].specs = append(groupMap[cs.Domain].specs, cs)
	}

	var entries []PlanQueueEntry
	for _, domain := range domainOrder {
		group := groupMap[domain]

		// Collect spec file paths.
		var specFiles []string
		for _, cs := range group.specs {
			specFiles = append(specFiles, cs.File)
		}

		// Determine code search roots from Domains metadata.
		var roots []string
		if spec.Domains != nil {
			if meta, ok := spec.Domains[domain]; ok {
				roots = meta.CodeSearchRoots
			}
		}
		if len(roots) == 0 {
			roots = []string{domain + "/"}
		}

		// Deduplicate commit hashes.
		seen := map[string]bool{}
		var commits []string
		for _, cs := range group.specs {
			for _, h := range cs.CommitHashes {
				if h != "" && !seen[h] {
					seen[h] = true
					commits = append(commits, h)
				}
			}
		}

		// Capitalize first letter of domain for display name.
		displayName := domain
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		}

		entries = append(entries, PlanQueueEntry{
			Name:            displayName + " Implementation Plan",
			Domain:          domain,
			File:            domain + "/.forge_workspace/implementation_plan/plan.json",
			Specs:           specFiles,
			SpecCommits:     commits,
			CodeSearchRoots: roots,
		})
	}

	relPath := filepath.Join(".forgectl", "state", "plan-queue.json")
	absPath := relPath
	if dir != "" {
		absPath = filepath.Join(dir, relPath)
	}
	data, err := json.MarshalIndent(PlanQueueInput{Plans: entries}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling plan queue: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("creating plan queue dir: %w", err)
	}
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing plan queue: %w", err)
	}
	return relPath, nil
}

// --- Helpers ---

func loadPlan(s *ForgeState, dir string) (*PlanJSON, error) {
	planPath := currentPlanFile(s)
	if planPath == "" {
		return nil, fmt.Errorf("no plan file configured")
	}

	fullPath := filepath.Join(dir, planPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("reading plan: %w", err)
	}

	var plan PlanJSON
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parsing plan: %w", err)
	}

	return &plan, nil
}

// currentPlanFile resolves the active plan.json path from whichever phase state
// is configured — implementing, ui_implementing, or (pre-init) the planning
// queue's current plan.
func currentPlanFile(s *ForgeState) string {
	if s.Implementing != nil && s.Implementing.CurrentPlanFile != "" {
		return s.Implementing.CurrentPlanFile
	}
	if s.UIImplementing != nil && s.UIImplementing.CurrentPlanFile != "" {
		return s.UIImplementing.CurrentPlanFile
	}
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		return s.Planning.CurrentPlan.File
	}
	return ""
}

func savePlan(s *ForgeState, dir string, plan *PlanJSON) error {
	planPath := currentPlanFile(s)
	if planPath == "" {
		return fmt.Errorf("no plan file configured")
	}

	fullPath := filepath.Join(dir, planPath)
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plan: %w", err)
	}
	return os.WriteFile(fullPath, data, 0644)
}

func selectBatch(plan *PlanJSON, layer PlanLayerDef, batchSize int) []string {
	var batch []string
	for _, itemID := range layer.Items {
		if len(batch) >= batchSize {
			break
		}
		item := findItem(plan, itemID)
		if item == nil {
			continue
		}
		if item.Passes != "pending" {
			continue
		}
		if !itemUnblocked(plan, item) {
			continue
		}
		batch = append(batch, itemID)
	}
	return batch
}

func itemUnblocked(plan *PlanJSON, item *PlanItem) bool {
	for _, depID := range item.DependsOn {
		dep := findItem(plan, depID)
		if dep == nil {
			continue
		}
		if dep.Passes != "passed" && dep.Passes != "failed" {
			return false
		}
	}
	return true
}

func findItem(plan *PlanJSON, id string) *PlanItem {
	for i := range plan.Items {
		if plan.Items[i].ID == id {
			return &plan.Items[i]
		}
	}
	return nil
}

func setItemPasses(plan *PlanJSON, id string, passes string) {
	for i := range plan.Items {
		if plan.Items[i].ID == id {
			plan.Items[i].Passes = passes
			return
		}
	}
}

func incrementItemRounds(plan *PlanJSON, id string) {
	for i := range plan.Items {
		if plan.Items[i].ID == id {
			plan.Items[i].Rounds++
			return
		}
	}
}

func allLayerItemsTerminal(plan *PlanJSON, layer PlanLayerDef) bool {
	for _, id := range layer.Items {
		item := findItem(plan, id)
		if item == nil {
			continue
		}
		if item.Passes != "passed" && item.Passes != "failed" {
			return false
		}
	}
	return true
}

func allLayersComplete(plan *PlanJSON) bool {
	for _, layer := range plan.Layers {
		if !allLayerItemsTerminal(plan, layer) {
			return false
		}
	}
	return true
}

func archiveBatch(s *ForgeState) {
	impl := s.Implementing
	batch := impl.CurrentBatch

	history := BatchHistory{
		BatchNumber: impl.BatchNumber,
		Items:       batch.Items,
		EvalRounds:  batch.EvalRound,
		Evals:       batch.Evals,
	}

	// Find or create layer history.
	found := false
	for i := range impl.LayerHistory {
		if impl.LayerHistory[i].LayerID == impl.CurrentLayer.ID {
			impl.LayerHistory[i].Batches = append(impl.LayerHistory[i].Batches, history)
			found = true
			break
		}
	}
	if !found {
		impl.LayerHistory = append(impl.LayerHistory, LayerHistory{
			LayerID: impl.CurrentLayer.ID,
			Batches: []BatchHistory{history},
		})
	}

	impl.CurrentBatch = nil
}

func checkEvalReportExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		// Common failure: the eval sub-agent described its findings without
		// writing the report file, so the engineer passed the prose as the
		// --eval-report value. A prose value has whitespace and no path
		// separator; point the engineer back at the file path contract.
		if looksLikeReportProse(path) {
			return fmt.Errorf("eval report %q does not exist — --eval-report expects the file path the eval sub-agent wrote, not the report text", path)
		}
		return fmt.Errorf("eval report %q does not exist", path)
	}
	return nil
}

// looksLikeReportProse reports whether v looks like report text rather than a
// file path: it contains whitespace and no path separator.
func looksLikeReportProse(v string) bool {
	return strings.ContainsAny(v, " \t\n") && !strings.ContainsAny(v, "/\\")
}

// --- Reverse Engineering Phase ---

// reverseEngineeringQueuePath returns the fixed convention path for the reverse
// engineering queue file. forgectl owns this path; it is never user-supplied.
func reverseEngineeringQueuePath(dir string) string {
	return filepath.Join(dir, ".forgectl", "state", "reverse-engineering-queue.json")
}

func advanceReverseEngineering(s *ForgeState, in AdvanceInput, dir string) error {
	re := s.ReverseEngineering
	if re == nil {
		return fmt.Errorf("reverse_engineering state is not initialized")
	}

	switch s.State {
	case StateOrient:
		// Begin the per-domain analysis loop on the first domain.
		re.DomainIndex = 1
		s.State = StateSurvey
		return nil

	case StateSurvey:
		s.State = StateGapAnalysis
		return nil

	case StateGapAnalysis:
		s.State = StateDecompose
		return nil

	case StateDecompose:
		s.State = StateQueue
		return nil

	case StateQueue:
		return advanceREQueue(s, in, dir)

	case StateExecuteReverseEngineer:
		// Defensive: the loop is never entered with an empty queue (QUEUE rejects
		// that), but guard anyway and stay in EXECUTE if it somehow happens.
		if len(re.Queue) == 0 {
			return fmt.Errorf("Queue contains zero entries. Nothing to execute.")
		}
		s.State = StatePostReverseEngineer
		return nil

	case StatePostReverseEngineer:
		// depends_on is RECONCILE metadata only — the execution loop walks the
		// queue purely in stored order via ExecuteItemIndex.
		if re.ExecuteItemIndex < len(re.Queue) {
			re.ExecuteItemIndex++
			s.State = StateExecuteReverseEngineer
			return ensureItemSpecsDir(re, dir)
		}
		// Last item done — begin the per-domain reconcile loop.
		re.DomainIndex = 1
		re.ReconcileRound = 1
		s.State = StateReconcile
		return nil

	case StateReconcile:
		// RECONCILE takes no transition flags. Target spec files the queue
		// expected for this domain that are absent on disk are surfaced as gaps
		// by the action output (see ReverseEngineeringDomainGaps); a gap is
		// reported, never fabricated, and never blocks the loop.
		s.State = StateReconcileEval
		return nil

	case StateReconcileEval:
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in RECONCILE_EVAL state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if in.EvalReport != "" {
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}
		if re.DomainIndex < 1 || re.DomainIndex > len(re.Domains) {
			return fmt.Errorf("reconcile domain index %d out of range", re.DomainIndex)
		}

		// Append the verdict to the current domain's reconcile history. The map
		// is created lazily: it is declared with `omitempty`, so a freshly
		// initialised state serialises without it and loads back as nil.
		domain := re.Domains[re.DomainIndex-1]
		if re.DomainReconcile == nil {
			re.DomainReconcile = map[string]*ReconcileState{}
		}
		rec := re.DomainReconcile[domain]
		if rec == nil {
			rec = &ReconcileState{}
			re.DomainReconcile[domain] = rec
		}
		rec.Round = re.ReconcileRound
		rec.Evals = append(rec.Evals, EvalRecord{
			Round:      re.ReconcileRound,
			Verdict:    in.Verdict,
			EvalReport: in.EvalReport,
		})

		cfg := s.Config.ReverseEngineering.Reconcile
		passed := in.Verdict == "PASS" && re.ReconcileRound >= cfg.MinRounds
		forced := in.Verdict == "FAIL" && re.ReconcileRound >= cfg.MaxRounds
		if passed || forced {
			// Terminal for this domain: gate through COLLEAGUE_REVIEW when
			// enabled, otherwise straight to RECONCILE_ADVANCE.
			if re.ColleagueReview {
				s.State = StateColleagueReview
			} else {
				s.State = StateReconcileAdvance
			}
		} else {
			// Minimum not yet met (PASS) or correctable failure — loop back.
			re.ReconcileRound++
			s.State = StateReconcile
		}
		return nil

	case StateColleagueReview:
		s.State = StateReconcileAdvance
		return nil

	case StateReconcileAdvance:
		if re.DomainIndex < re.DomainCount {
			re.DomainIndex++
			re.ReconcileRound = 1
			s.State = StateReconcile
			return nil
		}
		// All domains reconciled.
		s.State = StateDone
		return nil

	default:
		return fmt.Errorf("unexpected state %q in reverse_engineering phase", s.State)
	}
}

// ReverseEngineeringDomainGaps returns the queue-relative target spec files for
// the current reconcile domain whose files are absent on disk. RECONCILE reports
// these gaps rather than fabricating specs; a missing file never blocks the loop.
// Paths resolve against the absolute project root, not the working directory.
func ReverseEngineeringDomainGaps(re *ReverseEngineeringState, dir string) []string {
	if re.DomainIndex < 1 || re.DomainIndex > len(re.Domains) {
		return nil
	}
	domain := re.Domains[re.DomainIndex-1]
	var gaps []string
	for _, entry := range re.Queue {
		if entry.Domain != domain {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Domain, entry.File)); err != nil {
			gaps = append(gaps, entry.File)
		}
	}
	return gaps
}

// ensureItemSpecsDir creates the current execution item's domain specs directory
// (<project_root>/<domain>/specs/) before its action output is emitted. Paths
// resolve against the absolute project root, never the current working directory.
func ensureItemSpecsDir(re *ReverseEngineeringState, dir string) error {
	if re.ExecuteItemIndex < 1 || re.ExecuteItemIndex > len(re.Queue) {
		return nil
	}
	item := re.Queue[re.ExecuteItemIndex-1]
	specsDir := filepath.Join(dir, item.Domain, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return fmt.Errorf("creating domain specs directory %s: %w", specsDir, err)
	}
	return nil
}

// advanceREQueue handles the QUEUE state: it reads the fixed-path queue file,
// detects changes by content hash, validates schema/domains/paths, parses the
// queue, then transitions to the next domain's SURVEY or into the execution loop.
func advanceREQueue(s *ForgeState, in AdvanceInput, dir string) error {
	re := s.ReverseEngineering

	if in.File != "" {
		return fmt.Errorf("forgectl advance takes no --file flag in QUEUE. The queue file is fixed at .forgectl/state/reverse-engineering-queue.json.")
	}

	path := reverseEngineeringQueuePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("reverse engineering queue file not found at expected path: %s", path)
		}
		return fmt.Errorf("reading queue file: %w", err)
	}

	hash := HashBytes(data)
	if re.QueueContentHash != "" && hash == re.QueueContentHash {
		return fmt.Errorf("Queue file has not changed. Update the file and retry.")
	}

	if violations := ValidateReverseEngineeringQueue(data, dir, re.Domains); len(violations) > 0 {
		return &ValidationError{Errors: violations}
	}

	var input ReverseEngineeringQueueInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parsing queue file: %w", err)
	}

	// Record path + hash and the parsed (full, cross-domain) queue.
	re.QueueFilePath = path
	re.QueueContentHash = hash
	re.Queue = input.Specs

	// Advance to the next domain's SURVEY, or into the execution loop once every
	// domain has been processed.
	if re.DomainIndex < re.DomainCount {
		re.DomainIndex++
		s.State = StateSurvey
		return nil
	}

	if len(re.Queue) == 0 {
		return fmt.Errorf("Queue contains zero entries. Nothing to execute.")
	}
	re.ExecuteItemIndex = 1
	s.State = StateExecuteReverseEngineer
	return ensureItemSpecsDir(re, dir)
}

// effectivePlanStrategy returns the planning commit strategy, falling back to "strict".
func effectivePlanStrategy(s *ForgeState) string {
	if s.Config.Planning.CommitStrategy != "" {
		return s.Config.Planning.CommitStrategy
	}
	return "strict"
}

// planScopeTargets returns the git staging targets for a planning ACCEPT commit.
// strict stages plan.json and its adjacent notes/ directory.
// scoped stages the entire domain directory.
// all-specs stages domain/specs/.
// tracked/all return nil (AutoCommit handles those via -u / -A flags).
func planScopeTargets(s *ForgeState, strategy string) []string {
	plan := s.Planning.CurrentPlan
	if plan == nil {
		return nil
	}
	switch strategy {
	case "strict":
		targets := []string{plan.File}
		notesDir := filepath.Join(filepath.Dir(plan.File), "notes") + "/"
		targets = append(targets, notesDir)
		return targets
	case "scoped":
		if plan.Domain != "" {
			return []string{plan.Domain + "/"}
		}
		return nil
	case "all-specs":
		if plan.Domain != "" {
			return []string{plan.Domain + "/specs/"}
		}
		return nil
	default:
		return nil
	}
}

// effectiveImplStrategy returns the implementing commit strategy, falling back to "scoped".
func effectiveImplStrategy(s *ForgeState) string {
	if s.Config.Implementing.CommitStrategy != "" {
		return s.Config.Implementing.CommitStrategy
	}
	return "scoped"
}

// implScopeTargets returns the git staging targets for implementing phase commits.
// item is the specific plan item being committed (used by strict strategy); nil
// means use all items in the current batch (used at COMMIT time).
func implScopeTargets(impl *ImplementingState, item *PlanItem, strategy string) []string {
	switch strategy {
	case "strict":
		if item != nil {
			return item.Files
		}
		return nil
	case "scoped":
		if impl.CurrentPlanDomain != "" {
			return []string{impl.CurrentPlanDomain + "/"}
		}
		return nil
	case "all-specs":
		if impl.CurrentPlanDomain != "" {
			return []string{impl.CurrentPlanDomain + "/specs/"}
		}
		return nil
	default:
		return nil
	}
}

// effectiveUIStrategy returns the ui_implementing commit strategy, falling back to "scoped".
func effectiveUIStrategy(s *ForgeState) string {
	if s.Config.UIImplementing.CommitStrategy != "" {
		return s.Config.UIImplementing.CommitStrategy
	}
	return "scoped"
}

// uiScopeTargets returns the git staging targets for ui_implementing phase commits.
// item is the specific plan item being committed (used by strict strategy); nil
// means use the whole domain directory (used at COMMIT time).
func uiScopeTargets(ui *UIImplementingState, item *PlanItem, strategy string) []string {
	switch strategy {
	case "strict":
		if item != nil {
			return item.Files
		}
		return nil
	case "scoped":
		if ui.CurrentPlanDomain != "" {
			return []string{ui.CurrentPlanDomain + "/"}
		}
		return nil
	case "all-specs":
		if ui.CurrentPlanDomain != "" {
			return []string{ui.CurrentPlanDomain + "/specs/"}
		}
		return nil
	default:
		return nil
	}
}

// initPlanForImplementing reads the current implementing plan.json and adds
// passes/rounds fields to items that haven't been initialized yet, then writes
// it back. Called at all-first domain boundaries where the Planning→Implementing
// PHASE_SHIFT did not run for this plan.
func initPlanForImplementing(s *ForgeState, dir string) error {
	planPath := currentPlanFile(s)
	if planPath == "" {
		return nil
	}
	fullPath := filepath.Join(dir, planPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("reading plan.json: %w", err)
	}
	var plan PlanJSON
	if err := json.Unmarshal(data, &plan); err != nil {
		return fmt.Errorf("parsing plan.json: %w", err)
	}
	for i := range plan.Items {
		if plan.Items[i].Passes == "" {
			plan.Items[i].Passes = "pending"
		}
	}
	planData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plan: %w", err)
	}
	return os.WriteFile(fullPath, planData, 0644)
}

// ValidationError wraps multiple validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %d errors", len(e.Errors))
}
