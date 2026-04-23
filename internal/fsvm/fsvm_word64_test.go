package fsvm

import (
	"math/rand"
	"testing"
)

// TestStepWord64AllZeros verifies exact semantics against 64 bit-by-bit steps.
func TestStepWord64AllZeros(t *testing.T) {
	pre := New()
	// Feed some bits to get non-trivial initial state
	for _, b := range []uint8{1, 0, 1, 1, 0, 0, 1} {
		pre, _ = Step(pre, b)
	}

	// Reference: 64 zero bits one at a time
	ref := pre
	for i := 0; i < 64; i++ {
		ref, _ = Step(ref, 0)
	}

	// Word step
	got, batch := StepWord64(pre, 0)

	if got != ref {
		t.Fatalf("state mismatch after all-zeros word\nref: %+v\ngot: %+v", ref, got)
	}
	// All-zeros word with non-trivial initial state should have some markers
	// (depending on where ZeroRun started).  Just verify batch counts match.
	var expectedMarkers uint8
	zr := pre.ZeroRun
	for i := 0; i < 64; i++ {
		zr++
		if zr >= 8 && isPow2(zr) {
			expectedMarkers++
		}
	}
	if batch.MarkerCount != expectedMarkers {
		t.Fatalf("marker count mismatch: expected %d, got %d", expectedMarkers, batch.MarkerCount)
	}
}

// TestStepWord64AllOnes verifies exact semantics for a word of all ones.
func TestStepWord64AllOnes(t *testing.T) {
	for _, initLastBit := range []uint8{0, 1} {
		pre := New()
		if initLastBit == 1 {
			pre, _ = Step(pre, 1)
		}

		ref := pre
		for i := 0; i < 64; i++ {
			ref, _ = Step(ref, 1)
		}

		got, batch := StepWord64(pre, ^uint64(0))
		if got != ref {
			t.Fatalf("state mismatch (initLastBit=%d)\nref: %+v\ngot: %+v", initLastBit, ref, got)
		}

		expectedDilations := uint8(63)
		if initLastBit == 1 {
			expectedDilations = 64
		}
		if batch.DilateCount != expectedDilations {
			t.Fatalf("dilation count mismatch (initLastBit=%d): expected %d, got %d",
				initLastBit, expectedDilations, batch.DilateCount)
		}
	}
}

// TestStepWord64MixedExhaustive checks every possible low-6-bit pattern embedded
// in a 64-bit word (upper 58 bits zero) against the per-bit Step.
func TestStepWord64MixedExhaustive(t *testing.T) {
	for initW := uint8(0); initW < 64; initW++ {
		for initLast := uint8(0); initLast < 2; initLast++ {
			for pat := uint64(0); pat < 64; pat++ {
				pre := New()
				pre.W = initW
				pre.LastBit = initLast

				ref := pre
				for i := 0; i < 6; i++ {
					b := uint8((pat >> i) & 1)
					ref, _ = Step(ref, b)
				}

				// The remaining 58 bits are zero; continue with per-bit reference.
				for i := 6; i < 64; i++ {
					ref, _ = Step(ref, 0)
				}

				// Word step with the same 64-bit pattern.
				got, _ := StepWord64(pre, pat)
				if got != ref {
					t.Fatalf("mismatch initW=%d initLast=%d pat=%02b\nref: %+v\ngot: %+v",
						initW, initLast, pat, ref, got)
				}
			}
		}
	}
}

// TestStepWord64MixedRandom uses deterministic random words to stress-test.
func TestStepWord64MixedRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 10000; trial++ {
		pre := New()
		// Randomize initial state a bit
		for i := 0; i < int(rng.Uint32()%20); i++ {
			pre, _ = Step(pre, uint8(rng.Uint32()&1))
		}

		word := rng.Uint64()

		ref := pre
		for i := 0; i < 64; i++ {
			b := uint8((word >> i) & 1)
			ref, _ = Step(ref, b)
		}

		got, _ := StepWord64(pre, word)
		if got != ref {
			t.Fatalf("random trial %d word=%016x\nref: %+v\ngot: %+v", trial, word, ref, got)
		}
	}
}

// TestStepWord64Compat verifies the compat wrapper returns identical events.
func TestStepWord64Compat(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	for trial := 0; trial < 1000; trial++ {
		pre := New()
		for i := 0; i < int(rng.Uint32()%30); i++ {
			pre, _ = Step(pre, uint8(rng.Uint32()&1))
		}

		word := rng.Uint64()

		// Reference: bit-by-bit with event collection
		ref := pre
		var refEvs []Event
		for i := 0; i < 64; i++ {
			b := uint8((word >> i) & 1)
			var evs []Event
			ref, evs = Step(ref, b)
			refEvs = append(refEvs, evs...)
		}

		got, gotEvs := StepWord64Compat(pre, word)
		if got != ref {
			t.Fatalf("compat state mismatch trial %d", trial)
		}
		if len(gotEvs) != len(refEvs) {
			t.Fatalf("compat event count mismatch trial %d: ref=%d got=%d",
				trial, len(refEvs), len(gotEvs))
		}
		for i := range refEvs {
			if gotEvs[i] != refEvs[i] {
				t.Fatalf("compat event mismatch trial %d ev[%d]: ref=%+v got=%+v",
					trial, i, refEvs[i], gotEvs[i])
			}
		}
	}
}

// TestStepWord64PreservesPerBitSemantics runs long random streams and checks
// that word-at-a-time and bit-at-a-time produce identical final states.
func TestStepWord64PreservesPerBitSemantics(t *testing.T) {
	rng := rand.New(rand.NewSource(2025))
	for trial := 0; trial < 500; trial++ {
		ref := New()
		got := New()

		// Stream length: 1–128 words (64–8192 bits)
		words := 1 + int(rng.Uint32()%128)
		for w := 0; w < words; w++ {
			word := rng.Uint64()

			// Reference: bit-by-bit
			for i := 0; i < 64; i++ {
				b := uint8((word >> i) & 1)
				ref, _ = Step(ref, b)
			}

			// Got: word-at-a-time
			got, _ = StepWord64(got, word)
		}

		if got != ref {
			t.Fatalf("long stream trial %d (%d words): state mismatch\nref: %+v\ngot: %+v",
				trial, words, ref, got)
		}
	}
}
