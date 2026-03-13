package deltaqueue

// Classify annotates a CoreDelta with derived hints.
// Allocation-free: all output lives on the stack.
//
// Update order (frozen):
//  1. Increment StepsSince lanes (saturating)
//  2. Derive flags from CoreDelta step-truths
//  3. Apply suppress/reset logic
//  4. Compute AuxBuckets
//  5. Update Sketch (Zobrist fold)
//  6. Emit DerivedDelta
func Classify(core CoreDelta, cls *ClassifierState) DerivedDelta {
	out := DerivedDelta{Core: core}

	if cls == nil {
		return out
	}

	// ── 1. Increment StepsSince lanes (saturating) ──
	for i := range cls.StepsSince {
		if cls.StepsSince[i] < StepsSinceSaturation {
			cls.StepsSince[i]++
		}
	}

	// ── 2. Derive flags from CoreDelta step-truths ──
	var derived uint32

	// Bit value flags pass through directly (step-truths).
	// Derived flags are computed from core flag combinations.

	// SegmentCandidate: checkpoint crossing is the canonical signal.
	// (Segment boundaries are interpretation-layer, but checkpoint is the
	// deterministic candidate trigger from the core.)
	if core.CoreFlags&FlagCheckpointCrossed != 0 {
		derived |= FlagSegmentCandidate
	}

	// MarkerCandidate: emitted when core signals a marker-worthy event.
	// In the current FSVM, markers fire on sparse zero-run thresholds (8,16,32,...).
	// The core doesn't emit a dedicated marker flag yet, so we infer from
	// checkpoint crossing as the closest proxy until core flags are extended.
	// TODO(spec): confirm whether core should emit a dedicated FlagMarkerEmitted.
	if core.CoreFlags&FlagCheckpointCrossed != 0 {
		derived |= FlagMarkerCandidate
	}

	// NovelPattern: state change signals a transition to a new FSVM state.
	if core.CoreFlags&FlagStateChanged != 0 {
		derived |= FlagNovelPattern
	}

	// PatternRepeat: consecutive ticks in the same state indicate repetition.
	if cls.HasPrev && core.StateID == cls.PrevStateID {
		derived |= FlagPatternRepeat
	}

	// ViewCanonicalityBroken/Restored: track canonicality transitions.
	// These mirror core canonicality flags to the view layer.
	if core.CoreFlags&FlagCoreCanonicalityBroken != 0 {
		derived |= FlagViewCanonicalityBroken
	}
	if core.CoreFlags&FlagCoreCanonicalityRestored != 0 {
		derived |= FlagViewCanonicalityRestored
	}

	// StreamDiscontinuity triggers suppress reset (handled in step 3 below).
	// Promote/Demote/Tombstone/Materialize candidates are decision-layer flags
	// that require queue state; they are not set by the classifier alone.
	// PolicyRelevant, DeferredOnly, HighUrgency are reserved for extension use.

	// ── 3. Apply suppress/reset logic ──
	if core.CoreFlags&FlagStreamDiscontinuity != 0 {
		cls.SuppressFor = SuppressForOnStreamDiscontinuity
	}
	if cls.SuppressFor > 0 {
		cls.SuppressFor--
		derived = 0 // all derived hints suppressed during recovery window
	}

	// ── 4. Reset StepsSince lanes on matching events ──
	// Only reset when not suppressed (so StepsSince reflects actual distance).
	if cls.SuppressFor == 0 {
		if core.CoreFlags&FlagStateChanged != 0 {
			cls.StepsSince[LaneStateChange] = 0
		}
		if core.CoreFlags&FlagDilationTriggered != 0 {
			cls.StepsSince[LaneDilation] = 0
		}
		if derived&FlagSegmentCandidate != 0 {
			cls.StepsSince[LaneSegmentCandidate] = 0
		}
		if derived&FlagMarkerCandidate != 0 {
			cls.StepsSince[LaneMarkerCandidate] = 0
		}
	}

	out.DerivedFlags = derived

	// Copy StepsSince from classifier state to output delta.
	out.StepsSince = cls.StepsSince

	// ── 5. Compute AuxBuckets (ordinal hints, 8 bits each) ──
	out.Aux = packAuxBuckets(classifyAux(core, *cls, derived))

	// ── 6. Update Sketch (Zobrist fold) ──
	// Fold state transitions into sketch for cheap divergence detection.
	cls.Sketch ^= uint64(core.StateID) * ZobristSeed
	cls.Sketch ^= uint64(core.CoreFlags) * ZobristSeed
	cls.Sketch ^= uint64(derived) * ZobristSeed

	// ── Update classifier state for next step ──
	cls.HasPrev = true
	cls.PrevFlags = core.CoreFlags
	cls.PrevStateID = core.StateID

	return out
}

// classifyAux computes AuxBuckets from step-truths and classifier state.
// Allocation-free: returns a value, not a pointer.
func classifyAux(core CoreDelta, cls ClassifierState, derived uint32) AuxBuckets {
	var aux AuxBuckets

	// Recency: log-scale, higher = more recent activity.
	// Based on minimum StepsSince across lanes (closest event = most recent).
	aux.Recency = stepsSinceToLogScale(minStepsSince(cls.StepsSince))

	// Novelty: higher when state is changing.
	aux.Novelty = uint8(minU32(uint32(cls.StepsSince[LaneStateChange])>>8, 255))
	// Invert: fewer steps since change = higher novelty.
	if aux.Novelty < 255 {
		aux.Novelty = 255 - aux.Novelty
	}

	// Stability: higher when in a long repeat (PatternRepeat set, no state change).
	if derived&FlagPatternRepeat != 0 && derived&FlagNovelPattern == 0 {
		aux.Stability = stepsSinceToLogScale(cls.StepsSince[LaneStateChange])
	}

	// Urgency: higher for candidates that need queue attention.
	if derived&(FlagSegmentCandidate|FlagMarkerCandidate) != 0 {
		aux.Urgency = 0x80 // moderate urgency baseline for candidates
	}
	if core.CoreFlags&FlagStreamDiscontinuity != 0 {
		aux.Urgency = 0xFF // maximum urgency on discontinuity
	}

	return aux
}

// stepsSinceToLogScale maps a StepsSince value to a log-scale 8-bit ordinal.
// Higher value = more recent (fewer steps ago).
func stepsSinceToLogScale(v uint16) uint8 {
	if v == 0 {
		return 255 // most recent
	}
	// log2-based compression: each halving of steps doubles the ordinal.
	var log uint8
	n := v
	for n > 1 && log < 255 {
		n >>= 1
		log++
	}
	return 255 - minU8(log*32, 255)
}

// packAuxBuckets packs four 8-bit ordinal values into a single uint32.
// Layout: [Recency|Novelty|Stability|Urgency] MSB→LSB.
func packAuxBuckets(a AuxBuckets) uint32 {
	return uint32(a.Recency)<<24 | uint32(a.Novelty)<<16 | uint32(a.Stability)<<8 | uint32(a.Urgency)
}

func minStepsSince(s [4]uint16) uint16 {
	m := s[0]
	for _, v := range s[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func minU8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
