package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
)

// ImageHeader structure for compressed image files
type ImageHeader struct {
	OriginalWidth  uint32
	OriginalHeight uint32
	// fib_coder.Encode will write its own 8-byte header for originalLenInBits,
	// so we don't need to duplicate OriginalBitLen here.
	// We'll write this header FIRST, then call fib_coder.Encode which will write its own.
}

// Run executes the main logic of hilbert_gen, allowing it to be called programmatically for testing.
func Run(args []string) error {
	if len(args) < 7 { // Expect: hilbert_gen -i <image> -o <output> -order <order> -threshold <threshold>
		return fmt.Errorf("usage: %s -i <image_path> -o <output_fibimg_path> -order <hilbert_order> -threshold <threshold>", filepath.Base(args[0]))
	}

	var imagePath, outputPath string
	var order, threshold int

	// Simple flag parsing
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-i":
			i++
			if i < len(args) {
				imagePath = args[i]
			}
		case "-o":
			i++
			if i < len(args) {
				outputPath = args[i]
			}
		case "-order":
			i++
			if i < len(args) {
				var err error
				order, err = strconv.Atoi(args[i])
				if err != nil {
					return fmt.Errorf("invalid Hilbert order: %w", err)
				}
			}
		case "-threshold":
			i++
			if i < len(args) {
				var err error
				threshold, err = strconv.Atoi(args[i])
				if err != nil || threshold < 0 || threshold > 255 {
					return fmt.Errorf("invalid threshold (0-255): %w", err)
				}
			}
		}
	}

	if imagePath == "" || outputPath == "" || order == 0 { // Threshold can be 0, so order is better check
		return fmt.Errorf("missing required arguments. Use -i, -o, -order, -threshold")
	}


	// 1. Generate bitstream from image
	bitstreamString, width, height, err := image_hilbert.GenerateBitstream(imagePath, order, uint8(threshold))
	if err != nil {
		return fmt.Errorf("error generating bitstream from image: %w", err)
	}

	// 2. Open output file for writing
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %w", outputPath, err)
	}
	defer outFile.Close()

	// 3. Write Custom Image Header
	imgHeader := ImageHeader{
		OriginalWidth:  width,
		OriginalHeight: height,
	}
	if err := binary.Write(outFile, binary.BigEndian, imgHeader.OriginalWidth); err != nil { // uint32
		return fmt.Errorf("failed to write image width header: %w", err)
	}
	if err := binary.Write(outFile, binary.BigEndian, imgHeader.OriginalHeight); err != nil { // uint32
		return fmt.Errorf("failed to write image height header: %w", err)
	}
	
	// fib_coder.Encode will write its own 8-byte OriginalBitLen header.
	// We just pass it the io.Reader and io.Writer.

	// 4. Compress the bitstream using fib_coder.Encode
	// Create an io.Reader from the generated bitstream string
	bitstreamReader := strings.NewReader(bitstreamString)
	originalBitLen := uint64(len(bitstreamString))
	
	err = fib_coder.Encode(bitstreamReader, outFile, originalBitLen)
	if err != nil {
		return fmt.Errorf("error compressing bitstream: %w", err)
	}

	fmt.Printf("Successfully compressed image '%s' to '%s'\n", imagePath, outputPath)
	fmt.Printf("Original Bit Length: %d, Image Dimensions: %dx%d\n", originalBitLen, imgHeader.OriginalWidth, imgHeader.OriginalHeight)
	return nil
}

func main() {
	if err := Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
