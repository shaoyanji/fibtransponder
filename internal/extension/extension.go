package extension

import (
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// Output represents the displayable information from an extension.
type Output struct {
	Title string
	Lines []string
}

// Extension is an interface that all pluggable analysis modules should implement.
type Extension interface {
	// ProcessBit is called for each incoming bit, allowing the extension to update its internal state
	// and potentially react to FSVM events.
	// 'b' is the current bit, 'fsvmState' is the FSVM state *after* processing 'b',
	// 'zeroRunLength' is the current zero run length from fsvmState,
	// and 'events' are any events emitted by the FSVM for 'b'.
	ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event)

	// GetOutput returns the current displayable information from the extension.
	GetOutput() Output

	// GetTitle returns a short title for this extension.
	GetTitle() string
}
