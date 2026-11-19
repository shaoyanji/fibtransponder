package deltaqueue

import "testing"

// Replay semantics are provisional (see replay.go header): ops in a real log
// are post-gate, so these tests pin the kernel's behavior op by op.

func entryScore(t *testing.T, f *MemFrontier, id uint64) Score {
	t.Helper()
	s, ok := f.Score(id)
	if !ok {
		t.Fatalf("item %d missing from frontier", id)
	}
	return s
}

func TestReplayMemOpKinds(t *testing.T) {
	f := NewMemFrontier()
	l := NewMemOpLog()

	ReplayMemOp(MemOp{Tick: 1, Kind: MemOpAdd, Target: 1, Score: 10}, f, l)
	if s := entryScore(t, f, 1); s.Recency != 10 {
		t.Fatalf("add: recency = %d, want 10", s.Recency)
	}

	ReplayMemOp(MemOp{Tick: 2, Kind: MemOpPromote, Target: 1, Score: 5}, f, l)
	if s := entryScore(t, f, 1); s.Recency != 15 {
		t.Fatalf("promote: recency = %d, want 15", s.Recency)
	}

	// Demote saturates at zero.
	ReplayMemOp(MemOp{Tick: 3, Kind: MemOpDemote, Target: 1, Score: 25}, f, l)
	if s := entryScore(t, f, 1); s.Recency != 0 {
		t.Fatalf("demote saturation: recency = %d, want 0", s.Recency)
	}

	ReplayMemOp(MemOp{Tick: 4, Kind: MemOpMaterializeEligible, Target: 1}, f, l)
	if !f.Materialized(1) {
		t.Fatal("materialize: flag not set")
	}

	// Merge folds the secondary parent into the survivor and removes it.
	ReplayMemOp(MemOp{Tick: 5, Kind: MemOpAdd, Target: 2, Score: 7}, f, l)
	ReplayMemOp(MemOp{Tick: 6, Kind: MemOpMerge, Target: 1, ParentA: 1, ParentB: 2}, f, l)
	if s := entryScore(t, f, 1); s.Recency != 7 {
		t.Fatalf("merge survivor: recency = %d, want 7", s.Recency)
	}
	if f.Has(2) {
		t.Fatal("merge secondary still in frontier")
	}

	// Tombstone removes from view and persists in the log.
	ReplayMemOp(MemOp{Tick: 7, Kind: MemOpTombstone, Target: 1}, f, l)
	if f.Has(1) {
		t.Fatal("tombstoned item still in frontier")
	}
	if !l.HasLiveTombstone(1) {
		t.Fatal("tombstone not recorded in log")
	}
}

func TestReplayMemOpGuards(t *testing.T) {
	f := NewMemFrontier()
	l := NewMemOpLog()

	// Ops against absent items are no-ops (defensive on non-gated logs).
	ReplayMemOp(MemOp{Tick: 1, Kind: MemOpPromote, Target: 9, Score: 5}, f, l)
	ReplayMemOp(MemOp{Tick: 2, Kind: MemOpDemote, Target: 9, Score: 5}, f, l)
	ReplayMemOp(MemOp{Tick: 3, Kind: MemOpTombstone, Target: 9}, f, l)
	ReplayMemOp(MemOp{Tick: 4, Kind: MemOpMaterializeEligible, Target: 9}, f, l)
	ReplayMemOp(MemOp{Tick: 5, Kind: MemOpMerge, Target: 9, ParentA: 9, ParentB: 8}, f, l)
	if f.Has(9) || l.HasLiveTombstone(9) {
		t.Fatal("ops on absent items must not create state")
	}

	// Merge with one parent absent is a no-op.
	ReplayMemOp(MemOp{Tick: 6, Kind: MemOpAdd, Target: 1, Score: 3}, f, l)
	ReplayMemOp(MemOp{Tick: 7, Kind: MemOpMerge, Target: 1, ParentA: 1, ParentB: 8}, f, l)
	if s := entryScore(t, f, 1); s.Recency != 3 {
		t.Fatalf("partial merge mutated survivor: recency = %d, want 3", s.Recency)
	}
}

func TestReplayLogDeterministic(t *testing.T) {
	ops := []MemOp{
		{Tick: 1, Kind: MemOpAdd, Target: 1, Score: 10},
		{Tick: 2, Kind: MemOpAdd, Target: 2, Score: 20},
		{Tick: 3, Kind: MemOpPromote, Target: 1, Score: 5},
		{Tick: 4, Kind: MemOpMerge, Target: 1, ParentA: 1, ParentB: 2},
		{Tick: 5, Kind: MemOpTombstone, Target: 1},
		{Tick: 6, Kind: MemOpAdd, Target: 3, Score: 30},
	}
	f1, l1 := ReplayLog(ops)
	f2, l2 := ReplayLog(ops)
	if len(f1.entries) != len(f2.entries) || len(l1.tombstones) != len(l2.tombstones) {
		t.Fatal("replay is not deterministic")
	}
	for id, e := range f1.entries {
		if f2.entries[id] != e {
			t.Fatalf("entry %d diverged between identical replays", id)
		}
	}
}

// Tombstone-then-add inside a log: the item returns to the frontier while
// its tombstone stays live. This is the case a touched-items delta must
// record correctly (see ComputeBlockDelta).
func TestReplayTombstoneThenAdd(t *testing.T) {
	f, l := ReplayLog([]MemOp{
		{Tick: 1, Kind: MemOpAdd, Target: 1, Score: 10},
		{Tick: 2, Kind: MemOpTombstone, Target: 1},
		{Tick: 3, Kind: MemOpAdd, Target: 1, Score: 40},
	})
	if s := entryScore(t, f, 1); s.Recency != 40 {
		t.Fatalf("re-add: recency = %d, want 40", s.Recency)
	}
	if !l.HasLiveTombstone(1) {
		t.Fatal("tombstone must persist through re-add")
	}
}
