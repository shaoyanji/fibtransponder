package transponder

import (
	"fmt"
	"testing"
)

func TestPerTransponderCalibration(t *testing.T) {
	// Create array with 3 different calibrations.
	arr := NewArray([]Calibration{
		CalibrationTight,
		CalibrationMedium,
		CalibrationWide,
	})

	// Feed a deterministic bitstream with known adjacency patterns.
	// Pattern: alternating with one double-1 to trigger DILATE.
	bits := []uint8{1, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 0, 0, 0, 1}
	results := arr.ProcessStream(bits)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// All transponders must agree on DILATE count (same adjacency events).
	dil := results[0].Dilations
	for _, r := range results[1:] {
		if r.Dilations != dil {
			t.Errorf("dilation mismatch: %s=%d vs %s=%d",
				results[0].Name, dil, r.Name, r.Dilations)
		}
	}

	// Sketches MUST differ (different seed tables).
	sketches := make(map[uint64]string)
	for _, r := range results {
		if _, exists := sketches[r.Sketch]; exists {
			t.Errorf("sketch collision: %s and %s produced identical sketch 0x%x",
				sketches[r.Sketch], r.Name, r.Sketch)
		}
		sketches[r.Sketch] = r.Name
	}

	if len(sketches) != 3 {
		t.Errorf("expected 3 distinct sketches, got %d", len(sketches))
	}

	t.Logf("DILATE count (all): %d", dil)
	for _, r := range results {
		t.Logf("  %s: sketch=0x%016x r=%d", r.Name, r.Sketch, r.R)
	}
}

func TestCalibrationDivergence(t *testing.T) {
	// Longer stream to amplify divergence.
	arr := NewArray([]Calibration{
		CalibrationTight,
		CalibrationMedium,
		CalibrationWide,
	})

	// Generate 1000 bits with ~20% ones (sparse, many markers).
	bits := make([]uint8, 1000)
	for i := range bits {
		// Deterministic pattern: fibonacci-inspired bit placement
		if i%5 == 0 || i%13 == 0 {
			bits[i] = 1
		}
	}

	results := arr.ProcessStream(bits)

	// Report profiles.
	for _, r := range results {
		fmt.Printf("%-8s dilations=%-4d markers=%-4d r=%-2d sketch=0x%016x\n",
			r.Name, r.Dilations, r.Markers, r.R, r.Sketch)
	}

	// Verify sketches all differ.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Sketch == results[j].Sketch {
				t.Errorf("sketch collision between %s and %s: 0x%016x",
					results[i].Name, results[j].Name, results[i].Sketch)
			}
		}
	}

	// DILATE count should be identical (same input, same adjacency detection).
	for i := 1; i < len(results); i++ {
		if results[i].Dilations != results[0].Dilations {
			t.Errorf("dilation divergence: %s=%d vs %s=%d (should be identical)",
				results[0].Name, results[0].Dilations, results[i].Name, results[i].Dilations)
		}
	}
}
