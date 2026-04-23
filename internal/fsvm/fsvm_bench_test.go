package fsvm

import "testing"

func BenchmarkStep(b *testing.B) {
	st := New()
	// adversarial-ish pattern: long zeros, then 11, repeat
	pattern := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 1, 1}
	pi := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = Step(st, pattern[pi])
		pi++
		if pi >= len(pattern) {
			pi = 0
		}
	}
	_ = st
}

// BenchmarkStepWord64 measures the word-level fast path on random data.
func BenchmarkStepWord64(b *testing.B) {
	st := New()
	// Pre-generate random words so the RNG isn't in the timed loop.
	words := make([]uint64, 1024)
	for i := range words {
		words[i] = uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}
	wi := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = StepWord64(st, words[wi])
		wi++
		if wi >= len(words) {
			wi = 0
		}
	}
	_ = st
}

// BenchmarkStepWord64AllZeros isolates the all-zeros fast path.
func BenchmarkStepWord64AllZeros(b *testing.B) {
	st := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = StepWord64(st, 0)
	}
	_ = st
}

// BenchmarkStepWord64AllOnes isolates the all-ones fast path.
func BenchmarkStepWord64AllOnes(b *testing.B) {
	st := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = StepWord64(st, ^uint64(0))
	}
	_ = st
}

// BenchmarkStepSequence64 compares 64 per-bit Steps vs one word step.
func BenchmarkStepSequence64(b *testing.B) {
	word := uint64(0x123456789abcdef0)
	b.Run("bit-by-bit", func(b *testing.B) {
		st := New()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for bit := 0; bit < 64; bit++ {
				b := uint8((word >> bit) & 1)
				st, _ = Step(st, b)
			}
		}
		_ = st
	})
	b.Run("word", func(b *testing.B) {
		st := New()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			st, _ = StepWord64(st, word)
		}
		_ = st
	})
}
