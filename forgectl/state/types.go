package state

// PhaseName represents which phase is active.
type PhaseName string

const (
	PhaseSpecifying   PhaseName = "specifying"
	PhasePlanning     PhaseName = "planning"
	PhaseImplementing PhaseName = "implementing"
)

// StateName represents the current state within a phase.
type StateName string

const (
	StateOrient                StateName = "ORIENT"
	StateSelect                StateName = "SELECT"
	StateDraft                 StateName = "DRAFT"
	StateEvaluate              StateName = "EVALUATE"
	StateRefine                StateName = "REFINE"
	StateAccept                StateName = "ACCEPT"
	StateDone                  StateName = "DONE"
	StateReconcile             StateName = "RECONCILE"
	StateReconcileEval         StateName = "RECONCILE_EVAL"
	StateReconcileReview       StateName = "RECONCILE_REVIEW"
	StateCrossReference        StateName = "CROSS_REFERENCE"
	StateCrossReferenceEval    StateName = "CROSS_REFERENCE_EVAL"
	StateCrossReferenceReview  StateName = "CROSS_REFERENCE_REVIEW"
	StateComplete              StateName = "COMPLETE"
	StatePhaseShift            StateName = "PHASE_SHIFT"
	StateStudySpecs            StateName = "STUDY_SPECS"
	StateStudyCode             StateName = "STUDY_CODE"
	StateStudyPackages         StateName = "STUDY_PACKAGES"
	StateReview                StateName = "REVIEW"
	StateValidate              StateName = "VALIDATE"
	StateImplement             StateName = "IMPLEMENT"
	StateCommit                StateName = "COMMIT"
)

// --- Input file schemas ---

// SpecQueueEntry is a spec in the spec queue input file.
type SpecQueueEntry struct {
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	Topic           string   `json:"topic"`
	File            string   `json:"file"`
	PlanningSources []string `json:"planning_sources"`
	DependsOn       []string `json:"depends_on"`
}

// SpecQueueInput is the schema for --from with --phase specifying.
type SpecQueueInput struct {
	Specs []SpecQueueEntry `json:"specs"`
}

// PlanQueueEntry is a plan in the plan queue input file.
type PlanQueueEntry struct {
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	File            string   `json:"file"`
	Specs           []string `json:"specs"`
	SpecCommits     []string `json:"spec_commits"`
	CodeSearchRoots []string `json:"code_search_roots"`
}

// PlanQueueInput is the schema for --from at specifying→planning phase shift.
type PlanQueueInput struct {
	Plans []PlanQueueEntry `json:"plans"`
}

// --- Plan.json schema (for implementing phase) ---

// PlanContext is the context section of plan.json.
type PlanContext struct {
	Domain string `json:"domain"`
	Module string `json:"module"`
}

// PlanRef is a reference file entry.
type PlanRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// PlanTest is a test entry within a plan item.
type PlanTest struct {
	Category    string `json:"category"`
	Description string `json:"description"`
	Passes      bool   `json:"passes"`
}

// PlanItem is an item in the plan.
type PlanItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on"`
	Steps       []string `json:"steps,omitempty"`
	Files       []string `json:"files,omitempty"`
	Spec        string   `json:"spec,omitempty"`
	Ref         string   `json:"ref,omitempty"`
	Tests       []PlanTest `json:"tests"`
	// Added during implementing init:
	Passes string `json:"passes,omitempty"`
	Rounds int    `json:"rounds,omitempty"`
}

// PlanLayerDef is a layer definition in plan.json.
type PlanLayerDef struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

// PlanJSON is the full plan.json structure.
type PlanJSON struct {
	Context PlanContext  `json:"context"`
	Refs    []PlanRef    `json:"refs,omitempty"`
	Layers  []PlanLayerDef `json:"layers"`
	Items   []PlanItem   `json:"items"`
}

// --- Eval records ---

// EvalRecord captures one evaluation round's result.
type EvalRecord struct {
	Round      int    `json:"round"`
	Verdict    string `json:"verdict"`
	EvalReport string `json:"eval_report,omitempty"`
}

// --- Specifying phase state ---

// ActiveSpec is the spec currently being worked on.
type ActiveSpec struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	Domain          string       `json:"domain"`
	Topic           string       `json:"topic"`
	File            string       `json:"file"`
	DomainPath      string       `json:"domain_path,omitempty"`
	PlanningSources []string     `json:"planning_sources"`
	DependsOn       []string     `json:"depends_on"`
	Round           int          `json:"round"`
	Evals           []EvalRecord `json:"evals,omitempty"`
}

// CompletedSpec is a spec that has been accepted.
type CompletedSpec struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Domain       string       `json:"domain"`
	File         string       `json:"file"`
	DomainPath   string       `json:"domain_path,omitempty"`
	RoundsTaken  int          `json:"rounds_taken"`
	CommitHashes []string     `json:"commit_hashes,omitempty"`
	Evals        []EvalRecord `json:"evals,omitempty"`
}

// ReconcileState tracks reconciliation after all specs complete.
type ReconcileState struct {
	Round int          `json:"round"`
	Evals []EvalRecord `json:"evals,omitempty"`
}

// DomainMeta holds metadata for a spec domain.
type DomainMeta struct {
	CodeSearchRoots []string `json:"code_search_roots"`
}

// CrossReferenceState tracks cross-reference progress.
type CrossReferenceState struct {
	Domain string       `json:"domain"`
	Round  int          `json:"round"`
	Evals  []EvalRecord `json:"evals,omitempty"`
}

// SpecifyingState holds specifying phase data.
type SpecifyingState struct {
	CurrentSpecs   []*ActiveSpec                  `json:"current_specs"`
	Domains        map[string]DomainMeta          `json:"domains,omitempty"`
	CrossReference map[string]*CrossReferenceState `json:"cross_reference,omitempty"`
	Queue          []SpecQueueEntry                `json:"queue"`
	Completed      []CompletedSpec                 `json:"completed"`
	Reconcile      *ReconcileState                 `json:"reconcile,omitempty"`
}

// --- Planning phase state ---

// ActivePlan is the plan currently being worked on.
type ActivePlan struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	Topic           string   `json:"topic"`
	File            string   `json:"file"`
	Specs           []string `json:"specs"`
	SpecCommits     []string `json:"spec_commits,omitempty"`
	CodeSearchRoots []string `json:"code_search_roots"`
}

// PlanningState holds planning phase data.
type PlanningState struct {
	CurrentPlan *ActivePlan    `json:"current_plan"`
	Round       int            `json:"round"`
	Evals       []EvalRecord   `json:"evals,omitempty"`
	Queue       []PlanQueueEntry `json:"queue"`
	Completed   []interface{}  `json:"completed"`
}

// --- Implementing phase state ---

// LayerRef identifies the current layer.
type LayerRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BatchState tracks the current batch.
type BatchState struct {
	Items            []string     `json:"items"`
	CurrentItemIndex int          `json:"current_item_index"`
	EvalRound        int          `json:"eval_round"`
	Evals            []EvalRecord `json:"evals,omitempty"`
}

// BatchHistory is a completed batch record.
type BatchHistory struct {
	BatchNumber int          `json:"batch_number"`
	Items       []string     `json:"items"`
	EvalRounds  int          `json:"eval_rounds"`
	Evals       []EvalRecord `json:"evals,omitempty"`
}

// LayerHistory is the history for a completed layer.
type LayerHistory struct {
	LayerID string         `json:"layer_id"`
	Batches []BatchHistory `json:"batches,omitempty"`
}

// ImplementingState holds implementing phase data.
type ImplementingState struct {
	CurrentLayer *LayerRef      `json:"current_layer"`
	BatchNumber  int            `json:"batch_number"`
	CurrentBatch *BatchState    `json:"current_batch"`
	LayerHistory []LayerHistory `json:"layer_history,omitempty"`
}

// --- Phase shift info ---

// PhaseShiftInfo records the from→to of a phase shift.
type PhaseShiftInfo struct {
	From PhaseName `json:"from"`
	To   PhaseName `json:"to"`
}

// --- Top-level state ---

// ForgeState is the persistent state written to forgectl-state.json.
type ForgeState struct {
	Phase          PhaseName          `json:"phase"`
	State          StateName          `json:"state"`
	Config         Config             `json:"config"`
	SessionID      string             `json:"session_id,omitempty"`
	StartedAtPhase PhaseName          `json:"started_at_phase"`
	PhaseShift     *PhaseShiftInfo    `json:"phase_shift,omitempty"`
	Specifying     *SpecifyingState   `json:"specifying"`
	Planning       *PlanningState     `json:"planning"`
	Implementing   *ImplementingState `json:"implementing"`
}

// AdvanceInput carries flags from the advance command.
type AdvanceInput struct {
	Verdict    string
	EvalReport string
	Message    string
	File       string
	From       string
	Guided     *bool // nil = not set, true/false = explicit
}
