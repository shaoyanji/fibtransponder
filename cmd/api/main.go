package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	session "github.com/shaoyanji/fibtransponder/internal/session" // Import new session package
)

var (
	sessions = make(map[string]*session.SessionState) // Use session.SessionState
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
	s := session.NewSession(sessionID) // Use session.NewSession
	sessions[sessionID] = s
	sessionsMu.Unlock()

	if req.InitialBits != "" {
		if err := s.ProcessBits(req.InitialBits); err != nil { // Use s.ProcessBits
			http.Error(w, fmt.Sprintf("failed to process initial bits: %v", err), http.StatusBadRequest)
			sessionsMu.Lock()
			delete(sessions, sessionID) // Clean up session on error
			sessionsMu.Unlock()
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s) // Encode the session.SessionState
}

// processBitsHandler handles POST /api/sessions/{session_id}/process
func processBitsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	sessionsMu.RLock()
	s, found := sessions[sessionID]
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

	if err := s.ProcessBits(req.Bits); err != nil { // Use s.ProcessBits
		http.Error(w, fmt.Sprintf("failed to process bits: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s) // Encode the session.SessionState
}

// getSessionHandler handles GET /api/sessions/{session_id}
func getSessionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	sessionsMu.RLock()
	s, found := sessions[sessionID]
	sessionsMu.RUnlock()

	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s) // Encode the session.SessionState
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/api/sessions", createSessionHandler).Methods("POST")
	router.HandleFunc("/api/sessions/{session_id}/process", processBitsHandler).Methods("POST")
	router.HandleFunc("/api/sessions/{session_id}", getSessionHandler).Methods("GET")

	log.Println("FibTransponder API Service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
