package deltaqueue

const (
	// StepsSince lane indexes as frozen in the constants appendix.
	LaneStateChange      = 0
	LaneDilation         = 1
	LaneSegmentCandidate = 2
	LaneMarkerCandidate  = 3
)

const (
	// StepsSince saturates at 0xFFFF and never wraps.
	StepsSinceSaturation uint16 = 0xFFFF
)

// CoreDelta is emitted by fsvm after each Step().
// All fields reflect the state of the machine after the step completes.
// CoreFlags contains only step-truths. See §I.5 for flag definitions.
type CoreDelta struct {
	Tick      uint64 // monotonic step counter, never reset
	StateID   uint32 // FSVM state identifier after this step
	SegEpoch  uint32 // coarse segment epoch; incremented by fsvm on major boundaries
	CoreFlags uint32 // see CoreFlag constants, §I.5.1
}

// DerivedDelta is produced by the classifier immediately after each Step().
// It embeds CoreDelta by value. No heap allocation is permitted.
type DerivedDelta struct {
	Core         CoreDelta
	DerivedFlags uint32 // see DerivedFlag constants, §I.5.2
	Aux          uint32 // packed score buckets; see §I.5.3
	// StepsSince is per-lane monotonic step counter, computed by the classifier
	// and emitted on every DerivedDelta. Saturates at StepsSinceSaturation (0xFFFF).
	// Lane indices: StateChange=0, Dilation=1, SegmentCandidate=2, MarkerCandidate=3.
	//
	// Resolution note: the queue spec omits this field because it defines the
	// queue-side contract only. The classifier constants appendix is authoritative
	// for the classifier→delta interface surface; it correctly requires
	// DerivedDelta.StepsSince[N] for the conformance test.
	StepsSince [4]uint16
}

// ClassifierState is the classifier's local residue between steps.
// It is owned by the classifier and must not be read by any other layer.
type ClassifierState struct {
	HasPrev     bool
	SuppressFor uint8
	PrevFlags   uint32
	PrevStateID uint32
	StepsSince  [4]uint16 // lanes: [StateChange, Dilation, SegmentCandidate, MarkerCandidate]
	Sketch      uint64
}

// AuxBuckets is packed into DerivedDelta.Aux (uint32).
// Buckets are ordinal hints.
type AuxBuckets struct {
	Recency   uint8 // log-scale; higher = more recent
	Novelty   uint8
	Stability uint8
	Urgency   uint8
}

// Subscription defines which DerivedDeltas wake an extension.
// Any: wake if (flags & Any) != 0
// All: wake only if (flags & All) == All
// (0 means no All requirement).
type Subscription struct {
	CoreAny    uint32
	CoreAll    uint32
	DerivedAny uint32
	DerivedAll uint32
}

// MemOpKind is the operation kind in the append-only queue log.
type MemOpKind uint8

const (
	MemOpAdd MemOpKind = iota
	MemOpPromote
	MemOpDemote
	MemOpTombstone
	MemOpMerge
	MemOpMaterializeEligible
)

// MemOp is an append-only record in the operation log.
// It is never mutated after append.
type MemOp struct {
	Tick    uint64
	Kind    MemOpKind
	Target  uint64
	ParentA uint64
	ParentB uint64
	Score   uint32 // snapshot at append time only
}

// Score is the queue frontier score vector.
type Score struct {
	Recency   uint32
	Stability uint32
	Reuse     uint32
	Novelty   uint32
}

// FrontierIndex provides read-only access to active frontier state.
type FrontierIndex interface {
	Has(id uint64) bool
	Score(id uint64) (Score, bool)
	Materialized(id uint64) bool
}

// OpLog provides read-only tombstone visibility.
type OpLog interface {
	HasLiveTombstone(id uint64) bool
}

// PromotionQuota provides read-only promotion budget state.
type PromotionQuota interface {
	AllowPromotion() bool
}

// QueueState carries minimal read-only dependencies for revalidation/apply.
type QueueState struct {
	Frontier FrontierIndex
	Log      OpLog
	Quota    PromotionQuota
}
