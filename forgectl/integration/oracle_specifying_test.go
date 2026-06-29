//go:build integration

package integration

import (
	"fmt"
	"math/rand"
	"testing"

	"forgectl/state"
)

// This file is coverage item §4.1: an independent state-machine oracle for the
// specifying phase's evaluation loop — the counter-driven decision core.
//
// The transition rules below are transcribed by hand from the documented round
// semantics (OPERATING_MANUAL.md / spec-lifecycle.md): on a PASS, accept once
// round >= min_rounds else refine; on a FAIL, force-accept once round >=
// max_rounds else refine; REFINE increments the round. They are *not* imported
// from advance.go — this table is a second, independent encoding of the
// contract. Driving forgectl through randomized-but-legal verdict sequences and
// asserting its (state, round) equals the oracle's prediction at every step is
// the single strongest guard against the binary and the documented contract
// drifting apart. Any divergence is either an implementation bug or a stale
// manual; both are release blockers.

// evalOracle predicts the next specifying state and round from the current eval
// state, the round, the verdict, and the configured round budget. It models only
// the EVALUATE <-> REFINE loop (DRAFT having already set round to 1).
type evalOracle struct {
	min, max int
}

// nextFromEvaluate returns the predicted next state given a verdict at EVALUATE.
func (o evalOracle) nextFromEvaluate(round int, verdict string) state.StateName {
	switch verdict {
	case "PASS":
		if round >= o.min {
			return state.StateAccept
		}
		return state.StateRefine
	case "FAIL":
		if round >= o.max {
			return state.StateAccept // forced
		}
		return state.StateRefine
	default:
		panic("verdict must be PASS or FAIL at EVALUATE")
	}
}

// TestOracleSpecifyingEvalLoop drives several randomized legal verdict sequences
// through the real binary and checks each transition against the independent
// oracle, including the round counter the binary persists.
func TestOracleSpecifyingEvalLoop(t *testing.T) {
	const minRounds, maxRounds = 2, 4
	oracle := evalOracle{min: minRounds, max: maxRounds}

	// A fixed seed makes the "random" walks reproducible across runs.
	rng := rand.New(rand.NewSource(0x5eed))

	for walk := 0; walk < 8; walk++ {
		t.Run(fmt.Sprintf("walk_%d", walk), func(t *testing.T) {
			p := NewProject(t)
			p.WriteConfig(fmt.Sprintf(
				"[specifying]\nbatch = 1\n[specifying.eval]\nmin_rounds = %d\nmax_rounds = %d\n[general]\nuser_guided = false\n",
				minRounds, maxRounds))
			p.WriteFile("spec-queue.json", oneSpecQueue)
			p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

			// ORIENT → SELECT → DRAFT → EVALUATE (round 1).
			p.mustForge("advance")
			p.mustForge("advance")
			p.WriteFile("specs/a.md", "# Spec A\n")
			prev := p.mustForge("advance")
			p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

			round := 1
			for step := 0; step < 50; step++ {
				// Sanity-check the binary's persisted round against the oracle.
				if got := p.State().Specifying.CurrentSpecs[0].Round; got != round {
					t.Fatalf("at EVALUATE: binary round = %d, oracle round = %d", got, round)
				}

				// Pick a legal random verdict and predict the outcome.
				verdict := "PASS"
				if rng.Intn(2) == 0 {
					verdict = "FAIL"
				}
				want := oracle.nextFromEvaluate(round, verdict)

				p.PassEval(prev, verdict)
				p.AssertAt(state.PhaseSpecifying, want)

				if want == state.StateAccept {
					// Loop terminated. The terminating round must respect the budget.
					if verdict == "PASS" && round < minRounds {
						t.Fatalf("accepted on PASS at round %d below min_rounds %d", round, minRounds)
					}
					if verdict == "FAIL" && round < maxRounds {
						t.Fatalf("force-accepted on FAIL at round %d below max_rounds %d", round, maxRounds)
					}
					return
				}

				// Otherwise we are at REFINE; advancing returns to EVALUATE with the
				// round incremented.
				if want != state.StateRefine {
					t.Fatalf("unexpected predicted state %s", want)
				}
				prev = p.mustForge("advance")
				round++
				p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)
			}
			t.Fatal("eval loop did not terminate within 50 steps")
		})
	}
}

// TestOracleSpecifyingMinRoundsLoops covers §C1 as a focused oracle case: with
// min_rounds unmet, a PASS must loop (REFINE), not accept.
func TestOracleSpecifyingMinRoundsLoops(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[specifying]\nbatch = 1\n[specifying.eval]\nmin_rounds = 2\nmax_rounds = 4\n[general]\nuser_guided = false\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.mustForge("advance") // SELECT
	p.mustForge("advance") // DRAFT
	p.WriteFile("specs/a.md", "# Spec A\n")
	eval := p.mustForge("advance") // EVALUATE round 1

	// round 1 < min_rounds 2: a PASS must NOT accept.
	p.PassEval(eval, "PASS")
	p.AssertAt(state.PhaseSpecifying, state.StateRefine)

	// round 2 >= min_rounds: now a PASS accepts.
	eval2 := p.mustForge("advance") // back to EVALUATE round 2
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)
	p.PassEval(eval2, "PASS")
	p.AssertAt(state.PhaseSpecifying, state.StateAccept)
}

// TestOracleSpecifyingForceAccept covers §C2: max_rounds reached on FAIL forces
// acceptance instead of refining forever.
func TestOracleSpecifyingForceAccept(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[specifying]\nbatch = 1\n[specifying.eval]\nmin_rounds = 1\nmax_rounds = 2\n[general]\nuser_guided = false\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.mustForge("advance") // SELECT
	p.mustForge("advance") // DRAFT
	p.WriteFile("specs/a.md", "# Spec A\n")
	eval := p.mustForge("advance") // EVALUATE round 1

	// round 1 < max_rounds 2: FAIL refines.
	p.PassEval(eval, "FAIL")
	p.AssertAt(state.PhaseSpecifying, state.StateRefine)

	// round 2 == max_rounds: FAIL force-accepts.
	eval2 := p.mustForge("advance")
	p.PassEval(eval2, "FAIL")
	p.AssertAt(state.PhaseSpecifying, state.StateAccept)
}
