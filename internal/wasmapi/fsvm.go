package wasmapi

import (
	"encoding/json"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/transponder"
)

// fsvmAPI serves the live-stepper demo: one method for static metadata, one for
// the per-bit trace.

type fsvmAPI struct{}

// ── fsvm.meta ──

type seedOut struct {
	Bit0 U64 `json:"bit0"`
	Bit1 U64 `json:"bit1"`
}

type familyOut struct {
	ID  int  `json:"id"`
	A   U64  `json:"a"`
	B   U64  `json:"b"`
	R   int  `json:"r"`
	Odd bool `json:"aIsOdd"`
}

type fsvmMetaOut struct {
	Version      int         `json:"version"`
	DefaultSeeds seedOut     `json:"defaultSeeds"`
	SketchSpread U64         `json:"sketchWindowSpread"`
	WindowMask   int         `json:"windowMask"`
	WindowBits   int         `json:"windowBits"`
	MarkerFrom   int         `json:"markerFrom"`
	StateBytes   int         `json:"stateBytes"`
	StepNanos    int         `json:"stepNanosPerBit"`
	StepV2Nanos  int         `json:"stepV2NanosPerBit"`
	WordNanos    int         `json:"word64V2NanosPerWord"`
	WordNanosBit int         `json:"word64V2NanosPerBit"`
	Families     []familyOut `json:"hashFamilies"`
	Notes        []string    `json:"notes"`
}

func (*fsvmAPI) meta(json.RawMessage) (result, error) {
	fams := make([]familyOut, 0, fsvm.FamilyCount())
	for i, f := range fsvm.HashFamilies {
		fams = append(fams, familyOut{
			ID:  i,
			A:   U(f.A),
			B:   U(f.B),
			R:   int(f.R),
			Odd: f.A%2 == 1,
		})
	}
	out := fsvmMetaOut{
		Version:      1,
		DefaultSeeds: seedOut{Bit0: U(fsvm.DefaultSeeds[0]), Bit1: U(fsvm.DefaultSeeds[1])},
		// The multiplier is the whole point of the sketch-entropy demo, so
		// the page can show the constant rather than take it on faith.
		SketchSpread: U(0x9E3779B97F4A7C15),
		WindowMask:   0x3F,
		WindowBits:   6,
		MarkerFrom:   8,
		// unsafe.Sizeof(fsvm.State{}) is pinned at 96 by fsvm's TestStateSize.
		StateBytes: 96,
		// docs/BENCHMARKS.md, AMD EPYC 7763, Go 1.25.
		StepNanos:    30,
		StepV2Nanos:  35,
		WordNanos:    887,
		WordNanosBit: 14,
		Families:     fams,
	}
	return result{Data: out}, nil
}

// ── fsvm.run ──

type fsvmRunArgs struct {
	Text string `json:"text"`
	// Bits is a pointer so an explicitly empty string ("give me an empty
	// stream") is distinguishable from an absent field ("use the default
	// sample"). Without this, asking for zero bits silently returns the
	// sample text.
	Bits    *string `json:"bits"`
	Version int     `json:"version"`
	Family  int     `json:"family"`
	Width   int     `json:"width"`
	MaxBits int     `json:"maxBits"`
}

type fsvmEventOut struct {
	Pos     int    `json:"pos"`
	Kind    string `json:"kind"`
	Payload U64    `json:"payload"`
}

type fsvmFinalOut struct {
	R             uint32 `json:"r"`
	ZeroRun       U64    `json:"zeroRun"`
	W             int    `json:"w"`
	LastBit       int    `json:"lastBit"`
	Sketch        U64    `json:"sketch"`
	SketchDelta   int    `json:"sketchDelta"`
	Dilations     U64    `json:"dilations"`
	Markers       U64    `json:"markers"`
	BitsProcessed U64    `json:"bitsProcessed"`
	SketchPopcnt  int    `json:"sketchPopcount"`
}

type fsvmRunOut struct {
	BitCount    int            `json:"bitCount"`
	Bits        string         `json:"bits"`
	R           []uint32       `json:"r"`
	W           []int          `json:"w"`
	ZeroRun     []U64          `json:"zeroRun"`
	Sketch      []U64          `json:"sketch"`
	SketchDelta []int          `json:"sketchDelta"`
	Events      []fsvmEventOut `json:"events"`
	Final       fsvmFinalOut   `json:"final"`
	Truncated   bool           `json:"truncated"`
	Version     int            `json:"version"`
	Width       int            `json:"width"`
}

func (*fsvmAPI) run(raw json.RawMessage) (result, error) {
	var a fsvmRunArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "fsvm.run: %v", err)
	}
	// 0 means "unset" and takes the v1 core path; only an explicit 3+ is bad.
	if a.Version == 0 {
		a.Version = 1
	}
	if a.Version != 1 && a.Version != 2 {
		return result{}, errf(CodeBadArgs, "fsvm.run: version must be 1 or 2, got %d", a.Version)
	}
	a.Width = clampInt(a.Width, 1, 3)
	maxBits := clampInt(defaultIfZero(a.MaxBits, 16384), 1, MaxTraceBit)

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
		// True UTF-8 bytes: the same conversion the corpus experiments use.
		bits = transponder.BytesToBits([]byte(text))
		if !ascii {
			res = res.warn("input contains non-ASCII (%d runes); bytes are UTF-8 here, "+
				"but tree.build truncates each rune to its low byte, so the two demos "+
				"can disagree on the same text",
				countNonASCII(text))
		}
	default:
		bits = transponder.BytesToBits([]byte(defaultText))
	}

	truncated := false
	if len(bits) > maxBits {
		bits = bits[:maxBits]
		truncated = true
	}

	n := len(bits)
	out := fsvmRunOut{
		BitCount:    n,
		Bits:        bitsToString(bits),
		R:           make([]uint32, 0, n),
		W:           make([]int, 0, n),
		ZeroRun:     make([]U64, 0, n),
		Sketch:      make([]U64, 0, n),
		SketchDelta: make([]int, 0, n),
		Events:      make([]fsvmEventOut, 0, n/8),
		Version:     a.Version,
		Width:       a.Width,
		Truncated:   truncated,
	}

	var s fsvm.State
	if a.Version == 2 {
		s = fsvm.NewWithFamily(a.Family)
	} else {
		s = fsvm.New()
	}

	step := pickStepper(a.Version, a.Width)

	for i, b := range bits {
		var evs []fsvm.Event
		s, evs = step(s, b)
		for _, e := range evs {
			out.Events = append(out.Events, fsvmEventOut{
				Pos:     i,
				Kind:    eventName(e.Kind),
				Payload: U(e.Payload),
			})
		}
		out.R = append(out.R, s.R)
		out.W = append(out.W, int(s.W))
		out.ZeroRun = append(out.ZeroRun, U(s.ZeroRun))
		out.Sketch = append(out.Sketch, U(s.Sketch))
		out.SketchDelta = append(out.SketchDelta, int(s.SketchDelta))
	}

	out.Final = fsvmFinalOut{
		R:             s.R,
		ZeroRun:       U(s.ZeroRun),
		W:             int(s.W),
		LastBit:       int(s.LastBit),
		Sketch:        U(s.Sketch),
		SketchDelta:   int(s.SketchDelta),
		Dilations:     U(s.Dilations),
		Markers:       U(s.Markers),
		BitsProcessed: U(s.BitsProcessed),
		SketchPopcnt:  popcount(s.Sketch),
	}
	return res.with(out), nil
}

// pickStepper routes to the real ingest function for the requested sketch
// version and adjacency width. Width 1 at version 1 is the plain core path;
// wider geometries go through transponder.StepWidth, which is the same function
// the width experiment uses, so the stepper shows real narrowing rather than a
// re-implementation of it.
func pickStepper(version, width int) func(fsvm.State, uint8) (fsvm.State, []fsvm.Event) {
	if width > 1 {
		w := transponder.AdjacencyWidth(width)
		return func(s fsvm.State, b uint8) (fsvm.State, []fsvm.Event) {
			return transponder.StepWidth(s, b, w)
		}
	}
	if version == 2 {
		return fsvm.StepV2
	}
	return fsvm.Step
}

func eventName(k fsvm.EventKind) string {
	switch k {
	case fsvm.EventDilate:
		return "DILATE"
	case fsvm.EventMarker:
		return "MARKER"
	default:
		return "UNKNOWN"
	}
}

func parseBitString(s string) []uint8 {
	out := make([]uint8, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '0':
			out = append(out, 0)
		case '1':
			out = append(out, 1)
		}
	}
	return out
}

func bitsToString(bits []uint8) string {
	buf := make([]byte, len(bits))
	for i, b := range bits {
		buf[i] = '0' + (b & 1)
	}
	return string(buf)
}

func defaultIfZero(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
