package session

import (
	"github.com/shaoyanji/fibtransponder/internal/bitrope"
	"github.com/shaoyanji/fibtransponder/internal/entropy_estimator"
	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/image_analyzer"
	"github.com/shaoyanji/fibtransponder/internal/rosetta"
	"github.com/shaoyanji/fibtransponder/internal/segauto"
	"github.com/shaoyanji/fibtransponder/internal/signal"
	"github.com/shaoyanji/fibtransponder/internal/typing_analyzer"
	"github.com/shaoyanji/fibtransponder/internal/zeck_residual_ext"
)

// SessionState holds the entire state for one state machine instance.
// It encapsulates the core FSVM state and all pluggable extensions.
type SessionState struct {
	SessionID     string       `json:"sessionId"`
	FSVMState     fsvm.State   `json:"fsvmState"`
	ProcessedBits uint64       `json:"processedBits"`
	BitRope       *bitrope.Rope `json:"-"` // Not exposing raw bitrope for performance/size

	extensions       []extension.Extension `json:"-"` // Internal list of extension interfaces
	ExtensionOutputs []extension.Output   `json:"extensionOutputs"` // Public field for JSON serialization of extension outputs
}

// NewSession creates and initializes a new SessionState.
func NewSession(sessionID string) *SessionState {
	s := &SessionState{
		SessionID: sessionID,
		FSVMState: fsvm.New(),
		BitRope:   bitrope.New(1 << 16), // Default block size
	}

	// Initialize all extensions
	// Order might matter if extensions depend on each other's processing order.
	// segauto should typically process before others that might react to markers.
	segAutoExt := segauto.New()
	s.extensions = append(s.extensions, segAutoExt)
	s.extensions = append(s.extensions, rosetta.New())
	s.extensions = append(s.extensions, signal.NewFeatureExtractor())
	s.extensions = append(s.extensions, typing_analyzer.NewAnalyzer())
	s.extensions = append(s.extensions, entropy_estimator.NewEstimator())
	s.extensions = append(s.extensions, image_analyzer.NewAnalyzer())
	s.extensions = append(s.extensions, zkrext.NewEstimator())

	// Initialize outputs
	// Call ProcessBit on a dummy state to get initial outputs for all extensions
	s.stepExtensions(0, fsvm.New(), 0, []fsvm.Event{})
	s.RefreshOutputs()

	return s
}

// stepExtensions advances every extension's internal state by one input bit.
//
// This is the per-bit hot path: it must not render display strings. Calling
// GetOutput() here would rebuild every extension's human-readable output
// (each one a handful of fmt.Sprintf calls, several allocating []string) on
// every single input bit, which dominated ingest cost. Use RefreshOutputs to
// materialize the display layer.
func (s *SessionState) stepExtensions(b uint8, fsvmState fsvm.State, zeroRunLength uint64, fsvmEvents []fsvm.Event) {
	for _, ext := range s.extensions {
		ext.ProcessBit(b, fsvmState, zeroRunLength, fsvmEvents)
	}
}

// RefreshOutputs rebuilds the human-readable ExtensionOutputs from the current
// extension state. Call this after a batch of ProcessBits, or whenever the
// display layer is actually read -- not once per input bit.
func (s *SessionState) RefreshOutputs() {
	if cap(s.ExtensionOutputs) < len(s.extensions) {
		s.ExtensionOutputs = make([]extension.Output, len(s.extensions))
	} else {
		s.ExtensionOutputs = s.ExtensionOutputs[:len(s.extensions)]
	}
	for i, ext := range s.extensions {
		s.ExtensionOutputs[i] = ext.GetOutput()
	}
}

// ProcessBits takes a string of '0' and '1' and updates the session state.
//
// This drives the per-bit ingest path. Display outputs are NOT refreshed here;
// call RefreshOutputs afterwards.
//
// fsvm.Step allocates its own event slice per call (declared inside Step), so
// there is no caller-side slice to reuse across iterations.
func (s *SessionState) ProcessBits(bits string) error {
	var fsvmEvents []fsvm.Event
	for _, r := range bits {
		b := uint8(0)
		switch r {
		case '0':
			b = 0
		case '1':
			b = 1
		default:
			// Ignore non '0' or '1' characters, similar to TUI
			continue
		}

		s.ProcessedBits++
		s.BitRope.AppendBit(b)

		s.FSVMState, fsvmEvents = fsvm.Step(s.FSVMState, b)

		// Let all extensions process the bit and FSVM events
		s.stepExtensions(b, s.FSVMState, s.FSVMState.ZeroRun, fsvmEvents)
	}
	return nil
}
