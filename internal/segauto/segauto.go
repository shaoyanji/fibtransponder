package segauto

import (
	"fmt"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// State for NFA (bitset over three states)
type State uint8

const (
	stGap State = iota // between messages / before start
	stIn0              // in message, last bit 0
	stIn1              // in message, last bit 1
)

// NFA is represented as a bitset over the three states.
// bit 0: GAP, bit 1: IN0, bit 2: IN1
type NFA struct {
	Mask          uint8
	latestOutput  extension.Output
	segments      []Segment
	currentSeg    *Segment
	bitsProcessed uint64
	budget        Budget
}

// Segment records a contiguous run of non-zero bits between candidate cuts.
type Segment struct {
	Start uint64 // absolute bit position where segment begins
	End   uint64 // absolute bit position where segment ends (exclusive)
	Ones  uint64 // count of 1-bits in segment
}

// Budget limits output materialization to prevent unbounded growth.
type Budget struct {
	MaxSegments int // hard cap on stored segments
	MaxExemplars int // max exemplars to report per segment
}

// DefaultBudget returns sensible production defaults.
func DefaultBudget() Budget {
	return Budget{MaxSegments: 1000, MaxExemplars: 4}
}

// New creates a new NFA state machine.
func New() *NFA {
	n := &NFA{Mask: 1 << stGap, budget: DefaultBudget()}
	n.latestOutput = n.GetOutput()
	return n
}

// NewWithBudget creates an NFA with a custom output budget.
func NewWithBudget(b Budget) *NFA {
	n := &NFA{Mask: 1 << stGap, budget: b}
	n.latestOutput = n.GetOutput()
	return n
}

// GetTitle returns a short title for this extension.
func (n *NFA) GetTitle() string {
	return "Segmentation Automaton"
}

// ProcessBit consumes an observed bit and updates the NFA state.
func (n *NFA) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	b &= 1
	n.bitsProcessed++

	var out uint8
	gap := (n.Mask>>stGap)&1 == 1
	in0 := (n.Mask>>stIn0)&1 == 1
	in1 := (n.Mask>>stIn1)&1 == 1

	// From GAP: zeros keep you in GAP; one starts a message.
	if gap {
		if b == 0 {
			out |= 1 << stGap
		} else {
			out |= 1 << stIn1
			if n.currentSeg == nil {
				n.currentSeg = &Segment{Start: n.bitsProcessed - 1}
			}
		}
	}

	// From IN0/IN1: stay in message.
	if in0 {
		if b == 0 {
			out |= 1 << stIn0
		} else {
			out |= 1 << stIn1
		}
	}
	if in1 {
		if b == 0 {
			out |= 1 << stIn0
		} else {
			out |= 1 << stIn1
		}
	}
	n.Mask = out

	// Track ones in current segment
	if n.currentSeg != nil && b == 1 {
		n.currentSeg.Ones++
	}

	// Apply MarkerCut if an EventMarker is present
	for _, ev := range events {
		if ev.Kind == fsvm.EventMarker {
			n.applyMarkerCut()
		}
	}

	n.latestOutput = n.GetOutput()
}

// applyMarkerCut applies the "allowed" cut at a candidate marker:
// if you're in-message, you may also be in GAP.
func (n *NFA) applyMarkerCut() {
	in := (n.Mask&(1<<stIn0) != 0) || (n.Mask&(1<<stIn1) != 0)
	if in {
		n.Mask |= 1 << stGap
		if n.currentSeg != nil {
			n.currentSeg.End = n.bitsProcessed
			n.flushSegment()
		}
	}
}

// flushSegment stores the completed segment if budget allows.
func (n *NFA) flushSegment() {
	if n.currentSeg == nil {
		return
	}
	if len(n.segments) < n.budget.MaxSegments {
		n.segments = append(n.segments, *n.currentSeg)
	}
	// Start a new segment after the cut if we're still in-message.
	// (Mask already has both GAP and IN* set, so next non-zero bit
	// will start a fresh segment.)
	n.currentSeg = nil
}

// Segments returns a copy of all completed segments.
func (n *NFA) Segments() []Segment {
	out := make([]Segment, len(n.segments))
	copy(out, n.segments)
	return out
}

// Exemplars returns deterministic sample positions from each segment.
// Samples are spaced evenly: start, 1/3, 2/3, end (or fewer if budget limited).
func (n *NFA) Exemplars() [][]uint64 {
	out := make([][]uint64, 0, len(n.segments))
	for _, seg := range n.segments {
		ex := n.sampleSegment(seg)
		out = append(out, ex)
	}
	return out
}

func (n *NFA) sampleSegment(seg Segment) []uint64 {
	length := seg.End - seg.Start
	if length == 0 {
		return nil
	}
	// Deterministic positions: start, evenly spaced, end-1
	nSamples := n.budget.MaxExemplars
	if int(length) < nSamples {
		nSamples = int(length)
	}
	samples := make([]uint64, 0, nSamples)
	for i := 0; i < nSamples; i++ {
		pos := seg.Start + uint64(i)*length/uint64(nSamples)
		samples = append(samples, pos)
	}
	return samples
}

// GetOutput returns the current displayable information from the NFA.
func (n *NFA) GetOutput() extension.Output {
	lines := []string{
		fmt.Sprintf("  NFA Mask: 0x%02x (GAP:%t, IN0:%t, IN1:%t)",
			n.Mask, (n.Mask>>stGap)&1 == 1, (n.Mask>>stIn0)&1 == 1, (n.Mask>>stIn1)&1 == 1),
		fmt.Sprintf("  Segments: %d (budget: %d)", len(n.segments), n.budget.MaxSegments),
	}
	if len(n.segments) > 0 {
		last := n.segments[len(n.segments)-1]
		lines = append(lines, fmt.Sprintf("  Last segment: bits[%d:%d) ones=%d",
			last.Start, last.End, last.Ones))
	}
	return extension.Output{
		Title: n.GetTitle(),
		Lines: lines,
	}
}
