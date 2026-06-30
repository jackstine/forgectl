//go:build integration

package integration

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"forgectl/state"
)

// This file is the multi-domain cross-phase pipeline coverage (plan §3.B —
// B1/B4/B5/B6). Where lifecycle_specifying_test.go walks a single phase, these
// tests drive a whole plan-queue of *several* domains through their own
// planning + implementation cycles in one continuous session, including a domain
// whose plan `kind:"ui"` routes into ui_implementing instead of implementing.
//
// The hard part of a multi-domain run is that each phase has a different shape
// (planning's STUDY_* prologue, implementing's per-layer batches, ui's three
// loops) and a different eval handshake, yet they chain automatically through
// PHASE_SHIFT. Rather than hand-code a hundred advances, the tests use a small
// self-synchronizing driver (drivePipeline) that inspects the current state and
// supplies exactly the side effects that state needs — a PASS verdict + stub
// eval report at the path the phase printed, the QA step list ui_implementing
// demands — and records the (phase,state) it passed through. A test then asserts
// the whole recorded trace against an independently-encoded expectation (an
// oracle for the cross-phase chain) plus the persisted side effects.

// --- configs ---

// The ui_implementing block every pipeline config needs: the app/e2e keys must
// be non-empty or the planning→ui_implementing PHASE_SHIFT config gate (B5)
// rejects the shift. The values are inert strings — the phase surfaces them, it
// never runs them.
const pipelineUIBlock = `
[ui_implementing]
batch = 1
[ui_implementing.app]
launch_command = "echo run"
url = "http://localhost:5173"
[ui_implementing.eval]
min_rounds = 1
max_rounds = 3
[ui_implementing.qa]
min_rounds = 1
max_rounds = 3
[ui_implementing.e2e]
min_rounds = 1
max_rounds = 3
test_command = "echo e2e"
test_dir = "e2e"
`

// pipelineConfigInterleaved drives plan→impl per domain (interleaved), commits
// off. plan_all_before_implementing defaults to false.
const pipelineConfigInterleaved = `
[general]
user_guided = false
enable_commits = false
[planning.eval]
min_rounds = 1
max_rounds = 3
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 3
` + pipelineUIBlock

// --- plan-queue + plan fixtures (hand-authored, never produced by forgectl) ---

// threeDomainQueue: two code domains then one ui domain, in queue order. The
// order is the order planning must process them in.
const threeDomainQueue = `{
  "plans": [
    {"name":"Core Plan","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"},
    {"name":"Api Plan","domain":"api","file":"api/plan.json","specs":[],"spec_commits":[],"code_search_roots":["api/"],"kind":"code"},
    {"name":"Portal Plan","domain":"portal","file":"portal/plan.json","specs":[],"spec_commits":[],"code_search_roots":["portal/"],"kind":"ui"}
  ]
}`

// twoLayerCodePlan is a two-layer plan (L0→L1, b depends on a) so implementing
// runs two batches and exercises layer gating: L1 stays locked until L0 is
// terminal. No `files` — commits are off in the interleaved test, so nothing is
// staged and the item files need not exist on disk.
func twoLayerCodePlan(domain, module string) string {
	return `{
  "context": {"domain":"` + domain + `","module":"` + module + `"},
  "layers": [
    {"id":"L0","name":"Foundation","items":["a"]},
    {"id":"L1","name":"Wiring","items":["b"]}
  ],
  "items": [
    {"id":"a","name":"` + module + ` A","description":"first","depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]},
    {"id":"b","name":"` + module + ` B","description":"second","depends_on":["a"],"tests":[{"category":"rejection","description":"rejects bad input","passes":false}]}
  ]
}`
}

// uiSinglePlan is a one-item one-layer plan for the ui domain: a single batch
// through all three loops keeps the ui walk focused.
func uiSinglePlan(domain, module string) string {
	return `{
  "context": {"domain":"` + domain + `","module":"` + module + `"},
  "layers": [{"id":"L0","name":"UI","items":["u"]}],
  "items": [
    {"id":"u","name":"Login Screen","description":"login UI","depends_on":[],"tests":[{"category":"functional","description":"renders","passes":false}]}
  ]
}`
}

// --- the driver ---

// phaseState is one (phase,state) the pipeline visited.
type phaseState struct {
	Phase state.PhaseName
	State state.StateName
}

func (ps phaseState) String() string { return string(ps.Phase) + "/" + string(ps.State) }

// pipelineOpts configures drivePipeline.
type pipelineOpts struct {
	// commitMessage, when non-empty, turns the driver into its commits-on mode:
	// it passes `--message <commitMessage>:<domain>:<state>` at exactly the states
	// that require one when enable_commits is true (planning ACCEPT, first-round
	// IMPLEMENT, COMMIT). The domain/state suffix makes each commit identifiable.
	commitMessage string
	// maxSteps bounds the walk so a routing bug parks the test with a clear
	// failure instead of spinning forever.
	maxSteps int
}

// stepsPathRe pulls the QA step-list path out of a QA_TEST action block
// ("Steps:    <domain>/qa/batch-N-steps.json"). The phase requires that file to
// exist before QA_TEST can advance toward E2E_AUTHOR (invariant J1).
var stepsPathRe = regexp.MustCompile(`Steps:\s+(\S+)`)

// drivePipeline walks the state machine from wherever it currently sits until the
// session reports completion (or maxSteps is exceeded), supplying each state's
// required side effects and recording the (phase,state) visited before every
// advance. prevOut is the output of the command that produced the current state
// (init's output on the first call); the driver reads eval-report and QA-step
// paths out of it, exactly as a skill/agent would read them off the screen.
//
// It assumes every domain's plan.json already exists on disk (planning DRAFT only
// needs the file present) and that every eval verdict is PASS.
func (p *Project) drivePipeline(prevOut string, opts pipelineOpts) ([]phaseState, Result) {
	p.t.Helper()
	if opts.maxSteps == 0 {
		opts.maxSteps = 200
	}
	var trace []phaseState
	var last Result
	for i := 0; i < opts.maxSteps; i++ {
		s := p.State()
		trace = append(trace, phaseState{s.Phase, s.State})

		args := p.pipelineArgs(s, prevOut, opts)
		last = p.forge(args...)
		if last.Exit != 0 {
			// The only legitimate non-zero exit is advancing past the final DONE,
			// which forgectl reports as "session complete."
			if strings.Contains(last.Out(), "session complete") {
				return trace, last
			}
			p.t.Fatalf("pipeline stuck at %s: `advance %s` exited %d\nstdout:\n%s\nstderr:\n%s",
				trace[len(trace)-1], strings.Join(args[1:], " "), last.Exit, last.Stdout, last.Stderr)
		}
		prevOut = last.Out()
	}
	p.t.Fatalf("pipeline did not complete within %d steps; trace:\n%v", opts.maxSteps, trace)
	return trace, last
}

// pipelineArgs decides the advance invocation for the current state: a PASS-with-
// report for eval states, the QA handshake for QA_TEST, a --message where commits
// require one, and a plain advance everywhere else.
func (p *Project) pipelineArgs(s *state.ForgeState, prevOut string, opts pipelineOpts) []string {
	p.t.Helper()
	commits := opts.commitMessage != ""

	// msg builds an identifiable commit message for a commit point.
	msg := func(domain string) []string {
		return []string{"advance", "--message", opts.commitMessage + ":" + domain + ":" + string(s.State)}
	}
	// passReport stubs a report at `path` and advances PASS with it.
	passReport := func(path string) []string {
		p.WriteReport(path)
		return []string{"advance", "--verdict", "PASS", "--eval-report", path}
	}

	switch s.Phase {
	case state.PhasePlanning:
		switch s.State {
		case state.StateEvaluate:
			// Planning surfaces a concrete report path in its action block.
			return passReport(p.ExtractReportPath(prevOut))
		case state.StateAccept:
			if commits {
				return msg(s.Planning.CurrentPlan.Domain)
			}
		}

	case state.PhaseImplementing:
		switch s.State {
		case state.StateEvaluate:
			return passReport(p.ExtractReportPath(prevOut))
		case state.StateImplement, state.StateCommit:
			if commits {
				return msg(s.Implementing.CurrentPlanDomain)
			}
		}

	case state.PhaseUIImplementing:
		switch s.State {
		case state.StateEvaluate, state.StateE2EVerify:
			// ui_implementing's eval/e2e action blocks print a `<path>` placeholder,
			// not a concrete path — the path is in `forgectl eval`'s REPORT OUTPUT.
			// --eval-report only checks the file exists, so any stub path works.
			return passReport("ui-evals/" + strings.ToLower(string(s.State)) + ".md")
		case state.StateQATest:
			// The QA evaluator must write the e2e step list at the path the phase
			// named before QA_TEST will advance toward E2E_AUTHOR.
			p.writeQASteps(extractMatch(p.t, stepsPathRe, prevOut, "QA steps path"))
			return passReport("ui-evals/qa.md")
		case state.StateImplement, state.StateCommit:
			if commits {
				return msg(s.UIImplementing.CurrentPlanDomain)
			}
		}
	}
	return []string{"advance"}
}

// writeQASteps writes a minimal valid QA step list (one scenario) at the given
// project-relative path so QA_TEST can advance.
func (p *Project) writeQASteps(rel string) {
	p.t.Helper()
	p.WriteFile(rel, `{"batch":1,"round":1,"scenarios":[`+
		`{"id":"s1","name":"smoke","preconditions":[],"steps":["open the app"],"expected":["it renders"],"priority":"normal"}`+
		`]}`)
}

// extractMatch returns the first capture group of re in s, failing the test if
// absent. (ExtractReportPath is the equivalent for eval-report paths.)
func extractMatch(t *testing.T, re *regexp.Regexp, s, what string) string {
	t.Helper()
	m := re.FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("no %s found in output:\n%s", what, s)
	}
	return m[1]
}

// --- trace oracle ---

// Canonical per-phase state sequences, encoded here independently of advance.go
// so a mismatch flags either a binary regression or a stale expectation. Each is
// the ordered list of states the driver passes *through* (records before each
// advance) for one domain, ending at the PHASE_SHIFT that hands off to the next
// domain — except the ui sequence, which ends at the terminal DONE.
var (
	planningSeq = []state.StateName{
		state.StateOrient, state.StateStudySpecs, state.StateStudyCode,
		state.StateStudyPackages, state.StateReview, state.StateDraft,
		state.StateEvaluate, state.StateAccept, state.StatePhaseShift,
	}
	// Two-layer implementing: ORIENT→IMPLEMENT→EVALUATE→COMMIT for L0, the same
	// for L1, then DONE→PHASE_SHIFT.
	twoLayerImplSeq = []state.StateName{
		state.StateOrient, state.StateImplement, state.StateEvaluate, state.StateCommit,
		state.StateOrient, state.StateImplement, state.StateEvaluate, state.StateCommit,
		state.StateDone, state.StatePhaseShift,
	}
	// One-batch ui_implementing through all three loops to the terminal DONE.
	uiBatchSeq = []state.StateName{
		state.StateOrient, state.StateImplement, state.StateEvaluate,
		state.StateQATest, state.StateE2EAuthor, state.StateE2EVerify,
		state.StateCommit, state.StateDone,
	}
)

func seq(phase state.PhaseName, states []state.StateName) []phaseState {
	out := make([]phaseState, len(states))
	for i, st := range states {
		out[i] = phaseState{phase, st}
	}
	return out
}

// --- tests ---

// TestPipelineMultiDomainInterleaved is the headline scenario: a plan-queue of
// three domains — two code, one ui — driven through their own planning and
// implementation cycles in one continuous session, interleaved (plan a domain,
// implement it, move to the next). It asserts the whole cross-phase trace against
// the independent oracle, that every domain's plan reaches `completed[]` in queue
// order, that every item ends `passed`, and — the routing contract — that the
// `kind:"ui"` domain enters ui_implementing (not implementing) and runs all three
// loops. Commits are off here; the git side-channel is covered by
// TestPipelineMultiDomainCodeCommits. (Plan B1/B4.)
func TestPipelineMultiDomainInterleaved(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(pipelineConfigInterleaved)
	p.WriteFile("plan-queue.json", threeDomainQueue)
	p.WriteFile("core/plan.json", twoLayerCodePlan("core", "Core"))
	p.WriteFile("api/plan.json", twoLayerCodePlan("api", "Api"))
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	res := p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	p.AssertAt(state.PhasePlanning, state.StateOrient)

	trace, final := p.drivePipeline(res.Out(), pipelineOpts{})

	// The full cross-phase trace: core (plan+impl) → api (plan+impl) → portal
	// (plan+ui). Two code domains end their implementing at PHASE_SHIFT (handing
	// off to the next domain); the ui domain ends at the terminal DONE.
	var want []phaseState
	want = append(want, seq(state.PhasePlanning, planningSeq)...)
	want = append(want, seq(state.PhaseImplementing, twoLayerImplSeq)...)
	want = append(want, seq(state.PhasePlanning, planningSeq)...)
	want = append(want, seq(state.PhaseImplementing, twoLayerImplSeq)...)
	want = append(want, seq(state.PhasePlanning, planningSeq)...)
	want = append(want, seq(state.PhaseUIImplementing, uiBatchSeq)...)
	assertTrace(t, trace, want)

	// The session ended cleanly at the final DONE.
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("final advance was not the session-complete signal:\n%s", final.Out())
	}

	// All three plans completed, in queue order.
	s := p.State()
	var doneDomains []string
	for _, c := range s.Planning.Completed {
		doneDomains = append(doneDomains, c.Domain)
	}
	if want := []string{"core", "api", "portal"}; !equalStrings(doneDomains, want) {
		t.Errorf("completed plan domains = %v, want %v", doneDomains, want)
	}

	// Every domain implemented every item to `passed`.
	assertAllItemsPassed(t, p, "core/plan.json")
	assertAllItemsPassed(t, p, "api/plan.json")
	assertAllItemsPassed(t, p, "portal/plan.json")
}

// TestPipelineMultiDomainCodeCommits adds the git third side-channel to the
// pipeline: two code domains, interleaved, with commits on. Each domain produces
// three commits carrying our identifiable messages — planning ACCEPT (strict:
// plan.json + notes/), the first-round per-item commit (at IMPLEMENT), and the
// batch commit (at COMMIT) — with staged paths scoped to the domain. (Single-item
// plans keep the commit count deterministic; the multi-layer cycle is covered by
// the interleaved test above.) The ui domain is intentionally excluded here — see
// TestUIImplementingCommitsWhenEnabled for why. (Plan F2/F4.)
func TestPipelineMultiDomainCodeCommits(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = true
[planning.eval]
min_rounds = 1
max_rounds = 3
[implementing]
batch = 2
commit_strategy = "scoped"
[implementing.eval]
min_rounds = 1
max_rounds = 3
`)
	p.WriteFile("plan-queue.json", `{
  "plans": [
    {"name":"Core Plan","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"},
    {"name":"Api Plan","domain":"api","file":"api/plan.json","specs":[],"spec_commits":[],"code_search_roots":["api/"],"kind":"code"}
  ]
}`)
	// Commits stage real files, so the per-item `files` must exist on disk; the
	// planning commit's strict strategy also stages an adjacent notes/ directory,
	// which git can only add if it is non-empty.
	for _, d := range []string{"core", "api"} {
		p.WriteFile(d+"/plan.json", oneItemCodePlanWithFiles(d, strings.ToUpper(d[:1])+d[1:]))
		p.WriteFile(d+"/main.go", "package "+d+"\n")
		p.WriteFile(d+"/notes/study.md", "# notes\n")
	}

	res := p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	p.drivePipeline(res.Out(), pipelineOpts{commitMessage: "wip"})

	// Three commits per code domain (6 total): planning ACCEPT, the per-item
	// first-round commit, and the batch commit.
	log := p.GitLog()
	if len(log) != 6 {
		t.Fatalf("git log has %d commits, want 6:\n%v", len(log), log)
	}
	for _, d := range []string{"core", "api"} {
		accept := countContains(log, "wip:"+d+":ACCEPT")
		impl := countContains(log, "wip:"+d+":IMPLEMENT")
		commit := countContains(log, "wip:"+d+":COMMIT")
		if accept != 1 || impl != 1 || commit != 1 {
			t.Errorf("domain %s: ACCEPT=%d (want 1), IMPLEMENT=%d (want 1), COMMIT=%d (want 1)\nlog: %v",
				d, accept, impl, commit, log)
		}
	}

	// HEAD is api's last (L1) batch commit under the scoped strategy: it stages
	// api/ paths and nothing from core.
	stat := p.GitShowStat()
	if !strings.Contains(stat, "api/") {
		t.Errorf("HEAD commit did not stage any api/ path:\n%s", stat)
	}
	if strings.Contains(stat, "core/") {
		t.Errorf("HEAD (api batch) commit leaked core/ paths:\n%s", stat)
	}
}

// TestPipelinePlanAllBeforeImplementing pins the documented all-planning-first
// contract (plan-production.md §"ACCEPT/DONE", lines 656–658): with
// plan_all_before_implementing=true, forgectl should plan EVERY domain (planning
// → planning domain boundaries), then, once the planning queue empties, DONE →
// PHASE_SHIFT and implement EVERY completed plan in turn. The assertion below is
// that every domain's items end `passed`.
func TestPipelinePlanAllBeforeImplementing(t *testing.T) {

	p := NewProject(t)
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[planning]
plan_all_before_implementing = true
[planning.eval]
min_rounds = 1
max_rounds = 3
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 3
`)
	p.WriteFile("plan-queue.json", `{
  "plans": [
    {"name":"Core Plan","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"},
    {"name":"Api Plan","domain":"api","file":"api/plan.json","specs":[],"spec_commits":[],"code_search_roots":["api/"],"kind":"code"}
  ]
}`)
	p.WriteFile("core/plan.json", twoLayerCodePlan("core", "Core"))
	p.WriteFile("api/plan.json", twoLayerCodePlan("api", "Api"))

	res := p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	p.drivePipeline(res.Out(), pipelineOpts{})

	// The contract: both domains were implemented, not just the last.
	assertAllItemsPassed(t, p, "core/plan.json")
	assertAllItemsPassed(t, p, "api/plan.json")
}

// TestUIImplementingCommitsWhenEnabled pins that ui_implementing actually commits
// when enable_commits is true — symmetric with the implementing phase, which the
// per-domain commit test above proves does commit.
func TestUIImplementingCommitsWhenEnabled(t *testing.T) {

	p := NewProject(t)
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = true
` + pipelineUIBlock)
	p.WriteFile("portal/plan.json", uiSinglePlanWithFiles("portal", "Portal"))
	p.WriteFile("portal/login.tsx", "export const Login = () => null\n")

	res := p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.drivePipeline(res.Out(), pipelineOpts{commitMessage: "ui"})

	if log := p.GitLog(); len(log) == 0 {
		t.Fatal("ui_implementing made no commits with enable_commits=true; --message was required but ignored")
	}
}

// --- fixtures with files (for the commits-on tests) ---

func oneItemCodePlanWithFiles(domain, module string) string {
	return `{
  "context": {"domain":"` + domain + `","module":"` + module + `"},
  "layers": [{"id":"L0","name":"Foundation","items":["a"]}],
  "items": [
    {"id":"a","name":"` + module + ` A","description":"only item","files":["` + domain + `/main.go"],"depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]}
  ]
}`
}

func uiSinglePlanWithFiles(domain, module string) string {
	return `{
  "context": {"domain":"` + domain + `","module":"` + module + `"},
  "layers": [{"id":"L0","name":"UI","items":["u"]}],
  "items": [
    {"id":"u","name":"Login Screen","description":"login UI","files":["` + domain + `/login.tsx"],"depends_on":[],"tests":[{"category":"functional","description":"renders","passes":false}]}
  ]
}`
}

// --- assertions / small helpers ---

// assertTrace compares the recorded (phase,state) trace to the expected sequence
// element by element, reporting the first divergence with surrounding context.
func assertTrace(t *testing.T, got, want []phaseState) {
	t.Helper()
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			t.Fatalf("trace diverges at step %d: got %s, want %s\n  got around:  %v\n  want around: %v",
				i, got[i], want[i], window(got, i), window(want, i))
		}
	}
	if len(got) != len(want) {
		t.Fatalf("trace length = %d, want %d\n  got tail:  %v\n  want tail: %v",
			len(got), len(want), tail(got, len(want)), tail(want, len(got)))
	}
}

// planItemsFile reads just the item id/passes pairs out of a plan.json.
type planItemsFile struct {
	Items []struct {
		ID     string `json:"id"`
		Passes string `json:"passes"`
	} `json:"items"`
}

// assertAllItemsPassed fails unless every item in the plan ended `passed`.
func assertAllItemsPassed(t *testing.T, p *Project, planRel string) {
	t.Helper()
	var pf planItemsFile
	if err := json.Unmarshal([]byte(p.ReadFile(planRel)), &pf); err != nil {
		t.Fatalf("parsing %s: %v", planRel, err)
	}
	if len(pf.Items) == 0 {
		t.Fatalf("%s has no items", planRel)
	}
	for _, it := range pf.Items {
		if it.Passes != "passed" {
			t.Errorf("%s item %q passes = %q, want \"passed\"", planRel, it.ID, it.Passes)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func countContains(haystack []string, sub string) int {
	n := 0
	for _, s := range haystack {
		if strings.Contains(s, sub) {
			n++
		}
	}
	return n
}

// window returns up to two elements either side of index i, for error context.
func window(ps []phaseState, i int) []phaseState {
	lo, hi := i-2, i+3
	if lo < 0 {
		lo = 0
	}
	if hi > len(ps) {
		hi = len(ps)
	}
	return ps[lo:hi]
}

func tail(ps []phaseState, from int) []phaseState {
	if from < 0 {
		from = 0
	}
	if from > len(ps) {
		from = len(ps)
	}
	return ps[from:]
}

// TestB5UIConfigGateAtPhaseShift covers the config-gate that guards the
// planning→ui_implementing PHASE_SHIFT: when a plan with kind="ui" finishes
// planning and reaches PHASE_SHIFT, forgectl validates that
// ui_implementing.app.url (and other required fields) are set. If they are
// absent, the advance must exit non-zero, leave the state at PHASE_SHIFT, and
// name the missing field in its output.
func TestB5UIConfigGateAtPhaseShift(t *testing.T) {
	p := NewProject(t)
	// Config intentionally omits [ui_implementing.app] (no url or launch_command).
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[planning.eval]
min_rounds = 1
max_rounds = 3
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 3
[ui_implementing]
batch = 1
[ui_implementing.eval]
min_rounds = 1
max_rounds = 3
[ui_implementing.qa]
min_rounds = 1
max_rounds = 3
[ui_implementing.e2e]
min_rounds = 1
max_rounds = 3
test_command = "echo e2e"
test_dir = "e2e"
`)
	const b5PlanQueue = `{
  "plans": [
    {"name":"Portal Plan","domain":"portal","file":"portal/plan.json","specs":[],"spec_commits":[],"code_search_roots":["portal/"],"kind":"ui"}
  ]
}`
	p.WriteFile("plan-queue.json", b5PlanQueue)
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	p.AssertAt(state.PhasePlanning, state.StateOrient)

	// Drive planning through its full sequence:
	// ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT
	p.mustForge("advance") // ORIENT → STUDY_SPECS
	p.mustForge("advance") // STUDY_SPECS → STUDY_CODE
	p.mustForge("advance") // STUDY_CODE → STUDY_PACKAGES
	p.mustForge("advance") // STUDY_PACKAGES → REVIEW
	p.mustForge("advance") // REVIEW → DRAFT

	// DRAFT → EVALUATE (plan file already on disk)
	evalOut := p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateEvaluate)

	// EVALUATE → ACCEPT (PASS)
	p.PassEval(evalOut, "PASS")
	p.AssertAt(state.PhasePlanning, state.StateAccept)

	// ACCEPT → PHASE_SHIFT (commits off, no --message needed)
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StatePhaseShift)

	// Advance from PHASE_SHIFT: must fail because ui_implementing.app.url is missing.
	res := p.forge("advance")
	if res.Exit == 0 {
		t.Error("expected non-zero exit when ui_implementing.app.url is missing")
	}

	// State must remain at PHASE_SHIFT.
	p.AssertAt(state.PhasePlanning, state.StatePhaseShift)

	// Error output must mention the missing field.
	if !strings.Contains(res.Out(), "url") {
		t.Errorf("error should mention missing url field: %s", res.Out())
	}
}
