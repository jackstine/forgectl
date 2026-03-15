package state

// Phase represents the current state in the scaffold state machine.
type Phase string

const (
	PhaseOrient   Phase = "ORIENT"
	PhaseSelect   Phase = "SELECT"
	PhaseDraft    Phase = "DRAFT"
	PhaseEvaluate Phase = "EVALUATE"
	PhaseRefine   Phase = "REFINE"
	PhaseAccept   Phase = "ACCEPT"
	PhaseDone     Phase = "DONE"
)

var validPhases = map[Phase]bool{
	PhaseOrient:   true,
	PhaseSelect:   true,
	PhaseDraft:    true,
	PhaseEvaluate: true,
	PhaseRefine:   true,
	PhaseAccept:   true,
	PhaseDone:     true,
}

func (p Phase) IsValid() bool {
	return validPhases[p]
}

// QueueSpec is a spec entry in the input queue file.
type QueueSpec struct {
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

// ActiveSpec is the spec currently being worked on.
type ActiveSpec struct {
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	Topic           string   `json:"topic"`
	File            string   `json:"file"`
	PlanningSources []string `json:"planning_sources"`
	DependsOn       []string `json:"depends_on"`
	Round           int      `json:"round"`
}

// CompletedSpec is a spec that has been accepted.
type CompletedSpec struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	File        string `json:"file"`
	RoundsTaken int    `json:"rounds_taken"`
}

// ScaffoldState is the persistent state written to scaffold-state.json.
type ScaffoldState struct {
	EvaluationRounds int            `json:"evaluation_rounds"`
	UserGuided       bool           `json:"user_guided"`
	State            Phase          `json:"state"`
	CurrentSpec      *ActiveSpec    `json:"current_spec"`
	Queue            []QueueSpec    `json:"queue"`
	Completed        []CompletedSpec `json:"completed"`
}
