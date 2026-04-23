package fsvm

// EventBatch is a zero-alloc summary of events produced by a 64-bit word step.
// Callers that need individual event payloads should fall back to the per-bit Step.
type EventBatch struct {
	DilateCount uint8  // max 63 dilations per 64-bit word
	MarkerCount uint8  // max ~7 markers per word
	FinalR      uint32 // R after processing the word
}

// StepWord64 processes one 64-bit word (LSB-first: bit 0 is the first observed bit)
// and returns the updated State plus a compact EventBatch.
//
// Fast paths:
//   - word == 0       : all zeros; only zero-run / marker logic
//   - word == ^uint64(0): all ones; only dilation logic
//   - mixed           : run-based processing (CTZ/CLZ) for branch predictability.
func StepWord64(s State, word uint64) (State, EventBatch) {
	var batch EventBatch

	// ---- fast path: all zeros --------------------------------------------
	if word == 0 {
		s = stepWord64AllZeros(s, &batch)
		batch.FinalR = s.R
		return s, batch
	}

	// ---- fast path: all ones ---------------------------------------------
	if word == ^uint64(0) {
		s = stepWord64AllOnes(s, &batch)
		batch.FinalR = s.R
		return s, batch
	}

	// ---- general mixed word ----------------------------------------------
	s = stepWord64Mixed(s, word, &batch)
	batch.FinalR = s.R
	return s, batch
}

// stepWord64AllZeros handles the word==0 case.
// All 64 bits are zero, so LastBit becomes 0, W flushes to 0, and ZeroRun grows.
func stepWord64AllZeros(s State, batch *EventBatch) State {
	w := s.W
	for i := 0; i < 64; i++ {
		w = (w << 1) & 0x3F
		s.Sketch ^= s.Seeds[0] + uint64(w)
	}
	s.W = w
	s.LastBit = 0
	s.BitsProcessed += 64

	// Zero-run extension: check every pow2 crossing in [oldRun+1, oldRun+64].
	oldRun := s.ZeroRun
	s.ZeroRun += 64
	newMarkers := countPow2Crossings(oldRun, s.ZeroRun)
	s.Markers += newMarkers
	batch.MarkerCount = uint8(newMarkers)

	return s
}

// stepWord64AllOnes handles the word==^uint64(0) case.
func stepWord64AllOnes(s State, batch *EventBatch) State {
	// Dilations: every bit after the first causes a dilation.
	// If LastBit==1, even the first bit (LSB) causes one.
	if s.LastBit == 1 {
		s.R += 64
		s.Dilations += 64
		batch.DilateCount = 64
	} else {
		s.R += 63
		s.Dilations += 63
		batch.DilateCount = 63
	}
	s.LastBit = 1
	s.ZeroRun = 0
	s.BitsProcessed += 64

	// Sketch: b==1 every step. W goes through a fixed cycle.
	w := s.W
	for i := 0; i < 64; i++ {
		w = ((w << 1) | 1) & 0x3F
		s.Sketch ^= s.Seeds[1] + uint64(w)
	}
	s.W = w

	return s
}

// stepWord64Mixed handles arbitrary 64-bit words.
func stepWord64Mixed(s State, word uint64, batch *EventBatch) State {
	// We process bit-by-bit but inlined: no function calls, no allocations.
	// The compiler can unroll and keep State in registers.
	for i := 0; i < 64; i++ {
		b := uint8((word >> i) & 1)
		s.BitsProcessed++

		// ---- zero run + marker (inlined from Step) ----
		if b == 0 {
			s.ZeroRun++
			if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
				s.Markers++
				batch.MarkerCount++
			}
		} else {
			s.ZeroRun = 0
		}

		// ---- dilation (inlined from Step) --------------
		if s.LastBit == 1 && b == 1 {
			s.R++
			s.Dilations++
			batch.DilateCount++
		}

		s.LastBit = b
		s.W = ((s.W << 1) | b) & 0x3F
		s.Sketch ^= s.Seeds[b] + uint64(s.W)
	}
	return s
}

// countPow2Crossings returns how many power-of-two values p (p≥8) satisfy
// low < p ≤ high.  Used for marker accounting in bulk zero-run steps.
func countPow2Crossings(low, high uint64) uint64 {
	if high < 8 {
		return 0
	}
	// Powers of two we care about: 8, 16, 32, 64, 128, ...
	var count uint64
	for p := uint64(8); p <= high; p <<= 1 {
		if p > low {
			count++
		}
	}
	return count
}

// StepWord64Compat is a convenience wrapper that returns []Event for API
// compatibility.  It allocates only when events actually occur.
func StepWord64Compat(s State, word uint64) (State, []Event) {
	pre := s
	s, batch := StepWord64(s, word)
	if batch.DilateCount == 0 && batch.MarkerCount == 0 {
		return s, nil
	}
	// Reconstruct events by re-running the word bit-by-bit.  This is
	// intended for testing and transitional callers only; hot paths
	// should use the batch counts directly.
	_, evs := stepReconstructEvents(pre, word, batch)
	return s, evs
}

// stepReconstructEvents rebuilds the exact []Event slice from a word.
// It requires the *pre-step* state, so callers must save it.
func stepReconstructEvents(pre State, word uint64, batch EventBatch) (State, []Event) {
	if batch.DilateCount == 0 && batch.MarkerCount == 0 {
		return pre, nil
	}
	evs := make([]Event, 0, batch.DilateCount+batch.MarkerCount)
	s := pre
	for i := 0; i < 64; i++ {
		b := uint8((word >> i) & 1)
		s.BitsProcessed++
		if b == 0 {
			s.ZeroRun++
			if s.ZeroRun >= 8 && isPow2(s.ZeroRun) {
				evs = append(evs, Event{Kind: EventMarker, Payload: s.ZeroRun})
			}
		} else {
			s.ZeroRun = 0
		}
		if s.LastBit == 1 && b == 1 {
			s.R++
			s.Dilations++
			evs = append(evs, Event{Kind: EventDilate, Payload: uint64(s.R)})
		}
		s.LastBit = b
		s.W = ((s.W << 1) | b) & 0x3F
		s.Sketch ^= s.Seeds[b] + uint64(s.W)
	}
	return s, evs
}
