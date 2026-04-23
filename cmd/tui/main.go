package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"

	"github.com/shaoyanji/fibtransponder/internal/bitrope"
	"github.com/shaoyanji/fibtransponder/internal/entropy_estimator"
	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/image_analyzer" // New import
	"github.com/shaoyanji/fibtransponder/internal/rosetta"
	"github.com/shaoyanji/fibtransponder/internal/segauto"
	"github.com/shaoyanji/fibtransponder/internal/signal"
	"github.com/shaoyanji/fibtransponder/internal/typing_analyzer"
)

// Model holds the application's state.
type model struct {
	fsvmState     fsvm.State
	bitRope       *bitrope.Rope
	inputReader   *bufio.Reader
	log           []string // Event log for displaying general events (e.g., DILATE)
	processedBits uint64
	quitting      bool // Flag to indicate if the app is quitting

	extensions []extension.Extension
}

// Msg to process a new bit.
type bitMsg byte

// Msg to update the UI periodically.
type tickMsg time.Time

// Init initializes the model.
func (m model) Init() tea.Cmd {
	return tea.Batch(readStdin(m.inputReader), tick())
}

// Update handles messages and updates the model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case bitMsg:
		b := byte(msg)

		m.processedBits++
		m.bitRope.AppendBit(b)

		var fsvmEvents []fsvm.Event
		m.fsvmState, fsvmEvents = fsvm.Step(m.fsvmState, b)

		// Process general FSVM events for the main log
		for _, ev := range fsvmEvents {
			if ev.Kind == fsvm.EventDilate {
				m.log = append(m.log, fmt.Sprintf("i=%d DILATE r=%d", m.processedBits, m.fsvmState.R))
			}
		}
		// Keep log size manageable
		if len(m.log) > 20 { // Limit log to last 20 entries
			m.log = m.log[len(m.log)-20:]
		}

		// Let all extensions process the bit and FSVM events
		for _, ext := range m.extensions {
			ext.ProcessBit(b, m.fsvmState, m.fsvmState.ZeroRun, fsvmEvents)
		}

		return m, readStdin(m.inputReader) // Keep reading
	case tickMsg:
		// No state change needed here, just a trigger to redraw the view
		return m, tick() // Schedule next tick
	}
	return m, nil
}

// View renders the model's current state.
func (m model) View() string {
	s := strings.Builder{}

	// Header
	s.WriteString(fmt.Sprintf("FibTransponder TUI | Processed Bits: %d\n", m.processedBits))
	s.WriteString(fmt.Sprintf("r: %d | Dilations: %d | Markers: %d | ZeroRun: %d | W: %06b\n",
		m.fsvmState.R, m.fsvmState.Dilations, m.fsvmState.Markers, m.fsvmState.ZeroRun, m.fsvmState.W))
	s.WriteString("--------------------------------------------------\n")

	// Main Event Log
	s.WriteString("Events:\n")
	for _, entry := range m.log {
		s.WriteString(entry + "\n")
	}
	if len(m.log) == 0 {
		s.WriteString("No FSVM Dilate events yet...\n")
	}
	s.WriteString("--------------------------------------------------\n")

	// Render output from all extensions
	for _, ext := range m.extensions {
		output := ext.GetOutput()
		s.WriteString(fmt.Sprintf("%s:\n", output.Title))
		for _, line := range output.Lines {
			s.WriteString(line + "\n")
		}
		s.WriteString("--------------------------------------------------\n")
	}

	// Instructions
	s.WriteString("Press 'q' or 'ctrl+c' to quit.\n")

	if m.quitting {
		s.WriteString("Quitting...")
	}

	return s.String()
}

// Command to read a single bit from stdin.
func readStdin(reader *bufio.Reader) tea.Cmd {
	return func() tea.Msg {
		for { // Loop to ignore non '0' or '1' characters
			c, err := reader.ReadByte()
			if err != nil {
				time.Sleep(time.Millisecond * 50) // Give UI a moment to show "Quitting..."
				return tea.Quit
			}
			switch c {
			case '0':
				return bitMsg(0)
			case '1':
				return bitMsg(1)
			case '\n', '\r', '\t', ' ':
				// Ignore whitespace and continue reading
				continue
			default:
				// Ignore other characters and continue reading
				continue
			}
		}
	}
}

// Command for a periodic tick.
func tick() tea.Cmd {
	return tea.Tick(time.Second/4, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func initialModel() model {
	// Initialize core FSVM and bitrope
	initialFSVMState := fsvm.New()

	// Initialize all extensions
	var extensions []extension.Extension
	
	segAutoExt := segauto.New() 
	extensions = append(extensions, segAutoExt)

	extensions = append(extensions, rosetta.New())
	extensions = append(extensions, signal.NewFeatureExtractor())
	extensions = append(extensions, typing_analyzer.NewAnalyzer())
	extensions = append(extensions, entropy_estimator.NewEstimator())
	extensions = append(extensions, image_analyzer.NewAnalyzer()) // New extension

	return model{
		fsvmState:   initialFSVMState,
		bitRope:     bitrope.New(1 << 16), // Default block size
		inputReader: bufio.NewReader(os.Stdin),
		log:         make([]string, 0),
		extensions:  extensions,
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
