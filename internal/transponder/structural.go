package transponder

import (
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// AdjacencyWidth defines how many trailing bits must match for DILATE.
// Width 1: LastBit==1 && b==1 (current default)
// Width 2: (W & 0x03) == 0x03 && b==1
// Width 3: (W & 0x07) == 0x07 && b==1
type AdjacencyWidth int

const (
	Width1 AdjacencyWidth = 1
	Width2 AdjacencyWidth = 2
	Width3 AdjacencyWidth = 3
)

// StructuralCalibration holds geometry parameters, not hash seeds.
type StructuralCalibration struct {
	Name  string
	Width AdjacencyWidth
}

// StepWidth runs the FSVM Step with a custom adjacency width.
// All transponders share the same seed table (DefaultSeeds).
// Only the adjacency detection geometry differs.
func StepWidth(s fsvm.State, b uint8, width AdjacencyWidth) (fsvm.State, []fsvm.Event) {
	b &= 1
	var evs []fsvm.Event

	// update zero run + marker (unchanged)
	if b == 0 {
		s.ZeroRun++
		if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
			s.Markers++
			evs = append(evs, fsvm.Event{Kind: fsvm.EventMarker, Payload: s.ZeroRun})
		}
	} else {
		s.ZeroRun = 0
	}

	// adjacency detection with configurable width
	adjacent := false
	switch width {
	case Width1:
		adjacent = s.LastBit == 1 && b == 1
	case Width2:
		adjacent = (s.W&0x03) == 0x03 && b == 1
	case Width3:
		adjacent = (s.W&0x07) == 0x07 && b == 1
	}

	if adjacent {
		s.R++
		s.Dilations++
		evs = append(evs, fsvm.Event{Kind: fsvm.EventDilate, Payload: uint64(s.R)})
	}

	s.LastBit = b
	s.W = ((s.W << 1) | b) & 0x3F

	// Zobrist fold (same seeds for all transponders)
	s.Sketch ^= s.Seeds[b] + uint64(s.W)

	return s, evs
}

func isPow2(x uint64) bool { return x > 0 && (x&(x-1)) == 0 }

// StructuralTransponder is one detector with a specific geometry.
type StructuralTransponder struct {
	Name  string
	Width AdjacencyWidth
	State fsvm.State
}

// StructuralArray holds transponders with different geometries, same seeds.
type StructuralArray struct {
	Transponders []StructuralTransponder
}

// NewStructuralArray creates transponders with varying adjacency widths.
// All use DefaultSeeds — only geometry differs.
func NewStructuralArray(widths []StructuralCalibration) *StructuralArray {
	arr := &StructuralArray{Transponders: make([]StructuralTransponder, len(widths))}
	for i, w := range widths {
		arr.Transponders[i] = StructuralTransponder{
			Name:  w.Name,
			Width: w.Width,
			State: fsvm.New(), // DefaultSeeds for all
		}
	}
	return arr
}

// Step feeds one bit to all transponders.
func (a *StructuralArray) Step(b uint8) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		var evs []fsvm.Event
		t.State, evs = StepWidth(t.State, b, t.Width)
		_ = evs
		results[i] = Result{
			Name:      t.Name,
			Dilations: t.State.Dilations,
			Markers:   t.State.Markers,
			Sketch:    t.State.Sketch,
			R:         t.State.R,
		}
	}
	return results
}

// ProcessStream feeds entire bitstream.
func (a *StructuralArray) ProcessStream(bits []uint8) []Result {
	var last []Result
	for _, b := range bits {
		last = a.Step(b)
	}
	return last
}
