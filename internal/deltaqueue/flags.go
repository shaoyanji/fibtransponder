package deltaqueue

const (
	// ZobristSeed is frozen by the constants appendix and used as Sketch initial value on reset.
	ZobristSeed uint64 = 0x9e3779b97f4a7c15
)

const (
	// SuppressFor reset assignments from the frozen constants appendix.
	SuppressForOnStreamDiscontinuity uint8 = 2
)

const (
	FlagBitZero                  uint32 = 1 << 0
	FlagBitOne                   uint32 = 1 << 1
	FlagStateChanged             uint32 = 1 << 2
	FlagDilationTriggered        uint32 = 1 << 3
	FlagWindowChanged            uint32 = 1 << 4
	FlagCheckpointCrossed        uint32 = 1 << 5
	FlagCounterWrapped           uint32 = 1 << 6
	FlagCoreCanonicalityBroken   uint32 = 1 << 7
	FlagCoreCanonicalityRestored uint32 = 1 << 8
	FlagStreamDiscontinuity      uint32 = 1 << 9
)

const (
	FlagSegmentCandidate         uint32 = 1 << 0
	FlagSegmentBoundary          uint32 = 1 << 1
	FlagMarkerCandidate          uint32 = 1 << 2
	FlagNovelPattern             uint32 = 1 << 3
	FlagPatternRepeat            uint32 = 1 << 4
	FlagViewCanonicalityBroken   uint32 = 1 << 5
	FlagViewCanonicalityRestored uint32 = 1 << 6
	FlagPromoteCandidate         uint32 = 1 << 7
	FlagDemoteCandidate          uint32 = 1 << 8
	FlagTombstoneCandidate       uint32 = 1 << 9
	FlagMaterializeCandidate     uint32 = 1 << 10
	FlagPolicyRelevant           uint32 = 1 << 11
	FlagDeferredOnly             uint32 = 1 << 12
	FlagHighUrgency              uint32 = 1 << 13
)
