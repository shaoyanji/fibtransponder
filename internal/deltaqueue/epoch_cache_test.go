package deltaqueue

import (
	"math/rand"
	"testing"
)

// Equivalence between sequential replay and segmented/cached replay is the
// core contract of the epoch cache. These tests pin it.

const testEpoch = 0x9E3779B97F4A7C15

func entriesEqual(t *testing.T, a, b *MemFrontier) {
	t.Helper()
	if len(a.entries) != len(b.entries) {
		t.Fatalf("frontier size %d != %d", len(a.entries), len(b.entries))
	}
	for id, e := range a.entries {
		be, ok := b.entries[id]
		if !ok || be != e {
			t.Fatalf("entry %d diverged: %+v vs %+v", id, e, be)
		}
	}
}

func tombsEqual(t *testing.T, a, b *MemOpLog) {
	t.Helper()
	if len(a.tombstones) != len(b.tombstones) {
		t.Fatalf("tombstone count %d != %d", len(a.tombstones), len(b.tombstones))
	}
	for id := range a.tombstones {
		if !b.HasLiveTombstone(id) {
			t.Fatalf("tombstone %d missing in segmented replay", id)
		}
	}
}

// Deterministic op generator exercising every kind, including
// tombstone-then-readd, over a hot set so churn compresses.
func mixedLog(t *testing.T, nOps, hot int, seed int64) []MemOp {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	ops := make([]MemOp, 0, nOps)
	item := func() uint64 { return uint64(rng.Intn(hot) + 1) }
	for i := 0; i < nOps; i++ {
		tick := uint64(i + 1)
		switch r := rng.Intn(100); {
		case r < 25:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpAdd, Target: item(), Score: uint32(rng.Intn(90) + 10)})
		case r < 50:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpPromote, Target: item(), Score: uint32(rng.Intn(90) + 10)})
		case r < 70:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpDemote, Target: item(), Score: uint32(rng.Intn(90) + 10)})
		case r < 82:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpTombstone, Target: item()})
		case r < 92:
			a, b := item(), item()
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpMerge, Target: a, ParentA: a, ParentB: b})
		default:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpMaterializeEligible, Target: item()})
		}
	}
	return ops
}

func TestZeckendorfTerms(t *testing.T) {
	terms := ZeckendorfTerms(12)
	want := []int{8, 3, 1}
	if len(terms) != len(want) {
		t.Fatalf("terms(12) = %v, want %v", terms, want)
	}
	for i := range want {
		if terms[i] != want[i] {
			t.Fatalf("terms(12) = %v, want %v", terms, want)
		}
	}
	// Non-consecutive invariant: no two terms may be adjacent in fibTerms.
	pos := func(v int) int {
		for i, f := range fibTerms {
			if f == v {
				return i
			}
		}
		return -1
	}
	for _, n := range []int{1, 2, 7, 100, 10000, 999983} {
		ts := ZeckendorfTerms(n)
		sum := 0
		prev := -2
		for i, v := range ts {
			sum += v
			p := pos(v)
			if p < 0 {
				t.Fatalf("terms(%d): %d is not a fib term", n, v)
			}
			if i > 0 && p >= prev-1 {
				t.Fatalf("terms(%d): adjacent fib terms %v", n, ts)
			}
			prev = p
		}
		if sum != n {
			t.Fatalf("terms(%d) sums to %d", n, sum)
		}
	}
}

func TestZeckendorfBlocksPartition(t *testing.T) {
	ops := mixedLog(t, 1000, 64, 7)
	blocks := ZeckendorfBlocks(ops)
	total := 0
	for _, blk := range blocks {
		for j, op := range blk {
			if op != ops[total+j] {
				t.Fatal("blocks do not preserve op order")
			}
		}
		total += len(blk)
	}
	if total != len(ops) {
		t.Fatalf("blocks cover %d ops, want %d", total, len(ops))
	}
}

func TestSegmentedReplayEqualsFull(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 199} {
		ops := mixedLog(t, 3000, 64, seed)
		wantF, wantL := ReplayLog(ops)

		c := NewEpochCache(testEpoch)
		gotF, gotL := c.Replay(ops)
		entriesEqual(t, wantF, gotF)
		tombsEqual(t, wantL, gotL)

		// Second pass is all-cache and must produce the identical state.
		gotF2, gotL2 := c.Replay(ops)
		entriesEqual(t, wantF, gotF2)
		tombsEqual(t, wantL, gotL2)

		h, m, _ := c.Stats()
		if m == 0 || h == 0 {
			t.Fatalf("seed %d: expected misses then hits, got h=%d m=%d", seed, h, m)
		}
	}
}

func TestBlockDeltaPartialPrefix(t *testing.T) {
	ops := mixedLog(t, 2000, 64, 11)
	blocks := ZeckendorfBlocks(ops)
	fullF, fullL := ReplayLog(ops)

	// Cache the first block's delta the way a warm epoch cache would.
	c := NewEpochCache(testEpoch)
	warmF, warmL := NewMemFrontier(), NewMemOpLog()
	c.deltas[BlockKey(testEpoch, 0, blocks[0])] = ComputeBlockDelta(blocks[0], warmF, warmL)

	gotF, gotL := c.Replay(ops)
	entriesEqual(t, fullF, gotF)
	tombsEqual(t, fullL, gotL)
	if h, _, _ := c.Stats(); h != 1 {
		t.Fatalf("expected exactly one cache hit, got %d", h)
	}
}

func TestBlockDeltaTombstonePreserved(t *testing.T) {
	// add → tombstone → add inside one block: item returns to the frontier
	// but its tombstone stays live. A delta that records only absent items
	// would lose it.
	block := []MemOp{
		{Tick: 1, Kind: MemOpAdd, Target: 5, Score: 10},
		{Tick: 2, Kind: MemOpTombstone, Target: 5},
		{Tick: 3, Kind: MemOpAdd, Target: 5, Score: 40},
	}
	f1, l1 := NewMemFrontier(), NewMemOpLog()
	d := ComputeBlockDelta(block, f1, l1)

	f2, l2 := NewMemFrontier(), NewMemOpLog()
	d.ApplyTo(f2, l2)
	entriesEqual(t, f1, f2)
	tombsEqual(t, l1, l2)
	if !l2.HasLiveTombstone(5) {
		t.Fatal("delta application lost the live tombstone")
	}
}

func TestBlockAndQuotientKeys(t *testing.T) {
	ops := mixedLog(t, 500, 64, 3)
	blocks := ZeckendorfBlocks(ops)

	k0 := BlockKey(testEpoch, 0, blocks[0])
	if k0 != BlockKey(testEpoch, 0, blocks[0]) {
		t.Fatal("block key not deterministic")
	}
	if k0 == BlockKey(testEpoch, 1, blocks[0]) {
		t.Fatal("block key ignores position")
	}
	if k0 == BlockKey(testEpoch+1, 0, blocks[0]) {
		t.Fatal("block key ignores epoch")
	}

	c := NewEpochCache(testEpoch)
	q := c.Quotient(ops)
	if q != c.Quotient(ops) {
		t.Fatal("quotient not deterministic")
	}
	mutated := append([]MemOp(nil), ops...)
	mutated[10].Score ^= 0xFF
	if q == c.Quotient(mutated) {
		t.Fatal("quotient insensitive to op contents")
	}
	other := NewEpochCache(testEpoch + 1)
	if q == other.Quotient(ops) {
		t.Fatal("quotient ignores epoch")
	}
}

func TestBlockKeyOrderSensitive(t *testing.T) {
	// Same-tick ops in swapped order must hash differently: the delta they
	// produce can differ, so a commutative fold would alias them.
	a := MemOp{Tick: 1, Kind: MemOpPromote, Target: 7, Score: 5}
	b := MemOp{Tick: 1, Kind: MemOpTombstone, Target: 7}
	if BlockKey(testEpoch, 0, []MemOp{a, b}) == BlockKey(testEpoch, 0, []MemOp{b, a}) {
		t.Fatal("block key insensitive to op order within a block")
	}
}

func TestEpochCacheSnapshotRoundTrip(t *testing.T) {
	ops := mixedLog(t, 2000, 64, 5)
	c := NewEpochCache(testEpoch)

	if _, _, ok := c.Snapshot(ops); ok {
		t.Fatal("snapshot hit before any store")
	}

	f, l := ReplayLog(ops)
	c.StoreSnapshot(ops, f, l)

	entries, tombs, ok := c.Snapshot(ops)
	if !ok {
		t.Fatal("snapshot miss after store")
	}
	if len(entries) != len(f.entries) {
		t.Fatalf("snapshot entries %d != %d", len(entries), len(f.entries))
	}
	for id, e := range f.entries {
		if entries[id] != e {
			t.Fatalf("snapshot entry %d diverged", id)
		}
	}
	if len(tombs) != len(l.tombstones) {
		t.Fatalf("snapshot tombstones %d != %d", len(tombs), len(l.tombstones))
	}

	// The clone must be detached.
	entries[999999] = FrontierEntry{}
	entries2, _, _ := c.Snapshot(ops)
	if _, present := entries2[999999]; present {
		t.Fatal("snapshot is not a clone — cache corrupted by mutation")
	}
}
