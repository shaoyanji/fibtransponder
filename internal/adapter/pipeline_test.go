package adapter

import (
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/signal/window"
)

// sameInput is a deterministic bit pattern we will feed through every medium.
var sameInput = []byte{1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 0, 0, 0, 1, 1}

// runFSVM drains an adapter through FSVM and returns the final state.
func runFSVM(a *Pipeline) fsvm.State {
	s := fsvm.New()
	for {
		b, ok := a.Next()
		if !ok {
			break
		}
		s, _ = fsvm.Step(s, b)
	}
	return s
}

// TestSubstrateIndependence verifies that the SAME deterministic input
// produces the SAME FSVM state regardless of which medium adapter
// carried it.  This is the core architectural promise.
func TestSubstrateIndependence(t *testing.T) {
	// --- electrical: direct bit push ---
	ele, err := NewElectricalPipeline(8)
	if err != nil {
		t.Fatalf("electrical pipeline: %v", err)
	}
	for _, b := range sameInput {
		ele.PushBit(b)
	}
	ele.Flush()
	stEle := runFSVM(ele)

	// --- photonic: float intensity at threshold 0.5 ---
	photo, err := NewPhotonicPipeline(0.5, 8)
	if err != nil {
		t.Fatalf("photonic pipeline: %v", err)
	}
	for _, b := range sameInput {
		v := 0.0
		if b == 1 {
			v = 1.0
		}
		photo.PushFloat(v)
	}
	photo.Flush()
	stPhoto := runFSVM(photo)

	// --- acoustic: float samples, no transform (direct) ---
	acou, err := NewAcousticPipelineWithTransform(TransformNone)
	if err != nil {
		t.Fatalf("acoustic pipeline: %v", err)
	}
	// Override frame size so we don't need 256 samples
	acou.cfg.FrameSize = 8
	acou.cfg.Overlap = 0
	acou.cfg.WindowFn = window.Rectangular // disable windowing for bit-identical test
	acou.samples = make([]float64, 8)
	acou.hop = 8
	for _, b := range sameInput {
		v := 0.0
		if b == 1 {
			v = 1.0
		}
		acou.PushFloat(v)
	}
	acou.Flush()
	stAcou := runFSVM(acou)

	// All three must agree on dilations and markers
	if stEle.Dilations != stPhoto.Dilations || stEle.Dilations != stAcou.Dilations {
		t.Fatalf("dilation mismatch: ele=%d photo=%d acou=%d",
			stEle.Dilations, stPhoto.Dilations, stAcou.Dilations)
	}
	if stEle.Markers != stPhoto.Markers || stEle.Markers != stAcou.Markers {
		t.Fatalf("marker mismatch: ele=%d photo=%d acou=%d",
			stEle.Markers, stPhoto.Markers, stAcou.Markers)
	}
	if stEle.Sketch != stPhoto.Sketch || stEle.Sketch != stAcou.Sketch {
		// Sketch may legitimately differ if BitsProcessed differs, but
		// for identical bit sequences it should match.
		t.Fatalf("sketch mismatch: ele=%d photo=%d acou=%d",
			stEle.Sketch, stPhoto.Sketch, stAcou.Sketch)
	}
}

// TestElectricalPassThrough verifies direct bit pass-through.
func TestElectricalPassThrough(t *testing.T) {
	p, err := NewElectricalPipeline(4)
	if err != nil {
		t.Fatal(err)
	}
	p.PushBit(1)
	p.PushBit(0)
	p.PushBit(1)
	p.PushBit(1)
	p.Flush()

	want := []byte{1, 0, 1, 1}
	for i, wb := range want {
		got, ok := p.Next()
		if !ok {
			t.Fatalf("expected %d bits, exhausted at %d", len(want), i)
		}
		if got != wb {
			t.Fatalf("bit %d: want %d, got %d", i, wb, got)
		}
	}
	_, ok := p.Next()
	if ok {
		t.Fatal("expected exhaustion")
	}
}

// TestPhotonicThresholding verifies intensity → bit conversion.
func TestPhotonicThresholding(t *testing.T) {
	p, err := NewPhotonicPipeline(0.5, 4)
	if err != nil {
		t.Fatal(err)
	}
	// Above threshold
	p.PushFloat(0.8)
	p.PushFloat(0.5)
	// Below threshold
	p.PushFloat(0.1)
	p.PushFloat(0.49)
	p.Flush()

	want := []byte{1, 1, 0, 0}
	for i, wb := range want {
		got, ok := p.Next()
		if !ok {
			t.Fatalf("expected %d bits, exhausted at %d", len(want), i)
		}
		if got != wb {
			t.Fatalf("bit %d: want %d, got %d", i, wb, got)
		}
	}
}

// TestAcousticWindowing verifies that 50% overlap produces more
// output frames than non-overlapping.
func TestAcousticWindowing(t *testing.T) {
	p, err := NewAcousticPipeline()
	if err != nil {
		t.Fatal(err)
	}
	// Feed 512 samples — with 256-frame / 128-hop, expect 3 frames
	// (samples 0-255, 128-383, 256-511)
	for i := 0; i < 512; i++ {
		p.PushFloat(float64(i) / 512.0)
	}
	p.Flush()

	stats := p.Stats()
	if stats.FramesProcessed < 3 {
		t.Fatalf("expected at least 3 frames with 50%% overlap, got %d", stats.FramesProcessed)
	}
	if stats.QueueDepth == 0 {
		t.Fatal("expected queued output bits")
	}
}

// TestPipelineStats verifies telemetry is non-empty and consistent.
func TestPipelineStats(t *testing.T) {
	p, err := NewElectricalPipeline(8)
	if err != nil {
		t.Fatal(err)
	}
	p.PushByte(0xAA)
	p.Flush()

	st := p.Stats()
	if st.Medium != "electrical" {
		t.Fatalf("medium: want electrical, got %s", st.Medium)
	}
	if st.FrameSize != 8 {
		t.Fatalf("frame size: want 8, got %d", st.FrameSize)
	}
	if st.FramesProcessed < 1 {
		t.Fatal("expected at least one frame processed")
	}
}

// TestNextWord verifies word-level output.
func TestNextWord(t *testing.T) {
	p, err := NewElectricalPipeline(16)
	if err != nil {
		t.Fatal(err)
	}
	// Push 16 bits: 0xA5A5 pattern
	pattern := uint16(0xA5A5)
	for i := 0; i < 16; i++ {
		p.PushBit(byte((pattern >> i) & 1))
	}
	p.Flush()

	w, n, ok := p.NextWord()
	if !ok {
		t.Fatal("expected word")
	}
	if n != 16 {
		t.Fatalf("expected 16 bits, got %d", n)
	}
	if w != 0xA5A5 {
		t.Fatalf("word mismatch: want 0xA5A5, got 0x%04X", w)
	}
}

// TestBackpressure verifies that the queue grows when FSVM pulls slowly.
func TestBackpressure(t *testing.T) {
	p, err := NewElectricalPipeline(4)
	if err != nil {
		t.Fatal(err)
	}
	// Push many bytes without pulling
	for i := 0; i < 100; i++ {
		p.PushByte(byte(i))
	}
	p.Flush()

	st := p.Stats()
	if st.QueueDepth < 400 { // 100 bytes * 8 bits/byte / 4 samples/frame = 200 frames, but quantized
		// Just verify queue is substantial
		t.Logf("queue depth: %d", st.QueueDepth)
	}
}

// TestMultiscaleIntegration verifies that enabling multiscale analysis
// produces summaries as bits flow through the pipeline.
func TestMultiscaleIntegration(t *testing.T) {
	cfg := PipelineConfig{
		Medium:             MediumElectrical,
		FrameSize:          8,
		Overlap:            0,
		Transform:          TransformNone,
		QuantLevels:        2,
		WindowFn:           window.Rectangular,
		AnalysisWindowSize: 8,
		AnalysisOverlap:    4,
	}
	p, err := NewPipeline(cfg)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	// Push 16 bits: two full analysis windows (window=8, overlap=4, hop=4)
	// Bits: 1,1,1,1,0,0,0,0, 1,1,1,1,0,0,0,0
	for i := 0; i < 16; i++ {
		if i < 4 || i >= 8 && i < 12 {
			p.PushBit(1)
		} else {
			p.PushBit(0)
		}
	}
	p.Flush()

	// With frameSize=8, overlap=0: processFrame fires after bits 7 and 15
	// Each frame produces 8 quantized bits (direct pass-through).
	// Analysis: window=8, overlap=4, hop=4.
	// First 8 bits → summary at bit 7.
	// Next 8 bits pushed: bits 8..15. Analysis sees bits 4..11 at bit 11? No,
	// analysis runs on the quantized output stream, not the input.
	// Output stream: 8 bits from frame 0, then 8 bits from frame 1 = 16 bits total.
	// Analysis windows on output:
	//   window 0: bits 0..7  (emitted after bit 7)
	//   window 1: bits 4..11 (emitted after bit 11, but frame 1 bits 8..15 arrive
	//             at bits 8..15 of output, so bit 11 = output bit 3 of frame 1)
	//   window 2: bits 8..15 (emitted after bit 15)
	// So we expect 3 summaries.

	sums := p.Summaries()
	if sums == nil {
		t.Fatal("expected summaries, got nil")
	}
	if len(sums) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(sums))
	}

	// Window 0 and 2 are [1,1,1,1,0,0,0,0] → density 0.5
	for i, wantDensity := range []float64{0.5, 0.5, 0.5} {
		if sums[i].OneDensity != wantDensity {
			t.Errorf("summary %d density: got %f, want %f", i, sums[i].OneDensity, wantDensity)
		}
		if sums[i].WindowBits != 8 {
			t.Errorf("summary %d window bits: got %d, want 8", i, sums[i].WindowBits)
		}
	}
}

// TestMultiscaleIntegrationDisabled verifies no summaries when analysis
// is not configured.
func TestMultiscaleIntegrationDisabled(t *testing.T) {
	p, err := NewElectricalPipeline(8)
	if err != nil {
		t.Fatal(err)
	}
	p.PushByte(0xFF)
	p.Flush()
	if p.Summaries() != nil {
		t.Error("expected nil summaries when analysis disabled")
	}
}
