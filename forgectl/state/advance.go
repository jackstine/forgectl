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
	default:
		return fmt.Errorf("unknown phase %q", s.Phase)
	}
}

// --- Specifying Phase ---

func advanceSpecifying(s *ForgeState, in AdvanceInput, dir string) error {
	spec := s.Specifying

	switch s.State {
	case StateOrient:
		// Pull next from queue into current_spec.
		if len(spec.Queue) == 0 {
			return fmt.Errorf("queue is empty")
		}
		entry := spec.Queue[0]
		spec.Queue = spec.Queue[1:]
		spec.CurrentSpec = &ActiveSpec{
			ID:              len(spec.Completed) + 1,
			Name:            entry.Name,
			Domain:          entry.Domain,
			Topic:           entry.Topic,
			File:            entry.File,
			PlanningSources: entry.PlanningSources,
			DependsOn:       entry.DependsOn,
		}
		s.State = StateSelect

	case StateSelect:
		s.State = StateDraft

	case StateDraft:
		if in.File != "" {
			spec.CurrentSpec.File = in.File
		}
		spec.CurrentSpec.Round = 1
		s.State = StateEvaluate

	case StateEvaluate:
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in EVALUATE state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		evalEnabled := s.Config.Specifying.Eval.EnableEvalOutput
		if evalEnabled {
			if in.EvalReport == "" {
				return fmt.Errorf("--eval-report is required in EVALUATE state")
			}
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}
		evalReport := in.EvalReport
		if !evalEnabled {
			evalReport = ""
		}

		eval := EvalRecord{
			Round:      spec.CurrentSpec.Round,
			Verdict:    in.Verdict,
			EvalReport: evalReport,
		}
		spec.CurrentSpec.Evals = append(spec.CurrentSpec.Evals, eval)

		if in.Verdict == "PASS" {
			if spec.CurrentSpec.Round >= s.Config.Specifying.Eval.MinRounds {
				if in.Message == "" {
					return fmt.Errorf("--message is required when --verdict is PASS")
				}
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		} else {
			if spec.CurrentSpec.Round >= s.Config.Specifying.Eval.MaxRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		}

	case StateRefine:
		spec.CurrentSpec.Round++
		s.State = StateEvaluate

	case StateAccept:
		completed := CompletedSpec{
			ID:          spec.CurrentSpec.ID,
			Name:        spec.CurrentSpec.Name,
			Domain:      spec.CurrentSpec.Domain,
			File:        spec.CurrentSpec.File,
			RoundsTaken: spec.CurrentSpec.Round,
			Evals:       spec.CurrentSpec.Evals,
		}
		spec.Completed = append(spec.Completed, completed)
		spec.CurrentSpec = nil

		if len(spec.Queue) == 0 {
			s.State = StateDone
		} else {
			s.State = StateOrient
		}

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
		reconEvalEnabled := s.Config.Specifying.Eval.EnableEvalOutput
		reconEvalReport := in.EvalReport
		if reconEvalEnabled && in.EvalReport != "" {
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}
		if !reconEvalEnabled {
			reconEvalReport = ""
		}

		eval := EvalRecord{
			Round:      spec.Reconcile.Round,
			Verdict:    in.Verdict,
			EvalReport: reconEvalReport,
		}
		spec.Reconcile.Evals = append(spec.Reconcile.Evals, eval)

		if in.Verdict == "PASS" {
			if in.Message == "" {
				return fmt.Errorf("--message is required when --verdict is PASS")
			}
			s.State = StateComplete
		} else {
			s.State = StateReconcileReview
		}

	case StateReconcileReview:
		if in.Verdict == "FAIL" {
			s.State = StateReconcile
		} else {
			// No verdict or PASS = accept.
			s.State = StateComplete
		}

	case StateComplete:
		if s.Config.General.EnableCommits && in.Message == "" {
			return fmt.Errorf("--message is required when enable_commits is true")
		}
		if s.Config.General.EnableCommits && in.Message != "" {
			// Stage all completed spec files and commit.
			var specPaths []string
			for _, c := range s.Specifying.Completed {
				specPaths = append(specPaths, c.File)
			}
			hash, err := AutoCommit(dir, s.Config.Specifying.CommitStrategy, specPaths, in.Message)
			if err != nil {
				return err
			}
			// Register hash on all completed specs (only if a commit was made).
			if hash != "" {
				for i := range s.Specifying.Completed {
					s.Specifying.Completed[i].CommitHash = hash
					s.Specifying.Completed[i].CommitHashes = append(s.Specifying.Completed[i].CommitHashes, hash)
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

	case StateSelfReview:
		planPath := s.Planning.CurrentPlan.File
		fullPath := filepath.Join(dir, planPath)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			s.State = StateValidate
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

	case StateEvaluate:
		if in.Verdict == "" {
			return fmt.Errorf("--verdict is required in EVALUATE state")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("--verdict must be PASS or FAIL")
		}
		planEvalEnabled := s.Config.Planning.Eval.EnableEvalOutput
		if planEvalEnabled {
			if in.EvalReport == "" {
				return fmt.Errorf("--eval-report is required in EVALUATE state")
			}
			if err := checkEvalReportExists(in.EvalReport); err != nil {
				return err
			}
		}
		planEvalReport := in.EvalReport
		if !planEvalEnabled {
			planEvalReport = ""
		}

		eval := EvalRecord{
			Round:      s.Planning.Round,
			Verdict:    in.Verdict,
			EvalReport: planEvalReport,
		}
		s.Planning.Evals = append(s.Planning.Evals, eval)

		if in.Verdict == "PASS" {
			if s.Planning.Round >= s.Config.Planning.Eval.MinRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		} else {
			if s.Planning.Round >= s.Config.Planning.Eval.MaxRounds {
				s.State = StateAccept
			} else {
				s.State = StateRefine
			}
		}

	case StateRefine:
		s.Planning.Round++
		return advancePlanningFromDraftOrRefine(s, dir)

	case StateAccept:
		if s.Config.General.EnableCommits && in.Message == "" {
			return fmt.Errorf("--message is required in planning ACCEPT state when enable_commits is true")
		}
		if s.Config.General.EnableCommits && in.Message != "" && s.Planning.CurrentPlan != nil {
			// Stage plan.json + notes directory.
			notesDir := filepath.Join(filepath.Dir(s.Planning.CurrentPlan.File), "notes")
			stageTargets := []string{s.Planning.CurrentPlan.File, notesDir}
			if _, err := AutoCommit(dir, s.Config.Planning.CommitStrategy, stageTargets, in.Message); err != nil {
				return err
			}
		}
		if !s.Config.Planning.PlanAllBeforeImplementing {
			// Interleaved mode: always planning → implementing.
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhaseImplementing}
		} else if len(s.Planning.Queue) > 0 {
			// All-first: more plans remain — planning → planning.
			completed := CompletedPlan{
				ID:     len(s.Planning.Completed) + 1,
				Name:   s.Planning.CurrentPlan.Name,
				Domain: s.Planning.CurrentPlan.Domain,
				File:   s.Planning.CurrentPlan.File,
			}
			s.Planning.Completed = append(s.Planning.Completed, completed)
			// Pull next plan from queue.
			entry := s.Planning.Queue[0]
			s.Planning.Queue = s.Planning.Queue[1:]
			s.Planning.CurrentPlan = &ActivePlan{
				ID:              completed.ID + 1,
				Name:            entry.Name,
				Domain:          entry.Domain,
				Topic:           entry.Topic,
				File:            entry.File,
				Specs:           entry.Specs,
				CodeSearchRoots: entry.CodeSearchRoots,
			}
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhasePlanning}
		} else {
			// All-first: last plan accepted — planning → implementing.
			completed := CompletedPlan{
				ID:     len(s.Planning.Completed) + 1,
				Name:   s.Planning.CurrentPlan.Name,
				Domain: s.Planning.CurrentPlan.Domain,
				File:   s.Planning.CurrentPlan.File,
			}
			s.Planning.Completed = append(s.Planning.Completed, completed)
			s.Planning.CurrentPlan = nil
			s.State = StatePhaseShift
			s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhaseImplementing}
		}

	case StateDone:
		// Planning DONE is a pass-through — no flags accepted.
		if in.Verdict != "" || in.EvalReport != "" || in.Message != "" || in.From != "" || in.File != "" {
			return fmt.Errorf("DONE is a pass-through state. No flags accepted.")
		}
		return fmt.Errorf("cannot advance from state %q in planning phase", s.State)

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
	if s.Config.Planning.SelfReview {
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
		if s.Config.General.EnableCommits && in.Message == "" {
			return fmt.Errorf("--message is required in COMMIT state when enable_commits is true")
		}
		if s.Config.General.EnableCommits && in.Message != "" {
			// Stage the domain directory and commit.
			domain := impl.CurrentPlanDomain
			if domain == "" && s.Planning != nil && s.Planning.CurrentPlan != nil {
				domain = s.Planning.CurrentPlan.Domain
			}
			var stageTargets []string
			if domain != "" {
				stageTargets = []string{domain + "/"}
			}
			if _, err := AutoCommit(dir, s.Config.Implementing.CommitStrategy, stageTargets, in.Message); err != nil {
				return err
			}
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
		if !s.Config.Planning.PlanAllBeforeImplementing {
			// Interleaved mode: check Planning.Queue for remaining domains.
			if s.Planning != nil && len(s.Planning.Queue) > 0 {
				s.State = StatePhaseShift
				s.PhaseShift = &PhaseShiftInfo{From: PhaseImplementing, To: PhasePlanning}
				return nil
			}
		} else {
			// All-first mode: check Implementing.PlanQueue for remaining domains.
			if len(impl.PlanQueue) > 0 {
				s.State = StatePhaseShift
				s.PhaseShift = &PhaseShiftInfo{From: PhaseImplementing, To: PhaseImplementing}
				return nil
			}
		}
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

	// First round requires --message when commits are enabled.
	if batch.EvalRound == 0 && s.Config.General.EnableCommits && in.Message == "" {
		return fmt.Errorf("--message is required for first-round implementation when enable_commits is true")
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
	if in.Verdict != "PASS" && in.Verdict != "FAIL" {
		return fmt.Errorf("--verdict must be PASS or FAIL")
	}
	implEvalEnabled := s.Config.Implementing.Eval.EnableEvalOutput
	if implEvalEnabled {
		if in.EvalReport == "" {
			return fmt.Errorf("--eval-report is required in EVALUATE state")
		}
		if err := checkEvalReportExists(in.EvalReport); err != nil {
			return err
		}
	}
	implEvalReport := in.EvalReport
	if !implEvalEnabled {
		implEvalReport = ""
	}

	impl := s.Implementing
	batch := impl.CurrentBatch
	batch.EvalRound++

	eval := EvalRecord{
		Round:      batch.EvalRound,
		Verdict:    in.Verdict,
		EvalReport: implEvalReport,
	}
	batch.Evals = append(batch.Evals, eval)

	plan, err := loadPlan(s, dir)
	if err != nil {
		return err
	}

	if in.Verdict == "PASS" {
		if batch.EvalRound >= s.Config.Implementing.Eval.MinRounds {
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
		if batch.EvalRound >= s.Config.Implementing.Eval.MaxRounds {
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
	case s.PhaseShift.From == PhaseSpecifying && s.PhaseShift.To == PhaseGeneratePlanningQueue:
		if in.From != "" {
			// --from provided: validate override and skip genqueue entirely.
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
			s.Planning = populatePlanningFromQueue(input.Plans)
			s.Phase = PhasePlanning
			s.State = StateOrient
			s.PhaseShift = nil
		} else {
			// No --from: auto-generate plan queue, enter generate_planning_queue phase.
			planQueueFile, err := autoGeneratePlanQueue(s, dir)
			if err != nil {
				return fmt.Errorf("generating plan queue: %w", err)
			}
			s.GeneratePlanningQueue = &GeneratePlanningQueueState{
				PlanQueueFile: planQueueFile,
			}
			s.Phase = PhaseGeneratePlanningQueue
			s.State = StateOrient
			s.PhaseShift = nil
		}

	case s.PhaseShift.From == PhaseGeneratePlanningQueue && s.PhaseShift.To == PhasePlanning:
		var entries []PlanQueueEntry
		if in.From != "" {
			// --from override: validate and use it.
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
			entries = input.Plans
		} else {
			// Read from generated plan-queue.json.
			if s.GeneratePlanningQueue == nil || s.GeneratePlanningQueue.PlanQueueFile == "" {
				return fmt.Errorf("no plan queue file in state")
			}
			fullPath := filepath.Join(dir, s.GeneratePlanningQueue.PlanQueueFile)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return fmt.Errorf("reading plan queue: %w", err)
			}
			var input PlanQueueInput
			if err := json.Unmarshal(data, &input); err != nil {
				return fmt.Errorf("parsing plan queue: %w", err)
			}
			entries = input.Plans
		}
		s.Planning = populatePlanningFromQueue(entries)
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhasePlanning && s.PhaseShift.To == PhasePlanning:
		// All-first domain boundary: CurrentPlan was already set in ACCEPT, just reset and orient.
		s.Planning.Round = 0
		s.Planning.Evals = nil
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhasePlanning && s.PhaseShift.To == PhaseImplementing:
		var planFile, planDomain string
		var implPlanQueue []PlanQueueEntry

		if s.Config.Planning.PlanAllBeforeImplementing && len(s.Planning.Completed) > 0 {
			// All-first mode: implement first completed plan, queue the rest.
			first := s.Planning.Completed[0]
			planFile = first.File
			planDomain = first.Domain
			for _, cp := range s.Planning.Completed[1:] {
				implPlanQueue = append(implPlanQueue, PlanQueueEntry{
					Name:            cp.Name,
					Domain:          cp.Domain,
					File:            cp.File,
					Specs:           []string{},
					CodeSearchRoots: []string{cp.Domain + "/"},
				})
			}
		} else {
			// Interleaved mode: implement current plan.
			planFile = s.Planning.CurrentPlan.File
			planDomain = s.Planning.CurrentPlan.Domain
		}

		if err := mutatePlanForImplementing(dir, planFile); err != nil {
			return err
		}

		s.Implementing = NewImplementingState()
		s.Implementing.CurrentPlanFile = planFile
		s.Implementing.CurrentPlanDomain = planDomain
		s.Implementing.PlanQueue = implPlanQueue
		s.Phase = PhaseImplementing
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhaseImplementing && s.PhaseShift.To == PhasePlanning:
		// Interleaved: pop next plan from Planning.Queue.
		if s.Planning == nil || len(s.Planning.Queue) == 0 {
			return fmt.Errorf("no plans in planning queue")
		}
		entry := s.Planning.Queue[0]
		s.Planning.Queue = s.Planning.Queue[1:]
		s.Planning.CurrentPlan = &ActivePlan{
			ID:              len(s.Planning.Completed) + 1,
			Name:            entry.Name,
			Domain:          entry.Domain,
			Topic:           entry.Topic,
			File:            entry.File,
			Specs:           entry.Specs,
			CodeSearchRoots: entry.CodeSearchRoots,
		}
		s.Planning.Round = 0
		s.Planning.Evals = nil
		s.Phase = PhasePlanning
		s.State = StateOrient
		s.PhaseShift = nil

	case s.PhaseShift.From == PhaseImplementing && s.PhaseShift.To == PhaseImplementing:
		// All-first domain boundary: pop next plan from Implementing.PlanQueue.
		impl := s.Implementing
		if len(impl.PlanQueue) == 0 {
			return fmt.Errorf("no plans in implementing queue")
		}
		entry := impl.PlanQueue[0]
		impl.PlanQueue = impl.PlanQueue[1:]

		if err := mutatePlanForImplementing(dir, entry.File); err != nil {
			return err
		}

		// Reset implementing state for the new plan.
		impl.CurrentLayer = nil
		impl.BatchNumber = 0
		impl.CurrentBatch = nil
		impl.LayerHistory = []LayerHistory{}
		impl.CurrentPlanFile = entry.File
		impl.CurrentPlanDomain = entry.Domain
		s.Phase = PhaseImplementing
		s.State = StateOrient
		s.PhaseShift = nil

	default:
		return fmt.Errorf("unknown phase shift: %s → %s", s.PhaseShift.From, s.PhaseShift.To)
	}

	return nil
}

// --- Generate Planning Queue Phase ---

func advanceGeneratePlanningQueue(s *ForgeState, in AdvanceInput, dir string) error {
	switch s.State {
	case StateOrient:
		s.State = StateRefine

	case StateRefine:
		if s.GeneratePlanningQueue == nil || s.GeneratePlanningQueue.PlanQueueFile == "" {
			return fmt.Errorf("no plan queue file in state")
		}
		fullPath := filepath.Join(dir, s.GeneratePlanningQueue.PlanQueueFile)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("reading plan queue %q: %w", s.GeneratePlanningQueue.PlanQueueFile, err)
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

// autoGeneratePlanQueue groups completed specs by domain and writes plan-queue.json to the state dir.
// Returns the relative path to the written file (relative to dir).
func autoGeneratePlanQueue(s *ForgeState, dir string) (string, error) {
	// Group completed specs by domain, preserving first-appearance order.
	domainOrder := []string{}
	domainSpecs := map[string][]CompletedSpec{}
	if s.Specifying != nil {
		for _, spec := range s.Specifying.Completed {
			if _, ok := domainSpecs[spec.Domain]; !ok {
				domainOrder = append(domainOrder, spec.Domain)
			}
			domainSpecs[spec.Domain] = append(domainSpecs[spec.Domain], spec)
		}
	}

	workspaceDir := s.Config.Paths.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = ".forge_workspace"
	}

	var entries []PlanQueueEntry
	for _, domain := range domainOrder {
		specs := domainSpecs[domain]

		// Determine code search roots.
		var roots []string
		if s.Specifying != nil && s.Specifying.DomainRoots != nil {
			if r, ok := s.Specifying.DomainRoots[domain]; ok {
				roots = r
			}
		}
		if len(roots) == 0 {
			roots = []string{domain + "/"}
		}

		// Collect spec file paths.
		var specFiles []string
		for _, spec := range specs {
			specFiles = append(specFiles, spec.File)
		}

		// Deduplicate commit hashes.
		seen := map[string]bool{}
		var commits []string
		for _, spec := range specs {
			for _, h := range spec.CommitHashes {
				if h != "" && !seen[h] {
					seen[h] = true
					commits = append(commits, h)
				}
			}
			if spec.CommitHash != "" && !seen[spec.CommitHash] {
				seen[spec.CommitHash] = true
				commits = append(commits, spec.CommitHash)
			}
		}

		// Capitalize first letter of domain.
		displayName := domain
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		}

		entries = append(entries, PlanQueueEntry{
			Name:            displayName + " Implementation Plan",
			Domain:          domain,
			Topic:           "Implementation of " + domain,
			File:            domain + "/" + workspaceDir + "/implementation_plan/plan.json",
			Specs:           specFiles,
			CodeSearchRoots: roots,
			SpecCommits:     commits,
		})
	}

	input := PlanQueueInput{Plans: entries}
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling plan queue: %w", err)
	}

	stateDir := s.Config.Paths.StateDir
	if stateDir == "" {
		stateDir = ".forgectl/state"
	}
	outPath := filepath.Join(stateDir, "plan-queue.json")
	fullPath := filepath.Join(dir, outPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("creating state dir: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing plan queue: %w", err)
	}
	return outPath, nil
}

// populatePlanningFromQueue creates a PlanningState from queue entries, pulling the first as CurrentPlan.
func populatePlanningFromQueue(entries []PlanQueueEntry) *PlanningState {
	ps := NewPlanningState(entries)
	if len(ps.Queue) > 0 {
		entry := ps.Queue[0]
		ps.Queue = ps.Queue[1:]
		ps.CurrentPlan = &ActivePlan{
			ID:              1,
			Name:            entry.Name,
			Domain:          entry.Domain,
			Topic:           entry.Topic,
			File:            entry.File,
			Specs:           entry.Specs,
			CodeSearchRoots: entry.CodeSearchRoots,
		}
	}
	return ps
}

// mutatePlanForImplementing reads plan.json, sets passes=pending/rounds=0, and writes it back.
func mutatePlanForImplementing(dir, planFile string) error {
	fullPath := filepath.Join(dir, planFile)
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
	for i := range plan.Items {
		plan.Items[i].Passes = "pending"
		plan.Items[i].Rounds = 0
	}
	planData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plan: %w", err)
	}
	if err := os.WriteFile(fullPath, planData, 0644); err != nil {
		return fmt.Errorf("writing plan: %w", err)
	}
	return nil
}

// --- Helpers ---

func loadPlan(s *ForgeState, dir string) (*PlanJSON, error) {
	var planPath string
	if s.Implementing != nil && s.Implementing.CurrentPlanFile != "" {
		planPath = s.Implementing.CurrentPlanFile
	} else if s.Planning != nil && s.Planning.CurrentPlan != nil {
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
	if s.Implementing != nil && s.Implementing.CurrentPlanFile != "" {
		planPath = s.Implementing.CurrentPlanFile
	} else if s.Planning != nil && s.Planning.CurrentPlan != nil {
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
