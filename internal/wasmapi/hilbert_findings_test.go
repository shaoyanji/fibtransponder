package wasmapi

import (
	"math"
	"testing"
)

// Tests for the Hilbert codec demo.
//
// Two findings are pinned here. Both came out of building the demo and then
// measuring it, and both contradict something the repository's own docs imply,
// so they are worth failing a build over rather than leaving in a commit
// message.
//
// Finding 1: run-length coding over a thresholded 2D image mostly expands it.
// The Hilbert curve is space-filling but fractal, so it crosses a screen-space
// edge O(n^1.585) to O(n^2) times, not once. A four-region image measured 40
// runs at 8x8, 324 at 32x32 and 16376 at 256x256 -- about 3.7x per doubling.
// No choice of default image fixes this; it is a property of the curve.
//
// Finding 2: one-density does not determine compressibility. Clustering does.
// At 10% ones, contiguous gives ratio 0.24 and evenly scattered gives 1.52.
// docs/COMPRESSION.md quotes 0.44 for "10% ones" without saying which.

// encodePattern is a small helper: run the codec on a named pattern.
func encodePattern(t *testing.T, name string, length int) hilbertEncodeOut {
	t.Helper()
	var out hilbertEncodeOut
	callOK(t, "hilbert.encode", map[string]any{"pattern": name, "length": length}, &out)
	return out
}

// TestRatioIsPredictedByMeanRunLength is the demo's central chart: ratio
// against mean run length, monotonically, with break-even near 9.
//
// The arithmetic behind it: a run of length L costs 1 type bit + the Fibonacci
// code of (L+1) + 3 terminator bits, i.e. about log_phi(L) + 4 bits. That beats
// L only once L is around 9.
func TestRatioIsPredictedByMeanRunLength(t *testing.T) {
	names := PatternNames()
	type pt struct {
		meanRun, ratio float64
	}
	var pts []pt
	for _, n := range names {
		out := encodePattern(t, n, 1024)
		if out.MeanRunLength <= 0 {
			t.Fatalf("%s: meanRunLength = %d", n, out.MeanRunLength)
		}
		pts = append(pts, pt{float64(out.MeanRunLength), out.Ratio})
	}
	// More runs per bit means a worse ratio.
	for i := 1; i < len(pts); i++ {
		if pts[i].meanRun < pts[i-1].meanRun-1 {
			continue // order is not guaranteed; sort below
		}
	}
	sorted := append([]pt(nil), pts...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].meanRun < sorted[j-1].meanRun; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	for i := 1; i < len(sorted); i++ {
		if sorted[i].ratio > sorted[i-1].ratio+1e-9 {
			t.Errorf("ratio rose as mean run length fell: meanRun %.0f -> %.0f gave ratio %.3f -> %.3f",
				sorted[i-1].meanRun, sorted[i].meanRun, sorted[i-1].ratio, sorted[i].ratio)
		}
	}
	// Break-even should sit in the neighbourhood the arithmetic predicts.
	if !(sorted[len(sorted)-1].ratio < 1) {
		t.Errorf("the best case did not compress: ratio %.3f", sorted[len(sorted)-1].ratio)
	}
	if sorted[0].ratio <= 1 {
		t.Errorf("the worst case did not expand: ratio %.3f", sorted[0].ratio)
	}
}

// TestClusteringNotDensityDeterminesCompression is finding 2. Same one-density,
// three arrangements, and the docs' framing only holds for one of them.
func TestClusteringNotDensityDeterminesCompression(t *testing.T) {
	const n = 100
	clustered := encodePattern(t, "clustered-sparse", n)
	scattered := encodePattern(t, "scattered-ones", n)

	// Both must be 10% ones, or the comparison is meaningless.
	clDensity := float64(clustered.Ones) / float64(clustered.BitCount)
	scDensity := float64(scattered.Ones) / float64(scattered.BitCount)
	if math.Abs(clDensity-scDensity) > 0.02 {
		t.Fatalf("differing densities, so the comparison proves nothing: %.3f vs %.3f",
			clDensity, scDensity)
	}

	if clustered.Ratio >= 1 {
		t.Errorf("clustered 10%% ones expanded: ratio %.3f, runs %d", clustered.Ratio, len(clustered.Runs))
	}
	if scattered.Ratio <= 1 {
		t.Errorf("scattered 10%% ones compressed: ratio %.3f, runs %d; the contrast this "+
			"test exists to demonstrate has disappeared", scattered.Ratio, len(scattered.Runs))
	}
	// The gap has to be large enough to be worth showing on a chart.
	if scattered.Ratio/cluttered(clustered.Ratio) < 4 {
		t.Errorf("gap between clustered (%.3f) and scattered (%.3f) is too small to be "+
			"a useful illustration", clustered.Ratio, scattered.Ratio)
	}

	t.Logf("10%% ones: clustered ratio %.3f over %d runs; scattered ratio %.3f over %d runs",
		clustered.Ratio, len(clustered.Runs), scattered.Ratio, len(scattered.Runs))
}

func cluttered(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

// TestZerosReproducesDocumentedRatio pins the one figure in
// docs/COMPRESSION.md this shim can reproduce exactly: 100 zeros -> 16 coded
// bits, ratio 0.16.
func TestZerosReproducesDocumentedRatio(t *testing.T) {
	out := encodePattern(t, "zeros", 100)
	if out.CodedBits != 16 {
		t.Errorf("100 zeros encoded to %d bits; docs/COMPRESSION.md records 16", out.CodedBits)
	}
	if math.Abs(out.Ratio-0.16) > 5e-3 {
		t.Errorf("ratio = %.4f, docs/COMPRESSION.md records 0.16", out.Ratio)
	}
	if len(out.Runs) != 1 {
		t.Errorf("%d runs, want 1", len(out.Runs))
	}
}

// TestAlternatingIsTheWorstCase: 0101... spends 1 type bit + 1 Fibonacci bit +
// 3 terminator bits on every single input bit, so the ratio is exactly 6.
func TestAlternatingIsTheWorstCase(t *testing.T) {
	out := encodePattern(t, "alternating", 64)
	if math.Abs(out.Ratio-6.0) > 0.2 {
		t.Errorf("alternating ratio = %.3f, want about 6.0 (6 bits stored per bit)", out.Ratio)
	}
	if out.MeanRunLength != 1 {
		t.Errorf("meanRunLength = %d, want 1", out.MeanRunLength)
	}
}

// TestHilbertImageRunsGrowSuperlinearly is finding 1: the run count for a
// fixed-region image grows by roughly 3.7x per doubling of side length, so the
// codec cannot compress thresholded 2D images at scale.
func TestHilbertImageRunsGrowSuperlinearly(t *testing.T) {
	countRuns := func(order int) int {
		var out hilbertEncodeOut
		callOK(t, "hilbert.encode", map[string]any{
			"order": order, "threshold": 128, "preset": presetPhoto,
		}, &out)
		return len(out.Runs)
	}
	small, large := countRuns(6), countRuns(7)
	if small == 0 {
		t.Fatal("no runs at order 6")
	}
	growth := float64(large) / float64(small)
	// Linear growth would be 4x for a 4x increase in pixels. Observed is around
	// 3.7x, so the guard is deliberately loose: it is here to catch the finding
	// being invalidated, not to pin the exponent.
	if growth < 2.5 {
		t.Errorf("run count grew only %.2fx from order 6 to 7 (%d -> %d); the "+
			"superlinear-crossing finding may no longer hold", growth, small, large)
	}
	t.Logf("runs: order 6 = %d, order 7 = %d (%.2fx for 4x the pixels)", small, large, growth)

	// And the practical consequence: the ratio is an expansion, not a
	// compression, at the sizes a real image would use.
	var out hilbertEncodeOut
	callOK(t, "hilbert.encode", map[string]any{"order": 7, "threshold": 128, "preset": presetPhoto}, &out)
	if out.Ratio <= 1 {
		t.Logf("note: order 7 photo now compresses at ratio %.3f", out.Ratio)
	}
}

// TestOnlySmoothRegionImagesCompress guards the honest framing: among the
// generated presets, the ones with genuinely few edge crossings compress, and
// the speckled ones do not. If a future preset change makes everything
// compress, this test says the page lost its point.
func TestOnlySmoothRegionImagesCompress(t *testing.T) {
	compressing := map[string]bool{}
	for _, name := range PresetNames() {
		var out hilbertEncodeOut
		callOK(t, "hilbert.encode", map[string]any{
			"order": 5, "threshold": 128, "preset": name,
		}, &out)
		compressing[name] = out.Ratio < 1
		t.Logf("%-13s ratio %.3f over %d runs (mean run %d)",
			name, out.Ratio, len(out.Runs), out.MeanRunLength)
	}
	if !compressing[presetGradient] {
		t.Errorf("the gradient preset should compress (a single threshold edge)")
	}
	for _, name := range []string{presetNoise, presetCheckerboard} {
		if compressing[name] {
			t.Errorf("%s compressed, but every run in it is one bit by construction", name)
		}
	}
}
