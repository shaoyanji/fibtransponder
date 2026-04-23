package adapter

// Package adapter is the universal bridge between the sensorium and FSVM.
//
// Architecture
//
//	Sensorium (photonic, acoustic, electrical, visual)
//	        │
//	        ▼
//	  [Transducer] → native samples (float64, byte, photon count)
//	        │
//	        ▼
//	  [Adapter Pipeline] → buffering, windowing, transforms, quantization
//	        │
//	        ▼
//	      FSVM ← uniform bit/token stream
//
// The adapter layer is the ONLY medium-specific code in the system.
// FSVM core is substrate-agnostic — the same state machine runs unchanged
// whether its input comes from a microphone, a photodiode, or a file.
//
// Design principles
//
//   1. Push in, pull out. Sensors push data asynchronously; FSVM pulls
//      bits synchronously. The adapter decouples these rates.
//   2. Windowing is primitive. Every medium can configure frame size and
//      overlap. Transforms operate on frames, not the whole stream.
//   3. Transforms are pluggable. FFT, WHT, autocorrelation, or none.
//      The adapter quantizes transform output into bits.
//   4. Quantization is adaptive. Fixed thresholds, learned thresholds,
//      or entropy-preserving coding — all live here, not in FSVM.

import (
	"errors"
	"sync"

	"github.com/shaoyanji/fibtransponder/internal/signal/multiscale"
	"github.com/shaoyanji/fibtransponder/internal/signal/window"
)

// Medium identifies the physical or virtual substrate.
type Medium uint8

const (
	MediumUnknown    Medium = iota
	MediumElectrical        // Direct digital bits
	MediumPhotonic          // Light intensity → threshold
	MediumAcoustic          // Sound amplitude → FFT → spectral tokens
	MediumVisual            // Pixel/feature vectors
)

func (m Medium) String() string {
	switch m {
	case MediumElectrical:
		return "electrical"
	case MediumPhotonic:
		return "photonic"
	case MediumAcoustic:
		return "acoustic"
	case MediumVisual:
		return "visual"
	default:
		return "unknown"
	}
}

// Transform selects the signal decomposition applied per frame.
type Transform uint8

const (
	TransformNone Transform = iota
	TransformFFT
	TransformWHT
	TransformAutocorr
)

func (t Transform) String() string {
	switch t {
	case TransformNone:
		return "none"
	case TransformFFT:
		return "fft"
	case TransformWHT:
		return "wht"
	case TransformAutocorr:
		return "autocorr"
	default:
		return "unknown"
	}
}

// PipelineConfig tunes the adapter for a specific medium and use case.
type PipelineConfig struct {
	Medium      Medium    // substrate identifier (telemetry only)
	FrameSize   int       // samples per analysis frame (power of 2 recommended)
	Overlap     int       // samples overlapped between consecutive frames
	Transform   Transform // signal decomposition per frame
	QuantLevels int       // quantization levels (2 = binary, 4 = 2-bit, etc.)
	WindowFn    window.Kind

	// Medium-specific tuning
	Threshold float64 // for photonic/electrical thresholding

	// Multiscale analysis on the quantized output bit stream.
	// If AnalysisWindowSize > 0, a multiscale.Slider is attached
	// to the pipeline and summaries are computed as bits are produced.
	AnalysisWindowSize int
	AnalysisOverlap    int
}

// DefaultConfig returns a sensible default for electrical/bit streams.
func DefaultConfig() PipelineConfig {
	return PipelineConfig{
		Medium:      MediumElectrical,
		FrameSize:   64,
		Overlap:     0,
		Transform:   TransformNone,
		QuantLevels: 2,
		WindowFn:    window.Rectangular,
	}
}

// Validate checks configuration consistency.
func (c PipelineConfig) Validate() error {
	if c.FrameSize <= 0 {
		return errors.New("frame size must be > 0")
	}
	if c.Overlap < 0 || c.Overlap >= c.FrameSize {
		return errors.New("overlap must be in [0, frameSize)")
	}
	if c.QuantLevels < 2 {
		return errors.New("quant levels must be >= 2")
	}
	return nil
}

// Pipeline is the universal adapter: push samples in, pull bits out.
//
// It implements fsvm.Adapter so it plugs directly into the state machine.
// The zero value is NOT usable — use NewPipeline.
type Pipeline struct {
	cfg PipelineConfig

	// Input side: samples accumulate here
	samples []float64 // circular buffer, length = frameSize
	write   int       // write head in samples
	filled  bool      // true once samples has wrapped at least once
	pending bool      // true if new data arrived since last frame process

	// Output side: quantized bits queued for FSVM
	out    []byte
	outPos int

	// Frame processing state
	frameCount uint64
	dropCount  uint64 // samples dropped due to backpressure

	// Overlap management
	hop int // frameSize - overlap

	// Optional multiscale analysis on output bits
	slider *multiscale.Slider

	// Thread safety (optional — disabled if mu == nil, but we always init)
	mu sync.Mutex
}

// NewPipeline creates an adapter pipeline for a given configuration.
func NewPipeline(cfg PipelineConfig) (*Pipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	p := &Pipeline{
		cfg:     cfg,
		samples: make([]float64, cfg.FrameSize),
		out:     make([]byte, 0, cfg.FrameSize),
		hop:     cfg.FrameSize - cfg.Overlap,
	}
	if p.hop <= 0 {
		p.hop = cfg.FrameSize // full frames, no overlap
	}
	if cfg.AnalysisWindowSize > 0 {
		slider, err := multiscale.NewSlider(cfg.AnalysisWindowSize, cfg.AnalysisOverlap)
		if err != nil {
			return nil, err
		}
		p.slider = slider
	}
	return p, nil
}

// Medium returns the substrate this pipeline is configured for.
func (p *Pipeline) Medium() Medium { return p.cfg.Medium }

// ------------------------------------------------------------------
// Push side: medium-specific sample injection
// ------------------------------------------------------------------

// PushFloat feeds a floating-point sample (acoustic amplitude, photon
// intensity, etc.) into the pipeline.
func (p *Pipeline) PushFloat(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.samples[p.write] = v
	p.write++
	p.pending = true
	if p.write >= len(p.samples) {
		p.write = 0
		p.filled = true
	}

	// When we have a complete frame, process it
	if p.filled && p.write%p.hop == 0 {
		p.processFrame()
		p.pending = false
	}
}

// PushBit feeds a single binary sample (electrical/digital).
func (p *Pipeline) PushBit(b byte) {
	var v float64
	if b&1 == 1 {
		v = 1.0
	}
	p.PushFloat(v)
}

// PushByte feeds 8 bits from a byte, LSB-first.
func (p *Pipeline) PushByte(b byte) {
	for i := 0; i < 8; i++ {
		p.PushBit((b >> i) & 1)
	}
}

// ------------------------------------------------------------------
// Pull side: fsvm.Adapter implementation
// ------------------------------------------------------------------

// Next returns the next quantized bit (0 or 1) and true, or false if
// the output queue is empty.
func (p *Pipeline) Next() (byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.outPos >= len(p.out) {
		return 0, false
	}
	b := p.out[p.outPos] & 1
	p.outPos++
	// Compact slice if we've consumed a lot
	if p.outPos > 1024 {
		p.out = p.out[p.outPos:]
		p.outPos = 0
	}
	return b, true
}

// NextWord returns up to 64 bits packed into a uint64. The second
// return value is the actual number of bits (0–64).
func (p *Pipeline) NextWord() (uint64, int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	avail := len(p.out) - p.outPos
	if avail <= 0 {
		return 0, 0, false
	}
	n := avail
	if n > 64 {
		n = 64
	}
	var w uint64
	for i := 0; i < n; i++ {
		if p.out[p.outPos+i]&1 == 1 {
			w |= 1 << i
		}
	}
	p.outPos += n
	if p.outPos > 1024 {
		p.out = p.out[p.outPos:]
		p.outPos = 0
	}
	return w, n, true
}

// Flush forces any partial frame through the transform pipeline.
func (p *Pipeline) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending {
		p.processFrame()
		p.pending = false
	}
}

// ------------------------------------------------------------------
// Internal frame processing
// ------------------------------------------------------------------

func (p *Pipeline) processFrame() {
	p.frameCount++

	// Extract frame from circular buffer in chronological order
	frame := make([]float64, p.cfg.FrameSize)
	start := p.write // oldest sample is at write (circular)
	for i := 0; i < p.cfg.FrameSize; i++ {
		idx := (start + i) % p.cfg.FrameSize
		frame[i] = p.samples[idx]
	}

	// Apply window function
	window.Apply(frame, p.cfg.WindowFn)

	// Apply transform
	before := len(p.out)
	switch p.cfg.Transform {
	case TransformFFT:
		p.quantizeFFT(frame)
	case TransformWHT:
		p.quantizeWHT(frame)
	case TransformAutocorr:
		p.quantizeAutocorr(frame)
	default:
		p.quantizeDirect(frame)
	}

	// Feed newly produced bits into multiscale analysis
	if p.slider != nil {
		p.slider.Push(p.out[before:])
	}
}

// quantizeDirect thresholds each sample independently.
func (p *Pipeline) quantizeDirect(frame []float64) {
	thr := p.cfg.Threshold
	if thr == 0 {
		thr = 0.5 // default midpoint
	}
	for _, v := range frame {
		if v >= thr {
			p.out = append(p.out, 1)
		} else {
			p.out = append(p.out, 0)
		}
	}
}

// quantizeFFT runs a simple spectral energy → bit encoding.
// For now: high energy in upper half of spectrum → 1, else 0.
// This is a placeholder for richer spectral tokenization.
func (p *Pipeline) quantizeFFT(frame []float64) {
	// TODO: integrate internal/signal/fft for real spectral analysis
	// For now, use a crude energy ratio as proof of concept.
	var lowEnergy, highEnergy float64
	mid := len(frame) / 2
	for i, v := range frame {
		if i < mid {
			lowEnergy += v * v
		} else {
			highEnergy += v * v
		}
	}
	total := lowEnergy + highEnergy
	if total == 0 {
		p.out = append(p.out, 0)
		return
	}
	ratio := highEnergy / total
	// Quantize ratio into bits based on QuantLevels
	levels := p.cfg.QuantLevels
	for i := 0; i < levels-1; i++ {
		thr := float64(i+1) / float64(levels)
		if ratio >= thr {
			p.out = append(p.out, 1)
		} else {
			p.out = append(p.out, 0)
		}
	}
}

// quantizeWHT uses Walsh-Hadamard energy concentration.
func (p *Pipeline) quantizeWHT(frame []float64) {
	// TODO: integrate internal/signal/wht
	// Placeholder: same as direct for now
	p.quantizeDirect(frame)
}

// quantizeAutocorr uses lag-1 correlation sign.
func (p *Pipeline) quantizeAutocorr(frame []float64) {
	// TODO: integrate internal/signal/autocorr
	// Placeholder: lag-1 sign
	if len(frame) < 2 {
		p.out = append(p.out, 0)
		return
	}
	var sum float64
	for i := 1; i < len(frame); i++ {
		sum += frame[i] * frame[i-1]
	}
	if sum >= 0 {
		p.out = append(p.out, 1)
	} else {
		p.out = append(p.out, 0)
	}
}

// ------------------------------------------------------------------
// Telemetry
// ------------------------------------------------------------------

// Stats reports pipeline performance.
type Stats struct {
	Medium       string
	FrameSize    int
	Overlap      int
	Transform    string
	FramesProcessed uint64
	QueueDepth   int // bits currently queued
}

// Stats returns a snapshot of pipeline telemetry.
func (p *Pipeline) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Stats{
		Medium:       p.cfg.Medium.String(),
		FrameSize:    p.cfg.FrameSize,
		Overlap:      p.cfg.Overlap,
		Transform:    p.cfg.Transform.String(),
		FramesProcessed: p.frameCount,
		QueueDepth:   len(p.out) - p.outPos,
	}
}

// Summaries returns multiscale summaries computed from the quantized output
// bit stream.  Returns nil if multiscale analysis is not enabled.
func (p *Pipeline) Summaries() []multiscale.Summary {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.slider == nil {
		return nil
	}
	return p.slider.Summaries()
}
