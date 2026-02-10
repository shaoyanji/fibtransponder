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
