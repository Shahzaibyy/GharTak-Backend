package dispatch

import "testing"

func TestSteps(t *testing.T) {
	if got := Steps(8); len(got) != 3 || got[0] != 1 || got[2] != 5 {
		t.Fatalf("steps = %v", got)
	}
	if got := Steps(2); len(got) != 1 || got[0] != 1 {
		t.Fatalf("capped steps = %v", got)
	}
	if got := Steps(0); len(got) != 0 {
		t.Fatalf("empty steps = %v", got)
	}
}
