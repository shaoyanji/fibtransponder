package transponder

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// ── Structural calibration experiment ──
//
// Hold seeds constant (DefaultSeeds for all transponders).
// Vary only adjacency geometry: w=1, w=2, w=3.
// Same corpus, same harness as TestCorpusExperiment.
//
// NOTE ON WHAT THESE AXES ARE (asserted below by
// assertWidthOrderingIsNestedRandomized):
// Width1/2/3 are NOT independent detectors. They are nested run-length
// thresholds on one latent variable (the distribution of lengths of
// consecutive 1-runs), so dilation rate at w=1 is >= w=2 >= w=3 for every
// possible input, by construction. The experiment below therefore documents
// *how much* the sensitivity profile shifts, not an independent second axis.
//
// The verdicts printed by this test are observations about one small corpus;
// the structural invariant is asserted in this file. The threshold axis is a
// separate matter and is covered by TestSecondAxisCalibration, which asserts
// that zero markers fire on these corpora and therefore that no independence
// conclusion is available from them.

var structuralWidths = []StructuralCalibration{
	{"w=1", Width1},
	{"w=2", Width2},
	{"w=3", Width3},
}

func TestStructuralCalibration(t *testing.T) {
	corpus := []struct {
		label string
		data  []byte
	}{
		{"prose", corpusProse},
		{"code", corpusCode},
		{"synthetic", corpusSynthetic},
	}

	const structuralWindow = 2048 // bits per window

	type ClassReport struct {
		label    string
		bitCount int
		results  []Result
		windows  [][]WindowMetrics // [transponder][window]
	}

	allReports := make([]ClassReport, len(corpus))

	for ci, c := range corpus {
		bits := BytesToBits(c.data)
		arr := NewStructuralArray(structuralWidths)

		nWin := (len(bits) + structuralWindow - 1) / structuralWindow
		windows := make([][]WindowMetrics, len(structuralWidths))
		for wi := range windows {
			windows[wi] = make([]WindowMetrics, nWin)
		}

		windowDil := make([]uint64, len(structuralWidths))
		windowMark := make([]uint64, len(structuralWidths))

		for bitIdx, b := range bits {
			windowIdx := bitIdx / structuralWindow

			for ti := range arr.Transponders {
				t2 := &arr.Transponders[ti]
				prevDil := t2.State.Dilations
				prevMark := t2.State.Markers

				var evs []fsvm.Event
				t2.State, evs = StepWidth(t2.State, b, t2.Width)
				_ = evs

				windowDil[ti] += t2.State.Dilations - prevDil
				windowMark[ti] += t2.State.Markers - prevMark
			}

			atEnd := (bitIdx+1)%structuralWindow == 0
			atLast := bitIdx == len(bits)-1

			if atEnd || atLast {
				bitsInWin := structuralWindow
				if atLast && !atEnd {
					bitsInWin = (bitIdx % structuralWindow) + 1
				}
				for ti := range arr.Transponders {
					t2 := &arr.Transponders[ti]
					windows[ti][windowIdx] = WindowMetrics{
						WindowIndex: windowIdx,
						BitCount:    bitsInWin,
						Dilations:   windowDil[ti],
						Markers:     windowMark[ti],
						DilateRate:  float64(windowDil[ti]) / float64(bitsInWin),
						MarkerRate:  float64(windowMark[ti]) / float64(bitsInWin),
						Sketch:      t2.State.Sketch,
						R:           t2.State.R,
					}
					windowDil[ti] = 0
					windowMark[ti] = 0
				}
			}
		}

		finalResults := make([]Result, len(arr.Transponders))
		for i, t2 := range arr.Transponders {
			finalResults[i] = Result{
				Name:      t2.Name,
				Dilations: t2.State.Dilations,
				Markers:   t2.State.Markers,
				Sketch:    t2.State.Sketch,
				R:         t2.State.R,
			}
		}

		allReports[ci] = ClassReport{
			label:    c.label,
			bitCount: len(bits),
			results:  finalResults,
			windows:  windows,
		}
	}

	// ── Print results ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("STRUCTURAL CALIBRATION: fixed seeds, varying adjacency width")
	t.Log("═══════════════════════════════════════════════════════════════")

	for _, cr := range allReports {
		t.Logf("")
		t.Logf("── %s (%d bits) ──", cr.label, cr.bitCount)
		t.Logf("%-6s  %8s  %8s  %12s  %12s", "width", "dil", "mark", "dil-rate", "mark-rate")
		for _, r := range cr.results {
			dr := float64(r.Dilations) / float64(cr.bitCount)
			mr := float64(r.Markers) / float64(cr.bitCount)
			t.Logf("%-6s  %8d  %8d  %12.6f  %12.6f",
				r.Name, r.Dilations, r.Markers, dr, mr)
		}

		// Windowed DILATE rate heatmap
		t.Logf("")
		t.Logf("  Per-window dil-rate:")
		nWin := len(cr.windows[0])
		for w := 0; w < nWin; w++ {
			line := fmt.Sprintf("    W%-2d: ", w)
			for ti := range cr.windows {
				line += fmt.Sprintf("%s=%.4f  ", structuralWidths[ti].Name, cr.windows[ti][w].DilateRate)
			}
			t.Log(line)
		}
	}

	// ── Cross-class sensitivity matrix ──
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SENSITIVITY MATRIX: dil-rate per (width × class)")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	header := fmt.Sprintf("%-6s", "width")
	for _, cr := range allReports {
		header += fmt.Sprintf("  %10s", cr.label)
	}
	t.Log(header)
	for wi := range structuralWidths {
		row := fmt.Sprintf("%-6s", structuralWidths[wi].Name)
		for ci := range allReports {
			cr := allReports[ci]
			dr := float64(cr.results[wi].Dilations) / float64(cr.bitCount)
			row += fmt.Sprintf("  %10.6f", dr)
		}
		t.Log(row)
	}

	// ── Falsification analysis ──
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("FALSIFICATION CHECK (observations, not assertions -- see below)")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	// Check A: Do wider windows just monotonically scale counts?
	// Width1/2/3 are nested run-length thresholds, so rates are monotone
	// decreasing in width for EVERY input by construction. The meaningful
	// question is not monotonicity but how the *class ranking* shifts.
	allMonotonic := true
	for ci := range allReports {
		r1 := float64(allReports[ci].results[0].Dilations) / float64(allReports[ci].bitCount)
		r2 := float64(allReports[ci].results[1].Dilations) / float64(allReports[ci].bitCount)
		r3 := float64(allReports[ci].results[2].Dilations) / float64(allReports[ci].bitCount)
		if !(r1 > r2 && r2 > r3) {
			allMonotonic = false
		}
	}

	if allMonotonic {
		t.Log("OBS A: dil-rates strictly decreasing w=1 > w=2 > w=3 for ALL classes")
		t.Log("        → expected: widths are nested thresholds on 1-run length")
	} else {
		t.Log("OBS A: dil-rate ordering NOT strictly monotonic across widths")
		t.Log("        → this happens only when a class has zero dilations at some width (tie)")
	}

	// Check B: Does the "most sensitive class" change with width?
	// For each width, which class has the highest dil-rate?
	sensitiveTo := make([]string, len(structuralWidths))
	for wi := range structuralWidths {
		bestClass := ""
		bestRate := -1.0
		for ci := range allReports {
			dr := float64(allReports[ci].results[wi].Dilations) / float64(allReports[ci].bitCount)
			if dr > bestRate {
				bestRate = dr
				bestClass = allReports[ci].label
			}
		}
		sensitiveTo[wi] = bestClass
	}

	allSame := true
	for i := 1; i < len(sensitiveTo); i++ {
		if sensitiveTo[i] != sensitiveTo[0] {
			allSame = false
		}
	}

	if allSame {
		t.Logf("OBS B: all widths are most sensitive to class '%s'", sensitiveTo[0])
		t.Log("        → no cross-class sensitivity shift on this corpus")
	} else {
		t.Logf("OBS B: most-sensitive class differs by width (first = '%s')", sensitiveTo[0])
		for wi, cls := range sensitiveTo {
			t.Logf("        %s → most sensitive to '%s'", structuralWidths[wi].Name, cls)
		}
	}

	// Assert the structural property that actually holds and that the
	// "independent axes" narrative depends on being false.
	for _, cr := range allReports {
		for wi := 1; wi < len(cr.results); wi++ {
			prev := cr.results[wi-1].Dilations
			cur := cr.results[wi].Dilations
			if cur > prev {
				t.Errorf("%s: nested-width invariant violated: %s produced %d dilations > %s's %d",
					cr.label, structuralWidths[wi].Name, cur, structuralWidths[wi-1].Name, prev)
			}
		}
	}
	assertWidthOrderingIsNestedRandomized(t)

	// Check C: Do temporal distributions differ?
	// Compare windowed dil-rate variance across widths for each class.
	t.Log("")
	t.Log("Windowed dil-rate variance (σ²):")
	for _, cr := range allReports {
		for ti, ws := range cr.windows {
			mean := 0.0
			for _, w := range ws {
				mean += w.DilateRate
			}
			mean /= float64(len(ws))
			variance := 0.0
			for _, w := range ws {
				d := w.DilateRate - mean
				variance += d * d
			}
			variance /= float64(len(ws))
			t.Logf("  %s/%s: mean=%.4f σ²=%.6f", cr.label, structuralWidths[ti].Name, mean, variance)
		}
	}
}

// assertWidthOrderingIsNestedRandomized verifies the nested-width invariant on
// randomized inputs, independent of the fixed corpora, so that a refactor
// which breaks the nesting is caught.
//
// Width1/2/3 test for a 1-run of length >= 2/3/4. Those events are nested:
// {run>=4} is a subset of {run>=3}, which is a subset of {run>=2}. So
// dilation counts are non-increasing in width for EVERY possible input. This
// is why width is an ordered threshold on one latent variable, not an
// independent second axis.
func assertWidthOrderingIsNestedRandomized(t *testing.T) {
	t.Helper()
	rng := rand.New(rand.NewSource(20240914))
	violations := 0
	const trials = 2000
	for trial := 0; trial < trials; trial++ {
		n := 64 + rng.Intn(1024)
		bits := make([]uint8, n)
		for i := range bits {
			if rng.Intn(100) < 55 {
				bits[i] = 1
			}
		}
		rates := [3]float64{}
		for wi, w := range []AdjacencyWidth{Width1, Width2, Width3} {
			st := fsvm.New()
			for _, b := range bits {
				st, _ = StepWidth(st, b, w)
			}
			rates[wi] = float64(st.Dilations) / float64(n)
		}
		if !(rates[0] >= rates[1] && rates[1] >= rates[2]) {
			violations++
			if violations <= 3 {
				t.Errorf("trial %d: rates not nested: w1=%.6f w2=%.6f w3=%.6f", trial, rates[0], rates[1], rates[2])
			}
		}
	}
	if violations > 0 {
		t.Errorf("%d/%d randomized streams violated the nested-width invariant", violations, trials)
	}
	t.Logf("Nested-width invariant held on %d randomized streams (widths are ordered thresholds, not independent axes)", trials)
}

func TestStructuralByteCounts(t *testing.T) {
	t.Logf("prose:      %d bytes = %d bits", len(corpusProse), len(corpusProse)*8)
	t.Logf("code:       %d bytes = %d bits", len(corpusCode), len(corpusCode)*8)
	t.Logf("synthetic:  %d bytes = %d bits", len(corpusSynthetic), len(corpusSynthetic)*8)
}
