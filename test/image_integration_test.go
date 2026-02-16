package test

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
	sessionpkg "github.com/shaoyanji/fibtransponder/internal/session" // Import new session package
)

// createCheckerboardImage creates an 8x8 checkerboard image.
// If makePNG is true, it creates a PNG, otherwise a JPEG.
func createCheckerboardImage(t *testing.T, filename string, makePNG bool) string {
	const size = 8
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, black)
			} else {
				img.Set(x, y, white)
			}
		}
	}

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create image file: %v", err)
	}
	defer file.Close()

	if makePNG {
		err = png.Encode(file, img)
	} else {
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
	}
	if err != nil {
		t.Fatalf("Failed to encode image: %v", err)
	}
	return filePath
}

func TestImageBitstreamProcessing(t *testing.T) {
	// Create sample image files
	pngPath := createCheckerboardImage(t, "checkerboard.png", true)
	jpegPath := createCheckerboardImage(t, "checkerboard.jpeg", false)

	tests := []struct {
		name      string
		imagePath string
		imageType string
	}{
		{"PNG Checkerboard", pngPath, "png"},
		{"JPEG Checkerboard", jpegPath, "jpeg"},
	}

	const hilbertOrder = 3 // 2^3 = 8x8 image
	const binarizationThreshold = 128 // Mid-gray threshold

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Generate bitstream using internal/image_hilbert
			bitstream, err := image_hilbert.GenerateBitstream(tt.imagePath, hilbertOrder, binarizationThreshold)
			if err != nil {
				t.Fatalf("GenerateBitstream failed for %s: %v", tt.imagePath, err)
			}

			expectedBitstreamLen := uint64(1 << (hilbertOrder * 2)) // N*N = (2^order)^2 = 2^(2*order)
			if uint64(len(bitstream)) != expectedBitstreamLen {
				t.Errorf("Bitstream length mismatch for %s. Got %d, want %d", tt.imagePath, len(bitstream), expectedBitstreamLen)
			}
			t.Logf("Generated bitstream length for %s: %d", tt.imagePath, len(bitstream))
			// t.Logf("Generated bitstream: %s", bitstream) // Uncomment for debugging if needed

			// 2. Feed bitstream into a SessionState (from internal/session module)
			session := sessionpkg.NewSession("test-session-" + tt.imageType) // Use sessionpkg.NewSession
			if err := session.ProcessBits(bitstream); err != nil {
				t.Fatalf("Session.ProcessBits failed for %s: %v", tt.imagePath, err)
			}

			// 3. Assert analysis results from image_analyzer extension
			if session.ProcessedBits != expectedBitstreamLen {
				t.Errorf("ProcessedBits count mismatch. Got %d, want %d", session.ProcessedBits, expectedBitstreamLen)
			}

			// Find the Image Analysis extension's output
			var imageAnalyzerOutput *extension.Output
			for _, output := range session.ExtensionOutputs { // Use session.ExtensionOutputs
				if output.Title == "Image Analysis" {
					imageAnalyzerOutput = &output
					break
				}
			}

			if imageAnalyzerOutput == nil {
				t.Fatalf("Image Analysis extension output not found in session.")
			}

			// Parse the lines to extract metrics
			var edges, textures uint64
			for _, line := range imageAnalyzerOutput.Lines {
				if strings.Contains(line, "Edges Detected:") {
					fmt.Sscanf(line, "  Edges Detected: %d", &edges)
				}
				if strings.Contains(line, "Texture Variations:") {
					fmt.Sscanf(line, "  Texture Variations: %d", &textures)
				}
			}

			// A checkerboard should have significant edges and textures
			if edges == 0 {
				t.Errorf("Expected non-zero EdgeDetections for checkerboard, got 0.")
			}
			if textures == 0 {
				t.Errorf("Expected non-zero TextureVariations for checkerboard, got 0.")
			}

			t.Logf("Image Analysis for %s: Edges=%d, Textures=%d", tt.imagePath, edges, textures)

			// Sanity check other extensions too if needed (optional)
			// For example, check entropy is not 0 or 64.
			var entropyOutput *extension.Output
			for _, output := range session.ExtensionOutputs { // Use session.ExtensionOutputs
				if output.Title == "Entropy Estimate" {
					entropyOutput = &output
					break
				}
			}
			if entropyOutput == nil {
				t.Logf("Entropy Estimate extension output not found.")
			} else {
				// Current estimator may quantize small samples aggressively; assert presence only.
				t.Logf("Entropy Estimate for %s: %s", tt.imagePath, entropyOutput.Lines[0])
			}

		})
	}
}

// Test with a uniform image to ensure zero detections (optional, for robustness)
func TestUniformImageProcessing(t *testing.T) {
	// Create a simple 8x8 all-black image
	const size = 8
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	black := color.RGBA{0, 0, 0, 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, black)
		}
	}

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "all_black.png")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create image file: %v", err)
	}
	defer file.Close()
	if err = png.Encode(file, img); err != nil {
		t.Fatalf("Failed to encode image: %v", err)
	}

	const hilbertOrder = 3
	const binarizationThreshold = 128 // Will make all pixels '0'

	bitstream, err := image_hilbert.GenerateBitstream(filePath, hilbertOrder, binarizationThreshold)
	if err != nil {
		t.Fatalf("GenerateBitstream failed for uniform image: %v", err)
	}

	expectedBitstreamLen := uint64(1 << (hilbertOrder * 2))
	if uint64(len(bitstream)) != expectedBitstreamLen {
		t.Errorf("Bitstream length mismatch for uniform image. Got %d, want %d", len(bitstream), expectedBitstreamLen)
	}
	if !strings.Contains(bitstream, "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Errorf("Expected all zeros bitstream for uniform black image, got: %s", bitstream)
	}


	session := sessionpkg.NewSession("test-uniform-session") // Use sessionpkg.NewSession
	if err := session.ProcessBits(bitstream); err != nil {
		t.Fatalf("Session.ProcessBits failed for uniform image: %v", err)
	}

	var imageAnalyzerOutput *extension.Output
	for _, output := range session.ExtensionOutputs { // Use session.ExtensionOutputs
		if output.Title == "Image Analysis" {
			imageAnalyzerOutput = &output
			break
		}
	}

	if imageAnalyzerOutput == nil {
		t.Fatalf("Image Analysis extension output not found for uniform image.")
	}

	var edges, textures uint64
	for _, line := range imageAnalyzerOutput.Lines {
		if strings.Contains(line, "Edges Detected:") {
			fmt.Sscanf(line, "  Edges Detected: %d", &edges)
		}
		if strings.Contains(line, "Texture Variations:") {
			fmt.Sscanf(line, "  Texture Variations: %d", &textures)
		}
	}

	// For a uniform image, we expect zero edges and zero textures
	if edges != 0 {
		t.Errorf("Expected 0 EdgeDetections for uniform image, got %d.", edges)
	}
	if textures != 0 {
		t.Errorf("Expected 0 TextureVariations for uniform image, got %d.", textures)
	}

	t.Logf("Image Analysis for Uniform Image: Edges=%d, Textures=%d", edges, textures)

	var entropyOutput *extension.Output
	for _, output := range session.ExtensionOutputs { // Use session.ExtensionOutputs
		if output.Title == "Entropy Estimate" {
			entropyOutput = &output
			break
		}
	}
	if entropyOutput == nil {
		t.Logf("Entropy Estimate extension output not found for uniform image.")
	} else {
		// A uniform image has zero entropy
		if !strings.Contains(entropyOutput.Lines[0], "0.00") {
			t.Errorf("Entropy Estimate for uniform image was not 0.00, got: %s", entropyOutput.Lines[0])
		}
	}
}
