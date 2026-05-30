package state

import "testing"

// Functional: the reverse_engineering phase state constants carry the exact
// string values defined by the spec state machine. These strings are persisted
// into the state file and matched in the advance/output switches, so the values
// must be stable.
func TestReverseEngineeringStateConstants(t *testing.T) {
	cases := map[StateName]string{
		StateSurvey:                 "SURVEY",
		StateGapAnalysis:            "GAP_ANALYSIS",
		StateDecompose:              "DECOMPOSE",
		StateQueue:                  "QUEUE",
		StateExecuteReverseEngineer: "EXECUTE_REVERSE_ENGINEER",
		StatePostReverseEngineer:    "POST_REVERSE_ENGINEER",
		StateColleagueReview:        "COLLEAGUE_REVIEW",
		StateReconcileAdvance:       "RECONCILE_ADVANCE",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("state constant = %q, want %q", got, want)
		}
	}

	// The phase constant for reverse engineering.
	if PhaseReverseEngineering != "reverse_engineering" {
		t.Errorf("PhaseReverseEngineering = %q, want reverse_engineering", PhaseReverseEngineering)
	}
}
