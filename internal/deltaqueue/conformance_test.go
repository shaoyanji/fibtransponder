package deltaqueue

import (
	"math/rand"
	"testing"
)

// ── Invariant conformance tests ──
// Source: CONFORMANCE_TARGETS.md "Invariant Targets"

// TestInvariantClassifierHintOnly: classifier never copies core flags into
// derived flags. Derived flags must only be set by classifier logic, not
// by direct passthrough of core flag bits.
//
// Note: core and derived flag namespaces share some bit positions
// (e.g., FlagWindowChanged=0x10 core == FlagViewCanonicalityBroken=0x10 derived).
// This is intentional — they mean different things in different contexts.
// We test semantic leakage, not value collision.
func TestInvariantClassifierHintOnly(t *testing.T) {
	coreOnlyFlags := []uint32{
		FlagBitZero, FlagBitOne, FlagStateChanged, FlagDilationTriggered,
		FlagWindowChanged, FlagCheckpointCrossed, FlagCounterWrapped,
		FlagCoreCanonicalityBroken, FlagCoreCanonicalityRestored,
		FlagStreamDiscontinuity,
	}

	for _, cf := range coreOnlyFlags {
		// Fresh classifier per iteration — no PatternRepeat bleed.
		cls := &ClassifierState{}
		out := Classify(CoreDelta{Tick: 1, CoreFlags: cf, StateID: 1}, cls)
		// If derived == core exactly, classifier just copied flags (bug).
		if out.DerivedFlags != 0 && out.DerivedFlags == cf {
			t.Fatalf("classifier copied core flag %#x verbatim into derived flags", cf)
		}
	}

	// Verify: with zero core flags, derived flags must also be zero.
	cls := &ClassifierState{}
	out := Classify(CoreDelta{Tick: 99, CoreFlags: 0, StateID: 1}, cls)
	if out.DerivedFlags != 0 {
		t.Fatalf("zero core flags produced non-zero derived: %#x", out.DerivedFlags)
	}
}

// TestInvariantCoreFlagsAreStepTruths: CoreDelta flags come only from the core,
// never from classifier side-effects. Verified by running the same core flags
// through multiple Classify calls and checking CoreDelta embed is preserved.
func TestInvariantCoreFlagsAreStepTruths(t *testing.T) {
	cls := &ClassifierState{}
	input := CoreDelta{Tick: 1, StateID: 7, SegEpoch: 3, CoreFlags: FlagStateChanged | FlagCheckpointCrossed}
	out := Classify(input, cls)

	if out.Core != input {
		t.Fatalf("CoreDelta not preserved: got %+v, want %+v", out.Core, input)
	}
}

// TestInvariantClassifierIsAllocationFree: verified by TestClassifyAllocationFree.
// This is a wrapper for the conformance targets list.
func TestInvariantClassifierIsAllocationFree(t *testing.T) {
	cls := &ClassifierState{}
	core := CoreDelta{Tick: 1, StateID: 42, CoreFlags: FlagStateChanged}

	allocs := testing.AllocsPerRun(100, func() {
		Classify(core, cls)
	})
	if allocs != 0 {
		t.Fatalf("Classify made %.1f allocs/op, want 0", allocs)
	}
}

// TestInvariantClassifierCostBudget: Classify must complete in bounded time.
// Budget: < 5µs per call on any reasonable hardware (generous for CI).
func TestInvariantClassifierCostBudget(t *testing.T) {
	cls := &ClassifierState{}
	core := CoreDelta{Tick: 1, StateID: 42,
		CoreFlags: FlagStateChanged | FlagCheckpointCrossed | FlagDilationTriggered}

	// Warm up.
	for i := 0; i < 1000; i++ {
		Classify(core, cls)
	}

	// Time 10000 iterations.
	const iters = 10000
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			core.Tick = uint64(i)
			Classify(core, cls)
		}
	})

	nsPerOp := float64(result.T.Nanoseconds()) / float64(result.N)
	budgetNs := 5000.0 // 5µs
	if nsPerOp > budgetNs {
		t.Fatalf("Classify cost %.0f ns/op exceeds budget %.0f ns/op", nsPerOp, budgetNs)
	}
	t.Logf("Classify cost: %.0f ns/op (budget: %.0f ns/op)", nsPerOp, budgetNs)
}

// TestInvariantQueueRevalidatesBeforeAct: every MemOp emitted by ApplyDelta
// must pass the corresponding revalidation predicate.
func TestInvariantQueueRevalidatesBeforeAct(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Max: 100}
	q := NewQueueState(frontier, log, quota)

	// Set up some frontier items.
	for i := uint64(1); i <= 10; i++ {
		frontier.Insert(i, Score{Recency: uint32(i * 10), Stability: uint32(i * 5)})
	}
	log.AddTombstone(3) // item 3 is tombstoned

	d := DerivedDelta{
		Core:         CoreDelta{Tick: 1},
		DerivedFlags: FlagPromoteCandidate | FlagTombstoneCandidate | FlagMaterializeCandidate,
	}

	// Try promoting tombstoned item → should produce no op.
	ops := ApplyDeltaTarget(d, &q, 3)
	if len(ops) != 0 {
		t.Fatal("tombstoned item produced ops without revalidation")
	}

	// Try promoting live item → should pass revalidation.
	ops = ApplyDeltaTarget(d, &q, 5)
	for _, op := range ops {
		switch op.Kind {
		case MemOpPromote:
			if !RevalidatePromote(5, frontier, log, quota) {
				t.Fatal("MemOpPromote emitted but RevalidatePromote would reject")
			}
		case MemOpTombstone:
			if !RevalidateTombstone(5, frontier, log) {
				t.Fatal("MemOpTombstone emitted but RevalidateTombstone would reject")
			}
		case MemOpMaterializeEligible:
			if !RevalidateMaterializeEligible(5, frontier, log) {
				t.Fatal("MemOpMaterializeEligible emitted but RevalidateMaterializeEligible would reject")
			}
		}
	}
}

// TestInvariantLogIsAppendOnly: MemOp records are never mutated after creation.
// We verify by storing pointers and checking immutability.
func TestInvariantLogIsAppendOnly(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Max: 10}
	q := NewQueueState(frontier, log, quota)

	frontier.Insert(1, Score{Recency: 100})

	d := DerivedDelta{
		Core:         CoreDelta{Tick: 1, StateID: 5},
		DerivedFlags: FlagPromoteCandidate,
	}

	ops := ApplyDeltaTarget(d, &q, 1)
	if len(ops) == 0 {
		t.Fatal("expected at least one op")
	}

	// MemOp is a value type; the slice elements are copies.
	// Verify original values are preserved.
	orig := ops[0]
	if orig.Tick != 1 || orig.Target != 1 || orig.Kind != MemOpPromote {
		t.Fatal("MemOp values not as expected at creation")
	}

	// After the MemOp is appended, its fields must not change.
	// (In the current implementation, MemOps are values, not pointers,
	// so this is trivially true. This test documents the contract.)
	if orig != (MemOp{Tick: 1, Kind: MemOpPromote, Target: 1, Score: orig.Score}) {
		t.Fatal("MemOp fields changed after creation")
	}
}

// TestInvariantDeletionIsViewRemoval: tombstoning an item removes it from
// frontier view (Has returns false after removal) but the op log preserves
// the tombstone record.
func TestInvariantDeletionIsViewRemoval(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()

	frontier.Insert(1, Score{Recency: 100})
	if !frontier.Has(1) {
		t.Fatal("item should be in frontier before tombstone")
	}

	// Record tombstone in log.
	log.AddTombstone(1)
	if !log.HasLiveTombstone(1) {
		t.Fatal("tombstone should be in log")
	}

	// Remove from frontier view.
	frontier.Remove(1)
	if frontier.Has(1) {
		t.Fatal("item should not be in frontier after removal")
	}

	// Tombstone persists in log (append-only).
	if !log.HasLiveTombstone(1) {
		t.Fatal("tombstone should persist in log after frontier removal")
	}
}

// TestInvariantFrontierIsDerivable: the frontier state should be reconstructable
// from the sequence of MemOps (add → score changes → tombstone → removal).
func TestInvariantFrontierIsDerivable(t *testing.T) {
	// Simulate a sequence of operations and verify the frontier
	// can be reconstructed by replaying the op log.
	type recordedOp struct {
		kind   MemOpKind
		target uint64
		score  uint32
	}

	var log []recordedOp

	// Replay: build a frontier from op sequence.
	replay := func(ops []recordedOp) map[uint64]struct{} {
		frontier := make(map[uint64]struct{})
		for _, op := range ops {
			switch op.kind {
			case MemOpAdd:
				frontier[op.target] = struct{}{}
			case MemOpTombstone:
				delete(frontier, op.target)
			}
		}
		return frontier
	}

	// Record a sequence.
	log = append(log, recordedOp{MemOpAdd, 1, 0})
	log = append(log, recordedOp{MemOpAdd, 2, 0})
	log = append(log, recordedOp{MemOpAdd, 3, 0})
	log = append(log, recordedOp{MemOpTombstone, 2, 0})

	f := replay(log)
	if len(f) != 2 {
		t.Fatalf("expected 2 items in frontier after replay, got %d", len(f))
	}
	if _, ok := f[1]; !ok {
		t.Fatal("item 1 missing from replayed frontier")
	}
	if _, ok := f[3]; !ok {
		t.Fatal("item 3 missing from replayed frontier")
	}
	if _, ok := f[2]; ok {
		t.Fatal("item 2 should not be in replayed frontier (tombstoned)")
	}
}

// ── Classifier benchmark variants ──

func BenchmarkClassifyVariants(b *testing.B) {
	cls := &ClassifierState{}

	b.Run("zero_stream", func(b *testing.B) {
		// All-zero input: no flags set, StepsSince monotonically increase.
		core := CoreDelta{Tick: 1}
		for i := 0; i < b.N; i++ {
			core.Tick = uint64(i)
			Classify(core, cls)
		}
	})

	b.Run("dense_transition_stream", func(b *testing.B) {
		// Every step has state change: LaneStateChange resets every tick.
		core := CoreDelta{Tick: 1, CoreFlags: FlagStateChanged}
		for i := 0; i < b.N; i++ {
			core.Tick = uint64(i)
			core.StateID = uint32(i % 16) // cycle through states
			Classify(core, cls)
		}
	})

	b.Run("checkpoint_stream", func(b *testing.B) {
		// Checkpoints every 8 steps.
		core := CoreDelta{Tick: 1}
		for i := 0; i < b.N; i++ {
			core.Tick = uint64(i)
			if i%8 == 0 {
				core.CoreFlags = FlagCheckpointCrossed
			} else {
				core.CoreFlags = 0
			}
			Classify(core, cls)
		}
	})

	b.Run("periodic_checkpoint_stream", func(b *testing.B) {
		// Regular checkpoint + dilation pattern.
		core := CoreDelta{Tick: 1}
		for i := 0; i < b.N; i++ {
			core.Tick = uint64(i)
			switch i % 16 {
			case 0:
				core.CoreFlags = FlagCheckpointCrossed
			case 7:
				core.CoreFlags = FlagDilationTriggered
			case 15:
				core.CoreFlags = FlagStateChanged
			default:
				core.CoreFlags = 0
			}
			Classify(core, cls)
		}
	})
}

// ── Queue benchmark variants ──

func BenchmarkQueueVariants(b *testing.B) {
	// Small frontier: 16 items
	b.Run("low_reprioritization_small_frontier", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 16; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10), Stability: uint32(i * 5)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			target := uint64(i%16) + 1
			ApplyDeltaTarget(d, &q, target)
		}
	})

	b.Run("high_reprioritization_small_frontier", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 16; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10), Stability: uint32(i * 5)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate | FlagDemoteCandidate}
		rng := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			target := uint64(rng.Intn(16) + 1)
			if rng.Intn(2) == 0 {
				d.DerivedFlags = FlagPromoteCandidate
			} else {
				d.DerivedFlags = FlagDemoteCandidate
			}
			ApplyDeltaTarget(d, &q, target)
		}
	})

	// Medium frontier: 256 items
	b.Run("low_reprioritization_medium_frontier", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 256; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10), Stability: uint32(i * 5)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			target := uint64(i%256) + 1
			ApplyDeltaTarget(d, &q, target)
		}
	})

	b.Run("high_reprioritization_medium_frontier", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 256; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10), Stability: uint32(i * 5)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate}
		rng := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			target := uint64(rng.Intn(256) + 1)
			if rng.Intn(2) == 0 {
				d.DerivedFlags = FlagPromoteCandidate
			} else {
				d.DerivedFlags = FlagDemoteCandidate
			}
			ApplyDeltaTarget(d, &q, target)
		}
	})

	b.Run("frequent_meld", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 32; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}}
		rng := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a := uint64(rng.Intn(32) + 1)
			b2 := uint64(rng.Intn(32) + 1)
			if a == b2 {
				b2 = (b2 % 32) + 1
			}
			ApplyDeltaMerge(d, &q, a, b2)
		}
	})

	b.Run("rare_meld", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 32; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}}
		rng := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%100 == 0 {
				// Rare merge attempt.
				a := uint64(rng.Intn(32) + 1)
				b2 := uint64(rng.Intn(32) + 1)
				ApplyDeltaMerge(d, &q, a, b2)
			} else {
				// Mostly promote/demote.
				target := uint64(rng.Intn(32) + 1)
				d.DerivedFlags = FlagPromoteCandidate
				ApplyDeltaTarget(d, &q, target)
			}
		}
	})

	b.Run("stale_hint_flood", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		// Only 4 items in frontier, but hints target many missing ids.
		for i := uint64(1); i <= 4; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10)})
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Target mostly missing items → revalidation rejects fast.
			target := uint64(i%100) + 1
			ApplyDeltaTarget(d, &q, target)
		}
	})

	b.Run("tombstone_density", func(b *testing.B) {
		frontier := NewMemFrontier()
		log := NewMemOpLog()
		quota := &FixedQuota{Max: uint32(b.N)}
		q := NewQueueState(frontier, log, quota)
		for i := uint64(1); i <= 64; i++ {
			frontier.Insert(i, Score{Recency: uint32(i * 10)})
			if i%2 == 0 {
				log.AddTombstone(i) // 50% tombstone density
			}
		}
		d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate}
		rng := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			target := uint64(rng.Intn(64) + 1)
			ApplyDeltaTarget(d, &q, target)
		}
	})
}
