package transponder

import (
	"fmt"
	"math"
	"testing"
)

// ── Expanded corpus for second-axis testing ──
//
// The original corpora produce zero markers because they lack long zero runs.
// These expanded variants interleave original content with zero-padded blocks
// to create diverse zero-run distributions that different threshold families
// will slice differently.

// corpusProseExpanded: prose + 64 zero bytes + more prose
// The zero block creates a 512-bit zero run, triggering markers at all
// threshold families but at different points.
var corpusProseExpanded []byte

// corpusCodeExpanded: code + 128 zero bytes + more code
// Longer zero block to test higher thresholds (pow3≥27, pow2≥32).
var corpusCodeExpanded []byte

// corpusMixed: interleaved prose/code/zeros — tests real-world streaming
// where zero runs appear between document segments.
var corpusMixed []byte

func init() {
	// Prose expanded: original prose + 64 zero bytes + reversed prose snippet
	proseChunk1 := corpusProse[:len(corpusProse)/2]
	proseChunk2 := corpusProse[len(corpusProse)/2:]
	zeroBlock64 := make([]byte, 64) // 512-bit zero run
	corpusProseExpanded = make([]byte, 0, len(proseChunk1)+64+len(proseChunk2))
	corpusProseExpanded = append(corpusProseExpanded, proseChunk1...)
	corpusProseExpanded = append(corpusProseExpanded, zeroBlock64...)
	corpusProseExpanded = append(corpusProseExpanded, proseChunk2...)

	// Code expanded: original code + 128 zero bytes + code tail
	codeChunk1 := corpusCode[:len(corpusCode)/2]
	codeChunk2 := corpusCode[len(corpusCode)/2:]
	zeroBlock128 := make([]byte, 128) // 1024-bit zero run
	corpusCodeExpanded = make([]byte, 0, len(codeChunk1)+128+len(codeChunk2))
	corpusCodeExpanded = append(corpusCodeExpanded, codeChunk1...)
	corpusCodeExpanded = append(corpusCodeExpanded, zeroBlock128...)
	corpusCodeExpanded = append(corpusCodeExpanded, codeChunk2...)

	// Mixed: prose + 32 zeros + code + 16 zeros + synthetic + 8 zeros
	zeroBlock32 := make([]byte, 32)
	zeroBlock16 := make([]byte, 16)
	zeroBlock8 := make([]byte, 8)
	corpusMixed = make([]byte, 0)
	corpusMixed = append(corpusMixed, corpusProse...)
	corpusMixed = append(corpusMixed, zeroBlock32...)
	corpusMixed = append(corpusMixed, corpusCode...)
	corpusMixed = append(corpusMixed, zeroBlock16...)
	corpusMixed = append(corpusMixed, corpusSynthetic...)
	corpusMixed = append(corpusMixed, zeroBlock8...)
}

func TestExpandedCorpusByteCounts(t *testing.T) {
	t.Logf("prose-expanded: %d bytes = %d bits", len(corpusProseExpanded), len(corpusProseExpanded)*8)
	t.Logf("code-expanded:  %d bytes = %d bits", len(corpusCodeExpanded), len(corpusCodeExpanded)*8)
	t.Logf("mixed:          %d bytes = %d bits", len(corpusMixed), len(corpusMixed)*8)
}

func TestSecondAxisExpanded(t *testing.T) {
	corpus := []struct {
		label string
		data  []byte
	}{
		{"prose+zeros", corpusProseExpanded},
		{"code+zeros", corpusCodeExpanded},
		{"mixed", corpusMixed},
	}

	// Include linear≥4 threshold to test low-threshold sensitivity
	type thresholdVariant struct {
		name   string
		config MarkerThresholdConfig
	}
	thresholds := []thresholdVariant{
		{"t=pow2", ThresholdDefault},           // 8,16,32,...
		{"t=pow3", ThresholdPow3},              // 9,27,81,...
		{"t=lin12", ThresholdLinear},            // every 12
		{"t=lin4", MarkerThresholdConfig{Family: Linear, Min: 4}}, // every 4
		{"t=lin8", ThresholdLinear8},            // every 8
	}

	type widthVariant struct {
		name  string
		width AdjacencyWidth
	}
	widths := []widthVariant{
		{"w=1", Width1},
		{"w=2", Width2},
		{"w=3", Width3},
	}

	const windowSize = 2048

	// Build all configs
	type configDef struct {
		widthName  string
		threshName string
		width      AdjacencyWidth
		thresh     MarkerThresholdConfig
	}
	var configs []configDef
	for _, w := range widths {
		for _, th := range thresholds {
			configs = append(configs, configDef{
				widthName:  w.name,
				threshName: th.name,
				width:      w.width,
				thresh:     th.config,
			})
		}
	}

	type configResult struct {
		def        configDef
		dilations  uint64
		markers    uint64
		dilateRate float64
		markerRate float64
	}

	type classResult struct {
		label    string
		bitCount int
		results  []configResult
	}

	allResults := make([]classResult, len(corpus))

	for ci, c := range corpus {
		bits := BytesToBits(c.data)

		// Create joint array with all configs
		cals := make([]JointCalibration, len(configs))
		for i, cfg := range configs {
			cals[i] = JointCalibration{
				Name:      cfg.widthName + "+" + cfg.threshName,
				Width:     cfg.width,
				Threshold: cfg.thresh,
			}
		}
		arr := NewJointArray(cals)
		arr.ProcessStream(bits)

		results := make([]configResult, len(configs))
		for i, t := range arr.Transponders {
			results[i] = configResult{
				def:        configs[i],
				dilations:  t.State.Dilations,
				markers:    t.State.Markers,
				dilateRate: float64(t.State.Dilations) / float64(len(bits)),
				markerRate: float64(t.State.Markers) / float64(len(bits)),
			}
		}

		allResults[ci] = classResult{
			label:    c.label,
			bitCount: len(bits),
			results:  results,
		}
	}

	// ══════════════════════════════════════════════════════════════
	// PRINT RESULTS
	// ══════════════════════════════════════════════════════════════

	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SECOND AXIS EXPANDED: marker threshold × width (with zero runs)")
	t.Log("═══════════════════════════════════════════════════════════════")

	for _, cr := range allResults {
		t.Log("")
		t.Logf("── %s (%d bits) ──", cr.label, cr.bitCount)
		t.Logf("%-16s  %8s  %8s  %12s  %12s", "config", "dil", "mark", "dil-rate", "mark-rate")
		for _, r := range cr.results {
			t.Logf("%-16s  %8d  %8d  %12.6f  %12.6f",
				r.def.widthName+"+"+r.def.threshName,
				r.dilations, r.markers,
				r.dilateRate, r.markerRate)
		}
	}

	// MARKER RATE MATRIX — this is what we're testing
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("MARKER RATE MATRIX")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	markHeader := fmt.Sprintf("%-16s", "width+thresh")
	for _, cr := range allResults {
		markHeader += fmt.Sprintf("  %12s", cr.label)
	}
	t.Log(markHeader)

	for _, cfg := range configs {
		row := fmt.Sprintf("%-16s", cfg.widthName+"+"+cfg.threshName)
		for _, cr := range allResults {
			for _, r := range cr.results {
				if r.def.widthName == cfg.widthName && r.def.threshName == cfg.threshName {
					row += fmt.Sprintf("  %12.6f", r.markerRate)
					break
				}
			}
		}
		t.Log(row)
	}

	// ══════════════════════════════════════════════════════════════
	// INDEPENDENCE ANALYSIS
	// ══════════════════════════════════════════════════════════════

	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("INDEPENDENCE ANALYSIS: Do marker rankings change with threshold?")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, wv := range widths {
		t.Logf("  At %s:", wv.name)

		// For each threshold, collect marker rates for all classes
		type classMarkerRate struct {
			class string
			rate  float64
		}
		rankings := make(map[string][]classMarkerRate)

		for _, tv := range thresholds {
			var rates []classMarkerRate
			for _, cr := range allResults {
				for _, r := range cr.results {
					if r.def.widthName == wv.name && r.def.threshName == tv.name {
						rates = append(rates, classMarkerRate{cr.label, r.markerRate})
					}
				}
			}
			// Sort descending
			for i := 0; i < len(rates); i++ {
				for j := i + 1; j < len(rates); j++ {
					if rates[j].rate > rates[i].rate {
						rates[i], rates[j] = rates[j], rates[i]
					}
				}
			}
			rankings[tv.name] = rates
		}

		// Print rankings
		for _, tv := range thresholds {
			line := fmt.Sprintf("    %s: ", tv.name)
			for i, r := range rankings[tv.name] {
				if i > 0 {
					line += " > "
				}
				line += fmt.Sprintf("%s(%.6f)", r.class, r.rate)
			}
			t.Log(line)
		}

		// Check if rankings differ
		rankStrings := make(map[string]string)
		for tvName, rates := range rankings {
			s := ""
			for _, r := range rates {
				s += r.class + ">"
			}
			rankStrings[tvName] = s
		}

		allSame := true
		firstRank := ""
		for i, tv := range thresholds {
			if i == 0 {
				firstRank = rankStrings[tv.name]
			} else if rankStrings[tv.name] != firstRank {
				allSame = false
			}
		}

		if allSame {
			t.Logf("    → SAME ranking across all thresholds at %s (threshold is gain-only)", wv.name)
		} else {
			t.Logf("    → DIFFERENT rankings at %s ← INDEPENDENT AXIS", wv.name)
		}
		t.Log("")
	}

	// Verify dil-rate is unaffected
	t.Log("DILATE RATE VERIFICATION:")
	dilateConsistent := true
	for _, cr := range allResults {
		widthRates := make(map[string][]float64)
		for _, r := range cr.results {
			widthRates[r.def.widthName] = append(widthRates[r.def.widthName], r.dilateRate)
		}
		for _, rates := range widthRates {
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

	// Count total markers to confirm experiment is valid
	totalMarkers := uint64(0)
	for _, cr := range allResults {
		for _, r := range cr.results {
			totalMarkers += r.markers
		}
	}
	t.Logf("")
	t.Logf("Total markers across experiment: %d", totalMarkers)
	if totalMarkers == 0 {
		t.Log("WARNING: Still zero markers. Need even longer zero runs or lower thresholds.")
	}
}
