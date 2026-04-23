package fsvm

// StepWithExtractor runs the per-bit Step and, if an Extractor is provided,
// produces a FeatureEvent for every transponder event (Marker or Dilate).
//
// This is the recommended integration point for callers that need rich
// features.  The extractor is pushed with the input bit *before* extraction,
// so the descriptor reflects the local context at the moment of the event.
func StepWithExtractor(s State, bit byte, ex *Extractor) (State, []FeatureEvent) {
	pre := s
	s, _ = Step(s, bit)
	if ex == nil {
		return s, nil
	}
	ex.Push(bit)

	var feats []FeatureEvent
	if s.Markers > pre.Markers {
		feats = append(feats, FeatureEvent{
			BitPos:    s.BitsProcessed,
			EventKind: EventMarker,
			Depth:     uint(s.R),
			Sketch:    s.Sketch,
			Desc:      ex.Extract(&s, EventMarker),
		})
	}
	if s.Dilations > pre.Dilations {
		feats = append(feats, FeatureEvent{
			BitPos:    s.BitsProcessed,
			EventKind: EventDilate,
			Depth:     uint(s.R),
			Sketch:    s.Sketch,
			Desc:      ex.Extract(&s, EventDilate),
		})
	}
	return s, feats
}

// StepWord64WithExtractor runs the fast word-level step and produces
// FeatureEvents for every event inside the word.
//
// The extractor is fed each bit in LSB-first order so that the descriptor
// at each event reflects the exact local window up to that point.
func StepWord64WithExtractor(s State, word uint64, ex *Extractor) (State, EventBatch, []FeatureEvent) {
	if ex == nil {
		s, batch := StepWord64(s, word)
		return s, batch, nil
	}

	// We need per-event extraction, so we walk the word bit-by-bit but
	// reuse the core Step logic for state updates.
	var feats []FeatureEvent
	var batch EventBatch

	for i := 0; i < 64; i++ {
		b := byte((word >> i) & 1)
		pre := s
		s, _ = Step(s, b)
		ex.Push(b)

		if s.Markers > pre.Markers {
			batch.MarkerCount++
			feats = append(feats, FeatureEvent{
				BitPos:    s.BitsProcessed,
				EventKind: EventMarker,
				Depth:     uint(s.R),
				Sketch:    s.Sketch,
				Desc:      ex.Extract(&s, EventMarker),
			})
		}
		if s.Dilations > pre.Dilations {
			batch.DilateCount++
			feats = append(feats, FeatureEvent{
				BitPos:    s.BitsProcessed,
				EventKind: EventDilate,
				Depth:     uint(s.R),
				Sketch:    s.Sketch,
				Desc:      ex.Extract(&s, EventDilate),
			})
		}
	}
	batch.FinalR = s.R
	return s, batch, feats
}

// StepV2WithExtractor is the sketch-v2 variant of StepWithExtractor.
func StepV2WithExtractor(s State, bit byte, ex *Extractor) (State, []FeatureEvent) {
	pre := s
	s, _ = StepV2(s, bit)
	if ex == nil {
		return s, nil
	}
	ex.Push(bit)

	var feats []FeatureEvent
	if s.Markers > pre.Markers {
		feats = append(feats, FeatureEvent{
			BitPos:    s.BitsProcessed,
			EventKind: EventMarker,
			Depth:     uint(s.R),
			Sketch:    s.Sketch,
			Desc:      ex.Extract(&s, EventMarker),
		})
	}
	if s.Dilations > pre.Dilations {
		feats = append(feats, FeatureEvent{
			BitPos:    s.BitsProcessed,
			EventKind: EventDilate,
			Depth:     uint(s.R),
			Sketch:    s.Sketch,
			Desc:      ex.Extract(&s, EventDilate),
		})
	}
	return s, feats
}

// StepWord64V2WithExtractor is the sketch-v2 variant of StepWord64WithExtractor.
func StepWord64V2WithExtractor(s State, word uint64, ex *Extractor) (State, EventBatch, []FeatureEvent) {
	if ex == nil {
		s, batch := StepWord64V2(s, word)
		return s, batch, nil
	}

	var feats []FeatureEvent
	var batch EventBatch

	for i := 0; i < 64; i++ {
		b := byte((word >> i) & 1)
		pre := s
		s, _ = StepV2(s, b)
		ex.Push(b)

		if s.Markers > pre.Markers {
			batch.MarkerCount++
			feats = append(feats, FeatureEvent{
				BitPos:    s.BitsProcessed,
				EventKind: EventMarker,
				Depth:     uint(s.R),
				Sketch:    s.Sketch,
				Desc:      ex.Extract(&s, EventMarker),
			})
		}
		if s.Dilations > pre.Dilations {
			batch.DilateCount++
			feats = append(feats, FeatureEvent{
				BitPos:    s.BitsProcessed,
				EventKind: EventDilate,
				Depth:     uint(s.R),
				Sketch:    s.Sketch,
				Desc:      ex.Extract(&s, EventDilate),
			})
		}
	}
	batch.FinalR = s.R
	return s, batch, feats
}
