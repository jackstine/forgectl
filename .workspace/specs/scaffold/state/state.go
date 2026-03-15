package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const StateFileName = "scaffold-state.json"

// StatePath returns the path to the state file in the given directory.
func StatePath(dir string) string {
	return filepath.Join(dir, StateFileName)
}

// Exists checks if the state file exists in the given directory.
func Exists(dir string) bool {
	_, err := os.Stat(StatePath(dir))
	return err == nil
}

// Load reads and parses the state file.
func Load(dir string) (*ScaffoldState, error) {
	data, err := os.ReadFile(StatePath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no state file found. Run 'scaffold init' first")
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s ScaffoldState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("state file is corrupt: %s", err)
	}

	if !s.State.IsValid() {
		return nil, fmt.Errorf("invalid state in state file: %q. Valid states: ORIENT, SELECT, DRAFT, EVALUATE, REFINE, REVIEW, ACCEPT, DONE", s.State)
	}

	return &s, nil
}

// Save writes the state file atomically.
func Save(dir string, s *ScaffoldState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(StatePath(dir), data, 0644)
}

// NewState creates an initial scaffold state from validated input.
// Assigns sequential IDs (1-indexed) to each spec.
func NewState(minRounds, maxRounds int, userGuided bool, specs []QueueSpec) *ScaffoldState {
	for i := range specs {
		specs[i].ID = i + 1
	}
	return &ScaffoldState{
		MinRounds:  minRounds,
		MaxRounds:  maxRounds,
		UserGuided: userGuided,
		State:      PhaseOrient,
		Queue:      specs,
		Completed:  []CompletedSpec{},
	}
}

// AdvanceInput holds all parameters for a state transition.
type AdvanceInput struct {
	File         string
	Verdict      string
	Deficiencies []string
	Fixed        string
}

// Advance transitions the state machine. Returns an error if the transition is invalid.
func Advance(s *ScaffoldState, in AdvanceInput) error {
	switch s.State {
	case PhaseOrient:
		if err := rejectFlags(s.State, in); err != nil {
			return err
		}
		if len(s.Queue) == 0 && s.CurrentSpec == nil {
			return fmt.Errorf("no specs in queue")
		}
		next := s.Queue[0]
		s.Queue = s.Queue[1:]
		s.CurrentSpec = &ActiveSpec{
			ID:              next.ID,
			Name:            next.Name,
			Domain:          next.Domain,
			Topic:           next.Topic,
			File:            next.File,
			PlanningSources: next.PlanningSources,
			DependsOn:       next.DependsOn,
			Round:           0,
		}
		s.State = PhaseSelect

	case PhaseSelect:
		if err := rejectFlags(s.State, in); err != nil {
			return err
		}
		s.State = PhaseDraft

	case PhaseDraft:
		if in.Verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}
		if in.File != "" {
			s.CurrentSpec.File = file(in)
		}
		s.CurrentSpec.Round = 1
		s.State = PhaseEvaluate

	case PhaseEvaluate:
		if in.File != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if in.Verdict == "" {
			return fmt.Errorf("EVALUATE state requires --verdict PASS or --verdict FAIL")
		}
		if in.Verdict != "PASS" && in.Verdict != "FAIL" {
			return fmt.Errorf("invalid verdict: %q. Use PASS or FAIL", in.Verdict)
		}

		// Record eval result.
		eval := EvalRecord{
			Round:        s.CurrentSpec.Round,
			Verdict:      in.Verdict,
			Deficiencies: in.Deficiencies,
		}
		s.CurrentSpec.Evals = append(s.CurrentSpec.Evals, eval)

		if in.Verdict == "PASS" {
			s.State = PhaseAccept
		} else {
			if s.CurrentSpec.Round >= s.MaxRounds {
				// Max rounds reached — go to REVIEW for human decision.
				s.State = PhaseReview
			} else if s.CurrentSpec.Round >= s.MinRounds {
				// Past min rounds — go to REVIEW.
				s.State = PhaseReview
			} else {
				// Under min rounds — auto-refine.
				s.State = PhaseRefine
			}
		}

	case PhaseRefine:
		if in.File != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if in.Verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}

		// Record what was fixed on the last eval.
		if in.Fixed != "" && len(s.CurrentSpec.Evals) > 0 {
			s.CurrentSpec.Evals[len(s.CurrentSpec.Evals)-1].Fixed = in.Fixed
		}

		s.CurrentSpec.Round++
		s.State = PhaseEvaluate

	case PhaseReview:
		if in.File != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		// In REVIEW, the architect can:
		// --verdict PASS: accept as-is → ACCEPT
		// --verdict FAIL: request another round → REFINE
		// (no verdict): just advance → ACCEPT (user accepted)
		if in.Verdict == "FAIL" {
			// User grants extra round.
			s.State = PhaseRefine
		} else {
			// PASS or no verdict — accept.
			s.State = PhaseAccept
		}

	case PhaseAccept:
		if err := rejectFlags(s.State, in); err != nil {
			return err
		}
		if s.CurrentSpec == nil {
			return fmt.Errorf("no active spec. Run 'next' to see queue status")
		}
		s.Completed = append(s.Completed, CompletedSpec{
			ID:          s.CurrentSpec.ID,
			Name:        s.CurrentSpec.Name,
			Domain:      s.CurrentSpec.Domain,
			File:        s.CurrentSpec.File,
			RoundsTaken: s.CurrentSpec.Round,
			CommitHash:  s.LastCommitHash,
			Evals:       s.CurrentSpec.Evals,
		})
		s.LastCommitHash = ""
		s.CurrentSpec = nil
		if len(s.Queue) == 0 {
			s.State = PhaseDone
		} else {
			s.State = PhaseOrient
		}

	case PhaseDone:
		return fmt.Errorf("all specs complete. Nothing to advance")

	default:
		return fmt.Errorf("unknown state: %s", s.State)
	}

	return nil
}

func file(in AdvanceInput) string {
	return in.File
}

// rejectFlags returns an error if file or verdict flags are set in a state that doesn't use them.
func rejectFlags(phase Phase, in AdvanceInput) error {
	if in.File != "" {
		return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", phase)
	}
	if in.Verdict != "" {
		return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", phase)
	}
	return nil
}

// ActionDescription returns a human-readable description of what the architect
// should do in the current state.
func ActionDescription(s *ScaffoldState) string {
	switch s.State {
	case PhaseOrient:
		if len(s.Queue) > 0 {
			return fmt.Sprintf("Read planning docs and existing specs. Next up: %q (%s)", s.Queue[0].Name, s.Queue[0].Domain)
		}
		return "Read planning docs and existing specs."
	case PhaseSelect:
		if s.UserGuided {
			return "Discuss topic with user. Resolve open questions before advancing."
		}
		return "Review topic. Advance when ready to draft."
	case PhaseDraft:
		return "Write the spec file following SPEC_FORMAT.md. Advance when done."
	case PhaseEvaluate:
		if s.CurrentSpec.Round >= s.MaxRounds {
			return "Final evaluation round. Spawn Opus evaluation sub-agent."
		}
		return "Spawn Opus evaluation sub-agent for this spec."
	case PhaseRefine:
		desc := "Address deficiencies from evaluation. Edit the spec file."
		if len(s.CurrentSpec.Evals) > 0 {
			last := s.CurrentSpec.Evals[len(s.CurrentSpec.Evals)-1]
			if len(last.Deficiencies) > 0 {
				desc += fmt.Sprintf(" Deficiencies: %v.", last.Deficiencies)
			}
		}
		desc += " Advance with --fixed <description>."
		return desc
	case PhaseReview:
		desc := "Max evaluation rounds reached. Review deficiencies and fixes."
		if len(s.CurrentSpec.Evals) > 0 {
			last := s.CurrentSpec.Evals[len(s.CurrentSpec.Evals)-1]
			if len(last.Deficiencies) > 0 {
				desc += fmt.Sprintf(" Last deficiencies: %v.", last.Deficiencies)
			}
		}
		desc += " Advance to accept, or --verdict FAIL to grant another round."
		return desc
	case PhaseAccept:
		return "Spec finalized. Advance to move to next spec or complete session."
	case PhaseDone:
		return "All specs complete."
	default:
		return "Unknown state."
	}
}
