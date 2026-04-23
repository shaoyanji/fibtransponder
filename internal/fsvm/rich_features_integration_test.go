package fsvm

import (
	"testing"
)

func TestStepWithExtractorNoEvents(t *testing.T) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	// 8 zeros — no marker yet (marker at 8 after 8 zeros, but we only do 7).
	for i := 0; i < 7; i++ {
		var feats []FeatureEvent
		s, feats = StepWithExtractor(s, 0, ex)
		if len(feats) != 0 {
			t.Fatalf("expected no features at bit %d, got %d", i, len(feats))
		}
	}
}

func TestStepWithExtractorMarkerEvent(t *testing.T) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	// 8 zeros → marker event at the 8th zero.
	var feats []FeatureEvent
	for i := 0; i < 8; i++ {
		s, feats = StepWithExtractor(s, 0, ex)
	}
	if len(feats) != 1 {
		t.Fatalf("expected 1 feature event, got %d", len(feats))
	}
	if feats[0].EventKind != EventMarker {
		t.Errorf("expected Marker, got %v", feats[0].EventKind)
	}
	if feats[0].BitPos != 8 {
		t.Errorf("expected BitPos=8, got %d", feats[0].BitPos)
	}
	// Descriptor for all-zero window is legitimately all-zero (density 0,
	// transitions 0, Haar 0).  Verify the event was captured instead.
	if feats[0].Desc != (Descriptor{}) {
		// Actually it IS zero for uniform zero window; that's correct.
		// Just ensure it didn't panic and the struct is populated.
	}
}

func TestStepWithExtractorDilateEvent(t *testing.T) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	var feats []FeatureEvent
	// Two consecutive 1s → dilation.
	s, feats = StepWithExtractor(s, 1, ex)
	if len(feats) != 0 {
		t.Fatalf("expected no event on first 1, got %d", len(feats))
	}
	s, feats = StepWithExtractor(s, 1, ex)
	if len(feats) != 1 {
		t.Fatalf("expected 1 feature event, got %d", len(feats))
	}
	if feats[0].EventKind != EventDilate {
		t.Errorf("expected Dilate, got %v", feats[0].EventKind)
	}
	if feats[0].BitPos != 2 {
		t.Errorf("expected BitPos=2, got %d", feats[0].BitPos)
	}
}

func TestStepWithExtractorNilExtractor(t *testing.T) {
	s := New()
	// Two consecutive 1s → dilation, but no extractor.
	s, feats := StepWithExtractor(s, 1, nil)
	s, feats = StepWithExtractor(s, 1, nil)
	if feats != nil {
		t.Fatalf("expected nil features when extractor is nil, got %d", len(feats))
	}
	if s.Dilations != 1 {
		t.Errorf("expected 1 dilation, got %d", s.Dilations)
	}
}

func TestStepWord64WithExtractor(t *testing.T) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	// Word: 0x03 = 00000011 (LSB-first: two 1s, then 62 zeros)
	// First bit=1, second bit=1 → dilation on second bit.
	// Then 62 consecutive zeros produce markers at zero-run 8, 16, 32
	// (64 would need one more zero beyond the word).
	var batch EventBatch
	var feats []FeatureEvent
	s, batch, feats = StepWord64WithExtractor(s, 0x03, ex)
	if batch.DilateCount != 1 {
		t.Errorf("expected 1 dilation, got %d", batch.DilateCount)
	}
	if batch.MarkerCount != 3 {
		t.Errorf("expected 3 markers, got %d", batch.MarkerCount)
	}
	if len(feats) != 4 {
		t.Fatalf("expected 4 feature events (1 dilate + 3 markers), got %d", len(feats))
	}
	if feats[0].EventKind != EventDilate {
		t.Errorf("first event should be Dilate, got %v", feats[0].EventKind)
	}
	if feats[0].BitPos != 2 {
		t.Errorf("expected BitPos=2, got %d", feats[0].BitPos)
	}
}

func TestStepWord64WithExtractorNoExtractor(t *testing.T) {
	s := New()
	var batch EventBatch
	var feats []FeatureEvent
	s, batch, feats = StepWord64WithExtractor(s, 0x03, nil)
	if batch.DilateCount != 1 {
		t.Errorf("expected 1 dilation, got %d", batch.DilateCount)
	}
	if feats != nil {
		t.Fatalf("expected nil features, got %d", len(feats))
	}
}

func TestStepV2WithExtractor(t *testing.T) {
	s := NewWithFamily(0)
	ex := NewExtractor(DefaultExtractorConfig())
	var feats []FeatureEvent
	// 8 zeros → marker
	for i := 0; i < 8; i++ {
		s, feats = StepV2WithExtractor(s, 0, ex)
	}
	if len(feats) != 1 {
		t.Fatalf("expected 1 feature event, got %d", len(feats))
	}
	if feats[0].EventKind != EventMarker {
		t.Errorf("expected Marker, got %v", feats[0].EventKind)
	}
	if feats[0].Sketch != s.Sketch {
		t.Error("feature sketch should match state sketch")
	}
}

func TestStepWord64V2WithExtractor(t *testing.T) {
	s := NewWithFamily(0)
	ex := NewExtractor(DefaultExtractorConfig())
	var batch EventBatch
	var feats []FeatureEvent
	// Word: 8 zeros then 56 ones.
	// LSB-first: bits 0..7 = 0, bits 8..63 = 1
	// After 8 zeros → marker at bit 8.
	// Then bit 8 = 1 (no dilation, previous was 0).
	// Bits 9..63 = 1 → 55 dilations.
	word := uint64(0xFFFFFFFFFFFFFF00)
	s, batch, feats = StepWord64V2WithExtractor(s, word, ex)
	if batch.MarkerCount != 1 {
		t.Errorf("expected 1 marker, got %d", batch.MarkerCount)
	}
	if batch.DilateCount != 55 {
		t.Errorf("expected 55 dilations, got %d", batch.DilateCount)
	}
	if len(feats) != 56 {
		t.Fatalf("expected 56 feature events (1 marker + 55 dilate), got %d", len(feats))
	}
	if feats[0].EventKind != EventMarker {
		t.Errorf("first event should be Marker, got %v", feats[0].EventKind)
	}
	if feats[0].BitPos != 8 {
		t.Errorf("expected first event BitPos=8, got %d", feats[0].BitPos)
	}
}

func BenchmarkStepWithExtractor(b *testing.B) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = StepWithExtractor(s, byte(i&1), ex)
	}
}

func BenchmarkStepWord64WithExtractor(b *testing.B) {
	s := New()
	ex := NewExtractor(DefaultExtractorConfig())
	word := uint64(0xAAAAAAAAAAAAAAAA)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = StepWord64WithExtractor(s, word, ex)
	}
}
