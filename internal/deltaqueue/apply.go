package deltaqueue

// ApplyDelta translates a DerivedDelta into zero or more MemOps.
//
// The classifier produces candidate flags; ApplyDelta gates them through
// revalidation predicates before appending to the queue log.
//
// Target item selection is not handled here — the caller (policy layer
// or extension) is responsible for identifying which frontier items
// the delta's candidate flags apply to. This function applies the
// revalidation gates and MemOp emission for a single target.
//
// For bulk/multi-target application, call ApplyDeltaTarget per item.
func ApplyDelta(d DerivedDelta, q *QueueState) []MemOp {
	if q == nil {
		return nil
	}
	return ApplyDeltaTarget(d, q, 0)
}

// ApplyDeltaTarget applies DerivedDelta candidate flags against a specific
// frontier target item, gated by revalidation predicates.
//
// Returns zero or more MemOps. Empty slice means no valid operations
// for this delta+target combination.
func ApplyDeltaTarget(d DerivedDelta, q *QueueState, target uint64) []MemOp {
	if q == nil {
		return nil
	}

	var ops []MemOp

	// Order matters: tombstone before promote (can't promote a tombstoned item).
	// Merge before tombstone (can't tombstone a merge parent).

	// MaterializeEligible: item is ready for materialization.
	if d.DerivedFlags&FlagMaterializeCandidate != 0 {
		if RevalidateMaterializeEligible(target, q.Frontier, q.Log) {
			ops = append(ops, MemOp{
				Tick:   d.Core.Tick,
				Kind:   MemOpMaterializeEligible,
				Target: target,
			})
		}
	}

	// Promote: move item up in frontier priority.
	if d.DerivedFlags&FlagPromoteCandidate != 0 {
		if RevalidatePromote(target, q.Frontier, q.Log, q.Quota) {
			score, _ := q.Frontier.Score(target)
			ops = append(ops, MemOp{
				Tick:   d.Core.Tick,
				Kind:   MemOpPromote,
				Target: target,
				Score:  score.Stability + score.Recency, // snapshot at append time
			})
		}
	}

	// Demote: move item down in frontier priority.
	if d.DerivedFlags&FlagDemoteCandidate != 0 {
		if RevalidateDemote(target, q.Frontier, q.Log) {
			score, _ := q.Frontier.Score(target)
			ops = append(ops, MemOp{
				Tick:   d.Core.Tick,
				Kind:   MemOpDemote,
				Target: target,
				Score:  score.Stability + score.Recency,
			})
		}
	}

	// Tombstone: mark item as logically removed.
	if d.DerivedFlags&FlagTombstoneCandidate != 0 {
		if RevalidateTombstone(target, q.Frontier, q.Log) {
			ops = append(ops, MemOp{
				Tick:   d.Core.Tick,
				Kind:   MemOpTombstone,
				Target: target,
			})
		}
	}

	return ops
}

// ApplyDeltaMerge applies a merge operation between two frontier items.
// Caller must have already verified FlagPromoteCandidate or equivalent
// on the derived delta.
func ApplyDeltaMerge(d DerivedDelta, q *QueueState, parentA, parentB uint64) []MemOp {
	if q == nil {
		return nil
	}
	if !RevalidateMerge(parentA, parentB, q.Frontier, q.Log) {
		return nil
	}
	return []MemOp{{
		Tick:    d.Core.Tick,
		Kind:    MemOpMerge,
		Target:  parentA, // convention: first parent is the surviving target
		ParentA: parentA,
		ParentB: parentB,
	}}
}
