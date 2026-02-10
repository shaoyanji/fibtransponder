package fsvm

// FSVM: Fibonacci Stream Virtual Machine
//
// Deterministic, measurement-first streaming core.
// Tracks:
// - r: dilation exponent (retrospective upsample-by-2 count)
// - w: 6-bit hexagram window (recent logical bits)
// - zeroRun, lastBit
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

type State struct {
	R       uint32 // dilation exponent
	W       uint8  // 6-bit window
	LastBit uint8
	ZeroRun uint64
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

	return s, evs
}
