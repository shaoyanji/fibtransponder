package segauto

import "testing"

func TestAllowedMarkerCut(t *testing.T) {
	n := New()
	// start message with 1
	n = Step(n, 1)
	if n.Mask&(1<<stIn1) == 0 {
		t.Fatalf("expected in-message")
	}
	// marker allows cut but doesn't force
	n2 := MarkerCut(n)
	if n2.Mask&(1<<stGap) == 0 {
		t.Fatalf("expected GAP to be allowed after cut")
	}
	if n2.Mask&(1<<stIn1) == 0 {
		t.Fatalf("expected still in-message")
	}
}
