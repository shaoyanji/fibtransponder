package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Usage: %s <image_path> <hilbert_order> <threshold>\n", os.Args[0])
		fmt.Println("  <image_path>: Path to the input image (e.g., .png, .jpg)")
		fmt.Println("  <hilbert_order>: Order of the Hilbert curve (e.g., 5 for 32x32 image, N=2^order)")
		fmt.Println("  <threshold>: Grayscale threshold (0-255) for binarization (e.g., 128)")
		os.Exit(1)
	}

	imagePath := os.Args[1]
	order, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid Hilbert order: %v\n", err)
		os.Exit(1)
	}
	threshold, err := strconv.Atoi(os.Args[3])
	if err != nil || threshold < 0 || threshold > 255 {
		fmt.Fprintf(os.Stderr, "Invalid threshold (0-255): %v\n", err)
		os.Exit(1)
	}

	bitstream, err := image_hilbert.GenerateBitstream(imagePath, order, uint8(threshold))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating bitstream: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(bitstream)
}
