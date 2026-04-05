package state

// PhaseName represents which phase is active.
type PhaseName string

const (
	PhaseSpecifying            PhaseName = "specifying"
	PhasePlanning              PhaseName = "planning"
	PhaseGeneratePlanningQueue PhaseName = "generate_planning_queue"
	PhaseImplementing          PhaseName = "implementing"
	PhaseReverseEngineering    PhaseName = "reverse_engineering"
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
	StateSelfReview            StateName = "SELF_REVIEW"
	// Reverse engineering phase states
	StateSurvey                StateName = "SURVEY"
	StateGapAnalysis           StateName = "GAP_ANALYSIS"
	StateDecompose             StateName = "DECOMPOSE"
	StateQueue                 StateName = "QUEUE"
	StateExecute               StateName = "EXECUTE"
	StateColleagueReview       StateName = "COLLEAGUE_REVIEW"
	StateReconcileAdvance      StateName = "RECONCILE_ADVANCE"
)

// --- Configuration structs ---
// Loaded from .forgectl/config TOML and locked into ForgeState at init time.

// AgentConfig specifies which Claude model/agent type to use for a task.
type AgentConfig struct {
	Model string `json:"model"` // "opus", "haiku", "sonnet"
	Type  string `json:"type"`  // "eval", "explore", "refine"
	Count int    `json:"count"`
}

// EvalConfig configures evaluation rounds. AgentConfig fields are embedded
// (promoted to the same JSON level) to match the flat schema in state-persistence.md.
type EvalConfig struct {
	MinRounds        int  `json:"min_rounds"`
	MaxRounds        int  `json:"max_rounds"`
	AgentConfig           // embedded: model, type, count at same JSON level
	EnableEvalOutput bool `json:"enable_eval_output"`
}

// CrossRefConfig configures cross-reference evaluation.
type CrossRefConfig struct {
	MinRounds   int         `json:"min_rounds"`
	MaxRounds   int         `json:"max_rounds"`
	AgentConfig             // embedded: model, type, count at same JSON level
	UserReview  bool        `json:"user_review"`
	Eval        AgentConfig `json:"eval"` // separate eval agent for cross-ref
}

// ReconciliationConfig configures spec reconciliation.
type ReconciliationConfig struct {
	MinRounds   int  `json:"min_rounds"`
	MaxRounds   int  `json:"max_rounds"`
	AgentConfig      // embedded: model, type, count at same JSON level
	UserReview  bool `json:"user_review"`
}

// SpecifyingConfig configures the specifying phase.
type SpecifyingConfig struct {
	Batch          int                  `json:"batch"`
	CommitStrategy string               `json:"commit_strategy"`
	Eval           EvalConfig           `json:"eval"`
	CrossReference CrossRefConfig       `json:"cross_reference"`
	Reconciliation ReconciliationConfig `json:"reconciliation"`
}

// StudyCodeConfig configures the code-study agent for planning.
type StudyCodeConfig struct {
	AgentConfig // embedded: model, type, count
}

// RefineConfig configures the plan-refinement agent.
type RefineConfig struct {
	AgentConfig // embedded: model, type, count
}

// PlanningConfig configures the planning phase.
type PlanningConfig struct {
	Batch                     int             `json:"batch"`
	CommitStrategy            string          `json:"commit_strategy"`
	SelfReview                bool            `json:"self_review"`
	PlanAllBeforeImplementing bool            `json:"plan_all_before_implementing"`
	StudyCode                 StudyCodeConfig `json:"study_code"`
	Eval                      EvalConfig      `json:"eval"`
	Refine                    RefineConfig    `json:"refine"`
}

// ImplementingConfig configures the implementing phase.
type ImplementingConfig struct {
	Batch          int        `json:"batch"`
	CommitStrategy string     `json:"commit_strategy"`
	Eval           EvalConfig `json:"eval"`
}

// SubAgentConfig specifies a sub-agent used within a reverse engineering task.
type SubAgentConfig struct {
	Model string `json:"model" toml:"model"`
	Type  string `json:"type" toml:"type"`
	Count int    `json:"count" toml:"count"`
}

// DrafterConfig configures the primary drafting agent and its exploration sub-agents.
type DrafterConfig struct {
	Model     string         `json:"model" toml:"model"`
	Subagents SubAgentConfig `json:"subagents" toml:"subagents"`
}

// SelfRefineConfig configures the self-refine execution mode.
type SelfRefineConfig struct {
	Rounds int `json:"rounds" toml:"rounds"`
}

// MultiPassConfig configures the multi-pass execution mode.
type MultiPassConfig struct {
	Passes int `json:"passes" toml:"passes"`
}

// PeerReviewConfig configures the peer-review execution mode.
type PeerReviewConfig struct {
	Reviewers int            `json:"reviewers" toml:"reviewers"`
	Rounds    int            `json:"rounds" toml:"rounds"`
	Subagents SubAgentConfig `json:"subagents" toml:"subagents"`
}

// ReconcileConfig configures the reconciliation loop after EXECUTE.
type ReconcileConfig struct {
	MinRounds       int         `json:"min_rounds" toml:"min_rounds"`
	MaxRounds       int         `json:"max_rounds" toml:"max_rounds"`
	ColleagueReview bool        `json:"colleague_review" toml:"colleague_review"`
	Eval            AgentConfig `json:"eval" toml:"eval"`
}

// ReverseEngineeringConfig configures the reverse engineering phase.
type ReverseEngineeringConfig struct {
	Mode        string            `json:"mode" toml:"mode"`
	Drafter     DrafterConfig     `json:"drafter" toml:"drafter"`
	SelfRefine  *SelfRefineConfig `json:"self_refine,omitempty" toml:"self_refine"`
	MultiPass   *MultiPassConfig  `json:"multi_pass,omitempty" toml:"multi_pass"`
	PeerReview  *PeerReviewConfig `json:"peer_review,omitempty" toml:"peer_review"`
	Reconcile   ReconcileConfig   `json:"reconcile" toml:"reconcile"`
	Survey      SubAgentConfig    `json:"survey" toml:"survey"`
	GapAnalysis SubAgentConfig    `json:"gap_analysis" toml:"gap_analysis"`
}

// DomainConfig identifies a domain within the project.
type DomainConfig struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// PathsConfig configures directory locations.
type PathsConfig struct {
	StateDir     string `json:"state_dir"`
	WorkspaceDir string `json:"workspace_dir"`
}

// LogsConfig configures activity log retention.
type LogsConfig struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days"`
	MaxFiles      int  `json:"max_files"`
}

// GeneralConfig holds top-level behavioral flags.
type GeneralConfig struct {
	EnableCommits   bool `json:"enable_commits"`
	EnableEvalOutput bool `json:"enable_eval_output"`
	UserGuided      bool `json:"user_guided"`
}

// ForgeConfig is the full project configuration loaded from .forgectl/config.
// It is locked into ForgeState at init time so all commands use consistent settings.
type ForgeConfig struct {
	General            GeneralConfig            `json:"general"`
	Domains            []DomainConfig           `json:"domains,omitempty"`
	Specifying         SpecifyingConfig         `json:"specifying"`
	Planning           PlanningConfig           `json:"planning"`
	Implementing       ImplementingConfig       `json:"implementing"`
	ReverseEngineering ReverseEngineeringConfig `json:"reverse_engineering"`
	Paths              PathsConfig              `json:"paths"`
	Logs               LogsConfig               `json:"logs"`
}

// DefaultForgeConfig returns a ForgeConfig with all spec-defined default values applied.
func DefaultForgeConfig() ForgeConfig {
	return ForgeConfig{
		General: GeneralConfig{
			EnableCommits: false,
			UserGuided:    false,
		},
		Specifying: SpecifyingConfig{
			Batch:          1,
			CommitStrategy: "all-specs",
			Eval: EvalConfig{
				MinRounds: 1,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
			},
		},
		Planning: PlanningConfig{
			Batch:          1,
			CommitStrategy: "strict",
			Eval: EvalConfig{
				MinRounds: 1,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
			},
		},
		Implementing: ImplementingConfig{
			Batch:          1,
			CommitStrategy: "scoped",
			Eval: EvalConfig{
				MinRounds: 1,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
			},
		},
		ReverseEngineering: ReverseEngineeringConfig{
			Mode: "self_refine",
			Drafter: DrafterConfig{
				Model: "opus",
				Subagents: SubAgentConfig{
					Model: "opus",
					Type:  "explorer",
					Count: 3,
				},
			},
			SelfRefine: &SelfRefineConfig{
				Rounds: 2,
			},
			Reconcile: ReconcileConfig{
				MinRounds:       1,
				MaxRounds:       3,
				ColleagueReview: false,
				Eval: AgentConfig{
					Model: "opus",
					Type:  "general-purpose",
					Count: 1,
				},
			},
			Survey: SubAgentConfig{
				Model: "haiku",
				Type:  "explorer",
				Count: 2,
			},
			GapAnalysis: SubAgentConfig{
				Model: "sonnet",
				Type:  "explorer",
				Count: 5,
			},
		},
		Paths: PathsConfig{
			StateDir:     ".forgectl/state",
			WorkspaceDir: ".forge_workspace",
		},
		Logs: LogsConfig{
			Enabled:       true,
			RetentionDays: 90,
			MaxFiles:      50,
		},
	}
}

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

// ReverseEngineeringInitInput is the schema for --from with --phase reverse_engineering.
type ReverseEngineeringInitInput struct {
	Concept string   `json:"concept"`
	Domains []string `json:"domains"`
}

// ReverseEngineeringQueueEntry is a spec entry in the reverse engineering queue file.
type ReverseEngineeringQueueEntry struct {
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	Topic           string   `json:"topic"`
	File            string   `json:"file"`
	Action          string   `json:"action"`
	CodeSearchRoots []string `json:"code_search_roots"`
	DependsOn       []string `json:"depends_on"`
}

// ReverseEngineeringQueueInput is the schema for the reverse engineering queue file.
type ReverseEngineeringQueueInput struct {
	Specs []ReverseEngineeringQueueEntry `json:"specs"`
}

// ExecuteJSONConfig is the config section of execute.json.
// Only the active mode's config block is included.
type ExecuteJSONConfig struct {
	Mode       string            `json:"mode"`
	Drafter    DrafterConfig     `json:"drafter"`
	SelfRefine *SelfRefineConfig `json:"self_refine,omitempty"`
	MultiPass  *MultiPassConfig  `json:"multi_pass,omitempty"`
	PeerReview *PeerReviewConfig `json:"peer_review,omitempty"`
}

// ExecuteJSONSpecResult is the result field of a spec entry in execute.json.
type ExecuteJSONSpecResult struct {
	Status              string  `json:"status"`
	IterationsCompleted *int    `json:"iterations_completed,omitempty"`
	Error               *string `json:"error,omitempty"`
}

// ExecuteJSONSpec is a spec entry in execute.json.
type ExecuteJSONSpec struct {
	Name            string                 `json:"name"`
	Domain          string                 `json:"domain"`
	Topic           string                 `json:"topic"`
	File            string                 `json:"file"`
	Action          string                 `json:"action"`
	CodeSearchRoots []string               `json:"code_search_roots"`
	DependsOn       []string               `json:"depends_on"`
	Result          *ExecuteJSONSpecResult `json:"result"`
}

// ExecuteJSONFile is the complete execute.json structure written by forgectl for the Python subprocess.
type ExecuteJSONFile struct {
	ProjectRoot string            `json:"project_root"`
	Config      ExecuteJSONConfig `json:"config"`
	Specs       []ExecuteJSONSpec  `json:"specs"`
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
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	DependsOn   []string   `json:"depends_on"`
	Steps       []string   `json:"steps,omitempty"`
	Files       []string   `json:"files,omitempty"`
	Specs       []string   `json:"specs,omitempty"` // spec refs, display only, #anchors OK, not validated on disk
	Refs        []string   `json:"refs,omitempty"`  // notes refs, validated on disk, relative to plan.json dir
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

// GeneratePlanningQueueState holds state for the generate_planning_queue phase.
type GeneratePlanningQueueState struct {
	PlanQueueFile string       `json:"plan_queue_file"`    // path to generated plan-queue.json
	Evals         []EvalRecord `json:"evals,omitempty"`    // reserved for future use
}

// CompletedPlan is a plan that has been accepted in the planning phase.
type CompletedPlan struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
	File   string `json:"file"`
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
	BatchNumber  int          `json:"batch_number,omitempty"`
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
	CurrentSpecs   []*ActiveSpec                   `json:"current_specs"`
	CurrentDomain  string                          `json:"current_domain,omitempty"`
	BatchNumber    int                             `json:"batch_number,omitempty"`
	Domains        map[string]DomainMeta           `json:"domains,omitempty"`
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
	CurrentPlan *ActivePlan      `json:"current_plan"`
	Round       int              `json:"round"`
	Evals       []EvalRecord     `json:"evals,omitempty"`
	Queue       []PlanQueueEntry `json:"queue"`
	Completed   []CompletedPlan  `json:"completed"`
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
	CurrentLayer    *LayerRef      `json:"current_layer"`
	BatchNumber     int            `json:"batch_number"`
	CurrentBatch    *BatchState    `json:"current_batch"`
	LayerHistory    []LayerHistory `json:"layer_history,omitempty"`
	CurrentPlanFile   string           `json:"current_plan_file,omitempty"`
	CurrentPlanDomain string           `json:"current_plan_domain,omitempty"`
	PlanQueue         []PlanQueueEntry `json:"plan_queue,omitempty"`
}

// --- Reverse engineering phase state ---

// ReverseEngineeringState holds reverse engineering phase data.
type ReverseEngineeringState struct {
	Concept         string       `json:"concept"`
	Domains         []string     `json:"domains"`
	CurrentDomain   int          `json:"current_domain"`
	TotalDomains    int          `json:"total_domains"`
	QueueFile       string       `json:"queue_file,omitempty"`
	QueueHash       string       `json:"queue_hash,omitempty"`
	ExecuteFile     string       `json:"execute_file,omitempty"`
	Round           int          `json:"round"`
	ColleagueReview bool         `json:"colleague_review"`
	ReconcileDomain int          `json:"reconcile_domain"`
	Evals           []EvalRecord `json:"evals,omitempty"`
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
	Phase                 PhaseName                   `json:"phase"`
	State                 StateName                   `json:"state"`
	Config                ForgeConfig                 `json:"config"`
	SessionID             string                      `json:"session_id,omitempty"`
	StartedAtPhase        PhaseName                   `json:"started_at_phase"`
	PhaseShift            *PhaseShiftInfo             `json:"phase_shift,omitempty"`
	Specifying            *SpecifyingState            `json:"specifying"`
	GeneratePlanningQueue *GeneratePlanningQueueState `json:"generate_planning_queue,omitempty"`
	Planning              *PlanningState              `json:"planning"`
	Implementing          *ImplementingState          `json:"implementing"`
	ReverseEngineering    *ReverseEngineeringState    `json:"reverse_engineering,omitempty"`
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
