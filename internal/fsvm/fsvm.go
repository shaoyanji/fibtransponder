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

// DefaultSeeds is the standard Zobrist seed table.
// Index 0 for bit=0, index 1 for bit=1.
var DefaultSeeds = [2]uint64{
	0x517cc1b727220a95, // bit=0
	0x9e3779b97f4a7c15, // bit=1
}

type State struct {
	Seeds      [2]uint64 // per-instance Zobrist seed table
	MixA       uint64    // hash-family multiplier (v2)
	MixB       uint64    // hash-family addend (v2)
	MixR       uint8     // hash-family rotation (v2)
	Sketch     uint64    // Zobrist state sketch
	SketchDelta uint8    // rolling bits-changed in sketch (v2)
	ZeroRun    uint64
	Dilations  uint64
	Markers    uint64
	R          uint32 // dilation exponent
	W          uint8  // 6-bit window
	LastBit    uint8
	Width      uint8  // adjacency width (used by calibration/step-width)
}

// New returns a State with default seeds.
func New() State {
	return State{Seeds: DefaultSeeds}
}

// NewWithSeeds returns a State with caller-provided seeds.
func NewWithSeeds(seeds [2]uint64) State {
	return State{Seeds: seeds}
}

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

	// Zobrist fold: one XOR using per-instance seed, fused into hot path.
	// Fold both bit value and window state to avoid sketch collapse on
	// streams with equal 0/1 counts (XOR self-inverse property).
	s.Sketch ^= s.Seeds[b] + uint64(s.W)

	return s, evs
}
