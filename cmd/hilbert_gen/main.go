package main

import (
	"fmt"
	"os"

	hilbertgen "github.com/shaoyanji/fibtransponder/pkg/hilbertgen"
)

func main() {
	if err := hilbertgen.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
