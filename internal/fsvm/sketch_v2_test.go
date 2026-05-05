package fsvm

import (
	"math/bits"
	"testing"
)

// TestNewWithFamily verifies each family index produces distinct mixing params.
func TestNewWithFamily(t *testing.T) {
	seen := make(map[uint64]struct{})
	for i := 0; i < FamilyCount()*3; i++ {
		s := NewWithFamily(i)
		key := s.MixA ^ s.MixB<<1 ^ uint64(s.MixR)<<2
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
		}
	}
	if len(seen) != FamilyCount() {
		t.Fatalf("expected %d distinct families, got %d", FamilyCount(), len(seen))
	}
}

// TestStepV2PreservesDilationSemantics verifies dilation/marker counts match v1.
func TestStepV2PreservesDilationSemantics(t *testing.T) {
	seq := []uint8{1, 0, 0, 1, 1, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0}

	v1 := New()
	var v1Dil, v1Mark uint64
	for _, b := range seq {
		var evs []Event
		v1, evs = Step(v1, b)
		for _, ev := range evs {
			if ev.Kind == EventDilate {
				v1Dil++
			}
			if ev.Kind == EventMarker {
				v1Mark++
			}
		}
	}

	v2 := NewWithFamily(0)
	var v2Dil, v2Mark uint64
	for _, b := range seq {
		var evs []Event
		v2, evs = StepV2(v2, b)
		for _, ev := range evs {
			if ev.Kind == EventDilate {
				v2Dil++
			}
			if ev.Kind == EventMarker {
				v2Mark++
			}
		}
	}

	if v1Dil != v2Dil || v1Mark != v2Mark {
		t.Fatalf("event count mismatch: v1(dil=%d mark=%d) v2(dil=%d mark=%d)",
			v1Dil, v1Mark, v2Dil, v2Mark)
	}
	if v1.R != v2.R || v1.ZeroRun != v2.ZeroRun || v1.W != v2.W {
		t.Fatalf("state mismatch: v1(r=%d zr=%d w=%d) v2(r=%d zr=%d w=%d)",
			v1.R, v1.ZeroRun, v1.W, v2.R, v2.ZeroRun, v2.W)
	}
}

// TestStepV2SketchDifferentFromV1 verifies v2 produces different sketches.
func TestStepV2SketchDifferentFromV1(t *testing.T) {
	seq := []uint8{1, 0, 1, 1, 0, 1, 0, 0}
	v1 := New()
	v2 := NewWithFamily(0)
	for _, b := range seq {
		v1, _ = Step(v1, b)
		v2, _ = StepV2(v2, b)
	}
	if v1.Sketch == v2.Sketch {
		t.Fatalf("expected v2 sketch to differ from v1")
	}
}

// TestStepV2PerFamilyUniqueness verifies different families produce divergent sketches.
func TestStepV2PerFamilyUniqueness(t *testing.T) {
	seq := []uint8{1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1}
	states := make([]State, FamilyCount())
	for i := range states {
		states[i] = NewWithFamily(i)
	}
	for _, b := range seq {
		for i := range states {
			states[i], _ = StepV2(states[i], b)
		}
	}
	for i := 0; i < FamilyCount(); i++ {
		for j := i + 1; j < FamilyCount(); j++ {
			if states[i].Sketch == states[j].Sketch {
				t.Fatalf("family %d and %d produced identical sketches", i, j)
			}
		}
	}
}

// TestStepV2SketchDeltaNonZero verifies SketchDelta tracks bit changes.
func TestStepV2SketchDeltaNonZero(t *testing.T) {
	s := NewWithFamily(0)
	s, _ = StepV2(s, 1)
	if s.SketchDelta == 0 {
		t.Fatalf("expected non-zero sketch delta after first step")
	}
}

// TestStepWord64V2AllZeros matches v2 per-bit semantics.
func TestStepWord64V2AllZeros(t *testing.T) {
	pre := NewWithFamily(3)
	for _, b := range []uint8{1, 0, 1, 1, 0, 0, 1} {
		pre, _ = StepV2(pre, b)
	}

	ref := pre
	for i := 0; i < 64; i++ {
		ref, _ = StepV2(ref, 0)
	}

	got, batch := StepWord64V2(pre, 0)
	if got != ref {
		t.Fatalf("state mismatch after all-zeros word\nref: %+v\ngot: %+v", ref, got)
	}
	_ = batch
}

// TestStepWord64V2AllOnes matches v2 per-bit semantics.
func TestStepWord64V2AllOnes(t *testing.T) {
	for _, initLastBit := range []uint8{0, 1} {
		pre := NewWithFamily(2)
		if initLastBit == 1 {
			pre, _ = StepV2(pre, 1)
		}

		ref := pre
		for i := 0; i < 64; i++ {
			ref, _ = StepV2(ref, 1)
		}

		got, batch := StepWord64V2(pre, ^uint64(0))
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

// TestStepWord64V2MixedRandom random-stream parity with per-bit StepV2.
func TestStepWord64V2MixedRandom(t *testing.T) {
	seed := uint64(42)
	for trial := 0; trial < 5000; trial++ {
		pre := NewWithFamily(trial % FamilyCount())
		// randomize initial state a bit
		for i := 0; i < int(seed%20); i++ {
			pre, _ = StepV2(pre, uint8(seed&1))
			seed = uint64(seed)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
		}

		word := seed

		ref := pre
		for i := 0; i < 64; i++ {
			b := uint8((word >> i) & 1)
			ref, _ = StepV2(ref, b)
		}

		got, _ := StepWord64V2(pre, word)
		if got != ref {
			t.Fatalf("random trial %d word=%016x\nref: %+v\ngot: %+v", trial, word, ref, got)
		}
		seed = uint64(seed)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}
}

// TestMixSketchAvalanche verifies mixSketch changes roughly half the bits.
func TestMixSketchAvalanche(t *testing.T) {
	const trials = 1000
	var totalDelta float64
	for i := 0; i < trials; i++ {
		in := uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
		out := mixSketch(in, HashFamilies[0].A, HashFamilies[0].B, HashFamilies[0].R)
		delta := bits.OnesCount64(in ^ out)
		totalDelta += float64(delta)
	}
	avg := totalDelta / trials
	// ideal avalanche: ~32 bits changed
	if avg < 20 || avg > 44 {
		t.Fatalf("mixSketch average bit delta = %.1f, expected ~32", avg)
	}
}

// TestFoldZeroRunCoverage ensures all buckets are hit.
func TestFoldZeroRunCoverage(t *testing.T) {
	inputs := []uint64{0, 1, 4, 5, 8, 12, 16, 20, 32, 40, 64, 100}
	seen := make(map[uint64]struct{})
	for _, zr := range inputs {
		seen[foldZeroRun(zr)] = struct{}{}
	}
	if len(seen) < 5 {
		t.Fatalf("expected 5 distinct buckets, got %d", len(seen))
	}
}

// TestEventSaltDistinctness verifies different events produce different salts.
func TestEventSaltDistinctness(t *testing.T) {
	s1 := eventSalt(Event{Kind: EventDilate, Payload: 1})
	s2 := eventSalt(Event{Kind: EventDilate, Payload: 2})
	s3 := eventSalt(Event{Kind: EventMarker, Payload: 1})
	s4 := eventSalt(Event{Kind: EventKind(99), Payload: 1}) // unknown event type
	if s1 == s2 {
		t.Fatalf("dilate salts with different payload should differ")
	}
	if s1 == s3 {
		t.Fatalf("dilate and marker salts should differ")
	}
	if s4 != 0 {
		t.Fatalf("unknown event type should return salt 0, got %d", s4)
	}
}

// TestStepWord64V2LongStream parity over many words.
func TestStepWord64V2LongStream(t *testing.T) {
	seed := uint64(2025)
	for trial := 0; trial < 200; trial++ {
		pre := NewWithFamily(trial % FamilyCount())
		ref := pre

		words := 1 + int(seed%128)
		for w := 0; w < words; w++ {
			word := seed
			for i := 0; i < 64; i++ {
				b := uint8((word >> i) & 1)
				ref, _ = StepV2(ref, b)
			}
			got, _ := StepWord64V2(pre, word)
			pre = got
			seed = uint64(seed)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
		}

		if pre != ref {
			t.Fatalf("long stream trial %d (%d words): state mismatch", trial, words)
		}
	}
}
