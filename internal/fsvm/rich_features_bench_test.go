package fsvm

import (
	"testing"
)

func BenchmarkExtractorExtract(b *testing.B) {
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

func BenchmarkExtractorPush(b *testing.B) {
	ex := NewExtractor(DefaultExtractorConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ex.Push(byte(i & 1))
	}
}

func BenchmarkExtractorPushAndExtract(b *testing.B) {
	ex := NewExtractor(DefaultExtractorConfig())
	s := &State{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ex.Push(byte(i & 1))
		if i%8 == 0 {
			_ = ex.Extract(s, EventDilate)
		}
	}
}

func BenchmarkDescriptorDistance(b *testing.B) {
	a := Descriptor{0x0102030405060708, 0x0807060504030201, 0xAABBCCDDEEFF0011, 0x1122334455667788}
	c := Descriptor{0x0201030504070608, 0x0708050604030201, 0xAABBCCDDEEFF0012, 0x1122334455667789}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Distance(a, c)
	}
}

func BenchmarkDescriptorCosine(b *testing.B) {
	a := Descriptor{0x0101010101010101, 0, 0, 0}
	c := Descriptor{0xFFFFFFFF01010101, 0, 0, 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(a, c)
	}
}

func BenchmarkFeatureBufferMatch(b *testing.B) {
	fb := NewFeatureBuffer(1000)
	for i := 0; i < 1000; i++ {
		fb.Append(FeatureEvent{
			BitPos: uint64(i),
			Desc:   Descriptor{uint64(i), uint64(i * 2), 0, 0},
		})
	}
	query := Descriptor{500, 1000, 0, 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fb.Match(query)
	}
}
