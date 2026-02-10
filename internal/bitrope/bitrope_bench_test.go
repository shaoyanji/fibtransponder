package bitrope

import "testing"

func BenchmarkAppendBit(b *testing.B) {
	r := New(1 << 16)
	var x uint32 = 0xdeadbeef
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// xorshift-ish
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		r.AppendBit(uint8(x & 1))
	}
}
