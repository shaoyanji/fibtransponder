package segauto

// Package segauto: segmentation automaton (regular-language / NFA bitset) sketch.
//
// Goal: represent "allowed" segmentation choices at sparse candidate markers
// without enumerating hypotheses.
//
// This is a minimal, pragmatic start: a small NFA over two modes:
// - IN: inside a message (must have seen at least one 1 to be 'started')
// - GAP: between messages (leading zeros before next start)
//
// Candidate cut points (markers) allow a transition IN->GAP without forcing it.
//
// NOTE: This is just the skeleton; exemplar extraction and full constraints belong next.

type State uint8

const (
	stGap State = iota // between messages / before start
	stIn0             // in message, last bit 0
	stIn1             // in message, last bit 1
)

// NFA is represented as a bitset over the three states.
// bit 0: GAP, bit 1: IN0, bit 2: IN1

type NFA struct {
	Mask uint8
}

func New() NFA { return NFA{Mask: 1 << stGap} }

// Step consumes an observed bit. This is about segmentation structure only.
func Step(n NFA, b uint8) NFA {
	b &= 1
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

	return NFA{Mask: out}
}

// MarkerCut applies the "allowed" cut at a candidate marker:
// if you're in-message, you may also be in GAP.
func MarkerCut(n NFA) NFA {
	in := (n.Mask&(1<<stIn0) != 0) || (n.Mask&(1<<stIn1) != 0)
	if in {
		n.Mask |= 1 << stGap
	}
	return n
}
