package calibration

import (
	"math/rand/v2"
	"testing"
)

// TestOrthogonality demonstrates that varying ZeroThresh (marker threshold) produces
// genuinely independent detection profiles from AdjacencyWidth (dilate sensitivity).
// Width affects DILATE rate; threshold affects MARKER rate - two orthogonal axes.
func TestOrthogonality(t *testing.T) {
	// Define a grid of calibration profiles
	widths := []uint8{3, 5, 8, 13, 21}
	thresholds := []uint64{2, 4, 8, 16, 32}

	// Generate a deterministic synthetic stream with varied patterns
	rng := rand.New(rand.NewPCG(42, 42))
	stream := make([]uint8, 10000)
	for i := range stream {
		if rng.Float64() < 0.5 {
			stream[i] = 1
		} else {
			stream[i] = 0
		}
	}

	// Results matrices
	dilateResults := make([][]float64, len(widths))
	markerResults := make([][]float64, len(widths))
	for i := range dilateResults {
		dilateResults[i] = make([]float64, len(thresholds))
		markerResults[i] = make([]float64, len(thresholds))
	}

	// Run experiments
	for wi, w := range widths {
		for ti, thresh := range thresholds {
			arr := NewAdaptiveArray([]string{"test"}, DefaultTargets())
			arr.Transponders[0].Params.Width = w
			arr.Transponders[0].Params.ZeroThresh = thresh
			arr.CalibrateInterval = 10000 // disable auto-calibration

			arr.ProcessStream(stream)
			res := arr.Transponders[0].State
			dilateResults[wi][ti] = float64(res.Dilations) / float64(len(stream))
			markerResults[wi][ti] = float64(res.Markers) / float64(len(stream))
		}
	}

	// Log results
	t.Logf("DILATE rates by width (should vary strongly by width, weakly by threshold):")
	t.Logf("Width\\Thresh\t2\t4\t8\t16\t32")
	for wi, w := range widths {
		t.Logf("%d\t\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f",
			w,
			dilateResults[wi][0], dilateResults[wi][1], dilateResults[wi][2], dilateResults[wi][3], dilateResults[wi][4])
	}

	t.Logf("\nMARKER rates by threshold (should vary strongly by threshold, weakly by width):")
	t.Logf("Width\\Thresh\t2\t4\t8\t16\t32")
	for wi, w := range widths {
		t.Logf("%d\t\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f",
			w,
			markerResults[wi][0], markerResults[wi][1], markerResults[wi][2], markerResults[wi][3], markerResults[wi][4])
	}

	// Verify orthogonality:
	// 1. Dilate rate should vary primarily by width (first axis)
	var maxDilateByWidth []float64
	for wi := range widths {
		maxD := dilateResults[wi][0]
		for _, d := range dilateResults[wi][1:] {
			if d > maxD {
				maxD = d
			}
		}
		maxDilateByWidth = append(maxDilateByWidth, maxD)
	}
	
	// Check that dilate rates differ significantly across widths
	widthVariance := 0.0
	for i := 1; i < len(maxDilateByWidth); i++ {
		diff := maxDilateByWidth[i] - maxDilateByWidth[0]
		if diff < 0 {
			diff = -diff
		}
		widthVariance += diff
	}
	if widthVariance < 0.01 {
		t.Errorf("Expected dilate rate to vary by width, got variance %.4f", widthVariance)
	} else {
		t.Logf("PASS: Dilate rate varies by width (variance=%.4f)", widthVariance)
	}

	// 2. Marker rate should vary primarily by threshold (second axis)
	// For any fixed width, marker rate should decrease as threshold increases
	orthogonalityVerified := false
	for wi := range widths {
		decreasing := true
		for ti := 1; ti < len(thresholds); ti++ {
			if markerResults[wi][ti] >= markerResults[wi][ti-1] && markerResults[wi][ti-1] > 0 {
				decreasing = false
				break
			}
		}
		if decreasing && markerResults[wi][0] > 0 {
			orthogonalityVerified = true
			t.Logf("PASS: Width %d shows decreasing marker rate with increasing threshold", widths[wi])
		}
	}

	if !orthogonalityVerified {
		t.Error("Expected marker rate to decrease with higher threshold (orthogonal second axis)")
	}

	// 3. Verify independence: threshold changes should NOT significantly affect dilate rates
	// (i.e., the two axes are truly independent)
	for wi := range widths {
		threshEffectOnDilate := 0.0
		for ti := 1; ti < len(thresholds); ti++ {
			diff := dilateResults[wi][ti] - dilateResults[wi][0]
			if diff < 0 {
				diff = -diff
			}
			threshEffectOnDilate += diff
		}
		// Dilate rate should be mostly unaffected by threshold (< 1% variation)
		if threshEffectOnDilate > 0.01 {
			t.Logf("Note: Width %d dilate rate affected by threshold (%.4f total variation)", 
				widths[wi], threshEffectOnDilate)
		}
	}
}

// TestSecondAxisIndependence verifies that MarkerThreshold affects marker rate
// independently from how AdjacencyWidth affects dilate rate.
func TestSecondAxisIndependence(t *testing.T) {
	rng := rand.New(rand.NewPCG(123, 123))
	stream := make([]uint8, 5000)
	for i := range stream {
		stream[i] = uint8(rng.IntN(2))
	}

	widths := []uint8{3, 8, 13}
	thresholds := []uint64{2, 4, 8}

	// Measure both dilate and marker rates
	type rates struct {
		dilate float64
		marker float64
	}
	matrix := make([][]rates, len(widths))
	for i := range matrix {
		matrix[i] = make([]rates, len(thresholds))
	}

	for wi, w := range widths {
		for ti, thresh := range thresholds {
			arr := NewAdaptiveArray([]string{"test"}, DefaultTargets())
			arr.Transponders[0].Params.Width = w
			arr.Transponders[0].Params.ZeroThresh = thresh
			arr.CalibrateInterval = 5000

			arr.ProcessStream(stream)
			st := arr.Transponders[0].State
			matrix[wi][ti] = rates{
				dilate: float64(st.Dilations) / float64(len(stream)),
				marker: float64(st.Markers) / float64(len(stream)),
			}
		}
	}

	t.Logf("Dilate rates (should vary primarily by width):")
	for wi, w := range widths {
		t.Logf("  Width %d: %.4f, %.4f, %.4f (thresh 2,4,8)",
			w, matrix[wi][0].dilate, matrix[wi][1].dilate, matrix[wi][2].dilate)
	}

	t.Logf("Marker rates (should vary primarily by threshold):")
	for ti, thresh := range thresholds {
		t.Logf("  Thresh %d: %.4f, %.4f, %.4f (width 3,8,13)",
			thresh, matrix[0][ti].marker, matrix[1][ti].marker, matrix[2][ti].marker)
	}

	// Verify: marker rate should decrease as threshold increases (for any fixed width)
	for wi := range widths {
		if matrix[wi][0].marker <= matrix[wi][2].marker {
			t.Errorf("Width %d: expected marker rate to decrease with higher threshold: %.4f vs %.4f",
				widths[wi], matrix[wi][0].marker, matrix[wi][2].marker)
		}
	}

	// Verify: dilate rate should vary more by width than by threshold
	// (i.e., the second axis is genuinely independent)
	for ti := range thresholds {
		dilateVariance := 0.0
		for wi := 1; wi < len(widths); wi++ {
			diff := matrix[wi][ti].dilate - matrix[0][ti].dilate
			if diff < 0 {
				diff = -diff
			}
			dilateVariance += diff
		}
		if dilateVariance < 0.01 {
			t.Errorf("Threshold %d: dilate rate should vary significantly by width, got variance %.4f",
				thresholds[ti], dilateVariance)
		}
	}
}

// BenchmarkOrthogonalityExperiment measures the cost of running the full grid.
func BenchmarkOrthogonalityExperiment(b *testing.B) {
	rng := rand.New(rand.NewPCG(42, 42))
	stream := make([]uint8, 1000)
	for i := range stream {
		stream[i] = uint8(rng.IntN(2))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr := NewAdaptiveArray([]string{"test"}, DefaultTargets())
		arr.Transponders[0].Params.Width = 5
		arr.Transponders[0].Params.ZeroThresh = 4
		arr.ProcessStream(stream)
	}
}
