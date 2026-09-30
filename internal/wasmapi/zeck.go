package wasmapi

import (
	"encoding/json"

	"github.com/shaoyanji/fibtransponder/internal/transponder"
	"github.com/shaoyanji/fibtransponder/internal/zeckendorf"
)

// zeckAPI serves the multi-scale residual demo. It calls the real
// zeckendorf.Residual / ResidualWindow / Profile, which is the same code
// cmd/zeck_residual reports from.
type zeckAPI struct{}

type zeckRunArgs struct {
	Text    string  `json:"text"`
	Bits    *string `json:"bits"`
	Sizes   []int   `json:"sizes"`
	Window  int     `json:"window"`
	Bins    int     `json:"bins"`
	MaxBits int     `json:"maxBits"`
}

type zeckEntryOut struct {
	WindowSize int     `json:"windowSize"`
	Mean       float64 `json:"mean"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	StdDev     float64 `json:"stddev"`
	Windows    int     `json:"windows"`
	// Means is the per-window series, so the page can draw the band rather
	// than only its envelope.
	Means []float64 `json:"means"`
}

type zeckOut struct {
	BitCount       int            `json:"bitCount"`
	Residual       float64        `json:"residual"`
	AdjacencyPairs int            `json:"adjacencyPairs"`
	Profile        []zeckEntryOut `json:"profile"`
	Histogram      []int          `json:"histogram"`
	HistogramMax   int            `json:"histogramMax"`
	// Ones fraction of the whole stream, for the "density" readout that makes
	// residual interpretable: a stream of all-ones has residual 1.0, but that
	// is a statement about density as much as about structure.
	OneDensity float64 `json:"oneDensity"`
	Truncated  bool    `json:"truncated"`
}

func (*zeckAPI) profile(raw json.RawMessage) (result, error) {
	var a zeckRunArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "zeck.profile: %v", err)
	}
	var res result

	var bits []uint8
	switch {
	case a.Bits != nil:
		bits = parseBitString(*a.Bits)
	case a.Text != "":
		text, ascii, err := capText(a.Text)
		if err != nil {
			return result{}, err
		}
		bits = transponder.BytesToBits([]byte(text))
		if !ascii {
			res = res.warn("input contains non-ASCII; bytes are UTF-8, and residual is " +
				"measured on the resulting bytes rather than on rune-truncated bytes")
		}
	default:
		bits = transponder.BytesToBits([]byte(defaultText))
	}

	maxBits := clampInt(defaultIfZero(a.MaxBits, MaxStreamBit), 1, MaxStreamBit)
	truncated := false
	if len(bits) > maxBits {
		bits = bits[:maxBits]
		truncated = true
	}

	sizes := a.Sizes
	if len(sizes) == 0 {
		// Same ladder the CLI uses.
		sizes = []int{8, 16, 32, 64, 128, 256, 512, 1024}
	}

	out := zeckOut{
		BitCount: len(bits),
		Residual: safeFloat(zeckendorf.Residual(bits)),
		// The numerator behind residual, so the page can show that a high
		// residual can come from a low one-density rather than from disorder.
		AdjacencyPairs: countAdjacent11(bits),
		OneDensity:     ratio(onesCount(bits), len(bits)),
		Truncated:      truncated,
	}

	for _, ws := range sizes {
		wr := zeckendorf.ResidualWindow(bits, ws)
		means := wr.Means
		if len(means) > 4096 {
			// Keep the payload bounded; the envelope stats stay exact.
			means = means[:4096]
		}
		out.Profile = append(out.Profile, zeckEntryOut{
			WindowSize: ws,
			Mean:       safeFloat(wr.Global),
			Min:        safeFloat(wr.Min),
			Max:        safeFloat(wr.Max),
			StdDev:     safeFloat(wr.StdDev),
			Windows:    wr.Windows,
			Means:      means,
		})
	}

	out.Histogram, out.HistogramMax = residualHistogram(bits, clampInt(defaultIfZero(a.Bins, 20), 2, 100))
	return res.with(out), nil
}

// residualHistogram buckets per-window residuals, mirroring the █ bar chart
// cmd/zeck_residual prints. Windows come from the default window size so the
// buckets are comparable across inputs.
func residualHistogram(bits []uint8, bins int) ([]int, int) {
	ws := 128
	if len(bits) < ws {
		ws = len(bits)
	}
	if ws < 2 {
		return make([]int, bins), 0
	}
	wr := zeckendorf.ResidualWindow(bits, ws)
	hist := make([]int, bins)
	max := 0
	for _, m := range wr.Means {
		idx := int(m * float64(bins))
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		hist[idx]++
		if hist[idx] > max {
			max = hist[idx]
		}
	}
	return hist, max
}

func onesCount(bits []uint8) int {
	n := 0
	for _, b := range bits {
		if b&1 == 1 {
			n++
		}
	}
	return n
}

// countAdjacent11 is the numerator of zeckendorf.Residual, counted here so the
// page can display it without recomputing anything.
func countAdjacent11(bits []uint8) int {
	n := 0
	for i := 1; i < len(bits); i++ {
		if bits[i]&1 != 0 && bits[i-1]&1 != 0 {
			n++
		}
	}
	return n
}
