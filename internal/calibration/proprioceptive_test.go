package calibration

import (
	"math"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// --- helper: deterministic all-ones word (max dilate rate) ---
func onesWords(n int) []uint64 {
	w := make([]uint64, n)
	for i := range w {
		w[i] = ^uint64(0)
	}
	return w
}

// --- helper: deterministic all-zeros word (zero dilate rate) ---
func zerosWords(n int) []uint64 {
	w := make([]uint64, n)
	for i := range w {
		w[i] = 0
	}
	return w
}

// TestBasicStepSemantics verifies that adaptive step produces identical
// FSVM state to a vanilla step when width==1 and thresh==8.
func TestBasicStepSemantics(t *testing.T) {
	arr := NewAdaptiveArray([]string{"t1"}, DefaultTargets())
	arr.CalibrateInterval = 1 << 20 // never calibrate during test

	ref := fsvm.New()
	// deterministic mixed pattern
	for i := 0; i < 200; i++ {
		b := uint8(i & 1)
		if i%3 == 0 {
			b = 1
		}
		ref, _ = fsvm.Step(ref, b)
		arr.Step(b)
	}

	got := arr.Transponders[0].State
	if got != ref {
		t.Fatalf("state mismatch\nref: %+v\ngot: %+v", ref, got)
	}
}

// TestWidthDesensitization verifies that an all-ones stream (max dilate rate)
// causes width to increase until it hits the cap.
func TestWidthDesensitization(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.01,
		DilateRateHigh: 0.10, // very low → easy to trigger
		MarkerRateLow:  0.001,
		SketchDriftMax: 100.0,
		ConvergeEps:    0.1,
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.CalibrateInterval = 64 // calibrate every 64 bits

	// Feed all-ones words until calibration fires multiple times
	for i := 0; i < 20; i++ {
		arr.StepWord64(^uint64(0))
	}

	w := arr.Transponders[0].Params.Width
	if w <= 1 {
		t.Fatalf("expected width to increase under high dilate rate, got %d", w)
	}
	if w > 5 {
		t.Fatalf("width exceeded safety cap: %d", w)
	}
}

// TestWidthSensitization verifies that an all-zeros stream (zero dilate rate)
// causes width to decrease toward 1.
func TestWidthSensitization(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.05, // high low-target → easy to trigger decrease
		DilateRateHigh: 0.90,
		MarkerRateLow:  0.001,
		SketchDriftMax: 100.0,
		ConvergeEps:    0.1,
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.Transponders[0].Params.Width = 3 // start mid-range
	arr.CalibrateInterval = 64

	for i := 0; i < 20; i++ {
		arr.StepWord64(0)
	}

	w := arr.Transponders[0].Params.Width
	if w >= 3 {
		t.Fatalf("expected width to decrease under zero dilate rate, got %d", w)
	}
	if w < 1 {
		t.Fatalf("width below safety floor: %d", w)
	}
}

// TestThreshLowering verifies that low marker rate causes threshold to drop.
func TestThreshLowering(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.01,
		DilateRateHigh: 0.99,
		MarkerRateLow:  0.90, // absurdly high → forces lowering
		SketchDriftMax: 100.0,
		ConvergeEps:    0.1,
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.Transponders[0].Params.ZeroThresh = 32 // start high
	arr.CalibrateInterval = 64

	for i := 0; i < 20; i++ {
		arr.StepWord64(^uint64(0)) // no zero runs → marker rate = 0
	}

	th := arr.Transponders[0].Params.ZeroThresh
	if th >= 32 {
		t.Fatalf("expected threshold to drop under low marker rate, got %d", th)
	}
	if th < 4 {
		t.Fatalf("threshold below safety floor: %d", th)
	}
}

// TestUnstableMode verifies that high sketch drift triggers unstable flag.
func TestUnstableMode(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.01,
		DilateRateHigh: 0.99,
		MarkerRateLow:  0.001,
		SketchDriftMax: 1.0, // very low → easy to trigger
		ConvergeEps:    0.1,
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.CalibrateInterval = 64

	// deterministic but highly varying pattern to maximize sketch drift
	for i := 0; i < 30; i++ {
		arr.StepWord64(uint64(i) * 0x9e3779b97f4a7c15)
	}

	if !arr.Transponders[0].Params.Unstable {
		t.Fatalf("expected unstable mode under high sketch drift")
	}
}

// TestConvergenceDetection verifies that a stable stream eventually converges.
func TestConvergenceDetection(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.10,
		DilateRateHigh: 0.90,
		MarkerRateLow:  0.001,
		SketchDriftMax: 100.0,
		ConvergeEps:    10.0, // very tolerant
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.Transponders[0].Params.Width = 5 // max width → dilate rate drops to ~0
	arr.CalibrateInterval = 64

	// Feed a simple stable pattern (sparse 1s, no adjacency at width 5)
	for i := 0; i < 50; i++ {
		arr.StepWord64(0x0101010101010101) // isolated 1s every 8 bits
	}

	if !arr.Transponders[0].Params.Converged {
		t.Fatalf("expected convergence on stable low-activity stream")
	}
}

// TestSafetyCaps verifies width and threshold never escape bounds.
func TestSafetyCaps(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.50,
		DilateRateHigh: 0.50,
		MarkerRateLow:  0.50,
		SketchDriftMax: 0.0,
		ConvergeEps:    0.0,
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.CalibrateInterval = 64

	for i := 0; i < 200; i++ {
		if i%2 == 0 {
			arr.StepWord64(^uint64(0))
		} else {
			arr.StepWord64(0)
		}
	}

	p := arr.Transponders[0].Params
	if p.Width < 1 || p.Width > 5 {
		t.Fatalf("width out of bounds: %d", p.Width)
	}
	if p.ZeroThresh < 4 {
		t.Fatalf("threshold below floor: %d", p.ZeroThresh)
	}
}

// TestHysteresisDeadband verifies that small deviations do not flip width.
func TestHysteresisDeadband(t *testing.T) {
	// Set targets so dilate rate sits just inside the deadband
	targets := AdaptiveTargets{
		DilateRateLow:  0.01,
		DilateRateHigh: 0.30,
		MarkerRateLow:  0.001,
		SketchDriftMax: 100.0,
		ConvergeEps:    0.1,
	}
	arr := NewAdaptiveArray([]string{"t1"}, targets)
	arr.Transponders[0].Params.Width = 2
	arr.CalibrateInterval = 64

	// Feed a pattern that produces ~0.25 dilate rate at width 1
	// (inside deadband: need >0.33 to trigger up, <0.009 to trigger down)
	word := uint64(0x2727272727272727) // ~0.25 dilate rate at w=1
	for i := 0; i < 10; i++ {
		arr.StepWord64(word)
	}

	w := arr.Transponders[0].Params.Width
	if w != 2 {
		t.Fatalf("expected width to stay at 2 inside deadband, got %d", w)
	}
}

// TestResultPopulation checks that Result fields are populated correctly.
func TestResultPopulation(t *testing.T) {
	arr := NewAdaptiveArray([]string{"a", "b"}, DefaultTargets())
	arr.CalibrateInterval = 1 << 20
	res := arr.Step(1)

	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	for _, r := range res {
		if r.Width != 1 {
			t.Fatalf("expected default width 1, got %d", r.Width)
		}
		if r.ZeroThresh != 8 {
			t.Fatalf("expected default threshold 8, got %d", r.ZeroThresh)
		}
	}
}

// TestStepWord64Semantics matches StepWord64 against per-bit Step.
func TestStepWord64Semantics(t *testing.T) {
	arr := NewAdaptiveArray([]string{"t1"}, DefaultTargets())
	arr.CalibrateInterval = 1 << 20

	ref := NewAdaptiveArray([]string{"t1"}, DefaultTargets())
	ref.CalibrateInterval = 1 << 20

	words := []uint64{0, ^uint64(0), 0x123456789abcdef0, 0x0f0f0f0f0f0f0f0f}
	for _, w := range words {
		arr.StepWord64(w)
		for bit := 0; bit < 64; bit++ {
			b := uint8((w >> bit) & 1)
			ref.Step(b)
		}
	}

	if arr.Transponders[0].State != ref.Transponders[0].State {
		t.Fatalf("StepWord64 state mismatch\nref: %+v\ngot: %+v",
			ref.Transponders[0].State, arr.Transponders[0].State)
	}
}

// TestEMAUpdate verifies EMA converges to steady value.
func TestEMAUpdate(t *testing.T) {
	e := newEMA(0.2)
	for i := 0; i < 1000; i++ {
		e.Update(0.5)
	}
	if math.Abs(e.Value-0.5) > 0.01 {
		t.Fatalf("EMA did not converge to 0.5: got %f", e.Value)
	}
}

// TestCalibrationInterval verifies calibration only fires at interval.
func TestCalibrationInterval(t *testing.T) {
	arr := NewAdaptiveArray([]string{"t1"}, DefaultTargets())
	arr.CalibrateInterval = 128
	arr.Transponders[0].Params.Width = 1

	// Feed 127 bits of all-ones (should not calibrate yet)
	for i := 0; i < 127; i++ {
		arr.Step(1)
	}
	if arr.Transponders[0].Params.bitsObserved != 127 {
		t.Fatalf("bitsObserved mismatch: %d", arr.Transponders[0].Params.bitsObserved)
	}

	// 128th bit should trigger calibration
	arr.Step(1)
	if arr.Transponders[0].Params.bitsObserved != 0 {
		t.Fatalf("expected counters reset after calibration")
	}
}

// TestMultipleTranspondersIndependent verifies each transponder calibrates independently.
func TestMultipleTranspondersIndependent(t *testing.T) {
	targets := AdaptiveTargets{
		DilateRateLow:  0.01,
		DilateRateHigh: 0.10,
		MarkerRateLow:  0.001,
		SketchDriftMax: 100.0,
		ConvergeEps:    0.1,
	}
	arr := NewAdaptiveArray([]string{"high", "low"}, targets)
	arr.CalibrateInterval = 64

	// Feed mixed stream: high gets all-ones, low gets all-zeros
	// Run enough rounds for EMA to build up above threshold.
	for round := 0; round < 10; round++ {
		for i := 0; i < 20; i++ {
			arr.Transponders[0].State, _ = fsvm.StepWord64(arr.Transponders[0].State, ^uint64(0))
			arr.Transponders[0].Params.dilateCount += 64
			arr.Transponders[0].Params.bitsObserved += 64

			arr.Transponders[1].State, _ = fsvm.StepWord64(arr.Transponders[1].State, 0)
			arr.Transponders[1].Params.bitsObserved += 64
		}
		arr.calibrateOne(&arr.Transponders[0])
		arr.calibrateOne(&arr.Transponders[1])
	}

	if arr.Transponders[0].Params.Width <= 1 {
		t.Fatalf("high-rate transponder should have widened, got width=%d", arr.Transponders[0].Params.Width)
	}
	if arr.Transponders[1].Params.Width != 1 {
		t.Fatalf("low-rate transponder should stay at minimum width, got %d",
			arr.Transponders[1].Params.Width)
	}
}
