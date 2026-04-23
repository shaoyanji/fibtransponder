package fsvm

// Adapter converts various input formats into the bit-level FSVM interface.
// It is a convenience layer — the core Step/StepWord64 remain the source of truth.

type Adapter interface {
	// Next returns the next bit (0 or 1) and true, or false if exhausted.
	Next() (byte, bool)
}

// SliceAdapter wraps a []byte of bits (each element 0 or 1).
type SliceAdapter struct {
	bits []byte
	pos  int
}

func NewSliceAdapter(bits []byte) *SliceAdapter {
	return &SliceAdapter{bits: bits}
}

func (a *SliceAdapter) Next() (byte, bool) {
	if a.pos >= len(a.bits) {
		return 0, false
	}
	b := a.bits[a.pos] & 1
	a.pos++
	return b, true
}

// ByteAdapter streams bits from a []byte with configurable bit order.
// Order=0: LSB-first (bit 0 of byte[0] is first observed bit).
// Order=7: MSB-first (bit 7 of byte[0] is first observed bit).
type ByteAdapter struct {
	data  []byte
	idx   int // byte index
	bit   uint8 // 0..7, current bit within byte
	order uint8 // 0 = LSB-first, 7 = MSB-first
}

func NewByteAdapter(data []byte, msbFirst bool) *ByteAdapter {
	order := uint8(0)
	if msbFirst {
		order = 7
	}
	return &ByteAdapter{data: data, order: order, bit: order}
}

func (a *ByteAdapter) Next() (byte, bool) {
	if a.idx >= len(a.data) {
		return 0, false
	}
	b := byte((a.data[a.idx] >> a.bit) & 1)
	if a.order == 7 {
		if a.bit == 0 {
			a.bit = 7
			a.idx++
		} else {
			a.bit--
		}
	} else {
		if a.bit == 7 {
			a.bit = 0
			a.idx++
		} else {
			a.bit++
		}
	}
	return b, true
}

// Word64Adapter yields 64-bit words from a []uint64, LSB-first within each word.
type Word64Adapter struct {
	words []uint64
	pos   int
}

func NewWord64Adapter(words []uint64) *Word64Adapter {
	return &Word64Adapter{words: words}
}

func (a *Word64Adapter) Next() (uint64, bool) {
	if a.pos >= len(a.words) {
		return 0, false
	}
	w := a.words[a.pos]
	a.pos++
	return w, true
}

// RunAll steps an FSVM through an Adapter until exhaustion.
// Returns the final state and total bits processed.
func RunAll(s State, a Adapter) State {
	for {
		b, ok := a.Next()
		if !ok {
			break
		}
		s, _ = Step(s, b)
	}
	return s
}

// RunAllV2 is the sketch-v2 variant of RunAll.
func RunAllV2(s State, a Adapter) State {
	for {
		b, ok := a.Next()
		if !ok {
			break
		}
		s, _ = StepV2(s, b)
	}
	return s
}
