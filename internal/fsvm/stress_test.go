package fsvm

import (
	"testing"
)

// adversarialWord produces a worst-case mixed pattern:
// frequent 11 adjacencies (dilations) and long zero runs (markers).
func adversarialWord(i int) uint64 {
	// Pattern: groups of 2 ones followed by 6 zeros.
	// This maximizes both dilations and marker cadence.
	return 0x0303030303030303 // 00000011 repeated
}

func BenchmarkStepAdversarial(b *testing.B) {
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, _ = Step(s, byte(i&3>>1)) // 0,0,1,1,0,0,1,1 pattern
	}
}

func BenchmarkStepWord64Adversarial(b *testing.B) {
	s := New()
	word := adversarialWord(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, _ = StepWord64(s, word)
	}
}

func BenchmarkStepWord64V2Adversarial(b *testing.B) {
	s := NewWithFamily(0)
	word := adversarialWord(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, _ = StepWord64V2(s, word)
	}
}

func BenchmarkStepWord64AllOnesStress(b *testing.B) {
	s := New()
	word := ^uint64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, _ = StepWord64(s, word)
	}
}

func BenchmarkStepWord64AllZerosStress(b *testing.B) {
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, _ = StepWord64(s, 0)
	}
}

// Rich feature stress: high-event-rate stream.
func BenchmarkExtractorHighEventRate(b *testing.B) {
	ex := NewExtractor(DefaultExtractorConfig())
	// Stream that triggers dilations every other bit.
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bit := byte(1) // constant 1s: every bit is a dilation after the first
		pre := s
		s, _ = Step(s, bit)
		ex.Push(bit)
		if s.Dilations > pre.Dilations {
			_ = ex.Extract(&s, EventDilate)
		}
	}
}

// StepWithExtractor under adversarial conditions.
func BenchmarkStepWithExtractorAdversarial(b *testing.B) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bit := byte(i & 3 >> 1) // 0,0,1,1 repeating
		_, _ = StepWithExtractor(s, bit, ex)
	}
}

// Long-stream stability: does performance degrade over millions of bits?
func BenchmarkLongStreamStability(b *testing.B) {
	// We process 1M bits per iteration to measure any drift.
	const batch = 1 << 20
	bits := make([]byte, batch)
	for i := range bits {
		bits[i] = byte(i & 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := New()
		s = RunAll(s, NewSliceAdapter(bits))
		_ = s.BitsProcessed
	}
}
