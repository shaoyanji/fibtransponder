package main

import (
	"fmt"
	"os"
	"path/filepath"
	// Removed unused "io"
	// Removed unused "strings"

	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	
	// Parse flags for input/output files
	var inputFile, outputFile string
	args := os.Args[2:] // Skip command
	for i := 0; i < len(args); i++ {
		if args[i] == "-i" && i+1 < len(args) {
			inputFile = args[i+1]
			i++
		} else if args[i] == "-o" && i+1 < len(args) {
			outputFile = args[i+1]
			i++
		} else {
			fmt.Fprintf(os.Stderr, "Unknown flag or argument: %s\n", args[i])
			printUsage()
			os.Exit(1)
		}
	}

	if inputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Input file (-i) is required.\n")
		printUsage()
		os.Exit(1)
	}
	if outputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Output file (-o) is required.\n")
		printUsage()
		os.Exit(1)
	}

	switch command {
	case "compress":
		err := handleCompress(inputFile, outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error compressing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully compressed '%s' to '%s'\n", inputFile, outputFile)
	case "decompress":
		err := handleDecompress(inputFile, outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decompressing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully decompressed '%s' to '%s'\n", inputFile, outputFile)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	appName := filepath.Base(os.Args[0])
	fmt.Printf("Usage: %s <command> -i <input_file> -o <output_file>\n", appName)
	fmt.Println("Commands:")
	fmt.Println("  compress    - Compresses input file to output file")
	fmt.Println("  decompress  - Decompresses input file to output file")
	fmt.Println("Flags:")
	fmt.Println("  -i <input_file>   Path to the input file")
	fmt.Println("  -o <output_file>  Path to the output file")
}

func handleCompress(inputFile, outputFile string) error {
	in, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file '%s': %w", inputFile, err)
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %w", outputFile, err)
	}
	defer out.Close()

	// Get original file size in bytes
	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("failed to get input file info: %w", err)
	}
	originalLenInBytes := info.Size()
	originalLenInBits := uint64(originalLenInBytes * 8)

	return fib_coder.Encode(in, out, originalLenInBits)
}

func handleDecompress(inputFile, outputFile string) error {
	in, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file '%s': %w", inputFile, err)
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %w", outputFile, err)
	}
	defer out.Close()

	_, err = fib_coder.Decode(in, out) // Decode now returns originalLenInBits, but we don't need it here.
	return err
}
