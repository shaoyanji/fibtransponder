package test

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
)

// CreateCheckerboardImage creates an 8x8 checkerboard image for testing.
// If makePNG is true, it creates a PNG, otherwise a JPEG.
// It returns the path to the created image file.
func CreateCheckerboardImage(t *testing.T, filename string, makePNG bool) string {
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

// GenerateExpectedBitstream creates an expected Hilbert-ordered bitstream
// for a given image path, order, and threshold.
func GenerateExpectedBitstream(t *testing.T, imagePath string, hilbertOrder int, binarizationThreshold uint8) string {
	bitstream, _, _, err := image_hilbert.GenerateBitstream(imagePath, hilbertOrder, binarizationThreshold)
	if err != nil {
		t.Fatalf("GenerateBitstream failed: %v", err)
	}
	return bitstream
}
