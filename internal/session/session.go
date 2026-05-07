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
	s.updateExtensionOutputs(0, fsvm.New(), 0, []fsvm.Event{})

	return s
}

// updateExtensionOutputs collects outputs from all extensions.
// It also ensures that all extensions have their ProcessBit method called,
// typically after the core FSVM has processed a bit.
func (s *SessionState) updateExtensionOutputs(b uint8, fsvmState fsvm.State, zeroRunLength uint64, fsvmEvents []fsvm.Event) {
	s.ExtensionOutputs = make([]extension.Output, len(s.extensions))
	for i, ext := range s.extensions {
		ext.ProcessBit(b, fsvmState, zeroRunLength, fsvmEvents) // Ensure all extensions are updated
		s.ExtensionOutputs[i] = ext.GetOutput()
	}
}

// ProcessBits takes a string of '0' and '1' and updates the session state.
func (s *SessionState) ProcessBits(bits string) error {
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

		var fsvmEvents []fsvm.Event
		s.FSVMState, fsvmEvents = fsvm.Step(s.FSVMState, b)

		// Let all extensions process the bit and FSVM events
		// and then collect their outputs
		s.updateExtensionOutputs(b, s.FSVMState, s.FSVMState.ZeroRun, fsvmEvents)
	}
	return nil
}
