package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shaoyanji/fibtransponder/internal/bitio"
	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
	hilbertgen "github.com/shaoyanji/fibtransponder/pkg/hilbertgen" // Use the library's ImageHeader
)

// ImageHeader structure for compressed image files (must match hilbert_gen)
type ImageHeader = hilbertgen.ImageHeader

// PixelMsg is sent when a new pixel's bit value is decoded.
type PixelMsg struct {
	X, Y     uint32
	BitValue byte // '0' or '1'
}

// Model holds the TUI application's state for rendering.
type model struct {
	filePath            string
	header              ImageHeader
	grid                [][]rune // 2D array of ASCII characters representing the image
	currentRenderedBits uint64   // How many bits have been processed and rendered
	maxRenderedBits     uint64   // Total bits to render (header.OriginalWidth * header.OriginalHeight)
	err                 error
	done                bool // True when decompression/rendering is complete
	quit                chan struct{}
	newPixel            chan PixelMsg // Channel to receive new pixel updates
	width               int           // Terminal width
	height              int           // Terminal height
}

// Init starts the background decompression process.
func (m model) Init() tea.Cmd {
	return tea.Batch(m.startDecompression, waitForPixel(m.newPixel))
}

// startDecompression runs in a goroutine to decode the compressed image data.
func (m *model) startDecompression() tea.Msg {
	// Open compressed file
	compressedFile, err := os.Open(m.filePath)
	if err != nil {
		return errMsg{fmt.Errorf("failed to open compressed file: %w", err)}
	}
	defer compressedFile.Close()

	// Read custom ImageHeader (OriginalWidth, OriginalHeight)
	if err := binary.Read(compressedFile, binary.BigEndian, &m.header.OriginalWidth); err != nil {
		return errMsg{fmt.Errorf("failed to read image width from header: %w", err)}
	}
	if err := binary.Read(compressedFile, binary.BigEndian, &m.header.OriginalHeight); err != nil {
		return errMsg{fmt.Errorf("failed to read image height from header: %w", err)}
	}

	// Initialize grid
	m.grid = make([][]rune, m.header.OriginalHeight)
	for y := uint32(0); y < m.header.OriginalHeight; y++ {
		m.grid[y] = make([]rune, m.header.OriginalWidth)
		for x := uint32(0); x < m.header.OriginalWidth; x++ {
			m.grid[y][x] = ' ' // Initialize with blank or blurred char
		}
	}

	// Set max bits to render
	m.maxRenderedBits = uint64(m.header.OriginalWidth * m.header.OriginalHeight)

	// Create a pipe for decoding bits
	pr, pw := io.Pipe()

	// Run fib_coder.Decode in a separate goroutine
	decodeDone := make(chan error, 1)
	go func() {
		defer pw.Close()
		_, err := fib_coder.Decode(compressedFile, pw) // Decode writes decompressed bits to pw
		decodeDone <- err
	}()

	// Create a BitReader from the pipe to read bits one by one
	bitReader := bitio.NewBitReader(pr)

	currentBitIndex := uint64(0)
	n := m.header.OriginalWidth // N for Hilbert functions

	for {
		select {
		case <-m.quit:
			pr.Close() // Close reader to stop decoder goroutine if blocked on write
			return nil
		default:
			if currentBitIndex >= m.maxRenderedBits {
				goto decodeLoopEnd // Finished reading all original bits
			}

			bit, err := bitReader.ReadBit()
			if err != nil {
				if err == io.EOF {
					goto decodeLoopEnd // Reached end of decoded stream
				}
				return errMsg{fmt.Errorf("error reading decompressed bit: %w", err)}
			}

			x, y := image_hilbert.D2XY(n, uint32(currentBitIndex)) // Map 1D index to 2D Hilbert coordinates
			m.newPixel <- PixelMsg{X: x, Y: y, BitValue: bit}
			currentBitIndex++
		}
	}

decodeLoopEnd:
	pr.Close()         // Close pipe reader
	err = <-decodeDone // Wait for fib_coder.Decode to finish and get its error
	if err != nil && err != io.EOF {
		return errMsg{fmt.Errorf("fib_coder.Decode error: %w", err)}
	}
	return decompressionDoneMsg{} // Signal that decompression is complete
}

// waitForPixel is a command that waits for a new pixel to be available.
func waitForPixel(pixelChan chan PixelMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-pixelChan:
			return msg
		case <-time.After(time.Millisecond * 10): // Small delay to prevent busy-looping if channel is empty
			return nil // No pixel for now, check again later
		}
	}
}

// Update handles messages and updates the model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			close(m.quit) // Signal goroutine to quit
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case PixelMsg:
		// Update the grid with the new pixel
		if msg.Y < m.header.OriginalHeight && msg.X < m.header.OriginalWidth {
			m.grid[msg.Y][msg.X] = pixelToAscii(msg.BitValue)
		}
		m.currentRenderedBits++
		cmd = waitForPixel(m.newPixel) // Wait for the next pixel
	case decompressionDoneMsg:
		m.done = true
		cmd = tea.Quit // Quit after decompression is done and displayed
	case errMsg:
		m.err = msg.error
		m.done = true
		cmd = tea.Quit // Quit on error
	}
	return m, cmd
}

// View renders the model's current state.
func (m model) View() string {
	s := strings.Builder{}

	if m.err != nil {
		s.WriteString(fmt.Sprintf("Error: %s\n", m.err))
		s.WriteString("Press 'q' to quit.")
		return s.String()
	}

	s.WriteString(fmt.Sprintf("Rendering: %s\n", filepath.Base(m.filePath)))
	s.WriteString(fmt.Sprintf("Dimensions: %dx%d, Bits: %d / %d\n", m.header.OriginalWidth, m.header.OriginalHeight, m.currentRenderedBits, m.maxRenderedBits))

	// Calculate scaling factor to fit image into terminal, if needed
	scaleX := float64(m.width) / float64(m.header.OriginalWidth)
	scaleY := float64(m.height-4) / float64(m.header.OriginalHeight) // -4 for header/footer text
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}
	if scale < 1.0 {
		// Only scale down
		s.WriteString(fmt.Sprintf("Terminal too small, image scaled by %.2f\n", scale))
	} else {
		scale = 1.0 // Don't scale up
	}

	for y := uint32(0); y < m.header.OriginalHeight; y++ {
		if float64(y)*scale >= float64(m.height-4) {
			continue
		} // Don't render if outside scaled bounds
		for x := uint32(0); x < m.header.OriginalWidth; x++ {
			if float64(x)*scale >= float64(m.width) {
				continue
			} // Don't render if outside scaled bounds

			// Simple scaling: just pick the top-left pixel of the scaled block
			displayChar := m.grid[y][x]
			// We can implement more sophisticated scaling/averaging here if needed

			s.WriteRune(displayChar)
		}
		s.WriteRune('\n')
	}

	if m.done {
		s.WriteString("\nDecompression complete! Press 'q' to quit.\n")
	} else {
		s.WriteString(fmt.Sprintf("\nProgress: %.2f%%\n", float64(m.currentRenderedBits)/float64(m.maxRenderedBits)*100))
	}
	s.WriteString("Press 'q' or 'ctrl+c' to quit.\n")

	return s.String()
}

// pixelToAscii converts a '0' or '1' bit value to an ASCII character.
func pixelToAscii(bit byte) rune {
	if bit == '1' {
		return '#'
	}
	return ' ' // '0'
}

type decompressionDoneMsg struct{}
type errMsg struct{ error }

func main() {
	if err := runHilbertRenderMain(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runHilbertRenderMain is an exportable function for testing or programmatic use.
func runHilbertRenderMain(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s <compressed_fibimg_file>", filepath.Base(args[0]))
	}
	filePath := args[1]

	// Initial model without full header, it will be read in goroutine
	m := model{
		filePath: filePath,
		newPixel: make(chan PixelMsg),
		quit:     make(chan struct{}),
		width:    80, // Default terminal width
		height:   24, // Default terminal height
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("bubbletea program failed: %w", err)
	}
	return nil
}
