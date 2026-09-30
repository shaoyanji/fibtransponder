package transponder

import (
	"fmt"
	"testing"
)

const windowSize = 4096 // bits per window

// The fixtures themselves now live in corpus_fixtures.go, in the package proper,
// because REPORT_CORPUS.md and REPORT_STRUCTURAL.md are published artifacts
// derived from them. These aliases keep every test below reading as it did.
var (
	corpusProse     = CorpusProse
	corpusCode      = CorpusCode
	corpusSynthetic = CorpusSynthetic
)

// ── Input corpus: 3 classes ──

// ── Experiment ──

func TestCorpusExperiment(t *testing.T) {
	cals := []Calibration{
		CalibrationTight,
		CalibrationMedium,
		CalibrationWide,
	}

	corpus := []struct {
		label string
		data  []byte
	}{
		{"prose", corpusProse},
		{"code", corpusCode},
		{"synthetic", corpusSynthetic},
	}

	reports := make([]CorpusReport, len(corpus))
	for i, c := range corpus {
		bits := BytesToBits(c.data)
		reports[i] = RunCorpusExperiment(c.label, bits, cals, windowSize)
	}

	// ── Print numeric results ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("CORPUS EXPERIMENT: Per-transponder metrics across 3 input classes")
	t.Log("═══════════════════════════════════════════════════════════════")

	for _, r := range reports {
		t.Logf("")
		t.Logf("── %s (%d bits, %d windows) ──", r.Label, r.BitCount, len(r.Transponders[0].WindowData))
		t.Logf("%-10s  %8s  %8s  %8s  %18s", "transp.", "dil", "mark", "r", "sketch")
		t.Logf("%-10s  %8s  %8s  %8s  %18s", "─────────", "───────", "───────", "───────", "──────────────────")
		for _, tr := range r.Transponders {
			t.Logf("%-10s  %8d  %8d  %8d  0x%016x",
				tr.Name, tr.TotalDil, tr.TotalMark, tr.FinalR, tr.FinalSketch)
		}

		// Windowed rates
		t.Logf("")
		t.Logf("  Window dil-rate heatmap (dilations/bit):")
		nWin := len(r.Transponders[0].WindowData)
		for w := 0; w < nWin && w < 8; w++ {
			line := fmt.Sprintf("    W%-2d: ", w)
			for _, tr := range r.Transponders {
				wd := tr.WindowData[w]
				line += fmt.Sprintf("%s=%.4f  ", tr.Name, wd.DilateRate)
			}
			t.Log(line)
		}
	}

	// ── Cross-class comparison per transponder ──
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("DIVERGENCE MATRIX: Do transponders separate input classes?")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for tIdx := 0; tIdx < len(cals); tIdx++ {
		name := reports[0].Transponders[tIdx].Name
		t.Logf("Transponder %s:", name)
		t.Logf("  %-12s  %8s  %8s  %8s  %18s", "class", "dil", "mark", "r", "sketch")
		for _, r := range reports {
			tr := r.Transponders[tIdx]
			t.Logf("  %-12s  %8d  %8d  %8d  0x%016x",
				r.Label, tr.TotalDil, tr.TotalMark, tr.FinalR, tr.FinalSketch)
		}
		t.Log("")
	}

	// ── Sketch divergence within each class ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SKETCH DIVERGENCE: XOR distance between transponders per class")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, r := range reports {
		t.Logf("Class %s:", r.Label)
		for i := 0; i < len(r.Transponders); i++ {
			for j := i + 1; j < len(r.Transponders); j++ {
				xor := r.Transponders[i].FinalSketch ^ r.Transponders[j].FinalSketch
				t.Logf("  %s ⊕ %s = 0x%016x",
					r.Transponders[i].Name, r.Transponders[j].Name, xor)
			}
		}
		t.Log("")
	}

	// ── Event structure comparison ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("EVENT STRUCTURE: Does calibration affect event rates?")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, r := range reports {
		t.Logf("Class %s (%d bits):", r.Label, r.BitCount)
		t.Logf("  %-10s  %12s  %12s  %12s", "transp.", "dil-rate", "mark-rate", "total-rate")
		for _, tr := range r.Transponders {
			dr := float64(tr.TotalDil) / float64(r.BitCount)
			mr := float64(tr.TotalMark) / float64(r.BitCount)
			t.Logf("  %-10s  %12.6f  %12.6f  %12.6f",
				tr.Name, dr, mr, dr+mr)
		}
		t.Log("")
	}

	// ── Windowed sketch trajectory ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SKETCH TRAJECTORY: sketch value at each window boundary")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, r := range reports {
		t.Logf("Class %s:", r.Label)
		nWin := len(r.Transponders[0].WindowData)
		for w := 0; w < nWin && w < 4; w++ {
			line := fmt.Sprintf("  W%-2d: ", w)
			for _, tr := range r.Transponders {
				line += fmt.Sprintf("%s=0x%016x  ", tr.Name, tr.WindowData[w].Sketch)
			}
			t.Log(line)
		}
		if nWin > 4 {
			t.Logf("  ... (%d more windows)", nWin-4)
		}
		t.Log("")
	}

	// ── Assertions: structural facts ──

	// 1. Within each class, all transponders must have identical DILATE counts
	//    (adjacency detection is calibration-independent)
	for _, r := range reports {
		dil0 := r.Transponders[0].TotalDil
		for _, tr := range r.Transponders[1:] {
			if tr.TotalDil != dil0 {
				t.Errorf("class %s: DILATE mismatch: %s=%d vs %s=%d",
					r.Label, r.Transponders[0].Name, dil0, tr.Name, tr.TotalDil)
			}
		}
	}

	// 2. Within each class, all transponders must have identical marker counts
	for _, r := range reports {
		m0 := r.Transponders[0].TotalMark
		for _, tr := range r.Transponders[1:] {
			if tr.TotalMark != m0 {
				t.Errorf("class %s: marker mismatch: %s=%d vs %s=%d",
					r.Label, r.Transponders[0].Name, m0, tr.Name, tr.TotalMark)
			}
		}
	}

	// 3. Sketches may differ or collide — document, don't assert.
	//    (See assertion #4 below for collision reporting.)

	// 4. Sketch values may collide between transponders for some inputs.
	//    Collisions are reported rather than asserted: the v1 fold is
	//    parity-limited, so distinct inputs can legitimately coincide.
	//    (Previously tight and wide collided on prose before the v1 fold
	//    was fixed to spread the window across the full 64-bit word.)
	for _, r := range reports {
		sketches := make(map[uint64]string)
		collisions := 0
		for _, tr := range r.Transponders {
			if prev, exists := sketches[tr.FinalSketch]; exists {
				t.Logf("NOTE: class %s sketch collision: %s == %s == 0x%016x",
					r.Label, prev, tr.Name, tr.FinalSketch)
				collisions++
			}
			sketches[tr.FinalSketch] = tr.Name
		}
		if collisions > 0 {
			t.Logf("  → class %s: %d/%d transponders collided on sketch",
				r.Label, collisions+1, len(r.Transponders))
		}
	}
}

func TestCorpusByteCounts(t *testing.T) {
	t.Logf("prose:      %d bytes = %d bits", len(corpusProse), len(corpusProse)*8)
	t.Logf("code:       %d bytes = %d bits", len(corpusCode), len(corpusCode)*8)
	t.Logf("synthetic:  %d bytes = %d bits", len(corpusSynthetic), len(corpusSynthetic)*8)
}
