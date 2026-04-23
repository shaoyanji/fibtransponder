package fsvm

import "math/bits"

// HashFamily defines a set of mixing constants for the sketch v2 hash.
// Each transponder gets its own family, reducing linear dependencies
// and cross-class collisions.
type HashFamily struct {
	A uint64 // odd multiplier
	B uint64 // addend
	R uint8  // rotation amount
}

// Precomputed hash families — all multipliers are large odd 64-bit constants.
var HashFamilies = [8]HashFamily{
	{A: 0x9e3779b97f4a7c15, B: 0x517cc1b727220a95, R: 7},
	{A: 0xc6a4a7935bd1e995, B: 0x243f6a8885a308d3, R: 13},
	{A: 0xff51afd7ed558ccd, B: 0xa4093822299f31d0, R: 17},
	{A: 0xbd5e6fcb874dd4eb, B: 0x082efa98ec4e6c89, R: 23},
	{A: 0x636413c45c059c97, B: 0x13198a2e03707344, R: 31},
	{A: 0x78a5636f43172f60, B: 0xc0e6900f00000001, R: 37},
	{A: 0xb3e6c1f3e3b8e9d5, B: 0x71d7c0d99e9e9f45, R: 41},
	{A: 0xdf7e87c6d9e1a0b3, B: 0x28f149d2e6b7c3a9, R: 47},
}

// FamilyCount returns the number of available hash families.
func FamilyCount() int { return len(HashFamilies) }

// NewWithFamily returns a State using the hash family at index id % FamilyCount().
func NewWithFamily(id int) State {
	f := HashFamilies[id%FamilyCount()]
	s := State{
		Seeds: DefaultSeeds,
		MixA:  f.A,
		MixB:  f.B,
		MixR:  f.R,
		Width: 1,
	}
	return s
}

// StepV2 consumes one bit using the v2 sketch mixing algorithm.
// It preserves all dilation/marker semantics but replaces the
// simple XOR sketch with per-family avalanche mixing + richer folding.
func StepV2(s State, b uint8) (State, []Event) {
	b &= 1
	var evs []Event
	s.BitsProcessed++
	oldSketch := s.Sketch

	// ---- zero run + marker (identical to Step) ----
	if b == 0 {
		s.ZeroRun++
		if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
			s.Markers++
			evs = append(evs, Event{Kind: EventMarker, Payload: s.ZeroRun})
		}
	} else {
		s.ZeroRun = 0
	}

	// ---- dilation (identical to Step) ----
	if s.LastBit == 1 && b == 1 {
		s.R++
		s.Dilations++
		evs = append(evs, Event{Kind: EventDilate, Payload: uint64(s.R)})
	}

	s.LastBit = b
	s.W = ((s.W << 1) | b) & 0x3F

	// ---- v2 sketch update ----
	s.Sketch = mixSketch(s.Sketch, s.MixA, s.MixB, s.MixR)
	s.Sketch ^= s.Seeds[b] + uint64(s.W)
	s.Sketch ^= foldZeroRun(s.ZeroRun)
	s.Sketch ^= uint64(s.R) << 32
	for _, ev := range evs {
		s.Sketch ^= eventSalt(ev)
	}

	// ---- rolling sketch delta ----
	s.SketchDelta = uint8(bits.OnesCount64(oldSketch ^ s.Sketch))

	return s, evs
}

// StepWord64V2 is the word-level fast path for sketch v2.
//
// V2's sketch depends on per-step event payloads (R values, zero-run
// lengths), making bulk fast paths extremely complex.  We retain a
// bit-by-bit loop but improve branch predictability by hoisting the
// event-prediction logic ahead of state mutation.  On adversarial
// patterns the compiler can keep the hot path in registers.
func StepWord64V2(s State, word uint64) (State, EventBatch) {
	var batch EventBatch

	// ---- fast path: all zeros --------------------------------------------
	if word == 0 {
		s = stepWord64V2AllZeros(s, &batch)
		batch.FinalR = s.R
		return s, batch
	}

	// ---- fast path: all ones ---------------------------------------------
	if word == ^uint64(0) {
		s = stepWord64V2AllOnes(s, &batch)
		batch.FinalR = s.R
		return s, batch
	}

	// ---- general mixed word: reduced-branch loop -------------------------
	s = stepWord64V2Mixed(s, word, &batch)
	batch.FinalR = s.R
	return s, batch
}

// stepWord64V2AllZeros handles a word of all zeros.
// Markers occur at power-of-two crossings within the zero run.
func stepWord64V2AllZeros(s State, batch *EventBatch) State {
	startRun := s.ZeroRun
	for i := 0; i < 64; i++ {
		s.ZeroRun = startRun + uint64(i) + 1
		oldSketch := s.Sketch

		var evs []Event
		if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
			s.Markers++
			batch.MarkerCount++
			evs = append(evs, Event{Kind: EventMarker, Payload: s.ZeroRun})
		}

		s.W = (s.W << 1) & 0x3F
		s.Sketch = mixSketch(s.Sketch, s.MixA, s.MixB, s.MixR)
		s.Sketch ^= s.Seeds[0] + uint64(s.W)
		s.Sketch ^= foldZeroRun(s.ZeroRun)
		s.Sketch ^= uint64(s.R) << 32
		for _, ev := range evs {
			s.Sketch ^= eventSalt(ev)
		}
		s.SketchDelta = uint8(bits.OnesCount64(oldSketch ^ s.Sketch))
	}
	s.LastBit = 0
	s.BitsProcessed += 64
	s.ZeroRun = startRun + 64
	return s
}

// stepWord64V2AllOnes handles a word of all ones.
func stepWord64V2AllOnes(s State, batch *EventBatch) State {
	// Dilations: every bit after the first causes a dilation.
	startR := s.R
	if s.LastBit == 1 {
		for i := 0; i < 64; i++ {
			s.R = startR + uint32(i) + 1
			oldSketch := s.Sketch

			s.Dilations++
			batch.DilateCount++
			ev := Event{Kind: EventDilate, Payload: uint64(s.R)}

			s.W = ((s.W << 1) | 1) & 0x3F
			s.Sketch = mixSketch(s.Sketch, s.MixA, s.MixB, s.MixR)
			s.Sketch ^= s.Seeds[1] + uint64(s.W)
			s.Sketch ^= foldZeroRun(0)
			s.Sketch ^= uint64(s.R) << 32
			s.Sketch ^= eventSalt(ev)
			s.SketchDelta = uint8(bits.OnesCount64(oldSketch ^ s.Sketch))
		}
	} else {
		// First bit: no dilation
		s.W = ((s.W << 1) | 1) & 0x3F
		oldSketch := s.Sketch
		s.Sketch = mixSketch(s.Sketch, s.MixA, s.MixB, s.MixR)
		s.Sketch ^= s.Seeds[1] + uint64(s.W)
		s.Sketch ^= foldZeroRun(0)
		s.Sketch ^= uint64(s.R) << 32
		s.SketchDelta = uint8(bits.OnesCount64(oldSketch ^ s.Sketch))

		for i := 1; i < 64; i++ {
			s.R = startR + uint32(i)
			oldSketch := s.Sketch

			s.Dilations++
			batch.DilateCount++
			ev := Event{Kind: EventDilate, Payload: uint64(s.R)}

			s.W = ((s.W << 1) | 1) & 0x3F
			s.Sketch = mixSketch(s.Sketch, s.MixA, s.MixB, s.MixR)
			s.Sketch ^= s.Seeds[1] + uint64(s.W)
			s.Sketch ^= foldZeroRun(0)
			s.Sketch ^= uint64(s.R) << 32
			s.Sketch ^= eventSalt(ev)
			s.SketchDelta = uint8(bits.OnesCount64(oldSketch ^ s.Sketch))
		}
	}
	s.LastBit = 1
	s.ZeroRun = 0
	s.BitsProcessed += 64
	return s
}

// stepWord64V2Mixed processes arbitrary words bit-by-bit but with
// hoisted event detection to reduce branch mispredictions.
func stepWord64V2Mixed(s State, word uint64, batch *EventBatch) State {
	for i := 0; i < 64; i++ {
		b := uint8((word >> i) & 1)
		s.BitsProcessed++
		oldSketch := s.Sketch

		// Determine events BEFORE mutating state (matches StepV2 order)
		dilated := s.LastBit == 1 && b == 1
		marked := false
		if b == 0 {
			zr := s.ZeroRun + 1
			if zr >= 8 && isPow2(zr) {
				marked = true
			}
		}

		// Update zero run + marker
		if b == 0 {
			s.ZeroRun++
			if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
				s.Markers++
				batch.MarkerCount++
			}
		} else {
			s.ZeroRun = 0
		}

		// Dilation
		if dilated {
			s.R++
			s.Dilations++
			batch.DilateCount++
		}

		s.LastBit = b
		s.W = ((s.W << 1) | b) & 0x3F

		// v2 sketch update
		s.Sketch = mixSketch(s.Sketch, s.MixA, s.MixB, s.MixR)
		s.Sketch ^= s.Seeds[b] + uint64(s.W)
		s.Sketch ^= foldZeroRun(s.ZeroRun)
		s.Sketch ^= uint64(s.R) << 32
		if dilated {
			s.Sketch ^= eventSalt(Event{Kind: EventDilate, Payload: uint64(s.R)})
		}
		if marked {
			s.Sketch ^= eventSalt(Event{Kind: EventMarker, Payload: s.ZeroRun})
		}

		s.SketchDelta = uint8(bits.OnesCount64(oldSketch ^ s.Sketch))
	}
	return s
}

// mixSketch applies the family-specific avalanche function.
func mixSketch(sketch, a, b uint64, r uint8) uint64 {
	return bits.RotateLeft64(sketch*a+b, int(r))
}

// foldZeroRun buckets the zero run into 8 coarse buckets for sketch folding.
func foldZeroRun(zr uint64) uint64 {
	switch {
	case zr == 0:
		return 0
	case zr <= 4:
		return 0x1111111111111111
	case zr <= 8:
		return 0x2222222222222222
	case zr <= 16:
		return 0x4444444444444444
	case zr <= 32:
		return 0x8888888888888888
	default:
		return 0xFFFFFFFFFFFFFFFF
	}
}

// eventSalt returns a deterministic per-event-type salt.
func eventSalt(ev Event) uint64 {
	switch ev.Kind {
	case EventDilate:
		return 0xdeadbeefcafebabe ^ ev.Payload
	case EventMarker:
		return 0xfeedfacebaadf00d ^ ev.Payload
	default:
		return 0
	}
}
