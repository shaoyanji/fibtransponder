package deltaqueue

import "testing"

// ── Revalidation predicates ──

func TestRevalidateAdd(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()

	// Item not in frontier, no tombstone → valid.
	if !RevalidateAdd(1, frontier, log) {
		t.Fatal("fresh item should be addable")
	}

	// Item in frontier → invalid.
	frontier.Insert(2, Score{})
	if RevalidateAdd(2, frontier, log) {
		t.Fatal("existing item should not be addable")
	}

	// Item tombstoned → invalid.
	log.AddTombstone(3)
	if RevalidateAdd(3, frontier, log) {
		t.Fatal("tombstoned item should not be addable")
	}
}

func TestRevalidatePromote(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Used: 0, Max: 3}

	// Not in frontier → invalid.
	if RevalidatePromote(1, frontier, log, quota) {
		t.Fatal("missing item should not be promotable")
	}

	// In frontier, quota available → valid.
	frontier.Insert(1, Score{Recency: 100})
	if !RevalidatePromote(1, frontier, log, quota) {
		t.Fatal("frontier item with quota should be promotable")
	}

	// Tombstoned → invalid.
	log.AddTombstone(2)
	frontier.Insert(2, Score{})
	if RevalidatePromote(2, frontier, log, quota) {
		t.Fatal("tombstoned item should not be promotable")
	}

	// Quota exhausted → invalid.
	quota.Used = 3
	if RevalidatePromote(1, frontier, log, quota) {
		t.Fatal("exhausted quota should block promotion")
	}
}

func TestRevalidateTombstone(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()

	// Not in frontier → invalid.
	if RevalidateTombstone(1, frontier, log) {
		t.Fatal("missing item should not be tombstoneable")
	}

	// In frontier, not tombstoned → valid.
	frontier.Insert(1, Score{})
	if !RevalidateTombstone(1, frontier, log) {
		t.Fatal("active frontier item should be tombstoneable")
	}

	// Already tombstoned → invalid.
	log.AddTombstone(1)
	if RevalidateTombstone(1, frontier, log) {
		t.Fatal("already-tombstoned item should not be re-tombstoneable")
	}
}

func TestRevalidateMerge(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()

	frontier.Insert(1, Score{})
	frontier.Insert(2, Score{})

	// Both present, neither tombstoned → valid.
	if !RevalidateMerge(1, 2, frontier, log) {
		t.Fatal("two live frontier items should be mergeable")
	}

	// One tombstoned → invalid.
	log.AddTombstone(2)
	if RevalidateMerge(1, 2, frontier, log) {
		t.Fatal("tombstoned parent should block merge")
	}

	// Missing item → invalid.
	if RevalidateMerge(1, 999, frontier, log) {
		t.Fatal("missing item should block merge")
	}
}

func TestRevalidateMaterializeEligible(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()

	// Not in frontier → invalid.
	if RevalidateMaterializeEligible(1, frontier, log) {
		t.Fatal("missing item should not be materialize-eligible")
	}

	// In frontier, not materialized → valid.
	frontier.Insert(1, Score{})
	if !RevalidateMaterializeEligible(1, frontier, log) {
		t.Fatal("non-materialized frontier item should be eligible")
	}

	// Already materialized → invalid.
	frontier.SetMaterialized(1)
	if RevalidateMaterializeEligible(1, frontier, log) {
		t.Fatal("already-materialized item should not be re-eligible")
	}

	// Tombstoned → invalid.
	frontier.Insert(2, Score{})
	log.AddTombstone(2)
	if RevalidateMaterializeEligible(2, frontier, log) {
		t.Fatal("tombstoned item should not be materialize-eligible")
	}
}

// ── ApplyDelta ──

func TestApplyDeltaPromote(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Used: 0, Max: 5}
	q := NewQueueState(frontier, log, quota)

	frontier.Insert(42, Score{Recency: 100, Stability: 50})

	d := DerivedDelta{
		Core:         CoreDelta{Tick: 10},
		DerivedFlags: FlagPromoteCandidate,
	}

	ops := ApplyDeltaTarget(d, &q, 42)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Kind != MemOpPromote {
		t.Fatalf("expected MemOpPromote, got %v", ops[0].Kind)
	}
	if ops[0].Target != 42 {
		t.Fatalf("expected target 42, got %d", ops[0].Target)
	}
	if ops[0].Tick != 10 {
		t.Fatalf("expected tick 10, got %d", ops[0].Tick)
	}

	// Second promote: quota should still allow (Used=0 < Max=5).
	ops = ApplyDeltaTarget(d, &q, 42)
	if len(ops) != 1 {
		t.Fatalf("quota not consumed by read-only gate, expected 1 op")
	}
}

func TestApplyDeltaTombstoneBlocksPromote(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Max: 5}
	q := NewQueueState(frontier, log, quota)

	frontier.Insert(1, Score{})
	log.AddTombstone(1)

	d := DerivedDelta{
		Core:         CoreDelta{Tick: 1},
		DerivedFlags: FlagPromoteCandidate | FlagTombstoneCandidate,
	}

	// TombstoneCandidate: item is already tombstoned → no op.
	ops := ApplyDeltaTarget(d, &q, 1)
	for _, op := range ops {
		if op.Kind == MemOpPromote {
			t.Fatal("tombstoned item should not produce promote op")
		}
	}
}

func TestApplyDeltaMaterialize(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Max: 5}
	q := NewQueueState(frontier, log, quota)

	frontier.Insert(7, Score{Recency: 200})

	d := DerivedDelta{
		Core:         CoreDelta{Tick: 50},
		DerivedFlags: FlagMaterializeCandidate,
	}

	ops := ApplyDeltaTarget(d, &q, 7)
	if len(ops) != 1 || ops[0].Kind != MemOpMaterializeEligible {
		t.Fatalf("expected 1 MaterializeEligible op, got %v", ops)
	}
}

func TestApplyDeltaMerge(t *testing.T) {
	frontier := NewMemFrontier()
	log := NewMemOpLog()
	quota := &FixedQuota{Max: 5}
	q := NewQueueState(frontier, log, quota)

	frontier.Insert(10, Score{})
	frontier.Insert(20, Score{})

	d := DerivedDelta{Core: CoreDelta{Tick: 100}}

	ops := ApplyDeltaMerge(d, &q, 10, 20)
	if len(ops) != 1 {
		t.Fatalf("expected 1 merge op, got %d", len(ops))
	}
	if ops[0].Kind != MemOpMerge {
		t.Fatalf("expected MemOpMerge, got %v", ops[0].Kind)
	}
	if ops[0].ParentA != 10 || ops[0].ParentB != 20 {
		t.Fatalf("wrong parents: %d, %d", ops[0].ParentA, ops[0].ParentB)
	}

	// Tombstone one parent → merge blocked.
	log.AddTombstone(10)
	ops = ApplyDeltaMerge(d, &q, 10, 20)
	if len(ops) != 0 {
		t.Fatal("merge should be blocked when parent is tombstoned")
	}
}

func TestApplyDeltaNilState(t *testing.T) {
	d := DerivedDelta{Core: CoreDelta{Tick: 1}, DerivedFlags: FlagPromoteCandidate}
	if ops := ApplyDelta(d, nil); ops != nil {
		t.Fatal("nil QueueState should return nil ops")
	}
	if ops := ApplyDeltaTarget(d, nil, 1); ops != nil {
		t.Fatal("nil QueueState should return nil ops")
	}
}

func TestFixedQuota(t *testing.T) {
	q := &FixedQuota{Max: 2}
	if !q.AllowPromotion() {
		t.Fatal("fresh quota should allow")
	}
	q.RecordPromotion()
	if !q.AllowPromotion() {
		t.Fatal("after 1 of 2, should allow")
	}
	q.RecordPromotion()
	if q.AllowPromotion() {
		t.Fatal("after 2 of 2, should block")
	}
	q.RecordPromotion() // overflow guard
	if q.Used != 2 {
		t.Fatal("overflow should not increment past max")
	}
}
