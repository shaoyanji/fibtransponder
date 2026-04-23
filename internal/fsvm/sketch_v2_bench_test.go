package fsvm

import "testing"

func BenchmarkStepV2(b *testing.B) {
	st := NewWithFamily(0)
	pattern := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 1, 1}
	pi := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = StepV2(st, pattern[pi])
		pi++
		if pi >= len(pattern) {
			pi = 0
		}
	}
	_ = st
}

func BenchmarkStepWord64V2(b *testing.B) {
	st := NewWithFamily(0)
	words := make([]uint64, 1024)
	for i := range words {
		words[i] = uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}
	wi := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = StepWord64V2(st, words[wi])
		wi++
		if wi >= len(words) {
			wi = 0
		}
	}
	_ = st
}

func BenchmarkStepSequence64V2(b *testing.B) {
	word := uint64(0x123456789abcdef0)
	b.Run("bit-by-bit", func(b *testing.B) {
		st := NewWithFamily(0)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for bit := 0; bit < 64; bit++ {
				b := uint8((word >> bit) & 1)
				st, _ = StepV2(st, b)
			}
		}
		_ = st
	})
	b.Run("word", func(b *testing.B) {
		st := NewWithFamily(0)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			st, _ = StepWord64V2(st, word)
		}
		_ = st
	})
}
