package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	session "github.com/shaoyanji/fibtransponder/internal/session"
)

func main() {
	// Create a new session for this CLI run
	s := session.NewSession("cli-session")

	// Read bits from stdin
	reader := bufio.NewReader(os.Stdin)
	var bitInput strings.Builder
	for {
		char, _, err := reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
		// Only append '0' or '1' characters, ignore whitespace
		if char == '0' || char == '1' {
			bitInput.WriteRune(char)
		} else if char != '\n' && char != '\r' && char != '\t' && char != ' ' {
			fmt.Fprintf(os.Stderr, "Warning: Ignoring unexpected character '%c' from stdin\n", char)
		}
	}
	// No scanner.Err() for bufio.Reader, errors are handled in the loop.

	// Process the collected bits
	if bitInput.Len() > 0 {
		if err := s.ProcessBits(bitInput.String()); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing bits: %v\n", err)
			os.Exit(1)
		}
	}

	// Print final FSVM State
	fmt.Println("--- Final FSVM State ---")
	fmt.Printf("Processed Bits: %d\n", s.ProcessedBits)
	fmt.Printf("r: %d | Dilations: %d | Markers: %d | ZeroRun: %d | W: %06b\n",
		s.FSVMState.R, s.FSVMState.Dilations, s.FSVMState.Markers, s.FSVMState.ZeroRun, s.FSVMState.W)
	fmt.Println("------------------------")

	// Print summaries from extensions
	fmt.Println("--- Extension Summaries ---")
	for _, output := range s.ExtensionOutputs {
		fmt.Printf("%s:\n", output.Title)
		for _, line := range output.Lines {
			fmt.Println(line)
		}
	}
	fmt.Println("---------------------------")
}
