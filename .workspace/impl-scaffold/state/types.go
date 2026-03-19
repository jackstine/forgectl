package state

// Phase represents a state in the implementation plan lifecycle.
type Phase string

const (
	ORIENT         Phase = "ORIENT"
	STUDY_SPECS    Phase = "STUDY_SPECS"
	STUDY_CODE     Phase = "STUDY_CODE"
	STUDY_PACKAGES Phase = "STUDY_PACKAGES"
	SELECT         Phase = "SELECT"
	DRAFT          Phase = "DRAFT"
	EVALUATE       Phase = "EVALUATE"
	REFINE         Phase = "REFINE"
	ACCEPT         Phase = "ACCEPT"
	DONE           Phase = "DONE"
)

// QueuePlan is a plan item as read from the queue input file.
type QueuePlan struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	Topic           string   `json:"topic"`
	File            string   `json:"file"`
	Specs           []string `json:"specs"`
	CodeSearchRoots []string `json:"code_search_roots"`
}

// StudyNotes holds the architect's findings from each study phase.
type StudyNotes struct {
	SpecsNotes    string `json:"specs_notes"`
	CodeNotes     string `json:"code_notes"`
	PackagesNotes string `json:"packages_notes"`
}

// EvalRecord captures one evaluation round.
type EvalRecord struct {
	Round        int      `json:"round"`
	Verdict      string   `json:"verdict"`
	Deficiencies []string `json:"deficiencies"`
	Fixed        string   `json:"fixed"`
}

// ActivePlan is the plan currently being worked on.
type ActivePlan struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	Domain          string       `json:"domain"`
	Topic           string       `json:"topic"`
	File            string       `json:"file"`
	Specs           []string     `json:"specs"`
	CodeSearchRoots []string     `json:"code_search_roots"`
	Study           StudyNotes   `json:"study"`
	Round           int          `json:"round"`
	Evals           []EvalRecord `json:"evals"`
}

// CompletedPlan is a plan that has been accepted.
type CompletedPlan struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Domain      string       `json:"domain"`
	File        string       `json:"file"`
	RoundsTaken int          `json:"rounds_taken"`
	CommitHash  string       `json:"commit_hash"`
	Study       StudyNotes   `json:"study"`
	Evals       []EvalRecord `json:"evals"`
}

// ScaffoldState is the top-level persistent state.
type ScaffoldState struct {
	MinRounds   int            `json:"min_rounds"`
	MaxRounds   int            `json:"max_rounds"`
	SubAgents   int            `json:"sub_agents"`
	UserGuided  bool           `json:"user_guided"`
	State       Phase          `json:"state"`
	CurrentPlan *ActivePlan    `json:"current_plan"`
	Queue       []QueuePlan    `json:"queue"`
	Completed   []CompletedPlan `json:"completed"`
}

// AdvanceInput carries flags from the advance command.
type AdvanceInput struct {
	Notes        string
	File         string
	Verdict      string
	Message      string
	Deficiencies []string
	Fixed        string
}
