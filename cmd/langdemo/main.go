// langdemo: Language Sensing Demo for fibtransponder
//
// This demo prototypes a token-free language sensor by feeding UTF-8 text
// into an array of differently-calibrated FSVMs and outputting event streams.
//
// Usage: go run ./cmd/langdemo/main.go [input_file]
// If no input file is provided, uses a built-in sample text.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/shaoyanji/fibtransponder/internal/calibration"
)

// TransponderProfile defines a transponder's calibration
type TransponderProfile struct {
	ID       string
	Width    uint8
	ZeroThresh uint64
	Desc     string
}

// EventOutput represents a single event in the output stream
type EventOutput struct {
	ByteOffset  int    `json:"ts"`
	Transponder string `json:"transponder"`
	Event       string `json:"event"`
	Payload     uint64 `json:"payload,omitempty"`
}

func main() {
	var text string
	var err error

	if len(os.Args) > 1 {
		// Read from file
		text, err = readFile(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Use built-in sample
		text = getSampleText()
	}

	fmt.Fprintf(os.Stderr, "=== Fibtransponder Language Sensing Demo ===\n")
	fmt.Fprintf(os.Stderr, "Input: %d bytes, %d runes\n\n", len(text), len([]rune(text)))

	// Define transponder array with different profiles
	profiles := []TransponderProfile{
		{"A", 3, 4, "vowel-like (narrow width)"},
		{"B", 5, 8, "consonant-like (medium width)"},
		{"C", 8, 16, "prosody (wide width)"},
	}

	// Create transponder array
	arr := calibration.NewAdaptiveArray(
		makeProfileNames(profiles),
		calibration.DefaultTargets(),
	)

	// Configure each transponder
	for i, p := range profiles {
		arr.Transponders[i].Params.Width = p.Width
		arr.Transponders[i].Params.ZeroThresh = p.ZeroThresh
		arr.Transponders[i].Name = p.ID
	}

	// Convert text to bitstream and process
	events := processText(arr, text, profiles)

	// Output events as JSON lines
	fmt.Fprintf(os.Stderr, "\n--- Event Stream (%d events) ---\n", len(events))
	for _, ev := range events {
		jsonBytes, _ := json.Marshal(ev)
		fmt.Println(string(jsonBytes))
	}

	// Compute and display statistics
	displayStatistics(events, text, profiles)
}

func makeProfileNames(profiles []TransponderProfile) []string {
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.ID
	}
	return names
}

func processText(arr *calibration.AdaptiveArray, text string, profiles []TransponderProfile) []EventOutput {
	var events []EventOutput
	byteOffset := 0

	// Process each rune's UTF-8 encoding
	for _, r := range text {
		utf8Bytes := []byte(string(r))
		
		// Feed each bit of each byte to all transponders
		for _, b := range utf8Bytes {
			for bitIdx := 7; bitIdx >= 0; bitIdx-- {
				bit := (b >> bitIdx) & 1
				
				// Step all transponders
				results := arr.Step(bit)
				
				// Check for DILATE events from each transponder
				for i, res := range results {
					if res.Dilations > 0 {
						// Only emit if this is a new dilation
						events = append(events, EventOutput{
							ByteOffset:  byteOffset,
							Transponder: profiles[i].ID,
							Event:       "DILATE",
							Payload:     uint64(res.R),
						})
					}
				}
			}
			byteOffset++
		}
	}

	return events
}

func displayStatistics(events []EventOutput, text string, profiles []TransponderProfile) {
	fmt.Fprintf(os.Stderr, "\n--- Statistics ---\n")

	// Count events per transponder
	eventCounts := make(map[string]int)
	for _, ev := range events {
		eventCounts[ev.Transponder]++
	}

	fmt.Fprintf(os.Stderr, "Events per transponder:\n")
	for _, p := range profiles {
		count := eventCounts[p.ID]
		fmt.Fprintf(os.Stderr, "  %s (%s): %d events\n", p.ID, p.Desc, count)
	}

	// Word count
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	wordCount := len(words)

	// Compression ratio (events per word)
	totalEvents := len(events)
	var eventsPerWord float64
	if wordCount > 0 {
		eventsPerWord = float64(totalEvents) / float64(wordCount)
	}

	fmt.Fprintf(os.Stderr, "\nWord count: %d\n", wordCount)
	fmt.Fprintf(os.Stderr, "Total events: %d\n", totalEvents)
	fmt.Fprintf(os.Stderr, "Events per word: %.2f\n", eventsPerWord)
	fmt.Fprintf(os.Stderr, "\nNote: Traditional BPE tokenizers typically produce 1.0-1.5 tokens per word.\n")
	fmt.Fprintf(os.Stderr, "This event-driven approach produces a different granularity based on structural patterns.\n")
}

func readFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func getSampleText() string {
	return `Hello, 世界！The quick brown fox jumps over the lazy dog.
Language sensing through structural calibration reveals patterns invisible to traditional tokenizers.
fibtransponder treats text as a bitstream, detecting adjacency patterns and zero-runs.
Different transponder configurations sense different linguistic features.`
}
