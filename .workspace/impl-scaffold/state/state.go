package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const stateFileName = "impl-scaffold-state.json"

// Exists returns true if the state file exists in dir.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, stateFileName))
	return err == nil
}

// Load reads and parses the state file from dir.
func Load(dir string) (*ScaffoldState, error) {
	data, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return nil, fmt.Errorf("cannot read state file: %w", err)
	}
	var s ScaffoldState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("cannot parse state file: %w", err)
	}
	return &s, nil
}

// Save writes the state file to dir with indentation.
func Save(dir string, s *ScaffoldState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal state: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, stateFileName), data, 0644)
}

// NewState creates initial state from validated queue plans.
func NewState(minRounds, maxRounds, subAgents int, userGuided bool, plans []QueuePlan) *ScaffoldState {
	return &ScaffoldState{
		MinRounds:  minRounds,
		MaxRounds:  maxRounds,
		SubAgents:  subAgents,
		UserGuided: userGuided,
		State:      ORIENT,
		Queue:      plans,
		Completed:  []CompletedPlan{},
	}
}

// Advance transitions the state machine according to the current state and input.
func Advance(s *ScaffoldState, in AdvanceInput) error {
	switch s.State {
	case ORIENT:
		return advanceOrient(s)
	case STUDY_SPECS:
		return advanceStudySpecs(s, in)
	case STUDY_CODE:
		return advanceStudyCode(s, in)
	case STUDY_PACKAGES:
		return advanceStudyPackages(s, in)
	case SELECT:
		return advanceSelect(s, in)
	case DRAFT:
		return advanceDraft(s, in)
	case EVALUATE:
		return advanceEvaluate(s, in)
	case REFINE:
		return advanceRefine(s, in)
	case ACCEPT:
		return advanceAccept(s, in)
	case DONE:
		return fmt.Errorf("session complete, nothing to advance")
	default:
		return fmt.Errorf("unknown state: %s", s.State)
	}
}

func advanceOrient(s *ScaffoldState) error {
	if len(s.Queue) == 0 {
		return fmt.Errorf("queue is empty")
	}
	next := s.Queue[0]
	s.Queue = s.Queue[1:]
	s.CurrentPlan = &ActivePlan{
		ID:              next.ID,
		Name:            next.Name,
		Domain:          next.Domain,
		Topic:           next.Topic,
		File:            next.File,
		Specs:           next.Specs,
		CodeSearchRoots: next.CodeSearchRoots,
		Study:           StudyNotes{},
		Round:           0,
		Evals:           []EvalRecord{},
	}
	s.State = STUDY_SPECS
	return nil
}

func advanceStudySpecs(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectVerdictFlag(s, in); err != nil {
		return err
	}
	if err := rejectFileFlag(s, in); err != nil {
		return err
	}
	if in.Notes != "" {
		s.CurrentPlan.Study.SpecsNotes = in.Notes
	}
	s.State = STUDY_CODE
	return nil
}

func advanceStudyCode(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectVerdictFlag(s, in); err != nil {
		return err
	}
	if err := rejectFileFlag(s, in); err != nil {
		return err
	}
	if in.Notes != "" {
		s.CurrentPlan.Study.CodeNotes = in.Notes
	}
	s.State = STUDY_PACKAGES
	return nil
}

func advanceStudyPackages(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectVerdictFlag(s, in); err != nil {
		return err
	}
	if err := rejectFileFlag(s, in); err != nil {
		return err
	}
	if in.Notes != "" {
		s.CurrentPlan.Study.PackagesNotes = in.Notes
	}
	s.State = SELECT
	return nil
}

func advanceSelect(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectVerdictFlag(s, in); err != nil {
		return err
	}
	if err := rejectFileFlag(s, in); err != nil {
		return err
	}
	s.State = DRAFT
	return nil
}

func advanceDraft(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectVerdictFlag(s, in); err != nil {
		return err
	}
	if in.File != "" {
		s.CurrentPlan.File = in.File
	}
	s.CurrentPlan.Round = 1
	s.State = EVALUATE
	return nil
}

func advanceEvaluate(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectFileFlag(s, in); err != nil {
		return err
	}
	if in.Verdict == "" {
		return fmt.Errorf("EVALUATE requires --verdict (PASS or FAIL)")
	}
	verdict := strings.ToUpper(in.Verdict)
	if verdict != "PASS" && verdict != "FAIL" {
		return fmt.Errorf("--verdict must be PASS or FAIL, got %q", in.Verdict)
	}

	if verdict == "PASS" {
		if in.Message == "" {
			return fmt.Errorf("--verdict PASS requires --message")
		}
		s.CurrentPlan.Evals = append(s.CurrentPlan.Evals, EvalRecord{
			Round:   s.CurrentPlan.Round,
			Verdict: "PASS",
		})
		s.State = ACCEPT
		return nil
	}

	// FAIL
	eval := EvalRecord{
		Round:        s.CurrentPlan.Round,
		Verdict:      "FAIL",
		Deficiencies: in.Deficiencies,
	}
	s.CurrentPlan.Evals = append(s.CurrentPlan.Evals, eval)
	s.State = REFINE
	return nil
}

func advanceRefine(s *ScaffoldState, in AdvanceInput) error {
	if err := rejectVerdictFlag(s, in); err != nil {
		return err
	}
	if err := rejectFileFlag(s, in); err != nil {
		return err
	}
	// Record what was fixed on the last eval
	if in.Fixed != "" && len(s.CurrentPlan.Evals) > 0 {
		s.CurrentPlan.Evals[len(s.CurrentPlan.Evals)-1].Fixed = in.Fixed
	}
	s.CurrentPlan.Round++
	s.State = EVALUATE
	return nil
}

func advanceAccept(s *ScaffoldState, in AdvanceInput) error {
	completed := CompletedPlan{
		ID:          s.CurrentPlan.ID,
		Name:        s.CurrentPlan.Name,
		Domain:      s.CurrentPlan.Domain,
		File:        s.CurrentPlan.File,
		RoundsTaken: s.CurrentPlan.Round,
		Study:       s.CurrentPlan.Study,
		Evals:       s.CurrentPlan.Evals,
	}
	s.Completed = append(s.Completed, completed)
	s.CurrentPlan = nil

	if len(s.Queue) > 0 {
		s.State = ORIENT
	} else {
		s.State = DONE
	}
	return nil
}

// rejectVerdictFlag returns an error if --verdict is provided in a state that doesn't accept it.
func rejectVerdictFlag(s *ScaffoldState, in AdvanceInput) error {
	if in.Verdict != "" {
		return fmt.Errorf("--verdict is not valid in %s state", s.State)
	}
	return nil
}

// rejectFileFlag returns an error if --file is provided in a state that doesn't accept it.
func rejectFileFlag(s *ScaffoldState, in AdvanceInput) error {
	if in.File != "" {
		return fmt.Errorf("--file is not valid in %s state", s.State)
	}
	return nil
}

// ActionDescription returns guidance for the current state.
func ActionDescription(s *ScaffoldState) string {
	switch s.State {
	case ORIENT:
		if len(s.Queue) == 0 {
			return "No plans in queue."
		}
		return fmt.Sprintf("Advance to begin work on: %s (%s)", s.Queue[0].Name, s.Queue[0].Domain)

	case STUDY_SPECS:
		specs := "SPEC_MANIFEST.md"
		if len(s.CurrentPlan.Specs) > 0 {
			specs = strings.Join(s.CurrentPlan.Specs, ", ")
		}
		return fmt.Sprintf("Study the specs: %s\nReview git diffs for spec commits. Advance with --notes <summary>.", specs)

	case STUDY_CODE:
		roots := strings.Join(s.CurrentPlan.CodeSearchRoots, ", ")
		return fmt.Sprintf("Explore the codebase using sub-agents.\nSub-agents: %d. Search roots: %s.\nAdvance with --notes <summary>.", s.SubAgents, roots)

	case STUDY_PACKAGES:
		return "Study the project's technical stack: package manifests, library docs, CLAUDE.md references.\nAdvance with --notes <summary>."

	case SELECT:
		var lines []string
		lines = append(lines, "Review study findings before drafting:")
		if s.CurrentPlan.Study.SpecsNotes != "" {
			lines = append(lines, fmt.Sprintf("  Specs:    %s", s.CurrentPlan.Study.SpecsNotes))
		}
		if s.CurrentPlan.Study.CodeNotes != "" {
			lines = append(lines, fmt.Sprintf("  Code:     %s", s.CurrentPlan.Study.CodeNotes))
		}
		if s.CurrentPlan.Study.PackagesNotes != "" {
			lines = append(lines, fmt.Sprintf("  Packages: %s", s.CurrentPlan.Study.PackagesNotes))
		}
		lines = append(lines, "Advance to begin drafting.")
		return strings.Join(lines, "\n")

	case DRAFT:
		return fmt.Sprintf("Draft the implementation plan at: %s\nAdvance when ready. Use --file <path> to override output path.", s.CurrentPlan.File)

	case EVALUATE:
		return fmt.Sprintf("Run evaluation sub-agent against the plan (round %d/%d).\nAdvance with --verdict PASS --message <text> or --verdict FAIL --deficiencies <csv>.",
			s.CurrentPlan.Round, s.MaxRounds)

	case REFINE:
		deficiencies := ""
		if len(s.CurrentPlan.Evals) > 0 {
			last := s.CurrentPlan.Evals[len(s.CurrentPlan.Evals)-1]
			if len(last.Deficiencies) > 0 {
				deficiencies = strings.Join(last.Deficiencies, ", ")
			}
		}
		if deficiencies != "" {
			return fmt.Sprintf("Address deficiencies: [%s]\nEdit the plan file. Advance with --fixed <description>.", deficiencies)
		}
		return "Address evaluation feedback. Edit the plan file. Advance with --fixed <description>."

	case ACCEPT:
		return "Plan accepted. Advance to continue."

	case DONE:
		return "All plans complete. Session done."

	default:
		return ""
	}
}

// FormatState returns a structured output block for the current state.
func FormatState(s *ScaffoldState) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("State:   %s", s.State))

	if s.CurrentPlan != nil {
		lines = append(lines, fmt.Sprintf("ID:      %d", s.CurrentPlan.ID))
		lines = append(lines, fmt.Sprintf("Plan:    %s", s.CurrentPlan.Name))
		lines = append(lines, fmt.Sprintf("Domain:  %s", s.CurrentPlan.Domain))
		lines = append(lines, fmt.Sprintf("File:    %s", s.CurrentPlan.File))

		if len(s.CurrentPlan.Specs) > 0 {
			lines = append(lines, fmt.Sprintf("Specs:   %s", strings.Join(s.CurrentPlan.Specs, ", ")))
		}
		if len(s.CurrentPlan.CodeSearchRoots) > 0 && (s.State == STUDY_CODE || s.State == STUDY_SPECS || s.State == STUDY_PACKAGES) {
			lines = append(lines, fmt.Sprintf("Roots:   %s", strings.Join(s.CurrentPlan.CodeSearchRoots, ", ")))
		}
		if s.CurrentPlan.Round > 0 {
			lines = append(lines, fmt.Sprintf("Round:   %d/%d", s.CurrentPlan.Round, s.MaxRounds))
		}
		if s.State == REFINE && len(s.CurrentPlan.Evals) > 0 {
			last := s.CurrentPlan.Evals[len(s.CurrentPlan.Evals)-1]
			if len(last.Deficiencies) > 0 {
				lines = append(lines, fmt.Sprintf("Deficiencies: [%s]", strings.Join(last.Deficiencies, ", ")))
			}
		}
	} else if s.State == ORIENT && len(s.Queue) > 0 {
		lines = append(lines, fmt.Sprintf("Next:    %s (%s)", s.Queue[0].Name, s.Queue[0].Domain))
		lines = append(lines, fmt.Sprintf("Queue:   %d plan(s) remaining", len(s.Queue)))
	}

	lines = append(lines, fmt.Sprintf("Action:  %s", ActionDescription(s)))

	return strings.Join(lines, "\n")
}
