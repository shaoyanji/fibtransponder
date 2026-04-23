package deltaqueue

import "testing"

// TestClassifierStepsSinceOrdering verifies per-lane StepsSince semantics:
//   - all lanes increment on each Classify call
//   - matching-lane resets to 0 on event
//   - non-matching lanes continue monotonic increment
//   - saturating behavior at StepsSinceSaturation
func TestClassifierStepsSinceOrdering(t *testing.T) {
	cls := &ClassifierState{}

	// Step 1: no flags, all lanes increment to 1.
	out := Classify(CoreDelta{Tick: 1}, cls)
	for i, v := range cls.StepsSince {
		if v != 1 {
			t.Fatalf("step 1: StepsSince[%d] = %d, want 1", i, v)
		}
	}
	for i, v := range out.StepsSince {
		if v != 1 {
			t.Fatalf("step 1 delta: StepsSince[%d] = %d, want 1", i, v)
		}
	}

	// Step 2: state change resets LaneStateChange, others increment.
	out = Classify(CoreDelta{Tick: 2, CoreFlags: FlagStateChanged}, cls)
	if got := cls.StepsSince[LaneStateChange]; got != 0 {
		t.Fatalf("step 2: LaneStateChange = %d, want 0", got)
	}
	for _, lane := range []int{LaneDilation, LaneSegmentCandidate, LaneMarkerCandidate} {
		if got := cls.StepsSince[lane]; got != 2 {
			t.Fatalf("step 2: lane %d = %d, want 2", lane, got)
		}
	}

	// Step 3: dilation resets LaneDilation.
	out = Classify(CoreDelta{Tick: 3, CoreFlags: FlagDilationTriggered}, cls)
	if got := cls.StepsSince[LaneStateChange]; got != 1 {
		t.Fatalf("step 3: LaneStateChange = %d, want 1", got)
	}
	if got := cls.StepsSince[LaneDilation]; got != 0 {
		t.Fatalf("step 3: LaneDilation = %d, want 0", got)
	}

	// Step 4: checkpoint crossing resets SegmentCandidate + MarkerCandidate.
	out = Classify(CoreDelta{Tick: 4, CoreFlags: FlagCheckpointCrossed}, cls)
	if got := cls.StepsSince[LaneSegmentCandidate]; got != 0 {
		t.Fatalf("step 4: LaneSegmentCandidate = %d, want 0", got)
	}
	if got := cls.StepsSince[LaneMarkerCandidate]; got != 0 {
		t.Fatalf("step 4: LaneMarkerCandidate = %d, want 0", got)
	}

	// Verify derived flags: checkpoint → segment + marker candidates.
	if out.DerivedFlags&FlagSegmentCandidate == 0 {
		t.Fatal("step 4: FlagSegmentCandidate not set on checkpoint crossing")
	}
	if out.DerivedFlags&FlagMarkerCandidate == 0 {
		t.Fatal("step 4: FlagMarkerCandidate not set on checkpoint crossing")
	}

	// Step 5: suppression window zeros all derived flags.
	out = Classify(CoreDelta{Tick: 5, CoreFlags: FlagStreamDiscontinuity}, cls)
	if cls.SuppressFor != 1 {
		t.Fatalf("step 5: SuppressFor = %d, want 1", cls.SuppressFor)
	}
	if out.DerivedFlags != 0 {
		t.Fatalf("step 5: DerivedFlags = %#x, want 0 (suppressed)", out.DerivedFlags)
	}

	// Step 6: still suppressed.
	out = Classify(CoreDelta{Tick: 6, CoreFlags: FlagCheckpointCrossed}, cls)
	if out.DerivedFlags != 0 {
		t.Fatalf("step 6: DerivedFlags = %#x, want 0 (still suppressed)", out.DerivedFlags)
	}

	// Step 7: suppression expired, flags flow again.
	out = Classify(CoreDelta{Tick: 7, CoreFlags: FlagCheckpointCrossed}, cls)
	if out.DerivedFlags&FlagSegmentCandidate == 0 {
		t.Fatal("step 7: FlagSegmentCandidate not set after suppression expired")
	}

	// Saturation test: StepsSince saturates at 0xFFFF.
	cls2 := &ClassifierState{StepsSince: [4]uint16{0xFFFE, 0xFFFE, 0xFFFE, 0xFFFE}}
	Classify(CoreDelta{Tick: 100}, cls2)
	for i, v := range cls2.StepsSince {
		if v != 0xFFFF {
			t.Fatalf("saturation: StepsSince[%d] = %d, want 0xFFFF", i, v)
		}
	}
	// One more step should stay at 0xFFFF.
	Classify(CoreDelta{Tick: 101}, cls2)
	for i, v := range cls2.StepsSince {
		if v != 0xFFFF {
			t.Fatalf("saturation overflow: StepsSince[%d] = %d, want 0xFFFF", i, v)
		}
	}

	// Nil classifier state: returns minimal delta, no panic.
	out = Classify(CoreDelta{Tick: 200}, nil)
	if out.Core.Tick != 200 {
		t.Fatal("nil cls: Core not preserved")
	}
	if out.DerivedFlags != 0 {
		t.Fatal("nil cls: DerivedFlags should be 0")
	}
}

// TestClassifyAllocationFree verifies Classify allocates zero heap memory.
func TestClassifyAllocationFree(t *testing.T) {
	cls := &ClassifierState{}
	core := CoreDelta{Tick: 1, CoreFlags: FlagStateChanged | FlagCheckpointCrossed}

	allocs := testing.AllocsPerRun(100, func() {
		Classify(core, cls)
	})
	if allocs != 0 {
		t.Fatalf("Classify made %.1f allocs/op, want 0", allocs)
	}
}

// TestClassifyPatternRepeat verifies PatternRepeat detection on consecutive same-state ticks.
func TestClassifyPatternRepeat(t *testing.T) {
	cls := &ClassifierState{}

	// First tick: state 42, no PatternRepeat (no prev).
	out := Classify(CoreDelta{Tick: 1, StateID: 42}, cls)
	if out.DerivedFlags&FlagPatternRepeat != 0 {
		t.Fatal("first tick: unexpected PatternRepeat")
	}

	// Second tick: same state 42 → PatternRepeat.
	out = Classify(CoreDelta{Tick: 2, StateID: 42}, cls)
	if out.DerivedFlags&FlagPatternRepeat == 0 {
		t.Fatal("second tick same state: expected PatternRepeat")
	}

	// Third tick: different state 43 → no PatternRepeat, NovelPattern.
	out = Classify(CoreDelta{Tick: 3, StateID: 43, CoreFlags: FlagStateChanged}, cls)
	if out.DerivedFlags&FlagPatternRepeat != 0 {
		t.Fatal("state change: unexpected PatternRepeat")
	}
	if out.DerivedFlags&FlagNovelPattern == 0 {
		t.Fatal("state change: expected NovelPattern")
	}
}

func BenchmarkClassify(b *testing.B) {
	cls := &ClassifierState{}
	core := CoreDelta{Tick: 1, StateID: 42, CoreFlags: FlagStateChanged | FlagCheckpointCrossed}
	for i := 0; i < b.N; i++ {
		core.Tick = uint64(i)
		Classify(core, cls)
	}
}
