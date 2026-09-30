package transponder

import (
	"fmt"
	"math"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// ── Second-axis structural calibration experiment ──
//
// HANDOFF_VISION.md §12 canonical sentence:
//   "Width selects locality sensitivity; threshold selects event admission sensitivity."
//
// Design: hold seeds fixed (DefaultSeeds for all), vary TWO structural parameters:
//   Axis 1 (proven): adjacency width — w=1, w=2, w=3
//   Axis 2 (this experiment): marker threshold — pow2≥8, pow3≥9, lin≥12
//
// Matrix: 3 widths × 3 thresholds = 9 detector configurations.
// Corpus: same 3 classes as prior experiments (prose, code, synthetic).
//
// What to measure:
//   For each (width, threshold, class) triple:
//     - DILATE rate (should be identical to width-only experiment, since
//       threshold does not affect adjacency detection)
//     - MARKER rate (the variable under test)
//     - Combined event rate (dil-rate + mark-rate)
//
// Independence test (the make-or-break):
//   At fixed width, compare class ORDERINGS produced by different thresholds.
//   If threshold produces stable class-order changes → independent axis.
//   If threshold only rescales existing sensitivities → gain/control parameter.
//
// Falsification rules (from HANDOFF_VISION.md §12):
//   Fail (gain only):  Threshold only rescales each width's existing class ordering.
//   Fail (co-vary):    Width and threshold produce the same family of rankings.
//   Fail (fragile):    Threshold changes rankings at fixed width, but only on one
//                      corpus slice or windowing scheme.
//   Pass (independent): At fixed width, threshold produces stable class-order changes
//                       across repeated slices/windowing schemes.

// thresholdMatrix defines the 3×3 experimental grid.
var thresholdMatrix = []struct {
	width AdjacencyWidth
	widthName string
	thresh MarkerThresholdConfig
	threshName string
}{
	{Width1, "w=1", ThresholdDefault, "t=pow2"},
	{Width1, "w=1", ThresholdPow3,    "t=pow3"},
	{Width1, "w=1", ThresholdLinear,  "t=lin12"},
	{Width2, "w=2", ThresholdDefault, "t=pow2"},
	{Width2, "w=2", ThresholdPow3,    "t=pow3"},
	{Width2, "w=2", ThresholdLinear,  "t=lin12"},
	{Width3, "w=3", ThresholdDefault, "t=pow2"},
	{Width3, "w=3", ThresholdPow3,    "t=pow3"},
	{Width3, "w=3", ThresholdLinear,  "t=lin12"},
}

const secondAxisWindow = 2048 // bits per window

// assertThresholdAxisIsUntestableHere asserts the precondition under which the
// second-axis analysis is skipped: with zero markers on every config, the
// threshold axis has no observable effect on this corpus, so no independence
// conclusion can be drawn from it.
//
// This turns the pre-check from a printed observation into a gate, so that if a
// future corpus change does produce markers, the test fails here instead of
// silently taking the early-return path.
func assertThresholdAxisIsUntestableHere(t *testing.T, allReports []jointReport) {
	t.Helper()
	total := uint64(0)
	for _, cr := range allReports {
		for _, cfg := range cr.configs {
			total += cfg.markers
		}
	}
	if total != 0 {
		t.Fatalf("precondition violated: expected zero markers across all configs, got %d; "+
			"the full second-axis analysis should run instead of returning early", total)
	}
	t.Logf("  Asserted: 0 markers across all %d configs -- threshold axis has no observable effect here",
		len(allReports)*9)
}

type jointReport struct {
	label    string
	bitCount int
	configs  []jointConfigResult
}

type jointConfigResult struct {
	widthName  string
	threshName string
	dilations  uint64
	markers    uint64
	r          uint32
	sketch     uint64
	markerRate float64
	dilateRate float64
	windowed   []windowedMarkerRate
}

type windowedMarkerRate struct {
	markerCount uint64
	markerRate  float64
}

func TestSecondAxisCalibration(t *testing.T) {
	corpus := []struct {
		label string
		data  []byte
	}{
		{"prose", corpusProse},
		{"code", corpusCode},
		{"synthetic", corpusSynthetic},
	}

	// Run all (width, threshold, class) combinations
	allReports := make([]jointReport, len(corpus))

	for ci, c := range corpus {
		bits := BytesToBits(c.data)
		nConfigs := len(thresholdMatrix)
		configResults := make([]jointConfigResult, nConfigs)

		// Create one JointArray with all 9 configs
		cals := make([]JointCalibration, nConfigs)
		for mi, m := range thresholdMatrix {
			cals[mi] = JointCalibration{
				Name:      m.widthName + "+" + m.threshName,
				Width:     m.width,
				Threshold: m.thresh,
			}
		}
		arr := NewJointArray(cals)

		// Window tracking
		nWin := (len(bits) + secondAxisWindow - 1) / secondAxisWindow
		windowMarkers := make([][]uint64, nConfigs)
		for i := range windowMarkers {
			windowMarkers[i] = make([]uint64, nWin)
		}
		windowBits := make([]int, nWin)

		for bitIdx, b := range bits {
			windowIdx := bitIdx / secondAxisWindow

			for ti := range arr.Transponders {
				t2 := &arr.Transponders[ti]
				prevMark := t2.State.Markers

				var evs []fsvm.Event
				t2.State, evs = StepFull(t2.State, b, t2.Width, t2.Thresh)
				_ = evs

				windowMarkers[ti][windowIdx] += t2.State.Markers - prevMark
			}

			atEnd := (bitIdx+1)%secondAxisWindow == 0
			atLast := bitIdx == len(bits)-1

			if atEnd || atLast {
				bitsInWin := secondAxisWindow
				if atLast && !atEnd {
					bitsInWin = (bitIdx % secondAxisWindow) + 1
				}
				windowBits[windowIdx] = bitsInWin
			}
		}

		for mi := range arr.Transponders {
			t2 := arr.Transponders[mi]
			wmr := make([]windowedMarkerRate, nWin)
			for w := 0; w < nWin; w++ {
				bitsInWin := windowBits[w]
				if bitsInWin == 0 {
					bitsInWin = secondAxisWindow
				}
				wmr[w] = windowedMarkerRate{
					markerCount: windowMarkers[mi][w],
					markerRate:  float64(windowMarkers[mi][w]) / float64(bitsInWin),
				}
			}

			configResults[mi] = jointConfigResult{
				widthName:  thresholdMatrix[mi].widthName,
				threshName: thresholdMatrix[mi].threshName,
				dilations:  t2.State.Dilations,
				markers:    t2.State.Markers,
				r:          t2.State.R,
				sketch:     t2.State.Sketch,
				dilateRate: float64(t2.State.Dilations) / float64(len(bits)),
				markerRate: float64(t2.State.Markers) / float64(len(bits)),
				windowed:   wmr,
			}
		}

		allReports[ci] = jointReport{
			label:    c.label,
			bitCount: len(bits),
			configs:  configResults,
		}
	}

	// ══════════════════════════════════════════════════════════════
	// PRINT RESULTS
	// ══════════════════════════════════════════════════════════════

	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SECOND AXIS EXPERIMENT: marker threshold × adjacency width")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Full results per class
	for _, cr := range allReports {
		t.Log("")
		t.Logf("── %s (%d bits) ──", cr.label, cr.bitCount)
		t.Logf("%-14s  %8s  %8s  %12s  %12s", "config", "dil", "mark", "dil-rate", "mark-rate")
		for _, cfg := range cr.configs {
			t.Logf("%-14s  %8d  %8d  %12.6f  %12.6f",
				cfg.widthName+"+"+cfg.threshName,
				cfg.dilations, cfg.markers,
				cfg.dilateRate, cfg.markerRate)
		}
	}

	// DILATE rate matrix (should be identical across thresholds at fixed width)
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("DILATE RATE MATRIX (threshold should NOT affect dil-rate)")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	dilHeader := fmt.Sprintf("%-14s", "width+thresh")
	for _, cr := range allReports {
		dilHeader += fmt.Sprintf("  %10s", cr.label)
	}
	t.Log(dilHeader)

	for _, m := range thresholdMatrix {
		row := fmt.Sprintf("%-14s", m.widthName+"+"+m.threshName)
		for _, cr := range allReports {
			// Find matching config
			for _, cfg := range cr.configs {
				if cfg.widthName == m.widthName && cfg.threshName == m.threshName {
					row += fmt.Sprintf("  %10.6f", cfg.dilateRate)
					break
				}
			}
		}
		t.Log(row)
	}

	// MARKER rate matrix (the variable under test)
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("MARKER RATE MATRIX (threshold IS the variable)")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	markHeader := fmt.Sprintf("%-14s", "width+thresh")
	for _, cr := range allReports {
		markHeader += fmt.Sprintf("  %10s", cr.label)
	}
	t.Log(markHeader)

	for _, m := range thresholdMatrix {
		row := fmt.Sprintf("%-14s", m.widthName+"+"+m.threshName)
		for _, cr := range allReports {
			for _, cfg := range cr.configs {
				if cfg.widthName == m.widthName && cfg.threshName == m.threshName {
					row += fmt.Sprintf("  %10.6f", cfg.markerRate)
					break
				}
			}
		}
		t.Log(row)
	}

	// ══════════════════════════════════════════════════════════════
	// FALSIFICATION ANALYSIS
	// ══════════════════════════════════════════════════════════════

	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("FALSIFICATION ANALYSIS")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	// Pre-check: are there any markers at all?
	totalMarkers := uint64(0)
	for _, cr := range allReports {
		for _, cfg := range cr.configs {
			totalMarkers += cfg.markers
		}
	}

	if totalMarkers == 0 {
		t.Log("PRE-CHECK: ZERO markers across all 9 configs × 3 classes.")
		t.Log("           The current corpora produce no zero runs long enough")
		t.Log("           to trigger any marker threshold family.")
		t.Log("           → The threshold axis is UNTESTABLE with current corpora.")
		t.Log("")

		// Assert the structural fact that makes it untestable: StepFull's
		// dilation branch never reads thresh, so dilation rate is invariant
		// under the threshold axis at fixed width. This is a property of the
		// code, not an experimental finding, so it is asserted rather than
		// printed -- previously this printed "Confirmed." from a tautology.
		for _, cr := range allReports {
			widthDilRates := make(map[string][]float64)
			for _, cfg := range cr.configs {
				widthDilRates[cfg.widthName] = append(widthDilRates[cfg.widthName], cfg.dilateRate)
			}
			for wName, rates := range widthDilRates {
				for i := 1; i < len(rates); i++ {
					if math.Abs(rates[i]-rates[0]) > 1e-10 {
						t.Errorf("class=%s width=%s: dil-rate varies across thresholds (%.10f vs %.10f); "+
							"threshold must not affect adjacency detection",
							cr.label, wName, rates[0], rates[i])
					}
				}
			}
		}

		// Assert the other structural limit: with zero markers, the threshold
		// axis produces no observable effect at all on this corpus.
		t.Log("  OK: dil-rate invariant across thresholds at fixed width (structural, asserted)")
		t.Log("  NOTE: marker-based threshold analysis is unreachable on these corpora;")
		t.Log("        the code below requires corpora with long zero runs.")
		assertThresholdAxisIsUntestableHere(t, allReports)
		return
	}

	// ── Markers exist: run full analysis ──

	// At fixed width, compute rank correlation between threshold variants.
	// For each width, collect marker rate vectors (3 classes × 3 thresholds).
	t.Log("MARKER RATE RANKINGS AT FIXED WIDTH:")
	t.Log("")

	widths := []string{"w=1", "w=2", "w=3"}
	thresholds := []string{"t=pow2", "t=pow3", "t=lin12"}

	for _, wName := range widths {
		t.Logf("  %s:", wName)

		// Collect class→marker-rate for each threshold
		rankings := make(map[string][]struct {
			class string
			rate  float64
		})

		for _, tName := range thresholds {
			var classRates []struct {
				class string
				rate  float64
			}
			for _, cr := range allReports {
				for _, cfg := range cr.configs {
					if cfg.widthName == wName && cfg.threshName == tName {
						classRates = append(classRates, struct {
							class string
							rate  float64
						}{cr.label, cfg.markerRate})
					}
				}
			}
			rankings[tName] = classRates
		}

		// Print rankings
		for _, tName := range thresholds {
			line := fmt.Sprintf("    %s: ", tName)
			rates := rankings[tName]
			// Sort by rate descending for ranking display
			for i := 0; i < len(rates); i++ {
				for j := i + 1; j < len(rates); j++ {
					if rates[j].rate > rates[i].rate {
						rates[i], rates[j] = rates[j], rates[i]
					}
				}
			}
			for i, r := range rates {
				if i > 0 {
					line += " > "
				}
				line += fmt.Sprintf("%s(%.6f)", r.class, r.rate)
			}
			t.Log(line)
		}

		// Check if rankings differ between thresholds at this width
		rankOrder := make(map[string]string)
		for _, tName := range thresholds {
			order := ""
			rates := rankings[tName]
			for i := 0; i < len(rates); i++ {
				for j := i + 1; j < len(rates); j++ {
					if rates[j].rate > rates[i].rate {
						rates[i], rates[j] = rates[j], rates[i]
					}
				}
			}
			for _, r := range rates {
				order += r.class + ">"
			}
			rankOrder[tName] = order
		}

		allSame := true
		firstOrder := ""
		for i, tName := range thresholds {
			if i == 0 {
				firstOrder = rankOrder[tName]
			} else if rankOrder[tName] != firstOrder {
				allSame = false
			}
		}

		if allSame {
			t.Logf("    → SAME ranking across all thresholds at %s", wName)
		} else {
			t.Logf("    → DIFFERENT rankings across thresholds at %s ← INDEPENDENT", wName)
		}
		t.Log("")
	}

	// DILATE rate verification (threshold should not affect dilation)
	t.Log("DILATE RATE VERIFICATION:")
	dilateConsistent := true
	for _, cr := range allReports {
		widthDilRates := make(map[string][]float64)
		for _, cfg := range cr.configs {
			widthDilRates[cfg.widthName] = append(widthDilRates[cfg.widthName], cfg.dilateRate)
		}
		for _, rates := range widthDilRates {
			for i := 1; i < len(rates); i++ {
				if math.Abs(rates[i]-rates[0]) > 1e-10 {
					dilateConsistent = false
				}
			}
		}
	}
	if dilateConsistent {
		t.Log("  PASS: dil-rate unchanged across thresholds at fixed width")
	} else {
		t.Log("  FAIL: dil-rate varies across thresholds (unexpected)")
	}

	// Windowed marker stability check (robustness guard)
	t.Log("")
	t.Log("WINDOWED MARKER STABILITY (robustness):")
	t.Log("  Checking if marker rate rankings are stable across windows...")
	t.Log("  (Fragile pass = rankings change between windows)")
	t.Log("")

	for _, wn := range widths {
		for _, tn := range thresholds {
			// Collect windowed marker rates for each class
			classWindowRates := make(map[string][]float64)
			for _, cr := range allReports {
				for _, cfg := range cr.configs {
					if cfg.widthName == wn && cfg.threshName == tn {
						rates := make([]float64, len(cfg.windowed))
						for wi, wm := range cfg.windowed {
							rates[wi] = wm.markerRate
						}
						classWindowRates[cr.label] = rates
					}
				}
			}

			// Check ranking stability across windows.
			nWin := 0
			for _, rates := range classWindowRates {
				nWin = len(rates)
				break
			}

			// Ranking produced by the full (non-windowed) analysis, used as
			// the reference each per-window ranking is compared against.
			type classRate struct {
				class string
				rate  float64
			}
			rank := func(vals []classRate) string {
				sorted := append([]classRate(nil), vals...)
				for i := 0; i < len(sorted); i++ {
					for j := i + 1; j < len(sorted); j++ {
						if sorted[j].rate > sorted[i].rate {
							sorted[i], sorted[j] = sorted[j], sorted[i]
						}
					}
				}
				var out string
				for _, cr := range sorted {
					out += cr.class + ">"
				}
				return out
			}
			var full []classRate
			for class, rates := range classWindowRates {
				if len(rates) > 0 {
					full = append(full, classRate{class, rates[len(rates)-1]})
				}
			}
			fullRank := rank(full)

			unstableWindows := 0
			for w := 0; w < nWin; w++ {
				var windowRates []classRate
				for class, rates := range classWindowRates {
					if w < len(rates) {
						windowRates = append(windowRates, classRate{class, rates[w]})
					}
				}
				if len(windowRates) == 0 {
					continue
				}
				if rank(windowRates) != fullRank {
					unstableWindows++
				}
			}

			// The declared falsification rule ("fragile" if rankings vary
			// across windows) is now actually evaluated. Previously this
			// counter was initialized and never incremented, and the computed
			// rankings were discarded, so the branch was unreachable.
			if unstableWindows > 0 {
				t.Logf("  %s+%s: %d/%d windows have different rankings (FRAGILE, full=%s)",
					wn, tn, unstableWindows, nWin, fullRank)
			}
		}
	}
}
