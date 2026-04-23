package segauto

import (
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

func TestNFAStartsInGap(t *testing.T) {
	n := New()
	if (n.Mask & (1 << stGap)) == 0 {
		t.Error("NFA should start in GAP state")
	}
}

func TestNFATransitions(t *testing.T) {
	n := New()
	// Stream: 1 → IN1
	n.ProcessBit(1, fsvm.State{}, 0, nil)
	if (n.Mask & (1 << stIn1)) == 0 {
		t.Error("after bit 1, should be in IN1")
	}

	// Stream: 0 → IN0
	n.ProcessBit(0, fsvm.State{}, 0, nil)
	if (n.Mask & (1 << stIn0)) == 0 {
		t.Error("after bit 0, should be in IN0")
	}

	// Stream: 0 → IN0
	n.ProcessBit(0, fsvm.State{}, 0, nil)
	if (n.Mask & (1 << stIn0)) == 0 {
		t.Error("after another bit 0, should still be in IN0")
	}
}

func TestMarkerCutCreatesSegment(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 10, MaxExemplars: 2})
	// Stream: 1,1, then marker event
	// Two consecutive 1s produce a dilation, not a marker.
	// Let's emit a marker manually.
	n.ProcessBit(1, fsvm.State{}, 0, nil)
	n.ProcessBit(1, fsvm.State{}, 0, nil)
	n.ProcessBit(0, fsvm.State{}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	segs := n.Segments()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment after marker cut, got %d", len(segs))
	}
	if segs[0].Start != 0 {
		t.Errorf("segment should start at 0, got %d", segs[0].Start)
	}
	if segs[0].End != 3 {
		t.Errorf("segment should end at 3, got %d", segs[0].End)
	}
}

func TestBudgetCap(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 2, MaxExemplars: 2})
	for i := 0; i < 10; i++ {
		n.ProcessBit(1, fsvm.State{}, 0, nil)
		n.ProcessBit(0, fsvm.State{}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})
	}
	if len(n.segments) > 2 {
		t.Errorf("expected at most 2 segments due to budget, got %d", len(n.segments))
	}
}

func TestExemplars(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 10, MaxExemplars: 4})
	// Create a segment from bit 0 to bit 100
	for i := 0; i < 100; i++ {
		n.ProcessBit(1, fsvm.State{}, 0, nil)
	}
	n.ProcessBit(0, fsvm.State{}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	exs := n.Exemplars()
	if len(exs) != 1 {
		t.Fatalf("expected 1 exemplar list, got %d", len(exs))
	}
	if len(exs[0]) != 4 {
		t.Fatalf("expected 4 exemplars, got %d", len(exs[0]))
	}
	// Positions should be 0, 25, 50, 75
	want := []uint64{0, 25, 50, 75}
	for i, w := range want {
		if exs[0][i] != w {
			t.Errorf("exemplar %d: got %d, want %d", i, exs[0][i], w)
		}
	}
}

func BenchmarkNFAProcessBit(b *testing.B) {
	n := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.ProcessBit(byte(i&1), fsvm.State{}, 0, nil)
	}
}
