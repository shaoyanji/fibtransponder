package bitrope

import "testing"

func TestAppendAndGet(t *testing.T) {
	r := New(256)
	seq := []uint8{1, 0, 1, 1, 0, 0, 1}
	for _, b := range seq {
		r.AppendBit(b)
	}
	if r.LenBits() != uint64(len(seq)) {
		t.Fatalf("len mismatch: got %d", r.LenBits())
	}
	for i, b := range seq {
		g := r.Get(uint64(i))
		if g != (b & 1) {
			t.Fatalf("i=%d want %d got %d", i, b, g)
		}
	}
	// out of range
	if r.Get(999) != 0 {
		t.Fatalf("expected 0 out-of-range")
	}
}
