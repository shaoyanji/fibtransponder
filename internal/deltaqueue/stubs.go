package deltaqueue

// Classify annotates a CoreDelta with derived hints.
// TODO(spec): implement allocation-free classifier logic.
func Classify(core CoreDelta, cls *ClassifierState) DerivedDelta {
	out := DerivedDelta{Core: core}
	if cls == nil {
		// TODO(spec): nil classifier state contract is not specified.
		return out
	}
	// TODO(spec): constants appendix conformance text expects DerivedDelta.StepsSince.
	out.StepsSince = cls.StepsSince
	return out
}

// RevalidateAdd: item is not currently active in the frontier
// and does not currently have a live tombstone.
// TODO(spec): implement predicate semantics.
func RevalidateAdd(id uint64, frontier FrontierIndex, log OpLog) bool {
	_, _, _ = id, frontier, log
	return false
}

// RevalidatePromote: item exists in frontier, is not tombstoned,
// current score meets promotion threshold, promotion quota not exceeded.
// TODO(spec): implement predicate semantics.
func RevalidatePromote(id uint64, frontier FrontierIndex, log OpLog, quota PromotionQuota) bool {
	_, _, _, _ = id, frontier, log, quota
	return false
}

// RevalidateDemote: item exists in frontier, is not tombstoned,
// current score is below demotion threshold.
// TODO(spec): implement predicate semantics.
func RevalidateDemote(id uint64, frontier FrontierIndex, log OpLog) bool {
	_, _, _ = id, frontier, log
	return false
}

// RevalidateTombstone: item exists in frontier, is not already tombstoned.
// TODO(spec): implement predicate semantics.
func RevalidateTombstone(id uint64, frontier FrontierIndex, log OpLog) bool {
	_, _, _ = id, frontier, log
	return false
}

// RevalidateMerge: both parents exist in frontier, neither is tombstoned,
// merge candidate flag still active on at least one parent.
// TODO(spec): implement predicate semantics.
func RevalidateMerge(a, b uint64, frontier FrontierIndex, log OpLog) bool {
	_, _, _, _ = a, b, frontier, log
	return false
}

// RevalidateMaterializeEligible: item exists in frontier, is not tombstoned,
// score meets materialization threshold, item is not already materialized.
// TODO(spec): implement predicate semantics.
func RevalidateMaterializeEligible(id uint64, frontier FrontierIndex, log OpLog) bool {
	_, _, _ = id, frontier, log
	return false
}

// ApplyDelta translates a DerivedDelta into zero or more MemOps.
// TODO(spec): implement idempotent apply + revalidation.
func ApplyDelta(d DerivedDelta, q *QueueState) []MemOp {
	_ = d
	if q != nil {
		_, _, _ = q.frontier, q.log, q.quota
	}
	return nil
}
