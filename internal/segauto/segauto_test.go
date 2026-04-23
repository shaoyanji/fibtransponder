package segauto

import (
	"math/bits"
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

// TestDivergenceTracking verifies sketch snapshots are captured.
func TestDivergenceTracking(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 10, MaxExemplars: 2})
	st := fsvm.State{Sketch: 0xAAAA}
	n.ProcessBit(1, st, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0xBBBB}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	segs := n.Segments()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].StartSketch != 0xAAAA {
		t.Errorf("start sketch: want 0xAAAA, got 0x%04X", segs[0].StartSketch)
	}
	if segs[0].EndSketch != 0xBBBB {
		t.Errorf("end sketch: want 0xBBBB, got 0x%04X", segs[0].EndSketch)
	}
	if segs[0].Divergence() != bits.OnesCount64(0xAAAA^0xBBBB) {
		t.Errorf("divergence mismatch")
	}
}

// TestExemplarsByDivergence verifies top segments are selected by sketch change.
func TestExemplarsByDivergence(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 10, MaxExemplars: 2})

	// Segment 1: low divergence (sketch changes by 1 bit)
	n.ProcessBit(1, fsvm.State{Sketch: 0x0000}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0x0001}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	// Segment 2: medium divergence (sketch changes by 8 bits)
	n.ProcessBit(1, fsvm.State{Sketch: 0x0000}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0x00FF}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	// Segment 3: high divergence (sketch changes by 16 bits)
	n.ProcessBit(1, fsvm.State{Sketch: 0x0000}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0xFFFF}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	top := n.ExemplarsByDivergence()
	if len(top) != 2 { // budget is 2
		t.Fatalf("expected 2 exemplars, got %d", len(top))
	}
	// Highest divergence first: segment 3 (16 bits), then segment 2 (8 bits)
	if top[0].EndSketch != 0xFFFF {
		t.Errorf("expected highest-divergence segment first, got endSketch=0x%04X", top[0].EndSketch)
	}
	if top[1].EndSketch != 0x00FF {
		t.Errorf("expected second-highest divergence next, got endSketch=0x%04X", top[1].EndSketch)
	}
}

// TestExemplarPositionsByDivergence verifies position output.
func TestExemplarPositionsByDivergence(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 10, MaxExemplars: 1})

	n.ProcessBit(1, fsvm.State{Sketch: 0x0000}, 0, nil)
	n.ProcessBit(1, fsvm.State{Sketch: 0x0000}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0xFFFF}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	n.ProcessBit(1, fsvm.State{Sketch: 0x1234}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0x1234}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	poses := n.ExemplarPositionsByDivergence()
	if len(poses) != 1 { // budget=1, only highest-divergence segment
		t.Fatalf("expected 1 position list, got %d", len(poses))
	}
	if len(poses[0]) != 2 {
		t.Fatalf("expected 2 positions (start, end-1), got %d", len(poses[0]))
	}
	if poses[0][0] != 0 {
		t.Errorf("expected start=0, got %d", poses[0][0])
	}
	if poses[0][1] != 2 {
		t.Errorf("expected end-1=2, got %d", poses[0][1])
	}
}

// TestDivergenceHistogram verifies the histogram shape.
func TestDivergenceHistogram(t *testing.T) {
	n := NewWithBudget(Budget{MaxSegments: 10, MaxExemplars: 2})

	n.ProcessBit(1, fsvm.State{Sketch: 0x0000}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0x0001}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	n.ProcessBit(1, fsvm.State{Sketch: 0xFF00}, 0, nil)
	n.ProcessBit(0, fsvm.State{Sketch: 0x00FF}, 0, []fsvm.Event{{Kind: fsvm.EventMarker}})

	hist := n.DivergenceHistogram()
	if len(hist) != 2 {
		t.Fatalf("expected 2 divergence buckets, got %d", len(hist))
	}
	if hist[1] != 1 || hist[16] != 1 {
		t.Errorf("unexpected histogram: %+v", hist)
	}
}
