package deltaqueue

// Revalidation predicates for queue mutation gates.
// Each predicate returns true only when the operation is valid to apply.
//
// These are decision-layer gates: they consult queue state but do not mutate it.
// Actual mutation happens in ApplyDelta / MemOp append.

// RevalidateAdd: item is not currently active in the frontier
// and does not currently have a live tombstone.
func RevalidateAdd(id uint64, frontier FrontierIndex, log OpLog) bool {
	return !frontier.Has(id) && !log.HasLiveTombstone(id)
}

// RevalidatePromote: item exists in frontier, is not tombstoned,
// promotion quota is available.
//
// Threshold check is deferred to the caller (policy layer) since the
// Score vector is multi-dimensional and promotion thresholds are
// domain-specific. The quota gate is enforced here.
func RevalidatePromote(id uint64, frontier FrontierIndex, log OpLog, quota PromotionQuota) bool {
	if !frontier.Has(id) {
		return false
	}
	if log.HasLiveTombstone(id) {
		return false
	}
	return quota.AllowPromotion()
}

// RevalidateDemote: item exists in frontier, is not tombstoned.
//
// Like promote, score-threshold comparison is left to the caller.
// This predicate gates structural validity only.
func RevalidateDemote(id uint64, frontier FrontierIndex, log OpLog) bool {
	return frontier.Has(id) && !log.HasLiveTombstone(id)
}

// RevalidateTombstone: item exists in frontier, is not already tombstoned.
func RevalidateTombstone(id uint64, frontier FrontierIndex, log OpLog) bool {
	return frontier.Has(id) && !log.HasLiveTombstone(id)
}

// RevalidateMerge: both parents exist in frontier, neither is tombstoned.
// Merge candidate flag is a derived-flag concern, not checked here
// (the caller must verify FlagPromoteCandidate or equivalent on the delta).
func RevalidateMerge(a, b uint64, frontier FrontierIndex, log OpLog) bool {
	return frontier.Has(a) && frontier.Has(b) &&
		!log.HasLiveTombstone(a) && !log.HasLiveTombstone(b)
}

// RevalidateMaterializeEligible: item exists in frontier, is not tombstoned,
// and is not already materialized.
// Score threshold check is caller's responsibility (like promote/demote).
func RevalidateMaterializeEligible(id uint64, frontier FrontierIndex, log OpLog) bool {
	if !frontier.Has(id) {
		return false
	}
	if log.HasLiveTombstone(id) {
		return false
	}
	return !frontier.Materialized(id)
}
