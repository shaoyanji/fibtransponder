package deltaqueue

// Replay: derive frontier state from the append-only MemOp log.
//
// The op log is post-gate truth: every appended MemOp passed a Revalidate*
// predicate at append time, so replay applies ops unconditionally — there is
// no second validation pass. The per-kind guards below exist only so replay
// is also well-defined on hand-constructed logs that skip the gates.
//
// Provisional semantics (gap: frontier replay in HANDOFF.md): MemOp.Score is a
// combined snapshot (Stability+Recency at append time) and cannot be split on
// replay, so score effects are restored into Score.Recency only. Unknown
// MemOpKind values are skipped. This is the same kernel the epoch-replay
// probe measured; if the spec later defines different replay semantics, both
// this file and epoch_probe_test.go move together.

// ReplayMemOp applies one op's effect to frontier and log.
func ReplayMemOp(op MemOp, f *MemFrontier, l *MemOpLog) {
	replayMemOp(op, f, l, nil)
}

// replayMemOp is ReplayMemOp with an optional touched-item recorder used by
// ComputeBlockDelta. An id is recorded only when the op actually mutates
// frontier state for it.
func replayMemOp(op MemOp, f *MemFrontier, l *MemOpLog, touched map[uint64]struct{}) {
	mark := func(id uint64) {
		if touched != nil {
			touched[id] = struct{}{}
		}
	}
	switch op.Kind {
	case MemOpAdd:
		mark(op.Target)
		f.entries[op.Target] = FrontierEntry{Score: Score{Recency: op.Score}}
	case MemOpPromote:
		if e, ok := f.entries[op.Target]; ok {
			mark(op.Target)
			e.Score.Recency += op.Score
			f.entries[op.Target] = e
		}
	case MemOpDemote:
		if e, ok := f.entries[op.Target]; ok {
			mark(op.Target)
			e.Score.Recency = satSub32(e.Score.Recency, op.Score)
			f.entries[op.Target] = e
		}
	case MemOpTombstone:
		if _, ok := f.entries[op.Target]; ok {
			mark(op.Target)
			delete(f.entries, op.Target)
			l.tombstones[op.Target] = struct{}{}
		}
	case MemOpMerge:
		a, oka := f.entries[op.ParentA]
		b, okb := f.entries[op.ParentB]
		if oka && okb {
			mark(op.ParentA)
			mark(op.ParentB)
			a.Score.Recency += b.Score.Recency
			f.entries[op.ParentA] = a
			delete(f.entries, op.ParentB)
		}
	case MemOpMaterializeEligible:
		if e, ok := f.entries[op.Target]; ok {
			mark(op.Target)
			e.Materialized = true
			f.entries[op.Target] = e
		}
	}
}

func satSub32(a, b uint32) uint32 {
	if a < b {
		return 0
	}
	return a - b
}

// ReplayOps applies a sequence of ops onto existing frontier/log state.
// Use for incremental replay; use ReplayLog for from-genesis derivation.
func ReplayOps(ops []MemOp, f *MemFrontier, l *MemOpLog) {
	for _, op := range ops {
		replayMemOp(op, f, l, nil)
	}
}

// ReplayLog derives frontier and log state from genesis by replaying the
// full op sequence.
func ReplayLog(ops []MemOp) (*MemFrontier, *MemOpLog) {
	f := NewMemFrontier()
	l := NewMemOpLog()
	ReplayOps(ops, f, l)
	return f, l
}

// SnapshotFrontier clones the frontier's entry map. The result is detached
// from the frontier and safe to store or hand to a cache.
func SnapshotFrontier(f *MemFrontier) map[uint64]FrontierEntry {
	snap := make(map[uint64]FrontierEntry, len(f.entries))
	for id, e := range f.entries {
		snap[id] = e
	}
	return snap
}
