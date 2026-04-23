package fsvm

import (
	"math"
	"math/bits"
)

// Descriptor is a 256-bit rich local feature vector extracted around
// transponder events (markers, dilations).  It is the 1-D analogue of
// SIFT/SURF descriptors: a compact histogram of local pattern responses.
//
// Layout (4 × uint64 = 32 bytes):
//   uint64[0]: sub-region densities    (8 × 1-byte counts)
//   uint64[1]: sub-region transitions  (8 × 1-byte counts)
//   uint64[2]: Haar-X responses        (8 × signed bytes, left−right)
//   uint64[3]: Haar-Y responses        (8 × signed bytes, centre−surround)
type Descriptor [4]uint64

// FeatureEvent ties a Descriptor to the transponder state at extraction time.
type FeatureEvent struct {
	BitPos    uint64    // absolute bit position in the stream
	EventKind EventKind // Marker or Dilate
	Depth     uint      // dilate depth at extraction time
	Sketch    uint64    // intrinsic sketch at extraction time
	Desc      Descriptor
}

// ExtractorConfig controls the rich-feature extraction pipeline.
type ExtractorConfig struct {
	WindowBits int  // must be 64 for the current implementation
	Enabled    bool // if false, all extracts return zero descriptors
}

// DefaultExtractorConfig returns a production-ready configuration.
func DefaultExtractorConfig() ExtractorConfig {
	return ExtractorConfig{WindowBits: 64, Enabled: true}
}

// Extractor maintains a rolling bit-history window and produces Descriptors
// at transponder events.  It is safe for use by a single goroutine.
type Extractor struct {
	cfg ExtractorConfig
	// history is a ring buffer of the most recent WindowBits bits.
	// Bits are stored MSB-first: bit i of history[0] is the newest bit.
	history [64]byte // one byte per bit, 0 or 1; index 0 = newest
	filled  bool     // true once WindowBits bits have been seen
	count   int      // how many bits have been inserted (modulo behaviour)
}

// NewExtractor creates an Extractor from a config.
func NewExtractor(cfg ExtractorConfig) *Extractor {
	if cfg.WindowBits <= 0 {
		cfg.WindowBits = 64
	}
	return &Extractor{cfg: cfg}
}

// Push feeds one bit into the rolling history.
func (e *Extractor) Push(bit byte) {
	if !e.cfg.Enabled {
		return
	}
	// Shift everything right by one position (index 0 stays newest).
	copy(e.history[1:], e.history[:])
	e.history[0] = bit & 1
	e.count++
	if e.count >= e.cfg.WindowBits {
		e.filled = true
	}
}

// Extract computes a Descriptor from the current history window.
// If the window is not yet full, the missing older bits are treated as 0.
func (e *Extractor) Extract(s *State, kind EventKind) Descriptor {
	if !e.cfg.Enabled {
		return Descriptor{}
	}
	return e.extractFromHistory(s, kind)
}

// extractFromHistory assumes e.history is populated and produces the descriptor.
func (e *Extractor) extractFromHistory(s *State, kind EventKind) Descriptor {
	var d Descriptor

	// We divide the 64-bit window into 8 sub-regions of 8 bits each.
	// Sub-region 0 is the newest (indices 0..7), sub-region 7 the oldest.
	for region := 0; region < 8; region++ {
		off := region * 8

		// --- density (count of 1s) ---
		density := 0
		for i := 0; i < 8; i++ {
			density += int(e.history[off+i])
		}

		// --- transitions (0→1 or 1→0 within the region) ---
		transitions := 0
		for i := 1; i < 8; i++ {
			if e.history[off+i] != e.history[off+i-1] {
				transitions++
			}
		}

		// --- Haar-X: left half density minus right half density ---
		left := 0
		right := 0
		for i := 0; i < 4; i++ {
			left += int(e.history[off+i])
			right += int(e.history[off+i+4])
		}
		haarX := left - right // range [-4,4], fits in signed byte

		// --- Haar-Y: centre density minus surround density ---
		// centre = bits 2,3,4,5  (4 bits)
		// surround = bits 0,1,6,7 (4 bits)
		centre := 0
		surround := 0
		for i := 0; i < 2; i++ {
			surround += int(e.history[off+i])
			surround += int(e.history[off+6+i])
		}
		for i := 2; i < 6; i++ {
			centre += int(e.history[off+i])
		}
		haarY := centre - surround // range [-4,4]

		// Pack into descriptor words.
		bytePos := uint(region) * 8
		d[0] |= uint64(density) << bytePos
		d[1] |= uint64(transitions) << bytePos
		d[2] |= uint64(uint8(int8(haarX))) << bytePos
		d[3] |= uint64(uint8(int8(haarY))) << bytePos
	}

	return d
}

// ExtractWord64 is a batch variant: feed an entire word of bits and,
// if an event fires during that word, produce a descriptor from the
// local history (which includes bits from earlier in the same word).
//
// Because the Extractor history is updated bit-by-bit inside the word,
// the descriptor reflects the exact context at the event position.
func (e *Extractor) ExtractWord64(s *State, kind EventKind, word uint64, nBits int) Descriptor {
	if !e.cfg.Enabled {
		return Descriptor{}
	}
	// First push all bits of the word into history so that the window
	// is correct for an event that fires inside this word.
	for i := nBits - 1; i >= 0; i-- {
		bit := byte((word >> i) & 1)
		e.Push(bit)
	}
	return e.Extract(s, kind)
}

// Distance computes the L1 (Manhattan) distance between two descriptors.
// Each uint64 word is treated as 8 independent bytes; Haar words use
// unsigned subtraction to respect the signed-byte encoding.
func Distance(a, b Descriptor) int {
	dist := 0
	for i := 0; i < 4; i++ {
		dist += byteL1(a[i], b[i])
	}
	return dist
}

// byteL1 returns Σ |byte_k(x) - byte_k(y)| for k=0..7.
func byteL1(x, y uint64) int {
	// XOR gives per-byte absolute difference only when no borrow crosses
	// byte boundaries.  For general L1 we do it byte-by-byte.
	diff := x ^ y
	// Fast path: if x and y have the same value in every byte, diff is 0.
	if diff == 0 {
		return 0
	}
	sum := 0
	for i := 0; i < 8; i++ {
		bx := byte(x >> (i * 8))
		by := byte(y >> (i * 8))
		if bx > by {
			sum += int(bx-by)
		} else {
			sum += int(by-bx)
		}
	}
	return sum
}

// CosineSimilarity returns a float64 in [-1,1] representing the cosine
// of the angle between two descriptors treated as 32-dimensional vectors.
// A value of 1 means identical descriptors; 0 means orthogonal.
func CosineSimilarity(a, b Descriptor) float64 {
	var dot, na, nb int64
	for i := 0; i < 4; i++ {
		// Treat each uint64 as 8 signed bytes.
		for j := 0; j < 8; j++ {
			sa := int8(a[i] >> (j * 8))
			sb := int8(b[i] >> (j * 8))
			dot += int64(sa) * int64(sb)
			na += int64(sa) * int64(sa)
			nb += int64(sb) * int64(sb)
		}
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float64(dot) / (math.Sqrt(float64(na)) * math.Sqrt(float64(nb)))
}

// FeatureBuffer accumulates FeatureEvents for downstream matching or clustering.
type FeatureBuffer struct {
	Events []FeatureEvent
	Limit  int // max events to retain; 0 = unlimited
}

// NewFeatureBuffer creates a buffer with an optional size cap.
func NewFeatureBuffer(limit int) *FeatureBuffer {
	return &FeatureBuffer{Limit: limit}
}

// Append adds an event, dropping the oldest if over limit.
func (fb *FeatureBuffer) Append(ev FeatureEvent) {
	fb.Events = append(fb.Events, ev)
	if fb.Limit > 0 && len(fb.Events) > fb.Limit {
		fb.Events = fb.Events[len(fb.Events)-fb.Limit:]
	}
}

// Clear resets the buffer.
func (fb *FeatureBuffer) Clear() {
	fb.Events = fb.Events[:0]
}

// Match finds the nearest neighbour of query inside the buffer using
// descriptor distance.  Returns the index and distance, or -1 if empty.
func (fb *FeatureBuffer) Match(query Descriptor) (int, int) {
	if len(fb.Events) == 0 {
		return -1, 0
	}
	bestIdx := 0
	bestDist := Distance(query, fb.Events[0].Desc)
	for i := 1; i < len(fb.Events); i++ {
		d := Distance(query, fb.Events[i].Desc)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx, bestDist
}

// PopCountDescriptor returns the total Hamming weight of the descriptor
// interpreted as 256 raw bits.  It is a cheap global signature.
func PopCountDescriptor(d Descriptor) int {
	return bits.OnesCount64(d[0]) + bits.OnesCount64(d[1]) +
		bits.OnesCount64(d[2]) + bits.OnesCount64(d[3])
}
