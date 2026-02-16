package image_hilbert

import (
	"fmt"
	"image"
	_ "image/jpeg" // Import for JPEG support
	_ "image/png"  // Import for PNG support
	"os"
	"strings"
)

// d2xy converts a 1D distance 'd' along a Hilbert curve to 2D coordinates (x, y)
// on an n x n grid, where n = 2^order.
// Based on: https://en.wikipedia.org/wiki/Hilbert_curve#Applications_and_mapping_algorithms
func d2xy(n, d uint32) (x, y uint32) {
	t := d
	x, y = 0, 0
	s := uint32(1)
	for s < n {
		rx := uint32(1 & (t / 2))
		ry := uint32(1 & (t ^ x))
		x, y = rot(s, x, y, rx, ry)
		x += s * rx
		y += s * ry
		t /= 4
		s *= 2
	}
	return
}

// D2XY exposes Hilbert index-to-coordinate mapping for callers.
func D2XY(n, d uint32) (x, y uint32) {
	return d2xy(n, d)
}

// rot is a helper function for d2xy and xy2d.
func rot(n, x, y, rx, ry uint32) (uint32, uint32) {
	if ry == 0 {
		if rx == 1 {
			x = n - 1 - x
			y = n - 1 - y
		}
		x, y = y, x
	}
	return x, y
}

// GenerateBitstream takes an image file path, the order of the Hilbert curve (n=2^order),
// and a threshold for binarization, then returns a bitstream string.
func GenerateBitstream(imagePath string, order int, threshold uint8) (string, error) {
	n := uint32(1 << order) // Grid size n x n

	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != int(n) || bounds.Dy() != int(n) {
		return "", fmt.Errorf("image dimensions (%dx%d) do not match Hilbert curve order %d (expected %dx%d)",
			bounds.Dx(), bounds.Dy(), order, n, n)
	}

	var sb strings.Builder
	sb.Grow(int(n * n)) // Pre-allocate memory

	for d := uint32(0); d < n*n; d++ {
		x, y := d2xy(n, d)
		
		// Get pixel color at (x,y)
		c := img.At(int(x), int(y))
		r, g, b, _ := c.RGBA() // RGBA returns values 0-65535

		// Convert to grayscale (luminance) and then to binary
		// Luminance calculation: 0.299*R + 0.587*G + 0.114*B
		// Scale RGBA values (0-65535) to 0-255 for comparison with 8-bit threshold
		gray := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)

		if gray > threshold {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}

	return sb.String(), nil
}
