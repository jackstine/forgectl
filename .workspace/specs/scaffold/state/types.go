package state

// Phase represents the current state in the scaffold state machine.
type Phase string

const (
	PhaseOrient   Phase = "ORIENT"
	PhaseSelect   Phase = "SELECT"
	PhaseDraft    Phase = "DRAFT"
	PhaseEvaluate Phase = "EVALUATE"
	PhaseRefine   Phase = "REFINE"
	PhaseReview   Phase = "REVIEW"
	PhaseAccept   Phase = "ACCEPT"
	PhaseDone     Phase = "DONE"
)

var validPhases = map[Phase]bool{
	PhaseOrient:   true,
	PhaseSelect:   true,
	PhaseDraft:    true,
	PhaseEvaluate: true,
	PhaseRefine:   true,
	PhaseReview:   true,
	PhaseAccept:   true,
	PhaseDone:     true,
}

func (p Phase) IsValid() bool {
	return validPhases[p]
}

// QueueSpec is a spec entry in the input queue file.
type QueueSpec struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	Topic           string   `json:"topic"`
	File            string   `json:"file"`
	PlanningSources []string `json:"planning_sources"`
	DependsOn       []string `json:"depends_on"`
}

// QueueInput is the schema for the --from file provided at init.
type QueueInput struct {
	Specs []QueueSpec `json:"specs"`
}

// EvalRecord captures one evaluation round's result.
type EvalRecord struct {
	Round        int      `json:"round"`
	Verdict      string   `json:"verdict"`
	Deficiencies []string `json:"deficiencies,omitempty"`
	Fixed        string   `json:"fixed,omitempty"`
}

// ActiveSpec is the spec currently being worked on.
type ActiveSpec struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	Domain          string       `json:"domain"`
	Topic           string       `json:"topic"`
	File            string       `json:"file"`
	PlanningSources []string     `json:"planning_sources"`
	DependsOn       []string     `json:"depends_on"`
	Round           int          `json:"round"`
	Evals           []EvalRecord `json:"evals,omitempty"`
}

// CompletedSpec is a spec that has been accepted.
type CompletedSpec struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Domain      string       `json:"domain"`
	File        string       `json:"file"`
	RoundsTaken int          `json:"rounds_taken"`
	CommitHash  string       `json:"commit_hash,omitempty"`
	Evals       []EvalRecord `json:"evals,omitempty"`
}

// ScaffoldState is the persistent state written to scaffold-state.json.
type ScaffoldState struct {
	MinRounds      int             `json:"min_rounds"`
	MaxRounds      int             `json:"max_rounds"`
	UserGuided     bool            `json:"user_guided"`
	State          Phase           `json:"state"`
	CurrentSpec    *ActiveSpec     `json:"current_spec"`
	Queue          []QueueSpec     `json:"queue"`
	Completed      []CompletedSpec `json:"completed"`
	LastCommitHash string          `json:"last_commit_hash,omitempty"`
}

// EvaluationRounds returns MaxRounds for backward compatibility.
func (s *ScaffoldState) EvaluationRounds() int {
	return s.MaxRounds
}
