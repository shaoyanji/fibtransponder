package render

// Renderer is intentionally budgeted and returns summaries + exemplars.
// It should never force enumeration of all segmentations/hypotheses.

type Summary struct {
	R         uint32
	Dilations uint64
	ZeroRun   uint64
	LenBits   uint64
}

type Exemplar struct {
	Name  string
	Bits  string // short representation, truncated
	Notes string
}

// Render is a placeholder for a bounded-budget rendering pass.
func Render(sum Summary, budgetMicros int64) (Summary, []Exemplar) {
	// TODO: implement exemplar extraction from segmentation automaton.
	return sum, nil
}
