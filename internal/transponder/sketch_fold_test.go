package transponder

import (
	"math/bits"
	"math/rand"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// ── Sketch fold conformance ──
//
// fsvm.SketchTerm is documented as the single source of truth for the v1 fold:
//
//	"every ingest path that maintains a sketch by hand ... must use this, or
//	 its state will silently diverge from fsvm.Step."
//
// That contract was violated in two places: StepWidth and StepFull both inlined
// `s.Seeds[b] + uint64(s.W)`. Because the 6-bit window is only 0..63, adding it
// straight to a seed perturbs the low bits and leaves the upper 56 bits a
// function of bit counts alone -- the sketch collapses to at most
// 4*256 = 1024 distinct values, which is the exact defect REPORT_CORPUS.md
// documents as fixed in the core. The dilation and marker counters were
// unaffected (the fold touches only Sketch), so no published event-rate table
// was wrong; only the sketches emitted by these two paths were.
//
// A comment was not enough to prevent this, so these tests assert the property
// directly.

const sketchFoldTrials = 2000

func sketchFoldStreams(t *testing.T, seed int64) [][]uint8 {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	streams := make([][]uint8, sketchFoldTrials)
	for i := range streams {
		n := 1 + rng.Intn(1024)
		bits := make([]uint8, n)
		// Bias the 1-density so both degenerate (all-zero) and saturated
		// (all-one) windows get exercised alongside the ordinary case.
		onePct := []int{5, 35, 55, 95, 100}[i%5]
		for j := range bits {
			if rng.Intn(100) < onePct {
				bits[j] = 1
			}
		}
		streams[i] = bits
	}
	return streams
}

// TestStructuralStepMatchesCoreAtWidth1 asserts that StepWidth at Width1 is
// bit-identical to fsvm.Step: same sketch, same counters, same window. At
// Width1 the adjacency rule is the same LastBit==1 && b==1 and the marker rule
// is the same zeroRun>=8 && isPow2, so the two must agree on every field.
func TestStructuralStepMatchesCoreAtWidth1(t *testing.T) {
	for i, bits := range sketchFoldStreams(t, 20260930) {
		core := fsvm.New()
		alt := fsvm.New()
		for j, b := range bits {
			core, _ = fsvm.Step(core, b)
			alt, _ = StepWidth(alt, b, Width1)
			if core.Sketch != alt.Sketch {
				t.Fatalf("stream %d bit %d: StepWidth sketch diverged from fsvm.Step: core=%d alt=%d",
					i, j, core.Sketch, alt.Sketch)
			}
		}
		if core.Dilations != alt.Dilations || core.Markers != alt.Markers ||
			core.R != alt.R || core.W != alt.W || core.ZeroRun != alt.ZeroRun ||
			core.BitsProcessed != alt.BitsProcessed {
			t.Fatalf("stream %d: counter state diverged: core=%+v alt=%+v", i, core, alt)
		}
	}
	t.Logf("StepWidth(Width1) is bit-identical to fsvm.Step over %d randomized streams", sketchFoldTrials)
}

// TestStepFullMatchesCoreAtDefaultThreshold is the StepWidth analogue for
// StepFull. ThresholdDefault resolves to the same powers-of-two-from-8 rule the
// core hardcodes (thresholdFired, PowersOf2 branch), so at Width1 the two
// functions must agree completely.
func TestStepFullMatchesCoreAtDefaultThreshold(t *testing.T) {
	if !thresholdFired(8, ThresholdDefault) || !thresholdFired(16, ThresholdDefault) || thresholdFired(4, ThresholdDefault) {
		t.Fatalf("ThresholdDefault no longer matches the core's >=8 && isPow2 rule; " +
			"TestStepFullMatchesCoreAtDefaultThreshold is asserting against a stale rule")
	}
	for i, bits := range sketchFoldStreams(t, 20260931) {
		core := fsvm.New()
		alt := fsvm.New()
		for j, b := range bits {
			core, _ = fsvm.Step(core, b)
			alt, _ = StepFull(alt, b, Width1, ThresholdDefault)
			if core.Sketch != alt.Sketch {
				t.Fatalf("stream %d bit %d: StepFull sketch diverged from fsvm.Step: core=%d alt=%d",
					i, j, core.Sketch, alt.Sketch)
			}
		}
		if core.Dilations != alt.Dilations || core.Markers != alt.Markers ||
			core.R != alt.R || core.W != alt.W {
			t.Fatalf("stream %d: counter state diverged: core=%+v alt=%+v", i, core, alt)
		}
	}
	t.Logf("StepFull(Width1, ThresholdDefault) is bit-identical to fsvm.Step over %d randomized streams", sketchFoldTrials)
}

// TestSketchIndependentOfWidth asserts the sketch is a function of the bit
// stream alone: varying adjacency width changes which events fire, never the
// fingerprint. This is the invariant that makes cross-transponder sketch
// comparison meaningful, and the one the old inline fold quietly broke.
func TestSketchIndependentOfWidth(t *testing.T) {
	for i, bits := range sketchFoldStreams(t, 20260932) {
		var sketches [3]uint64
		var dils [3]uint64
		for wi, w := range []AdjacencyWidth{Width1, Width2, Width3} {
			st := fsvm.New()
			for _, b := range bits {
				st, _ = StepWidth(st, b, w)
			}
			sketches[wi] = st.Sketch
			dils[wi] = st.Dilations
		}
		if sketches[0] != sketches[1] || sketches[1] != sketches[2] {
			t.Fatalf("stream %d: sketch varied with width: w1=%d w2=%d w3=%d",
				i, sketches[0], sketches[1], sketches[2])
		}
		// The event side must still nest; otherwise this test is vacuous.
		if !(dils[0] >= dils[1] && dils[1] >= dils[2]) {
			t.Fatalf("stream %d: dilation counts not nested across widths: w1=%d w2=%d w3=%d",
				i, dils[0], dils[1], dils[2])
		}
	}
	t.Logf("Sketch is width-independent while dilation counts nest, over %d randomized streams", sketchFoldTrials)
}

// TestStructuralSketchUsesFullRange is the entropy guard for these two paths
// specifically. The core has TestSketchEntropy for fsvm.Step; without this,
// the structural paths could regress to a collapsed fold while the core test
// stayed green, which is precisely what happened.
func TestStructuralSketchUsesFullRange(t *testing.T) {
	const streams = 2000
	seen := make(map[uint64]struct{}, streams)
	for i := 0; i < streams; i++ {
		rng := rand.New(rand.NewSource(int64(7000 + i)))
		n := 1 + rng.Intn(400)
		st := fsvm.New()
		for j := 0; j < n; j++ {
			var b uint8
			if rng.Intn(2) == 1 {
				b = 1
			}
			st, _ = StepWidth(st, b, Width1)
		}
		seen[st.Sketch] = struct{}{}
	}
	// A correctly spread fold yields close to one distinct sketch per stream.
	// The collapsed fold caps out at 4*256 = 1024.
	if len(seen) < streams*9/10 {
		t.Errorf("StepWidth sketch collapsed: %d distinct over %d random streams (collapsed fold caps at 1024)",
			len(seen), streams)
	}
	popc := 0
	for k := range seen {
		popc += bits.OnesCount64(k)
	}
	t.Logf("%d/%d distinct sketches from StepWidth, mean popcount %.1f/64",
		len(seen), streams, float64(popc)/float64(len(seen)))
}
