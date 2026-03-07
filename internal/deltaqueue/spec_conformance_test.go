package deltaqueue

import "testing"

func TestClassifierStepsSinceOrdering(t *testing.T) {
	t.Skip("TODO(spec): implement conformance test from delta-classifier-constants appendix")
}

func BenchmarkClassify(b *testing.B) {
	b.Run("zero_stream", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkClassify/zero_stream")
	})
	b.Run("dense_transition_stream", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkClassify/dense_transition_stream")
	})
	b.Run("checkpoint_stream", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkClassify/checkpoint_stream")
	})
	b.Run("periodic_checkpoint_stream", func(b *testing.B) {
		b.Skip("TODO(spec): implement BenchmarkClassify/periodic_checkpoint_stream")
	})
}

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
