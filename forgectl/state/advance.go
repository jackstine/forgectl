package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		return advanceSpecifying(s, in)
	case PhasePlanning:
		return advancePlanning(s, in, dir)
	case PhaseImplementing:
		return advanceImplementing(s, in, dir)
	default:
		return fmt.Errorf("unknown phase %q", s.Phase)
	}
}

// --- Specifying Phase ---

func advanceSpecifying(s *ForgeState, in AdvanceInput) error {
	spec := s.Specifying

	switch s.State {
	case StateOrient:
		// Select batch: take up to BatchSize contiguous specs from the first domain.
		if len(spec.Queue) == 0 {
			return fmt.Errorf("queue is empty")
		}
		firstDomain := spec.Queue[0].Domain
		batchSize := s.Config.Specifying.Batch
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
		if in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in EVALUATE state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
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
				if s.Config.General.EnableCommits && in.Message == "" {
					return fmt.Errorf("--message is required when --verdict is PASS and enable_commits is true")
				}
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
		if in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in CROSS_REFERENCE_EVAL state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
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
		if in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in RECONCILE_EVAL state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
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
			if s.Config.General.EnableCommits && in.Verdict == "PASS" && in.Message == "" {
				return fmt.Errorf("--message is required when --verdict is PASS and enable_commits is true")
			}
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
		s.State = StatePhaseShift
		s.PhaseShift = &PhaseShiftInfo{From: PhaseSpecifying, To: PhasePlanning}

	default:
		return fmt.Errorf("cannot advance from state %q in specifying phase", s.State)
	}

	return nil
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
		if in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in EVALUATE state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
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

	case StateAccept:
		if in.Message == "" {
			return fmt.Errorf("--message is required in planning ACCEPT state")
		}
		s.State = StatePhaseShift
		s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhaseImplementing}

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
	s.State = StateEvaluate
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

	s.State = StateEvaluate
	return nil
}

// --- Implementing Phase ---

func advanceImplementing(s *ForgeState, in AdvanceInput, dir string) error {
	impl := s.Implementing

	switch s.State {
	case StateOrient:
		return advanceImplFromOrient(s, dir)

	case StateImplement:
		return advanceImplFromImplement(s, in, dir)

	case StateEvaluate:
		return advanceImplFromEvaluate(s, in, dir)

	case StateCommit:
		if in.Message == "" {
			return fmt.Errorf("--message is required in COMMIT state")
		}
		// Archive batch to history.
		archiveBatch(s)

		// Check if all layers complete.
		plan, err := loadPlan(s, dir)
		if err != nil {
			return err
		}
		if allLayersComplete(plan, impl) {
			s.State = StateDone
		} else {
			s.State = StateOrient
		}

	case StateDone:
		return fmt.Errorf("session complete.")

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

	// First round requires --message.
	if batch.EvalRound == 0 && in.Message == "" {
		return fmt.Errorf("--message is required for first-round implementation")
	}

	// Mark current item as done.
	itemID := batch.Items[batch.CurrentItemIndex]
	setItemPasses(plan, itemID, "done")

	// Save plan.
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
		s.State = StateEvaluate
	}

	return nil
}

func advanceImplFromEvaluate(s *ForgeState, in AdvanceInput, dir string) error {
	if in.Verdict == "" {
		return fmt.Errorf("--verdict is required in EVALUATE state")
	}
	if in.EvalReport == "" {
		return fmt.Errorf("--eval-report is required in EVALUATE state")
	}
	if in.Verdict != "PASS" && in.Verdict != "FAIL" {
		return fmt.Errorf("--verdict must be PASS or FAIL")
	}
	if err := checkEvalReportExists(in.EvalReport); err != nil {
		return err
	}

	impl := s.Implementing
	batch := impl.CurrentBatch
	batch.EvalRound++

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

// --- Phase Shift ---

func advancePhaseShift(s *ForgeState, in AdvanceInput, dir string) error {
	if s.PhaseShift == nil {
		return fmt.Errorf("no phase shift info")
	}

	switch {
	case s.PhaseShift.From == PhaseSpecifying && s.PhaseShift.To == PhasePlanning:
		if in.From == "" {
			return fmt.Errorf("--from <plans-queue.json> is required at this phase shift")
		}
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
		// Pull first plan.
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
			}
		}
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhasePlanning && s.PhaseShift.To == PhaseImplementing:
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

		s.Implementing = NewImplementingState()
		s.Phase = PhaseImplementing
		s.State = StateOrient
		s.PhaseShift = nil

	default:
		return fmt.Errorf("unknown phase shift: %s → %s", s.PhaseShift.From, s.PhaseShift.To)
	}

	return nil
}

// --- Helpers ---

func loadPlan(s *ForgeState, dir string) (*PlanJSON, error) {
	var planPath string
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		planPath = s.Planning.CurrentPlan.File
	}
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

func savePlan(s *ForgeState, dir string, plan *PlanJSON) error {
	var planPath string
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		planPath = s.Planning.CurrentPlan.File
	}
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

func allLayersComplete(plan *PlanJSON, impl *ImplementingState) bool {
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

// specCrossRefNextOrDone advances to ORIENT if any specs remain in the queue,
// or to DONE if the queue is exhausted (all domains cross-referenced).
func specCrossRefNextOrDone(s *ForgeState) {
	if len(s.Specifying.Queue) > 0 {
		s.State = StateOrient
	} else {
		s.State = StateDone
	}
}

func checkEvalReportExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("eval report %q does not exist", path)
	}
	return nil
}

// ValidationError wraps multiple validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %d errors", len(e.Errors))
}
