package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/shaoyanji/fibtransponder/internal/bitrope"
	"github.com/shaoyanji/fibtransponder/internal/entropy_estimator"
	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/rosetta"
	"github.com/shaoyanji/fibtransponder/internal/segauto"
	"github.com/shaoyanji/fibtransponder/internal/signal"
	"github.com/shaoyanji/fibtransponder/internal/typing_analyzer"
)

// SessionState holds the entire state for one state machine instance
// that can be exposed via the API.
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

	// Initialize outputs
	s.updateExtensionOutputs(0, fsvm.New(), 0, []fsvm.Event{})

	return s
}

// updateExtensionOutputs collects outputs from all extensions.
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

var (
	sessions = make(map[string]*SessionState)
	sessionsMu sync.RWMutex
)

// Request body for creating a session
type CreateSessionRequest struct {
	InitialBits string `json:"initial_bits"`
}

// Request body for processing bits
type ProcessBitsRequest struct {
	Bits string `json:"bits"`
}

// createSessionHandler handles POST /api/sessions
func createSessionHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != http.ErrMissingContentLength {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionID := uuid.New().String()
	sessionsMu.Lock()
	session := NewSession(sessionID)
	sessions[sessionID] = session
	sessionsMu.Unlock()

	if req.InitialBits != "" {
		if err := session.ProcessBits(req.InitialBits); err != nil {
			http.Error(w, fmt.Sprintf("failed to process initial bits: %v", err), http.StatusBadRequest)
			sessionsMu.Lock()
			delete(sessions, sessionID) // Clean up session on error
			sessionsMu.Unlock()
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// processBitsHandler handles POST /api/sessions/{session_id}/process
func processBitsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	sessionsMu.RLock()
	session, found := sessions[sessionID]
	sessionsMu.RUnlock()

	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req ProcessBitsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := session.ProcessBits(req.Bits); err != nil {
		http.Error(w, fmt.Sprintf("failed to process bits: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// getSessionHandler handles GET /api/sessions/{session_id}
func getSessionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	sessionsMu.RLock()
	session, found := sessions[sessionID]
	sessionsMu.RUnlock()

	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/api/sessions", createSessionHandler).Methods("POST")
	router.HandleFunc("/api/sessions/{session_id}/process", processBitsHandler).Methods("POST")
	router.HandleFunc("/api/sessions/{session_id}", getSessionHandler).Methods("GET")

	log.Println("FibTransponder API Service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
