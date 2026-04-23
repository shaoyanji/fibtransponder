package calibration

import (
	"math/bits"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// AdaptiveTargets holds the desired operating ranges for a transponder.
// All rates are per-bit (0.0–1.0).
type AdaptiveTargets struct {
	DilateRateLow  float64 // below this → sensitize (decrease width)
	DilateRateHigh float64 // above this → desensitize (increase width)
	MarkerRateLow  float64 // below this → lower threshold
	SketchDriftMax float64 // bits.OnesCount64 delta EMA above this → unstable
	ConvergeEps    float64 // sketchDrift EMA below this → consider converged
}

// DefaultTargets are sensible defaults derived from corpus analysis.
func DefaultTargets() AdaptiveTargets {
	return AdaptiveTargets{
		DilateRateLow:  0.01,
		DilateRateHigh: 0.25,
		MarkerRateLow:  0.001,
		SketchDriftMax: 8.0,
		ConvergeEps:    0.5,
	}
}

// EMA is a cheap exponential moving average.
type EMA struct {
	Value float64
	Alpha float64 // smoothing factor (0 < α ≤ 1)
}

func newEMA(alpha float64) EMA {
	return EMA{Alpha: alpha}
}

func (e *EMA) Update(x float64) {
	if e.Alpha == 0 {
		e.Alpha = 0.1 // default
	}
	e.Value = e.Value*(1-e.Alpha) + x*e.Alpha
}

// AdaptiveState is the runtime state of one adaptive transponder.
type AdaptiveState struct {
	Width     uint8  // adjacency width [1,5]
	ZeroThresh uint64 // min zero-run before marker check

	// EMA trackers — updated every CalibrateInterval bits
	dilateRate   EMA
	markerRate   EMA
	sketchDrift  EMA

	// windowed counters (reset every CalibrateInterval)
	dilateCount  uint64
	markerCount  uint64
	lastSketch   uint64
	bitsObserved uint64

	// status
	Converged    bool
	Unstable     bool
}

// AdaptiveTransponder couples an FSVM state with adaptive calibration.
type AdaptiveTransponder struct {
	Name   string
	State  fsvm.State
	Params AdaptiveState
}

// CalibrationEvent signals a calibration change or convergence.
type CalibrationEventKind uint8

const (
	EventWidthUp CalibrationEventKind = iota + 1
	EventWidthDown
	EventThreshDown
	EventUnstable
	EventConverged
)

// CalibrationEvent is emitted when the loop adjusts parameters.
type CalibrationEvent struct {
	Kind    CalibrationEventKind
	Transponder string
	OldVal  uint64
	NewVal  uint64
}

// AdaptiveArray runs a proprioceptive feedback loop over N transponders.
type AdaptiveArray struct {
	Transponders      []AdaptiveTransponder
	Targets           AdaptiveTargets
	CalibrateInterval uint64 // bits between calibration rounds (default 256)
}

// NewAdaptiveArray creates an array with default-width transponders.
func NewAdaptiveArray(names []string, targets AdaptiveTargets) *AdaptiveArray {
	arr := &AdaptiveArray{
		Transponders:      make([]AdaptiveTransponder, len(names)),
		Targets:           targets,
		CalibrateInterval: 256,
	}
	for i, name := range names {
		arr.Transponders[i] = AdaptiveTransponder{
			Name:  name,
			State: fsvm.New(),
			Params: AdaptiveState{
				Width:      1,
				ZeroThresh: 8,
				dilateRate: newEMA(0.1),
				markerRate: newEMA(0.05),
				sketchDrift: newEMA(0.1),
			},
		}
	}
	return arr
}

// Result mirrors transponder.Result for API compatibility.
type Result struct {
	Name      string
	Dilations uint64
	Markers   uint64
	Sketch    uint64
	R         uint32
	Width     uint8
	ZeroThresh uint64
	Converged bool
	Unstable  bool
}

// Step feeds one bit to all transponders, calibrating when the interval fires.
func (a *AdaptiveArray) Step(b uint8) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		st, evs := a.stepOne(t, b)
		t.State = st
		for _, ev := range evs {
			switch ev.Kind {
			case fsvm.EventDilate:
				t.Params.dilateCount++
			case fsvm.EventMarker:
				t.Params.markerCount++
			}
		}
		t.Params.bitsObserved++
		results[i] = a.resultFrom(t)
	}
	a.maybeCalibrate()
	return results
}

// StepWord64 feeds a 64-bit word to all transponders.
func (a *AdaptiveArray) StepWord64(word uint64) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		st, batch := fsvm.StepWord64(t.State, word)
		t.State = st
		t.Params.dilateCount += uint64(batch.DilateCount)
		t.Params.markerCount += uint64(batch.MarkerCount)
		t.Params.bitsObserved += 64
		results[i] = a.resultFrom(t)
	}
	a.maybeCalibrate()
	return results
}

// stepOne runs the FSVM step with the transponder's current Width.
func (a *AdaptiveArray) stepOne(t *AdaptiveTransponder, b uint8) (fsvm.State, []fsvm.Event) {
	b &= 1
	var evs []fsvm.Event
	st := t.State

	// zero run + marker (custom threshold)
	if b == 0 {
		st.ZeroRun++
		if st.ZeroRun >= t.Params.ZeroThresh && isPow2(st.ZeroRun) {
			st.Markers++
			evs = append(evs, fsvm.Event{Kind: fsvm.EventMarker, Payload: st.ZeroRun})
		}
	} else {
		st.ZeroRun = 0
	}

	// adjacency with configurable width
	adjacent := false
	w := t.Params.Width
	if w < 1 {
		w = 1
	}
	if w > 5 {
		w = 5
	}
	switch w {
	case 1:
		adjacent = st.LastBit == 1 && b == 1
	case 2:
		adjacent = (st.W&0x03) == 0x03 && b == 1
	case 3:
		adjacent = (st.W&0x07) == 0x07 && b == 1
	case 4:
		adjacent = (st.W&0x0F) == 0x0F && b == 1
	case 5:
		adjacent = (st.W&0x1F) == 0x1F && b == 1
	}

	if adjacent {
		st.R++
		st.Dilations++
		evs = append(evs, fsvm.Event{Kind: fsvm.EventDilate, Payload: uint64(st.R)})
	}

	st.LastBit = b
	st.W = ((st.W << 1) | b) & 0x3F
	st.Sketch ^= st.Seeds[b] + uint64(st.W)

	return st, evs
}

// resultFrom builds a Result snapshot.
func (a *AdaptiveArray) resultFrom(t *AdaptiveTransponder) Result {
	return Result{
		Name:       t.Name,
		Dilations:  t.State.Dilations,
		Markers:    t.State.Markers,
		Sketch:     t.State.Sketch,
		R:          t.State.R,
		Width:      t.Params.Width,
		ZeroThresh: t.Params.ZeroThresh,
		Converged:  t.Params.Converged,
		Unstable:   t.Params.Unstable,
	}
}

// maybeCalibrate checks whether the calibration interval has elapsed.
func (a *AdaptiveArray) maybeCalibrate() {
	if len(a.Transponders) == 0 {
		return
	}
	bits := a.Transponders[0].Params.bitsObserved
	if bits < a.CalibrateInterval {
		return
	}
	for i := range a.Transponders {
		a.calibrateOne(&a.Transponders[i])
	}
}

// calibrateOne applies EMA updates and adjustment rules to a single transponder.
func (a *AdaptiveArray) calibrateOne(t *AdaptiveTransponder) {
	p := &t.Params

	// --- compute per-bit rates since last calibration ---
	n := float64(p.bitsObserved)
	if n < 1 {
		n = 1
	}
	dilateRate := float64(p.dilateCount) / n
	markerRate := float64(p.markerCount) / n

	// sketch drift: count changed bits in sketch vs last calibration
	sketchDelta := float64(bits.OnesCount64(p.lastSketch ^ t.State.Sketch))

	// --- update EMAs ---
	p.dilateRate.Update(dilateRate)
	p.markerRate.Update(markerRate)
	p.sketchDrift.Update(sketchDelta)

	// --- reset windowed counters ---
	p.dilateCount = 0
	p.markerCount = 0
	p.bitsObserved = 0
	p.lastSketch = t.State.Sketch

	// --- calibration rules with hysteresis ---
	// Hysteresis deadband: require 10% margin outside target to flip
	deadband := 0.1

	// Rule 1: dilate rate too high → desensitize (increase width)
	if p.dilateRate.Value > a.Targets.DilateRateHigh*(1+deadband) && p.Width < 5 {
		p.Width++
		p.Converged = false
	}

	// Rule 2: dilate rate too low → sensitize (decrease width)
	if p.dilateRate.Value < a.Targets.DilateRateLow*(1-deadband) && p.Width > 1 {
		p.Width--
		p.Converged = false
	}

	// Rule 3: marker rate too low → lower threshold (more sensitive)
	if p.markerRate.Value < a.Targets.MarkerRateLow*(1-deadband) {
		// lower threshold by halving, but not below 4
		if p.ZeroThresh > 4 {
			p.ZeroThresh /= 2
			p.Converged = false
		}
	}

	// Rule 4: sketch drift too high → unstable mode
	if p.sketchDrift.Value > a.Targets.SketchDriftMax {
		p.Unstable = true
		p.Converged = false
	} else {
		p.Unstable = false
	}

	// Rule 5: convergence detection
	// All three conditions must hold:
	//   sketchDrift < ε
	//   dilateRate ≈ 0 (below low target)
	//   not currently unstable
	if p.sketchDrift.Value < a.Targets.ConvergeEps &&
		p.dilateRate.Value < a.Targets.DilateRateLow &&
		!p.Unstable {
		if !p.Converged {
			p.Converged = true
		}
	}
}

func isPow2(x uint64) bool { return x > 0 && (x&(x-1)) == 0 }

// ProcessStream feeds a bitstream and returns final results.
func (a *AdaptiveArray) ProcessStream(bits []uint8) []Result {
	var last []Result
	for _, b := range bits {
		last = a.Step(b)
	}
	return last
}

// ProcessStreamWords feeds 64-bit words and returns final results.
func (a *AdaptiveArray) ProcessStreamWords(words []uint64) []Result {
	var last []Result
	for _, w := range words {
		last = a.StepWord64(w)
	}
	return last
}
