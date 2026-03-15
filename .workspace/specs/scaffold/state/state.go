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
		return nil, fmt.Errorf("invalid state in state file: %q. Valid states: ORIENT, SELECT, DRAFT, EVALUATE, REFINE, ACCEPT, DONE", s.State)
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
func NewState(rounds int, userGuided bool, specs []QueueSpec) *ScaffoldState {
	return &ScaffoldState{
		EvaluationRounds: rounds,
		UserGuided:       userGuided,
		State:            PhaseOrient,
		CurrentSpec:      nil,
		Queue:            specs,
		Completed:        []CompletedSpec{},
	}
}

// Advance transitions the state machine. Returns an error if the transition is invalid.
func Advance(s *ScaffoldState, file string, verdict string) error {
	switch s.State {
	case PhaseOrient:
		if file != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}
		if len(s.Queue) == 0 && s.CurrentSpec == nil {
			return fmt.Errorf("no specs in queue")
		}
		// Pull next from queue into current_spec.
		next := s.Queue[0]
		s.Queue = s.Queue[1:]
		s.CurrentSpec = &ActiveSpec{
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
		if file != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}
		s.State = PhaseDraft

	case PhaseDraft:
		if verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}
		if file == "" {
			return fmt.Errorf("DRAFT state requires --file <path>")
		}
		s.CurrentSpec.File = file
		s.CurrentSpec.Round = 1
		s.State = PhaseEvaluate

	case PhaseEvaluate:
		if file != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if verdict == "" {
			return fmt.Errorf("EVALUATE state requires --verdict PASS or --verdict FAIL")
		}
		if verdict != "PASS" && verdict != "FAIL" {
			return fmt.Errorf("invalid verdict: %q. Use PASS or FAIL", verdict)
		}
		if verdict == "PASS" {
			s.State = PhaseAccept
		} else {
			if s.CurrentSpec.Round >= s.EvaluationRounds {
				s.State = PhaseAccept
			} else {
				s.State = PhaseRefine
			}
		}

	case PhaseRefine:
		if file != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}
		s.CurrentSpec.Round++
		s.State = PhaseEvaluate

	case PhaseAccept:
		if file != "" {
			return fmt.Errorf("--file is only valid in DRAFT state. Current state: %s", s.State)
		}
		if verdict != "" {
			return fmt.Errorf("--verdict is only valid in EVALUATE state. Current state: %s", s.State)
		}
		if s.CurrentSpec == nil {
			return fmt.Errorf("no active spec. Run 'next' to see queue status")
		}
		// Move current to completed.
		s.Completed = append(s.Completed, CompletedSpec{
			Name:        s.CurrentSpec.Name,
			Domain:      s.CurrentSpec.Domain,
			File:        s.CurrentSpec.File,
			RoundsTaken: s.CurrentSpec.Round,
		})
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
		return "Write the spec file following SPEC_FORMAT.md. Advance with --file <path>."
	case PhaseEvaluate:
		if s.CurrentSpec.Round >= s.EvaluationRounds {
			return "Final evaluation round. Spawn Opus evaluation sub-agent. If FAIL, present to user for final decision."
		}
		return "Spawn Opus evaluation sub-agent for this spec."
	case PhaseRefine:
		return "Address deficiencies from evaluation. Edit the spec file, then advance."
	case PhaseAccept:
		return "Spec finalized. Advance to move to next spec or complete session."
	case PhaseDone:
		return "All specs complete."
	default:
		return "Unknown state."
	}
}
