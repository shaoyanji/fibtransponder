package fsvm

import (
	"math/rand"
	"testing"
	"unsafe"
)

// TestSketchEntropy verifies the v1 sketch actually uses its full 64-bit range.
//
// Regression test: the fold was `Seeds[b] + uint64(W)`, and since W is 0..63
// the addition only ever perturbed the low bits. The upper 56 bits were then a
// 2-bit function of the bit counts, so a nominal 64-bit sketch took at most
// 4*256 = 1024 distinct values and collided after ~18 random streams instead
// of ~2^32.
func TestSketchEntropy(t *testing.T) {
	const trials = 20000
	seen := make(map[uint64]struct{}, trials)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < trials; i++ {
		s := New()
		n := 1 + rng.Intn(400)
		for j := 0; j < n; j++ {
			s, _ = Step(s, uint8(rng.Intn(2)))
		}
		seen[s.Sketch] = struct{}{}
	}
	// Random streams almost never repeat, so distinctness should be near-total.
	// A collapsed sketch scores ~256/20000 (1.3%).
	if len(seen) < trials*9/10 {
		t.Errorf("sketch entropy too low: %d distinct sketches from %d random streams (want >= %d)",
			len(seen), trials, trials*9/10)
	}
}

// TestSketchHighBitsCarryEntropy verifies the upper 56 bits are not a low-
// entropy function of bit counts, which was the specific failure mode.
func TestSketchHighBitsCarryEntropy(t *testing.T) {
	const trials = 20000
	hi := make(map[uint64]struct{}, trials)
	rng := rand.New(rand.NewSource(43))
	for i := 0; i < trials; i++ {
		s := New()
		n := 1 + rng.Intn(400)
		for j := 0; j < n; j++ {
			s, _ = Step(s, uint8(rng.Intn(2)))
		}
		hi[s.Sketch>>8] = struct{}{}
	}
	if len(hi) < trials*9/10 {
		t.Errorf("upper 56 bits are low-entropy: %d distinct values from %d streams (want >= %d)",
			len(hi), trials, trials*9/10)
	}
}

// TestSketchCollisionRate verifies collisions are rare at realistic stream
// lengths.
//
// The old fold collided after ~18 distinct streams (0% distinct). Measured
// behavior after the fix: ~95% of 100k random streams of length 1..200 are
// distinct, with the first collision around stream 350. The residual rate is
// inherent to XOR-folding a 128-symbol alphabet (2 bits x 64 windows): a
// stream is summarized by a parity vector, so short streams can coincide.
// This asserts the measured rate rather than an idealized birthday bound --
// see the "not claimed" note in SPEC.md section 7.
func TestSketchCollisionRate(t *testing.T) {
	const trials = 50000
	seen := make(map[uint64]struct{}, trials)
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < trials; i++ {
		s := New()
		n := 1 + rng.Intn(200)
		for j := 0; j < n; j++ {
			s, _ = Step(s, uint8(rng.Intn(2)))
		}
		seen[s.Sketch] = struct{}{}
	}
	ratio := float64(len(seen)) / float64(trials)
	if ratio < 0.90 {
		t.Errorf("sketch distinctness too low: %.1f%% of %d random streams distinct (want >= 90%%)",
			ratio*100, trials)
	}
}

// TestSketchDegenerateStreamLimitation documents a known, unchanged property:
// for an all-zero stream the window stays 0, so the fold reduces to a parity
// count of Seeds[0] and yields only 2 distinct sketches across all lengths.
// This is a property of pure-XOR folding, not of the window spreading; it is
// recorded so a future change does not mistake it for a regression.
func TestSketchDegenerateStreamLimitation(t *testing.T) {
	seen := make(map[uint64]struct{})
	for n := 1; n <= 500; n++ {
		s := New()
		for j := 0; j < n; j++ {
			s, _ = Step(s, 0)
		}
		seen[s.Sketch] = struct{}{}
	}
	if len(seen) > 2 {
		t.Logf("all-zero streams now yield %d distinct sketches (improvement over the known limit of 2)", len(seen))
	}
}

// TestSketchTermDistinctPerWindow verifies the 64 window values map to
// well-separated fold contributions rather than 64 adjacent small integers.
func TestSketchTermDistinctPerWindow(t *testing.T) {
	hi := make(map[uint64]struct{})
	for w := 0; w < 64; w++ {
		term := SketchTerm(DefaultSeeds, 1, uint8(w))
		if term < 1<<32 {
			t.Errorf("window %d: fold contribution 0x%016x is confined to the low 32 bits", w, term)
		}
		hi[term>>32] = struct{}{}
	}
	if len(hi) < 64 {
		t.Errorf("64 window values produced only %d distinct upper halves; window is not spread", len(hi))
	}
}

// TestStateSize pins the concrete footprint of State.
//
// docs/SPEC.md and docs/FORMAL_ANALYSIS.md previously documented 56/64
// bytes, which was wrong for the current struct (14 fields including the v2
// mix parameters and BitsProcessed).
func TestStateSize(t *testing.T) {
	const want = 96
	got := int(unsafe.Sizeof(State{}))
	if got != want {
		t.Errorf("sizeof(fsvm.State) = %d, want %d; update docs/SPEC.md and docs/FORMAL_ANALYSIS.md",
			got, want)
	}
}
