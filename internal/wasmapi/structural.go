package wasmapi

import (
	"encoding/json"
	"math/rand"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/transponder"
)

// structuralAPI serves the width-sensitivity demo.
//
// This is the demo that carries the retraction: REPORT_STRUCTURAL.md originally
// read the width axis as an independent second detector axis, and then
// withdrew that. The reason is structural, and it is checkable: Width1/2/3
// test for a 1-run of length >= 2/3/4, so the event sets nest, which forces
// dilation counts to be non-increasing in width for every possible input.
// nested() therefore runs the same assertion the Go test runs, live.
type structuralAPI struct{}

type structuralMatrixArgs struct {
	// Texts is the class list. Empty means the three repo corpora.
	Texts  []string `json:"texts"`
	Labels []string `json:"labels"`
	Widths []int    `json:"widths"`
}

type structuralCell struct {
	Width         int     `json:"width"`
	Dilations     U64     `json:"dilations"`
	DilRate       float64 `json:"dilateRate"`
	Markers       U64     `json:"markers"`
	MarkRate      float64 `json:"markerRate"`
	MostSensitive bool    `json:"mostSensitive"`
}

type structuralRow struct {
	Label      string           `json:"label"`
	BitCount   int              `json:"bitCount"`
	OneDensity float64          `json:"oneDensity"`
	Residual   float64          `json:"residual"`
	Cells      []structuralCell `json:"cells"`
}

type structuralMatrixOut struct {
	Widths []int           `json:"widths"`
	Rows   []structuralRow `json:"rows"`
	Note   string          `json:"note"`
	// Sketches is per width, from a single default-seed transponder. After the
	// StepWidth fold fix these are identical across widths, which is the
	// property TestSketchIndependentOfWidth asserts: geometry changes events,
	// never the fingerprint.
	SketchPerWidth []U64 `json:"sketchPerWidth"`
}

func (*structuralAPI) matrix(raw json.RawMessage) (result, error) {
	var a structuralMatrixArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "structural.matrix: %v", err)
	}

	texts := a.Texts
	labels := a.Labels
	if len(texts) == 0 {
		texts = []string{defaultText}
		labels = []string{"prose"}
	}
	if len(labels) != len(texts) {
		return result{}, errf(CodeBadArgs,
			"structural.matrix: %d labels for %d texts", len(labels), len(texts))
	}

	widths := a.Widths
	if len(widths) == 0 {
		widths = []int{1, 2, 3}
	}
	for i := range widths {
		widths[i] = clampInt(widths[i], 1, 3)
	}

	cals := widthCals(widths)
	out := structuralMatrixOut{
		Widths:         widths,
		Note:           nestingNote,
		SketchPerWidth: make([]U64, 0, len(widths)),
	}

	// Each class gets its own array over its own bit loop, matching how
	// RunCorpusExperiment treats one stream at a time.
	for i, text := range texts {
		if _, _, err := capText(text); err != nil {
			return result{}, err
		}
		bits := transponder.BytesToBits([]byte(text))
		row := structuralRow{
			Label:      labels[i],
			BitCount:   len(bits),
			OneDensity: ratio(onesCount(bits), len(bits)),
			Residual:   safeFloat(residualOf(bits)),
		}

		last := transponder.NewStructuralArray(cals).ProcessStream(bits)
		bestRate, bestIdx := -1.0, -1
		for j, r := range last {
			rate := ratio(int(r.Dilations), len(bits))
			row.Cells = append(row.Cells, structuralCell{
				Width:     widths[j],
				Dilations: U(r.Dilations),
				DilRate:   rate,
				Markers:   U(r.Markers),
				MarkRate:  ratio(int(r.Markers), len(bits)),
			})
			if rate > bestRate {
				bestRate, bestIdx = rate, j
			}
		}
		if bestIdx >= 0 {
			row.Cells[bestIdx].MostSensitive = true
		}
		out.Rows = append(out.Rows, row)
	}

	// Sketch independence: one stream, every width, one fold. After the
	// StepWidth fix these are identical, which is what
	// TestSketchIndependentOfWidth asserts: geometry changes events, never the
	// fingerprint.
	bits := transponder.BytesToBits([]byte(texts[0]))
	for _, w := range widths {
		st := fsvm.New()
		for _, b := range bits {
			st, _ = transponder.StepWidth(st, b, transponder.AdjacencyWidth(w))
		}
		out.SketchPerWidth = append(out.SketchPerWidth, U(st.Sketch))
	}

	var res result
	for i, text := range texts {
		if !isASCII(text) {
			res = res.warn("%q contains non-ASCII; UTF-8 bytes are used here", labels[i])
		}
	}
	return res.with(out), nil
}

// ── structural.corpus ──

type structuralCorpusArgs struct {
	Widths []int `json:"widths"`
	// Windows is unused for the rate table itself (rates are whole-stream) but
	// is reported alongside so the page can state which window size the
	// published numbers were measured at.
	Windows int `json:"windows"`
}

type structuralCorpusOut struct {
	Widths  []int           `json:"widths"`
	Classes []structuralRow `json:"classes"`
	Windows int             `json:"windowBits"`
	Note    string          `json:"note"`
	// Published reproduces the exact rate table from REPORT_STRUCTURAL.md so
	// the page can prove the live run still agrees with the published document,
	// rather than asking the reader to take the live numbers on trust.
	Published []structuralRow `json:"published"`
	// MarkersAllZero is the second-axis retraction: across all 3x3
	// configurations no marker ever fires, so the threshold axis is untested
	// on this corpus rather than proven independent.
	MarkersAllZero bool           `json:"markersAllZero"`
	CorpusBytes    map[string]int `json:"corpusBytes"`
}

// publishedCorpusRates is REPORT_STRUCTURAL.md §1, transcribed. The live run
// is diffed against this on every test run, so a change in internal/ that
// would invalidate the published table fails here rather than quietly making
// the site disagree with the repo's own documentation.
var publishedCorpusRates = map[string]map[int]float64{
	"prose":     {1: 0.1992, 2: 0.0582, 3: 0.0079},
	"code":      {1: 0.1763, 2: 0.0609, 3: 0.0150},
	"synthetic": {1: 0.0313, 2: 0.0000, 3: 0.0000},
}

func (*structuralAPI) corpus(raw json.RawMessage) (result, error) {
	var a structuralCorpusArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return result{}, errf(CodeBadArgs, "structural.corpus: %v", err)
		}
	}
	widths := a.Widths
	if len(widths) == 0 {
		widths = []int{1, 2, 3}
	}
	for i := range widths {
		widths[i] = clampInt(widths[i], 1, 3)
	}
	windows := clampInt(defaultIfZero(a.Windows, 2048), 64, 1<<20)

	cals := widthCals(widths)
	out := structuralCorpusOut{
		Widths:  widths,
		Windows: windows,
		Note:    nestingNote,
		// REPORT_STRUCTURAL.md notes a 2048-bit window for the structural
		// experiment and REPORT_CORPUS.md uses 4096 for the corpus one.
		CorpusBytes: map[string]int{},
	}

	allZero := true
	for _, c := range transponder.CorpusClasses() {
		bits := transponder.BytesToBits(c.Data)
		out.CorpusBytes[c.Label] = len(c.Data)

		row := structuralRow{
			Label:      c.Label,
			BitCount:   len(bits),
			OneDensity: ratio(onesCount(bits), len(bits)),
			Residual:   safeFloat(residualOf(bits)),
		}
		last := transponder.NewStructuralArray(cals).ProcessStream(bits)
		bestRate, bestIdx := -1.0, -1
		for j, r := range last {
			rate := ratio(int(r.Dilations), len(bits))
			if r.Markers != 0 {
				allZero = false
			}
			row.Cells = append(row.Cells, structuralCell{
				Width:     widths[j],
				Dilations: U(r.Dilations),
				DilRate:   rate,
				Markers:   U(r.Markers),
				MarkRate:  ratio(int(r.Markers), len(bits)),
			})
			if rate > bestRate {
				bestRate, bestIdx = rate, j
			}
		}
		if bestIdx >= 0 {
			row.Cells[bestIdx].MostSensitive = true
		}
		out.Classes = append(out.Classes, row)

		pub := structuralRow{Label: c.Label + " (published)", BitCount: len(bits)}
		for _, w := range widths {
			if v, ok := publishedCorpusRates[c.Label][w]; ok {
				pub.Cells = append(pub.Cells, structuralCell{Width: w, DilRate: v})
			}
		}
		out.Published = append(out.Published, pub)
	}
	out.MarkersAllZero = allZero

	return result{Data: out}, nil
}

const nestingNote = "Width1/2/3 test for a 1-run of length >= 2/3/4, so the event sets " +
	"nest: {run>=4} is a subset of {run>=3} is a subset of {run>=2}. Dilation counts are " +
	"therefore non-increasing in width for every possible input, by construction. Width is " +
	"one scalar read at three ordered thresholds, not a second free parameter. " +
	"See structural.nesting to run that assertion live."

// ── structural.nesting ──

type structuralNestingArgs struct {
	Streams int   `json:"streams"`
	Seed    int64 `json:"seed"`
	MinBits int   `json:"minBits"`
	MaxBits int   `json:"maxBits"`
	Widths  []int `json:"widths"`
	OnePct  int   `json:"onePercent"`
}

type nestingOut struct {
	Streams    int    `json:"streams"`
	Violations int    `json:"violations"`
	FirstFail  string `json:"firstFailure,omitempty"`
	Seed       int64  `json:"seed"`
	Widths     []int  `json:"widths"`
	// DistinctRankings counts how many streams had a *strict* ordering between
	// the classes, as opposed to ties. It is why the original report's "not
	// strictly monotonic" observation was a tie artefact.
	StrictlyRanked int       `json:"strictlyRanked"`
	TiedAtTop      int       `json:"tiedAtTop"`
	MeanRate       []float64 `json:"meanRatePerWidth"`
	Note           string    `json:"note"`
}

func (*structuralAPI) nesting(raw json.RawMessage) (result, error) {
	var a structuralNestingArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "structural.nesting: %v", err)
	}
	streams := clampInt(defaultIfZero(a.Streams, 2000), 1, 20000)
	seed := a.Seed
	if seed == 0 {
		seed = 20240914
	}
	minBits := clampInt(defaultIfZero(a.MinBits, 64), 1, 4096)
	maxBits := clampInt(defaultIfZero(a.MaxBits, 1088), minBits, 8192)
	onePct := a.OnePct
	if onePct == 0 {
		onePct = 55
	}
	widths := a.Widths
	if len(widths) == 0 {
		widths = []int{1, 2, 3}
	}

	rng := rand.New(rand.NewSource(seed))
	out := nestingOut{
		Streams: streams,
		Seed:    seed,
		Widths:  widths,
		Note:    nestingNote,
	}
	sums := make([]float64, len(widths))

	for s := 0; s < streams; s++ {
		n := minBits + rng.Intn(maxBits-minBits+1)
		bits := make([]uint8, n)
		for i := range bits {
			if rng.Intn(100) < onePct {
				bits[i] = 1
			}
		}
		rates := make([]float64, len(widths))
		for wi, w := range widths {
			st := fsvm.New()
			for _, b := range bits {
				st, _ = transponder.StepWidth(st, b, transponder.AdjacencyWidth(w))
			}
			rates[wi] = ratio(int(st.Dilations), n)
			sums[wi] += rates[wi]
		}
		for wi := 1; wi < len(rates); wi++ {
			if rates[wi-1] < rates[wi] {
				out.Violations++
				if out.FirstFail == "" {
					out.FirstFail = itoa(s)
				}
				break
			}
		}
		if strictlyDecreasing(rates) {
			out.StrictlyRanked++
		}
		if len(rates) >= 2 && rates[0] == rates[1] {
			out.TiedAtTop++
		}
	}
	for wi := range sums {
		out.MeanRate = append(out.MeanRate, safeFloat(sums[wi]/float64(streams)))
	}
	return result{Data: out}, nil
}

func strictlyDecreasing(r []float64) bool {
	for i := 1; i < len(r); i++ {
		if !(r[i-1] > r[i]) {
			return false
		}
	}
	return len(r) > 1
}

func widthCals(widths []int) []transponder.StructuralCalibration {
	cals := make([]transponder.StructuralCalibration, len(widths))
	for i, w := range widths {
		cals[i] = transponder.StructuralCalibration{
			Name:  "w=" + itoa(w),
			Width: transponder.AdjacencyWidth(w),
		}
	}
	return cals
}
