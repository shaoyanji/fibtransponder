package fsvm

import "testing"

func TestDilateOnAdjacency(t *testing.T) {
	st := New()
	seq := []uint8{1, 0, 0, 1, 1}
	var dil uint64
	for i, b := range seq {
		var evs []Event
		st, evs = Step(st, b)
		_ = i
		for _, ev := range evs {
			if ev.Kind == EventDilate {
				dil++
			}
		}
	}
	if dil != 1 {
		t.Fatalf("expected 1 dilation, got %d", dil)
	}
	if st.R != 1 {
		t.Fatalf("expected r=1, got %d", st.R)
	}
}
