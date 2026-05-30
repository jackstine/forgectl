package state

import "testing"

// Functional: HashBytes is deterministic, sensitive to content changes, and
// returns a 64-char sha256 hex digest.
func TestHashBytes(t *testing.T) {
	a := HashBytes([]byte(`{"specs": []}`))
	b := HashBytes([]byte(`{"specs": []}`))
	if a != b {
		t.Errorf("HashBytes not deterministic: %q != %q", a, b)
	}
	if len(a) != 64 {
		t.Errorf("digest length = %d, want 64", len(a))
	}

	c := HashBytes([]byte(`{"specs": [1]}`))
	if a == c {
		t.Error("different content produced the same hash")
	}

	// Known sha256 of the empty input.
	if got := HashBytes(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("sha256 of empty input = %q", got)
	}
}
