package fsvm

import (
	"testing"
)

func TestSliceAdapter(t *testing.T) {
	bits := []byte{1, 0, 1, 1, 0}
	a := NewSliceAdapter(bits)
	for i, want := range bits {
		got, ok := a.Next()
		if !ok {
			t.Fatalf("unexpected end at position %d", i)
		}
		if got != want {
			t.Errorf("position %d: got %d, want %d", i, got, want)
		}
	}
	_, ok := a.Next()
	if ok {
		t.Error("expected exhaustion after last bit")
	}
}

func TestByteAdapterLSBFirst(t *testing.T) {
	// byte 0x03 = 00000011, LSB-first: bit0=1, bit1=1, rest=0
	data := []byte{0x03}
	a := NewByteAdapter(data, false)
	want := []byte{1, 1, 0, 0, 0, 0, 0, 0}
	for i, w := range want {
		got, ok := a.Next()
		if !ok {
			t.Fatalf("unexpected end at position %d", i)
		}
		if got != w {
			t.Errorf("position %d: got %d, want %d", i, got, w)
		}
	}
}

func TestByteAdapterMSBFirst(t *testing.T) {
	// byte 0x03 = 00000011, MSB-first: bit7=0, bit6=0, ..., bit1=1, bit0=1
	data := []byte{0x03}
	a := NewByteAdapter(data, true)
	want := []byte{0, 0, 0, 0, 0, 0, 1, 1}
	for i, w := range want {
		got, ok := a.Next()
		if !ok {
			t.Fatalf("unexpected end at position %d", i)
		}
		if got != w {
			t.Errorf("position %d: got %d, want %d", i, got, w)
		}
	}
}

func TestByteAdapterMultiByte(t *testing.T) {
	data := []byte{0x01, 0x80} // LSB-first: first byte ends with 1, second starts with 0
	a := NewByteAdapter(data, false)
	// Byte 0: bits 0..7 = 1,0,0,0,0,0,0,0
	// Byte 1: bits 0..7 = 0,0,0,0,0,0,0,1
	want := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	for i, w := range want {
		got, ok := a.Next()
		if !ok {
			t.Fatalf("unexpected end at position %d", i)
		}
		if got != w {
			t.Errorf("position %d: got %d, want %d", i, got, w)
		}
	}
}

func TestByteAdapterEmpty(t *testing.T) {
	a := NewByteAdapter([]byte{}, false)
	_, ok := a.Next()
	if ok {
		t.Error("expected exhaustion on empty data")
	}
}

func TestByteAdapterMSBEdgeCase(t *testing.T) {
	// Test MSB-first with byte that has bit pattern triggering edge case
	data := []byte{0xFF}
	a := NewByteAdapter(data, true)
	for i := 0; i < 8; i++ {
		got, ok := a.Next()
		if !ok {
			t.Fatalf("unexpected end at position %d", i)
		}
		if got != 1 {
			t.Errorf("position %d: got %d, want 1", i, got)
		}
	}
	_, ok := a.Next()
	if ok {
		t.Error("expected exhaustion after 8 bits")
	}
}

func TestWord64Adapter(t *testing.T) {
	words := []uint64{0x01, 0x02}
	a := NewWord64Adapter(words)
	w0, ok := a.Next()
	if !ok || w0 != 0x01 {
		t.Errorf("first word: got %x, want 0x01", w0)
	}
	w1, ok := a.Next()
	if !ok || w1 != 0x02 {
		t.Errorf("second word: got %x, want 0x02", w1)
	}
	_, ok = a.Next()
	if ok {
		t.Error("expected exhaustion")
	}
}

func TestRunAll(t *testing.T) {
	bits := []byte{1, 1} // two consecutive 1s → one dilation
	s := New()
	s = RunAll(s, NewSliceAdapter(bits))
	if s.Dilations != 1 {
		t.Errorf("expected 1 dilation, got %d", s.Dilations)
	}
	if s.BitsProcessed != 2 {
		t.Errorf("expected 2 bits processed, got %d", s.BitsProcessed)
	}
}

func BenchmarkSliceAdapter(b *testing.B) {
	bits := make([]byte, 1024)
	for i := range bits {
		bits[i] = byte(i & 1)
	}
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RunAll(s, NewSliceAdapter(bits))
	}
}

func BenchmarkByteAdapterLSB(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RunAll(s, NewByteAdapter(data, false))
	}
}

func TestRunAllV2(t *testing.T) {
	bits := []byte{1, 1} // two consecutive 1s → one dilation
	s := NewWithFamily(0)
	s = RunAllV2(s, NewSliceAdapter(bits))
	if s.Dilations != 1 {
		t.Errorf("expected 1 dilation, got %d", s.Dilations)
	}
	if s.BitsProcessed != 2 {
		t.Errorf("expected 2 bits processed, got %d", s.BitsProcessed)
	}
}

func TestNewWithSeeds(t *testing.T) {
	seeds := [2]uint64{0x1234567890abcdef, 0xfedcba0987654321}
	s := NewWithSeeds(seeds)
	if s.Seeds[0] != seeds[0] || s.Seeds[1] != seeds[1] {
		t.Errorf("seeds not set correctly: got %v, want %v", s.Seeds, seeds)
	}
}
