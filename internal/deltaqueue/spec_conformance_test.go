package deltaqueue

// Conformance tests for delta-classifier-constants appendix.
// Implementations live in classifier_test.go.
// This file retains scaffolded benchmarks that haven't been implemented yet.

import "testing"

// BenchmarkClassify scaffolds are now in classifier_test.go.

func BenchmarkQueue(b *testing.B) {
	b.Run("low_reprioritization_small_frontier", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/low_reprioritization_small_frontier")
	})
	b.Run("high_reprioritization_small_frontier", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/high_reprioritization_small_frontier")
	})
	b.Run("low_reprioritization_medium_frontier", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/low_reprioritization_medium_frontier")
	})
	b.Run("high_reprioritization_medium_frontier", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/high_reprioritization_medium_frontier")
	})
	b.Run("frequent_meld", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/frequent_meld")
	})
	b.Run("rare_meld", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/rare_meld")
	})
	b.Run("stale_hint_flood", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/stale_hint_flood")
	})
	b.Run("tombstone_density", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkQueue/tombstone_density")
	})
}
