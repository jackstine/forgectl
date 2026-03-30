package state

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PrintAdvanceOutput prints the action description for the new state after advance.
func PrintAdvanceOutput(w io.Writer, s *ForgeState, dir string) {
	switch s.Phase {
	case PhaseSpecifying:
		printSpecifyingOutput(w, s)
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

func printSpecifyingOutput(w io.Writer, s *ForgeState) {
	spec := s.Specifying
	var cs *ActiveSpec
	if len(spec.CurrentSpecs) > 0 {
		cs = spec.CurrentSpecs[0]
	}

	switch s.State {
	case StateSelect:
		fmt.Fprintf(w, "State:   SELECT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "ID:      %d\n", cs.ID)
		fmt.Fprintf(w, "Spec:    %s\n", cs.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "File:    %s\n", cs.File)
		fmt.Fprintf(w, "Topic:   %s\n", cs.Topic)
		if len(cs.PlanningSources) > 0 {
			fmt.Fprintf(w, "Sources: %s\n", strings.Join(cs.PlanningSources, ", "))
		}
		fmt.Fprintf(w, "Action:  Review topic and planning sources.\n")
		if s.Config.General.UserGuided {
			fmt.Fprintf(w, "         Stop and review and discuss with user before continuing.\n")
		}
		fmt.Fprintf(w, "         Advance to begin drafting.\n")

	case StateDraft:
		fmt.Fprintf(w, "State:   DRAFT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "ID:      %d\n", cs.ID)
		fmt.Fprintf(w, "Spec:    %s\n", cs.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "File:    %s\n", cs.File)
		fmt.Fprintf(w, "Action:  Draft the spec. Advance when ready.\n")
		fmt.Fprintf(w, "         Use --file <path> if the file path changed.\n")

	case StateEvaluate:
		evalDir := filepath.Dir(cs.File)
		specBase := strings.TrimSuffix(filepath.Base(cs.File), filepath.Ext(cs.File))
		evalFile := filepath.Join(evalDir, ".eval", fmt.Sprintf("%s-r%d.md", specBase, cs.Round))

		fmt.Fprintf(w, "State:   EVALUATE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "ID:      %d\n", cs.ID)
		fmt.Fprintf(w, "Spec:    %s\n", cs.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "File:    %s\n", cs.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", cs.Round, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Action:  Spawn evaluation sub-agent against the spec.\n")
		fmt.Fprintf(w, "         Eval output: %s\n", evalFile)
		fmt.Fprintf(w, "         Advance with --verdict PASS --eval-report <path> --message <commit msg>\n")
		fmt.Fprintf(w, "           or --verdict FAIL --eval-report <path>\n")

	case StateRefine:
		evalDir := filepath.Dir(cs.File)
		specBase := strings.TrimSuffix(filepath.Base(cs.File), filepath.Ext(cs.File))
		evalFile := filepath.Join(evalDir, ".eval", fmt.Sprintf("%s-r%d.md", specBase, cs.Round))

		fmt.Fprintf(w, "State:   REFINE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "ID:      %d\n", cs.ID)
		fmt.Fprintf(w, "Spec:    %s\n", cs.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "File:    %s\n", cs.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", cs.Round, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Action:  Read the eval report and address any findings in the spec file.\n")
		fmt.Fprintf(w, "         Eval report: %s\n", evalFile)
		fmt.Fprintf(w, "         When changes are complete, run: forgectl advance\n")

	case StateAccept:
		fmt.Fprintf(w, "State:   ACCEPT\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "ID:      %d\n", cs.ID)
		fmt.Fprintf(w, "Spec:    %s\n", cs.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cs.Domain)
		fmt.Fprintf(w, "File:    %s\n", cs.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", cs.Round, s.Config.Specifying.Eval.MaxRounds)
		fmt.Fprintf(w, "Action:  Spec accepted. Advance to continue.\n")

	case StateDone:
		fmt.Fprintf(w, "State:   DONE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Specs:   %d completed\n", len(spec.Completed))
		fmt.Fprintf(w, "Action:  All individual specs complete. Advance to begin reconciliation.\n")

	case StateReconcile:
		fmt.Fprintf(w, "State:   RECONCILE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		if len(spec.Completed) > 0 {
			fmt.Fprintf(w, "Domain:  %s\n", spec.Completed[0].Domain)
		}
		fmt.Fprintf(w, "Specs:   %d completed\n", len(spec.Completed))
		fmt.Fprintf(w, "Action:  Cross-validate all specs: verify Depends On entries, Integration Points\n")
		fmt.Fprintf(w, "         symmetry, naming consistency. Stage changes with git add.\n")
		fmt.Fprintf(w, "         Advance when ready.\n")

	case StateReconcileEval:
		fmt.Fprintf(w, "State:   RECONCILE_EVAL\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Round:   %d\n", spec.Reconcile.Round)
		fmt.Fprintf(w, "Action:  Tell the sub-agent to run git diff --staged and evaluate\n")
		fmt.Fprintf(w, "         consistency across all specs.\n")
		fmt.Fprintf(w, "         Advance with --verdict PASS --message <commit msg>\n")
		fmt.Fprintf(w, "           or --verdict FAIL.\n")

	case StateReconcileReview:
		fmt.Fprintf(w, "State:   RECONCILE_REVIEW\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Round:   %d\n", spec.Reconcile.Round)
		fmt.Fprintf(w, "Action:  Reconciliation eval found issues.\n")
		fmt.Fprintf(w, "         Accept: advance (or --verdict PASS)\n")
		fmt.Fprintf(w, "         Fix and re-evaluate: advance --verdict FAIL\n")

	case StateComplete:
		fmt.Fprintf(w, "State:   COMPLETE\n")
		fmt.Fprintf(w, "Phase:   specifying\n")
		fmt.Fprintf(w, "Specs:   %d completed, reconciled\n", len(spec.Completed))
		fmt.Fprintf(w, "Action:  Specifying phase complete. Advance to continue.\n")
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
		fmt.Fprintf(w, "Action:  Explore the codebase in relation to the specs under study.\n")
		fmt.Fprintf(w, "         Sub-agents: 3. Search roots: %s.\n", strings.Join(cp.CodeSearchRoots, ", "))
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

	case StateEvaluate:
		fmt.Fprintf(w, "State:   EVALUATE\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		fmt.Fprintf(w, "Action:  Run evaluation sub-agent against the plan (round %d/%d).\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		fmt.Fprintf(w, "         Sub-agent: forgectl eval\n")
		fmt.Fprintf(w, "         Advance with --verdict PASS|FAIL --eval-report <path>.\n")

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
		if lastEval.Verdict == "FAIL" {
			fmt.Fprintf(w, "Action:  Evaluation found deficiencies. Spawn a sub-agent to update the plan and notes.\n")
		} else {
			fmt.Fprintf(w, "Action:  Minimum evaluation rounds not met. Spawn a sub-agent to re-evaluate the plan.\n")
		}
		fmt.Fprintf(w, "         Eval report: %s\n", evalFile)
		fmt.Fprintf(w, "         Advance when plan is updated.\n")

	case StateAccept:
		fmt.Fprintf(w, "State:   ACCEPT\n")
		fmt.Fprintf(w, "Phase:   planning\n")
		fmt.Fprintf(w, "Plan:    %s\n", cp.Name)
		fmt.Fprintf(w, "Domain:  %s\n", cp.Domain)
		fmt.Fprintf(w, "File:    %s\n", cp.File)
		fmt.Fprintf(w, "Round:   %d/%d\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
		lastEval := plan.Evals[len(plan.Evals)-1]
		if lastEval.Verdict == "FAIL" && plan.Round >= s.Config.Planning.Eval.MaxRounds {
			fmt.Fprintf(w, "Action:  Plan accepted (max rounds reached).\n")
		} else {
			fmt.Fprintf(w, "Action:  Plan accepted.\n")
		}
		fmt.Fprintf(w, "         Run: forgectl advance --message <commit msg>\n")
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
			fmt.Fprintf(w, "Config:  batch_size=%d, rounds=%d-%d\n", s.Config.Implementing.Batch, s.Config.Implementing.Eval.MinRounds, s.Config.Implementing.Eval.MaxRounds)
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
					fmt.Fprintf(w, " — layer complete")
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
			actionText := "Selecting next batch. Run: forgectl advance"
			layerDef := findLayer(plan, impl.CurrentLayer.ID)
			if layerDef != nil && allLayerItemsTerminal(plan, *layerDef) {
				actionText = "Advancing to next layer. Run: forgectl advance"
			}
			if s.Config.General.UserGuided {
				fmt.Fprintf(w, "Action:   Stop and review and discuss with user before continuing.\n")
				fmt.Fprintf(w, "          %s\n", actionText)
			} else {
				fmt.Fprintf(w, "Action:   %s\n", actionText)
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
				evalDir := filepath.Join(filepath.Dir(s.Planning.CurrentPlan.File), "evals")
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
		if item.Spec != "" {
			fmt.Fprintf(w, "Spec:    %s\n", item.Spec)
		}
		if item.Ref != "" {
			fmt.Fprintf(w, "Ref:     %s\n", item.Ref)
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
			evalDir := filepath.Join(filepath.Dir(s.Planning.CurrentPlan.File), "evals")
			lastEval := batch.Evals[len(batch.Evals)-1]
			evalFile := filepath.Join(evalDir, fmt.Sprintf("batch-%d-round-%d.md", impl.BatchNumber, lastEval.Round))
			fmt.Fprintf(w, "Action:  Study the eval file %q\n", evalFile)
			fmt.Fprintf(w, "         and implement any corrections as needed. If none found during the eval,\n")
			fmt.Fprintf(w, "         please verify and look for corrections. Apply them.\n")
			fmt.Fprintf(w, "         When complete, run: forgectl advance\n")
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
		fmt.Fprintf(w, "            forgectl advance --eval-report <path> --verdict PASS|FAIL\n")
		fmt.Fprintf(w, "Sub-agent: forgectl eval\n")

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

		fmt.Fprintf(w, "Action:  Commit your changes before continuing.\n")
		fmt.Fprintf(w, "         When ready, run: forgectl advance --message <commit msg>\n")

	case StateDone:
		plan, _ := loadPlan(s, dir)
		fmt.Fprintf(w, "State:   DONE\n")
		fmt.Fprintf(w, "Phase:   implementing\n")
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
		fmt.Fprintf(w, "Action:  All items complete. Session done.\n")
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

	fmt.Fprintf(w, "\nStop and refresh your context, please.\n")
	if ps.From == PhaseSpecifying && ps.To == PhasePlanning {
		fmt.Fprintf(w, "When ready, run: forgectl advance --from <plans-queue.json>\n")
	} else {
		fmt.Fprintf(w, "When ready, run: forgectl advance\n")
	}
}

// --- Status ---

// PrintStatus prints the full session status.
func PrintStatus(w io.Writer, s *ForgeState, dir string) {
	fmt.Fprintf(w, "Session: forgectl-state.json\n")
	fmt.Fprintf(w, "Phase:   %s", s.Phase)
	if s.StartedAtPhase != "" && s.StartedAtPhase == s.Phase && s.StartedAtPhase != PhaseSpecifying {
		fmt.Fprintf(w, " (started here)")
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Config:  batch_size=%d, rounds=%d-%d, guided=%v\n", s.Config.Implementing.Batch, s.Config.Implementing.Eval.MinRounds, s.Config.Implementing.Eval.MaxRounds, s.Config.General.UserGuided)
	fmt.Fprintln(w)

	// Current state.
	fmt.Fprintf(w, "--- Current ---\n\n")
	PrintAdvanceOutput(w, s, dir)
	fmt.Fprintln(w)

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

	// Planning section.
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

	// Implementing section.
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

// evaluatorDir returns the directory containing the forgectl binary, which is
// where the evaluators/ subdirectory lives. Falls back to the current working
// directory if the executable path cannot be resolved.
func evaluatorDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// PrintEvalOutput prints the evaluation context for the sub-agent.
func PrintEvalOutput(w io.Writer, s *ForgeState, dir string) error {
	switch s.Phase {
	case PhasePlanning:
		return printPlanningEval(w, s, dir)
	case PhaseImplementing:
		return printImplementingEval(w, s, dir)
	default:
		return fmt.Errorf("eval is only valid in planning or implementing EVALUATE state (current: %s %s)", s.Phase, s.State)
	}
}

func printPlanningEval(w io.Writer, s *ForgeState, dir string) error {
	if s.State != StateEvaluate {
		return fmt.Errorf("eval is only valid in EVALUATE state (current: %s)", s.State)
	}

	plan := s.Planning

	fmt.Fprintf(w, "=== PLAN EVALUATION ROUND %d/%d ===\n", plan.Round, s.Config.Planning.Eval.MaxRounds)
	fmt.Fprintf(w, "Plan:   %s\n", plan.CurrentPlan.Name)
	fmt.Fprintf(w, "Domain: %s\n", plan.CurrentPlan.Domain)
	fmt.Fprintf(w, "File:   %s\n", plan.CurrentPlan.File)

	// Evaluator instructions.
	evalPromptPath := filepath.Join(evaluatorDir(), "evaluators", "plan-eval.md")
	evalPrompt, err := os.ReadFile(evalPromptPath)
	if err != nil {
		fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
		fmt.Fprintf(w, "(could not read %s: %s)\n", evalPromptPath, err)
	} else {
		fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
		fmt.Fprintf(w, "%s\n", string(evalPrompt))
	}

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

	// Report output.
	evalDir := filepath.Join(filepath.Dir(plan.CurrentPlan.File), "evals")
	reportFile := filepath.Join(evalDir, fmt.Sprintf("round-%d.md", plan.Round))
	fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
	fmt.Fprintf(w, "Write your evaluation report to:\n")
	fmt.Fprintf(w, "  %s\n", reportFile)

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
	evalPromptPath := filepath.Join(evaluatorDir(), "evaluators", "impl-eval.md")
	evalPrompt, err := os.ReadFile(evalPromptPath)
	if err != nil {
		fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
		fmt.Fprintf(w, "(could not read %s: %s)\n", evalPromptPath, err)
	} else {
		fmt.Fprintf(w, "\n--- EVALUATOR INSTRUCTIONS ---\n\n")
		fmt.Fprintf(w, "%s\n", string(evalPrompt))
	}

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
		if item.Spec != "" {
			fmt.Fprintf(w, "    Spec:        %s\n", item.Spec)
		}
		if item.Ref != "" {
			fmt.Fprintf(w, "    Ref:         %s\n", item.Ref)
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

	// Report output.
	evalDir := filepath.Join(filepath.Dir(s.Planning.CurrentPlan.File), "evals")
	reportFile := filepath.Join(evalDir, fmt.Sprintf("batch-%d-round-%d.md", impl.BatchNumber, evalRound))
	fmt.Fprintf(w, "\n--- REPORT OUTPUT ---\n\n")
	fmt.Fprintf(w, "Write your evaluation report to:\n")
	fmt.Fprintf(w, "  %s\n", reportFile)

	return nil
}

// --- Helpers ---

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
