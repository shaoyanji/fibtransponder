// Package transponder implements a fibtransponder array — multiple FSVM instances
// with different ZobristSeed calibrations, processing the same bitstream.
//
// Each transponder owns its seed table. Different calibrations produce different
// Sketch values and DILATE profiles from the same input, analogous to how
// different hair cells in the cochlea resonate at different frequencies.
package transponder

import "github.com/shaoyanji/fibtransponder/internal/fsvm"

// Calibration defines a transponder's sensitivity profile.
type Calibration struct {
	Name  string
	Seeds [2]uint64 // [bit0_seed, bit1_seed]
}

var (
	CalibrationTight = Calibration{
		Name:  "tight",
		Seeds: [2]uint64{0x517cc1b727220a95, 0x9e3779b97f4a7c15},
	}
	CalibrationMedium = Calibration{
		Name:  "medium",
		Seeds: [2]uint64{0x243f6a8885a308d3, 0x13198a2e03707344},
	}
	CalibrationWide = Calibration{
		Name:  "wide",
		Seeds: [2]uint64{0xa4093822299f31d0, 0x082efa98ec4e6c89},
	}
)

// Transponder is one FSVM instance with its own calibration.
type Transponder struct {
	Name  string
	State fsvm.State
}

// Result captures one transponder's state after processing.
type Result struct {
	Name      string
	Dilations uint64
	Markers   uint64
	Sketch    uint64
	R         uint32
}

// Array is a set of differently-calibrated transponders.
type Array struct {
	Transponders []Transponder
}

// NewArray creates transponders from calibrations, each owning its seed table.
func NewArray(cals []Calibration) *Array {
	arr := &Array{Transponders: make([]Transponder, len(cals))}
	for i, cal := range cals {
		arr.Transponders[i] = Transponder{
			Name:  cal.Name,
			State: fsvm.NewWithSeeds(cal.Seeds),
		}
	}
	return arr
}

// Step feeds one bit to all transponders.
func (a *Array) Step(b uint8) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		var evs []fsvm.Event
		t.State, evs = fsvm.Step(t.State, b)
		_ = evs
		results[i] = Result{
			Name:      t.Name,
			Dilations: t.State.Dilations,
			Markers:   t.State.Markers,
			Sketch:    t.State.Sketch,
			R:         t.State.R,
		}
	}
	return results
}

// StepWord64 feeds one 64-bit word to all transponders.
// This is the fast bulk-processing path; events are dropped (same as Step).
func (a *Array) StepWord64(word uint64) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		t.State, _ = fsvm.StepWord64(t.State, word)
		results[i] = Result{
			Name:      t.Name,
			Dilations: t.State.Dilations,
			Markers:   t.State.Markers,
			Sketch:    t.State.Sketch,
			R:         t.State.R,
		}
	}
	return results
}

// ProcessStream feeds entire bitstream, returns final results.
func (a *Array) ProcessStream(bits []uint8) []Result {
	var last []Result
	for _, b := range bits {
		last = a.Step(b)
	}
	return last
}

// ProcessStreamWords feeds a slice of 64-bit words, returns final results.
func (a *Array) ProcessStreamWords(words []uint64) []Result {
	var last []Result
	for _, w := range words {
		last = a.StepWord64(w)
	}
	return last
}
