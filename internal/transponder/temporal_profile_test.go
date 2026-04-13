package transponder

import (
	"fmt"
	"math"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// ── Temporal Profile Analysis (Path A, second pass) ──
//
// The aggregate rate vectors have >0.99 cosine similarity because
// real text has similar UTF-8 bit statistics on average. But the
// TEMPORAL DISTRIBUTION of events across windows may differ.
//
// Hypothesis: code (with indentation, keywords, punctuation) produces
// bursty DILATE profiles, while prose produces uniform profiles.
// The temporal signature IS the structural token.
//
// Method:
//   1. Divide bitstream into windows (512 bits each)
//   2. Compute per-window DILATE rate
//   3. Treat the rate trajectory as a feature vector
//   4. Measure separability on the trajectory, not the aggregate

const temporalWindowSize = 512 // bits per window

type TemporalProfile struct {
	Label     string
	ByteCount int
	BitCount  int
	Windows   int
	Rates     []float64 // per-window DILATE rate (w=1 as representative)
	Variance  float64
	Skewness  float64
	Burstiness float64 // (σ - μ) / (σ + μ) — normalized measure
}

func computeTemporalProfile(label string, data []byte) TemporalProfile {
	bits := BytesToBits(data)
	nWin := (len(bits) + temporalWindowSize - 1) / temporalWindowSize

	s := fsvm.New()
	rates := make([]float64, nWin)
	windowDil := 0
	bitsInWindow := 0

	for bitIdx, b := range bits {
		windowIdx := bitIdx / temporalWindowSize
		prevDil := s.Dilations

		var evs []fsvm.Event
		s, evs = StepWidth(s, b, Width1)
		_ = evs

		windowDil += int(s.Dilations - prevDil)
		bitsInWindow++

		atEnd := (bitIdx+1)%temporalWindowSize == 0
		atLast := bitIdx == len(bits)-1

		if atEnd || atLast {
			if bitsInWindow > 0 {
				rates[windowIdx] = float64(windowDil) / float64(bitsInWindow)
			}
			windowDil = 0
			bitsInWindow = 0
		}
	}

	// Compute variance, skewness, burstiness
	mean := 0.0
	for _, r := range rates {
		mean += r
	}
	mean /= float64(len(rates))

	variance := 0.0
	for _, r := range rates {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(rates))
	stddev := math.Sqrt(variance)

	skewness := 0.0
	if stddev > 0 {
		for _, r := range rates {
			d := (r - mean) / stddev
			skewness += d * d * d
		}
		skewness /= float64(len(rates))
	}

	burstiness := 0.0
	if stddev+mean > 0 {
		burstiness = (stddev - mean) / (stddev + mean)
	}

	return TemporalProfile{
		Label:      label,
		ByteCount:  len(data),
		BitCount:   len(bits),
		Windows:    nWin,
		Rates:      rates,
		Variance:   variance,
		Skewness:   skewness,
		Burstiness: burstiness,
	}
}

// euclideanDistance computes Euclidean distance between two rate vectors.
// Pads shorter vector with its last value if lengths differ.
func euclideanDistance(a, b []float64) float64 {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	dist := 0.0
	for i := 0; i < maxLen; i++ {
		va := 0.0
		vb := 0.0
		if i < len(a) {
			va = a[i]
		} else {
			va = a[len(a)-1]
		}
		if i < len(b) {
			vb = b[i]
		} else {
			vb = b[len(b)-1]
		}
		d := va - vb
		dist += d * d
	}
	return math.Sqrt(dist)
}

func TestTemporalProfiles(t *testing.T) {
	corpus := []struct {
		label string
		data  []byte
	}{
		{"english", corpusEnglish},
		{"french", corpusFrench},
		{"python", corpusPython},
		{"json", corpusJSON},
		{"html", corpusHTML},
	}

	profiles := make([]TemporalProfile, len(corpus))
	for i, c := range corpus {
		profiles[i] = computeTemporalProfile(c.label, c.data)
	}

	// ══════════════════════════════════════════════════════════════
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("TEMPORAL PROFILES: DILATE rate trajectories across windows")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Print per-window rates
	for _, p := range profiles {
		t.Log("")
		t.Logf("── %s (%d bytes, %d windows) ──", p.Label, p.ByteCount, p.Windows)
		line := "  rates: "
		for i, r := range p.Rates {
			if i > 0 {
				line += ", "
			}
			line += fmt.Sprintf("%.4f", r)
		}
		t.Log(line)
		t.Logf("  mean=%.4f  var=%.6f  skew=%.4f  burstiness=%.4f",
			p.Rates[len(p.Rates)-1], // last value is not mean, compute properly
			p.Variance, p.Skewness, p.Burstiness)
	}

	// Burstiness comparison
	t.Log("")
	t.Log("── Burstiness comparison (higher = more bursty) ──")
	t.Logf("%-10s  %10s  %10s  %10s", "corpus", "variance", "skewness", "burstiness")
	for _, p := range profiles {
		mean := 0.0
		for _, r := range p.Rates {
			mean += r
		}
		mean /= float64(len(p.Rates))
		t.Logf("%-10s  %10.6f  %10.4f  %10.4f", p.Label, p.Variance, p.Skewness, p.Burstiness)
	}

	// Euclidean distance matrix on temporal profiles
	t.Log("")
	t.Log("── Euclidean distance between temporal profiles ──")
	t.Log("  (lower = more similar temporal structure)")

	// Normalize rates to zero-mean unit-variance for fair comparison
	normalizedRates := make([][]float64, len(profiles))
	for i, p := range profiles {
		mean := 0.0
		for _, r := range p.Rates {
			mean += r
		}
		mean /= float64(len(p.Rates))

		stddev := math.Sqrt(p.Variance)
		if stddev == 0 {
			stddev = 1
		}

		norm := make([]float64, len(p.Rates))
		for j, r := range p.Rates {
			norm[j] = (r - mean) / stddev
		}
		normalizedRates[i] = norm
	}

	header := fmt.Sprintf("%-10s", "")
	for _, p := range profiles {
		header += fmt.Sprintf("  %10s", p.Label)
	}
	t.Log(header)

	for i, pi := range profiles {
		row := fmt.Sprintf("%-10s", pi.Label)
		for j := range profiles {
			if i == j {
				row += fmt.Sprintf("  %10s", "---")
			} else {
				dist := euclideanDistance(normalizedRates[i], normalizedRates[j])
				row += fmt.Sprintf("  %10.4f", dist)
			}
		}
		t.Log(row)
	}

	// Windowed cosine similarity (shape matching)
	t.Log("")
	t.Log("── Cosine similarity on temporal SHAPE (normalized rates) ──")

	header = fmt.Sprintf("%-10s", "")
	for _, p := range profiles {
		header += fmt.Sprintf("  %10s", p.Label)
	}
	t.Log(header)

	for i, pi := range profiles {
		row := fmt.Sprintf("%-10s", pi.Label)
		for j := range profiles {
			if i == j {
				row += fmt.Sprintf("  %10s", "---")
			} else {
				sim := cosineSimilarity(normalizedRates[i], normalizedRates[j])
				row += fmt.Sprintf("  %10.4f", sim)
			}
		}
		t.Log(row)
	}

	// ══════════════════════════════════════════════════════════════
	// CLASSIFICATION TEST
	// ══════════════════════════════════════════════════════════════

	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("CLASSIFICATION: nearest-neighbor on temporal profiles")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Leave-one-out nearest neighbor on the full profile vector
	// (aggregate stats + temporal shape combined)
	for i, pi := range profiles {
		// Build feature vector: [mean_dil_rate, variance, skewness, burstiness, ...normalized_rates]
		features_i := make([]float64, 0)
		mean_i := 0.0
		for _, r := range pi.Rates {
			mean_i += r
		}
		mean_i /= float64(len(pi.Rates))
		features_i = append(features_i, mean_i, pi.Variance, pi.Skewness, pi.Burstiness)
		features_i = append(features_i, normalizedRates[i]...)

		bestDist := math.MaxFloat64
		bestLabel := ""
		for j, pj := range profiles {
			if i == j {
				continue
			}
			mean_j := 0.0
			for _, r := range pj.Rates {
				mean_j += r
			}
			mean_j /= float64(len(pj.Rates))
			features_j := make([]float64, 0)
			features_j = append(features_j, mean_j, pj.Variance, pj.Skewness, pj.Burstiness)
			features_j = append(features_j, normalizedRates[j]...)

			dist := euclideanDistance(features_i, features_j)
			if dist < bestDist {
				bestDist = dist
				bestLabel = pj.Label
			}
		}
		t.Logf("  %s → nearest neighbor: %s (dist=%.4f)", pi.Label, bestLabel, bestDist)
	}
}
