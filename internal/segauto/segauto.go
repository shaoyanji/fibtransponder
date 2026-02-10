package segauto

import (
	"fmt"
	"strings"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// State for NFA (bitset over three states)
const (
	stGap State = iota // between messages / before start
	stIn0             // in message, last bit 0
	stIn1             // in message, last bit 1
)

// NFA is represented as a bitset over the three states.
// bit 0: GAP, bit 1: IN0, bit 2: IN1
type NFA struct {
	Mask uint8
	latestOutput extension.Output
}

// New creates a new NFA state machine.
func New() *NFA {
	n := &NFA{Mask: 1 << stGap} // Start in GAP state
	n.latestOutput = n.GetOutput() // Initialize output
	return n
}

// GetTitle returns a short title for this extension.
func (n *NFA) GetTitle() string {
	return "Segmentation Automaton"
}

// ProcessBit consumes an observed bit and updates the NFA state.
// This is about segmentation structure only.
func (n *NFA) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	b &= 1 // Ensure bit is 0 or 1
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
	}
}

// GetOutput returns the current displayable information from the NFA.
func (n *NFA) GetOutput() extension.Output {
	return extension.Output{
		Title: n.GetTitle(),
		Lines: []string{fmt.Sprintf("  NFA Mask: 0x%02x (GAP:%t, IN0:%t, IN1:%t)",
			n.Mask, (n.Mask>>stGap)&1 == 1, (n.Mask>>stIn0)&1 == 1, (n.Mask>>stIn1)&1 == 1)},
	}
}
