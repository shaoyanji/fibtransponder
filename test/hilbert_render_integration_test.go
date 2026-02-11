package test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/bitio"
	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
	render_cmd "github.com/shaoyanji/fibtransponder/cmd/hilbert_render" // Import with alias
	hilbert_gen_cmd "github.com/shaoyanji/fibtransponder/cmd/hilbert_gen" // Import with alias
)

func TestHilbertRenderFullPipeline(t *testing.T) {
	// 1. Create a dummy image for compression
	pngPath := CreateCheckerboardImage(t, "checkerboard.png", true)
	defer os.Remove(pngPath) // Clean up temp file

	// Define parameters
	const hilbertOrder = 3 // 2^3 = 8x8 image
	const binarizationThreshold = 128
	const imageSize = 1 << hilbertOrder // 8

	// 2. Simulate cmd/hilbert_gen to create a compressed .fibimg file
	compressedFibimgFilePath := filepath.Join(t.TempDir(), "compressed.fibimg")

	// Call hilbert_gen's main logic directly, but capturing output
	// To do this programmatically, we need a wrapper around hilbert_gen's main func.
	// For testing, we can extract the logic from hilbert_gen's main.
	// Let's create a wrapper function in hilbert_gen for testing.
	
	// Temporarily redirect os.Args for hilbert_gen
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"hilbert_gen", "-i", pngPath, "-o", compressedFibimgFilePath, "-order", fmt.Sprintf("%d", hilbertOrder), "-threshold", fmt.Sprintf("%d", binarizationThreshold)}

	// Capture stdout/stderr of hilbert_gen if needed, but it prints success message.
	// For this test, we just want it to create the file.
	
	// Re-run hilbert_gen main logic
	if err := runHilbertGenMain(); err != nil { // This is where we need to call hilbert_gen's main logic
		t.Fatalf("hilbert_gen failed: %v", err)
	}

	// 3. Read the compressed .fibimg file to simulate hilbert_render's decompression
	compressedFile, err := os.Open(compressedFibimgFilePath)
	if err != nil {
		t.Fatalf("Failed to open compressed fibimg file: %v", err)
	}
	defer compressedFile.Close()

	// Read custom ImageHeader (OriginalWidth, OriginalHeight)
	var originalWidth, originalHeight uint32
	if err := binary.Read(compressedFile, binary.BigEndian, &originalWidth); err != nil {
		t.Fatalf("Failed to read image width from header: %v", err)
	}
	if err := binary.Read(compressedFile, binary.BigEndian, &originalHeight); err != nil {
		t.Fatalf("Failed to read image height from header: %v", err)
	}

	// Verify dimensions
	if originalWidth != imageSize || originalHeight != imageSize {
		t.Errorf("Header dimensions mismatch. Got %dx%d, want %dx%d", originalWidth, originalHeight, imageSize, imageSize)
	}

	// Call fib_coder.Decode and capture output
	var decompressedBuf bytes.Buffer
	originalLenInBits, err := fib_coder.Decode(compressedFile, &decompressedBuf)
	if err != nil {
		t.Fatalf("fib_coder.Decode failed: %v", err)
	}

	// 4. Verify the decompressed bitstream against the expected bitstream
	expectedBitstream := GenerateExpectedBitstream(t, pngPath, hilbertOrder, binarizationThreshold)
	decompressedBitstream := decompressedBuf.String() // BitStringToBytes writes '0's and '1's

	if uint64(len(decompressedBitstream)) != originalLenInBits {
		t.Errorf("Decompressed bitstream length mismatch. Got %d, want %d", len(decompressedBitstream), originalLenInBits)
	}
	if decompressedBitstream != expectedBitstream {
		t.Errorf("Decompressed bitstream does not match original expected bitstream.\nGot:  %s\nWant: %s", decompressedBitstream, expectedBitstream)
	}

	// 5. Simulate rendering (optional, for visual confirmation, but hard to automate)
	// We've already verified the bitstream is correct, which is the core of lossless.
	// Progressive rendering is a UI aspect.

	t.Logf("Successfully processed and verified image. Dimensions: %dx%d, Original Bits: %d", originalWidth, originalHeight, originalLenInBits)
}

// runHilbertGenMain is a helper to run the main logic of cmd/hilbert_gen programmatically.
// This requires hilbert_gen to expose its main logic in a testable function.
// Temporarily, this will be a placeholder and will need modification to hilbert_gen.
func runHilbertGenMain() error {
	// To avoid calling os.Exit(1) directly in main(), we need to wrap its logic.
	// For now, this is a simplified stub.
	// In a real scenario, cmd/hilbert_gen should expose a func(args []string) error.
	
	// Create temporary files to simulate stdin/stdout if hilbert_gen were to use them
	// For now, hilbert_gen uses files directly.
	
	// Execute the main logic of hilbert_gen
	// This can be done by copying the relevant parts of hilbert_gen/main.go here
	// or by refactoring hilbert_gen to export a testable function.
	
	// Let's refactor hilbert_gen/main.go to export a testable function.
	// This will be done in the next step.
	// For now, return a placeholder error.
	return fmt.Errorf("hilbert_gen logic needs to be refactored into an exportable function for testing")
}
