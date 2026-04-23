package fsvm

import (
	"testing"
)

func TestExtractorBasics(t *testing.T) {
	ex := NewExtractor(DefaultExtractorConfig())

	// Push 64 ones — every sub-region has density 8, transitions 0,
	// Haar-X = 0, Haar-Y = 0 (centre 4 ones, surround 4 ones).
	for i := 0; i < 64; i++ {
		ex.Push(1)
	}

	s := &State{}
	d := ex.Extract(s, EventDilate)

	if d[0] != 0x0808080808080808 {
		t.Errorf("density word mismatch: got %016x, want %016x", d[0], 0x0808080808080808)
	}
	if d[1] != 0 {
		t.Errorf("transition word should be 0 for constant stream, got %016x", d[1])
	}
	if d[2] != 0 || d[3] != 0 {
		t.Errorf("Haar words should be 0 for uniform stream")
	}
}

func TestExtractorAlternating(t *testing.T) {
	ex := NewExtractor(DefaultExtractorConfig())

	// 64 alternating bits: 101010... starting with newest bit = 1.
	for i := 0; i < 64; i++ {
		ex.Push(byte(i % 2))
	}

	s := &State{}
	d := ex.Extract(s, EventMarker)

	// Every sub-region of 8 alternating bits has density 4, transitions 7.
	wantDensity := uint64(0x0404040404040404)
	wantTrans := uint64(0x0707070707070707)
	if d[0] != wantDensity {
		t.Errorf("density word: got %016x, want %016x", d[0], wantDensity)
	}
	if d[1] != wantTrans {
		t.Errorf("transition word: got %016x, want %016x", d[1], wantTrans)
	}

	// Haar-X = left(4 bits) - right(4 bits). Alternating pattern:
	// left half (bits 0,1,2,3) = 1,0,1,0 → density 2
	// right half (bits 4,5,6,7) = 1,0,1,0 → density 2
	// So Haar-X = 0 for every region.
	if d[2] != 0 {
		t.Errorf("Haar-X should be 0 for alternating, got %016x", d[2])
	}

	// Haar-Y = centre(bits 2,3,4,5) - surround(bits 0,1,6,7)
	// centre = 1,0,1,0 = 2, surround = 1,0,1,0 = 2 → 0
	if d[3] != 0 {
		t.Errorf("Haar-Y should be 0 for alternating, got %016x", d[3])
	}
}

func TestExtractorHaarResponses(t *testing.T) {
	ex := NewExtractor(DefaultExtractorConfig())

	// Push a pattern where the newest 8 bits are all 1s and the rest all 0s.
	// This gives one region with density 8, others 0.
	for i := 0; i < 56; i++ {
		ex.Push(0)
	}
	for i := 0; i < 8; i++ {
		ex.Push(1)
	}

	s := &State{}
	d := ex.Extract(s, EventDilate)

	// Region 0 (newest): density 8
	// Region 1..7: density 0
	if byte(d[0]) != 8 {
		t.Errorf("region 0 density should be 8, got %d", byte(d[0]))
	}
	if (d[0] >> 8) != 0 {
		t.Errorf("region 1 density should be 0")
	}

	// Region 0: transitions = 0 (all ones, no internal transitions)
	// Regions 1..6: transitions = 0
	// Region 7: transitions = 0
	if byte(d[1]) != 0 {
		t.Errorf("region 0 transitions should be 0, got %d", byte(d[1]))
	}

	// Region 0 Haar-X: left(4 ones) - right(4 ones) = 0
	if byte(d[2]) != 0 {
		t.Errorf("region 0 Haar-X should be 0, got %d", byte(d[2]))
	}

	// Region 1 Haar-X: all zeros → 0
	if byte(d[2]>>8) != 0 {
		t.Errorf("region 1 Haar-X should be 0")
	}
}

func TestExtractorPartialWindow(t *testing.T) {
	ex := NewExtractor(DefaultExtractorConfig())
	// Only 10 bits — window not full, but extraction should still work.
	for i := 0; i < 10; i++ {
		ex.Push(1)
	}
	s := &State{}
	d := ex.Extract(s, EventDilate)
	// Region 0 should have density from the 10 newest bits.
	// 10 ones in history[0..9], rest zeros.
	// Region 0 (bits 0..7): 8 ones → density 8
	// Region 1 (bits 8..15): 2 ones → density 2
	if byte(d[0]) != 8 {
		t.Errorf("region 0 density should be 8, got %d", byte(d[0]))
	}
	if byte(d[0]>>8) != 2 {
		t.Errorf("region 1 density should be 2, got %d", byte(d[0]>>8))
	}
}

func TestExtractorDisabled(t *testing.T) {
	cfg := DefaultExtractorConfig()
	cfg.Enabled = false
	ex := NewExtractor(cfg)
	for i := 0; i < 64; i++ {
		ex.Push(1)
	}
	s := &State{}
	d := ex.Extract(s, EventDilate)
	if d != (Descriptor{}) {
		t.Error("expected zero descriptor when extractor disabled")
	}
}

func TestDistanceIdentical(t *testing.T) {
	a := Descriptor{0x0102030405060708, 0x0807060504030201, 0, 0}
	b := Descriptor{0x0102030405060708, 0x0807060504030201, 0, 0}
	if d := Distance(a, b); d != 0 {
		t.Errorf("distance of identical descriptors should be 0, got %d", d)
	}
}

func TestDistanceDifferent(t *testing.T) {
	a := Descriptor{0, 0, 0, 0}
	b := Descriptor{0x0101010101010101, 0, 0, 0}
	// 8 bytes each = 1, total diff = 8
	if d := Distance(a, b); d != 8 {
		t.Errorf("expected distance 8, got %d", d)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := Descriptor{0x0101010101010101, 0, 0, 0}
	b := Descriptor{0x0202020202020202, 0, 0, 0}

	// Both vectors point in the same direction (all ones vs all twos)
	// Cosine should be 1.
	sim := CosineSimilarity(a, b)
	if sim < 0.99 || sim > 1.01 {
		t.Errorf("expected similarity ~1 for proportional vectors, got %f", sim)
	}

	// Orthogonal vectors: a = (1,1,1,1,1,1,1,1), c = (1,1,1,1,-1,-1,-1,-1)
	// dot = 4 - 4 = 0
	c := Descriptor{0xFFFFFFFF01010101, 0, 0, 0}
	sim2 := CosineSimilarity(a, c)
	if sim2 != 0 {
		t.Errorf("expected similarity 0 for orthogonal vectors, got %f", sim2)
	}

	// Opposite vectors: a = (1,...), d = (-1,...)
	d := Descriptor{0xFFFFFFFFFFFFFFFF, 0, 0, 0}
	sim3 := CosineSimilarity(a, d)
	if sim3 < -1.01 || sim3 > -0.99 {
		t.Errorf("expected similarity ~-1 for opposite vectors, got %f", sim3)
	}
}

func TestFeatureBuffer(t *testing.T) {
	fb := NewFeatureBuffer(3)
	fb.Append(FeatureEvent{BitPos: 1})
	fb.Append(FeatureEvent{BitPos: 2})
	fb.Append(FeatureEvent{BitPos: 3})
	if len(fb.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(fb.Events))
	}

	fb.Append(FeatureEvent{BitPos: 4})
	if len(fb.Events) != 3 {
		t.Fatalf("expected 3 events after overflow, got %d", len(fb.Events))
	}
	if fb.Events[0].BitPos != 2 {
		t.Errorf("expected oldest event=2, got %d", fb.Events[0].BitPos)
	}
	if fb.Events[2].BitPos != 4 {
		t.Errorf("expected newest event=4, got %d", fb.Events[2].BitPos)
	}
}

func TestFeatureBufferMatch(t *testing.T) {
	fb := NewFeatureBuffer(10)
	fb.Append(FeatureEvent{BitPos: 1, Desc: Descriptor{0, 0, 0, 0}})
	fb.Append(FeatureEvent{BitPos: 2, Desc: Descriptor{8, 0, 0, 0}})

	idx, dist := fb.Match(Descriptor{9, 0, 0, 0})
	if idx != 1 {
		t.Errorf("expected match index 1, got %d", idx)
	}
	if dist != 1 {
		t.Errorf("expected distance 1, got %d", dist)
	}
}

func TestPopCountDescriptor(t *testing.T) {
	d := Descriptor{0xFF, 0xFF00, 0, 0}
	if pc := PopCountDescriptor(d); pc != 16 {
		t.Errorf("expected popcount 16, got %d", pc)
	}
}

func TestExtractorWord64(t *testing.T) {
	ex := NewExtractor(DefaultExtractorConfig())
	// Pre-fill with 32 zeros.
	for i := 0; i < 32; i++ {
		ex.Push(0)
	}

	// Now push a word: bottom 16 bits = 1, top 48 bits = 0.
	word := uint64(0xFFFF) // bits 15..0 = 1
	// nBits = 16 means we process bits 15..0 (LSB-first).
	s := &State{}
	d := ex.ExtractWord64(s, EventDilate, word, 16)

	// After pushing 16 ones, the newest 16 bits are 1s.
	// Regions 0 and 1 (8 bits each) should have density 8.
	if byte(d[0]) != 8 {
		t.Errorf("region 0 density should be 8, got %d", byte(d[0]))
	}
	if byte(d[0]>>8) != 8 {
		t.Errorf("region 1 density should be 8, got %d", byte(d[0]>>8))
	}
	// Region 2 should be 0 (still from pre-fill).
	if byte(d[0]>>16) != 0 {
		t.Errorf("region 2 density should be 0, got %d", byte(d[0]>>16))
	}
}

func BenchmarkExtractorExtractInTestFile(b *testing.B) {
	ex := NewExtractor(DefaultExtractorConfig())
	for i := 0; i < 64; i++ {
		ex.Push(byte(i % 2))
	}
	s := &State{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ex.Extract(s, EventDilate)
	}
}
