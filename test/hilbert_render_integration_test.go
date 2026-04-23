//go:build legacy
// +build legacy

package test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
	hilbert_gen_cmd "github.com/shaoyanji/fibtransponder/pkg/hilbertgen"
)

func runHilbertGenMain(t *testing.T, args []string) error {
	// Temporarily redirect os.Stderr to capture any output/errors from hilbert_gen
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Execute the Run function from hilbert_gen's package
	err := hilbert_gen_cmd.Run(args)

	w.Close()
	os.Stderr = oldStderr // Restore stderr

	var stderrOutput bytes.Buffer
	io.Copy(&stderrOutput, r)
	if stderrOutput.Len() > 0 {
		t.Logf("hilbert_gen stderr: %s", stderrOutput.String())
	}
	return err
}

func TestHilbertRenderFullPipeline(t *testing.T) {
	// 1. Create a dummy image for compression
	pngPath := CreateCheckerboardImage(t, "checkerboard.png", true)
	defer os.Remove(pngPath) // Clean up temp file

	// Define parameters
	const hilbertOrder = 3 // 2^3 = 8x8 image
	const binarizationThreshold = 128
	const imageDim = 1 << hilbertOrder // 8 (width and height)

	// 2. Simulate cmd/hilbert_gen to create a compressed .fibimg file
	compressedFibimgFilePath := filepath.Join(t.TempDir(), "compressed.fibimg")

	// Prepare arguments for hilbert_gen_cmd.Run
	genArgs := []string{
		"hilbert_gen", // App name (ignored by Run, but good practice)
		"-i", pngPath,
		"-o", compressedFibimgFilePath,
		"-order", fmt.Sprintf("%d", hilbertOrder),
		"-threshold", fmt.Sprintf("%d", binarizationThreshold),
	}
	if err := runHilbertGenMain(t, genArgs); err != nil {
		t.Fatalf("hilbert_gen failed: %v", err)
	}
	defer os.Remove(compressedFibimgFilePath) // Clean up compressed file

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
	if originalWidth != imageDim || originalHeight != imageDim {
		t.Errorf("Header dimensions mismatch. Got %dx%d, want %dx%d", originalWidth, originalHeight, imageDim, imageDim)
	}

	// Call fib_coder.Decode and capture output
	// The fib_coder.Decode will read its own 8-byte OriginalBitLen header.
	// We pass a bytes.Buffer as the output to capture the decompressed bitstream.
	var decompressedBitBuf bytes.Buffer
	decodedOriginalBitLen, err := fib_coder.Decode(compressedFile, &decompressedBitBuf)
	if err != nil {
		t.Fatalf("fib_coder.Decode failed: %v", err)
	}

	decompressedBitstream := decompressedBitBuf.String() // BitStringToBytes writes '0's and '1's

	// 4. Verify the decompressed bitstream against the expected bitstream
	expectedBitstream, err := image_hilbert.GenerateBitstream(pngPath, hilbertOrder, binarizationThreshold)
	if err != nil {
		t.Fatalf("GenerateExpectedBitstream failed: %v", err)
	}

	if decodedOriginalBitLen != uint64(len(expectedBitstream)) {
		t.Errorf("Decoded Original Bit Length mismatch. Got %d, want %d", decodedOriginalBitLen, len(expectedBitstream))
	}

	if decompressedBitstream != expectedBitstream {
		// Log detailed difference if failure
		t.Errorf("Decompressed bitstream does not match original expected bitstream.\nGot len: %d\nWant len: %d\nGot:  %s\nWant: %s\nDiff at: %s",
			len(decompressedBitstream), len(expectedBitstream), decompressedBitstream, expectedBitstream, findDiffIndex(decompressedBitstream, expectedBitstream))
	}

	t.Logf("Successfully processed and verified image. Dimensions: %dx%d, Original Bits: %d", originalWidth, originalHeight, decodedOriginalBitLen)
}

// findDiffIndex finds the first index where two strings differ.
func findDiffIndex(s1, s2 string) string {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}
	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return fmt.Sprintf("index %d (s1[%c] != s2[%c])", i, s1[i], s2[i])
		}
	}
	if len(s1) != len(s2) {
		return fmt.Sprintf("length difference (s1 len %d, s2 len %d)", len(s1), len(s2))
	}
	return "no difference"
}

// Ensure createCheckerboardImage is accessible by this package. It should be in test_utils.go.
// (Not part of this file, just a note for context.)
// GenerateExpectedBitstream is also in test_utils.go.
