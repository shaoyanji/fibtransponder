package fsvm

// FSVM: Fibonacci Stream Virtual Machine
//
// Deterministic, measurement-first streaming core.
// Tracks:
// - r: dilation exponent (retrospective upsample-by-2 count)
// - w: 6-bit hexagram window (recent logical bits)
// - zeroRun, lastBit
// - sketch: Zobrist state sketch (XOR-folded, for cheap divergence detection)
//
// Dilation protocol:
// - if observed adjacency 11 occurs, emit DILATE and increment r.
//
// Segmentation is NOT decided here; it is an interpretation layer (NFA) driven by markers.

type EventKind uint8

const (
	EventDilate EventKind = iota + 1
	EventMarker
)

type Event struct {
	Kind    EventKind
	Payload uint64
}

// ZobristSeedBit is the per-bit Zobrist seed table.
// Index 0 for bit=0, index 1 for bit=1. One XOR per Step, fused into the hot path.
// These are frozen constants; changing them invalidates all prior Sketch values.
var ZobristSeedBit = [2]uint64{
	0x517cc1b727220a95, // bit=0
	0x9e3779b97f4a7c15, // bit=1
}

type State struct {
	R       uint32 // dilation exponent
	W       uint8  // 6-bit window
	LastBit uint8
	ZeroRun uint64
	Sketch  uint64 // Zobrist state sketch (XOR-folded per bit)
	// Stats
	Dilations uint64
	Markers   uint64
}

func New() State { return State{} }

func isPow2(x uint64) bool { return x > 0 && (x&(x-1)) == 0 }

// Step consumes one observed bit and returns zero or more events.
//
// Policy (measurement-first, unDoSable):
// - Adjacent 1s => DILATE (r++).
// - Zero-run markers emitted at sparse powers-of-two crossings >= 8: 8,16,32,...
func Step(s State, b uint8) (State, []Event) {
	b &= 1
	var evs []Event

	// update zero run + marker
	if b == 0 {
		s.ZeroRun++
		if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
			s.Markers++
			evs = append(evs, Event{Kind: EventMarker, Payload: s.ZeroRun})
		}
	} else {
		s.ZeroRun = 0
	}

	// detect adjacency
	if s.LastBit == 1 && b == 1 {
		s.R++
		s.Dilations++
		evs = append(evs, Event{Kind: EventDilate, Payload: uint64(s.R)})
	}

	s.LastBit = b
	s.W = ((s.W << 1) | b) & 0x3F

	// Zobrist fold: one XOR into sketch, zero-cost with compiler fusion.
	s.Sketch ^= ZobristSeedBit[b]

	return s, evs
}
