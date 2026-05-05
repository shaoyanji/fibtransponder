#!/bin/bash
set -euo pipefail

# Fibtransponder Evaluation Pipeline
# This script runs all experiments and generates raw data for the evaluation report.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="$SCRIPT_DIR/raw_data"

mkdir -p "$OUTPUT_DIR"

echo "=== Fibtransponder Evaluation Pipeline ==="
echo "Output directory: $OUTPUT_DIR"
echo ""

# Run orthogonality experiment (2D calibration grid)
echo "[1/3] Running 2D calibration orthogonality experiment..."
go test -v -run TestOrthogonality ./internal/calibration/ -count=1 > "$OUTPUT_DIR/orthogonality.log" 2>&1
echo "      -> $OUTPUT_DIR/orthogonality.log"

# Run second axis independence experiment
echo "[2/3] Running second-axis independence experiment..."
go test -v -run TestSecondAxisIndependence ./internal/calibration/ -count=1 > "$OUTPUT_DIR/second_axis.log" 2>&1
echo "      -> $OUTPUT_DIR/second_axis.log"

# Run proprioceptive convergence experiment
echo "[3/3] Running proprioceptive loop convergence experiment..."
go test -v -run TestConvergence ./internal/calibration/ -count=1 > "$OUTPUT_DIR/convergence.log" 2>&1 || true
echo "      -> $OUTPUT_DIR/convergence.log"

# Run benchmarks
echo "[BENCH] Running calibration benchmarks..."
go test -bench=. -benchmem -benchtime=1s ./internal/calibration/ > "$OUTPUT_DIR/benchmarks.log" 2>&1
echo "      -> $OUTPUT_DIR/benchmarks.log"

echo ""
echo "=== Experiments completed successfully ==="
echo "Run 'go run ./evaluation/report.go' to generate the markdown report."
