# Fibtransponder CI targets.
# Canonical check: make ci

.PHONY: test conformance bench ci vet evaluate report run_experiments calibrate2d langdemo

# Full CI pipeline (canonical).
ci: vet test conformance

# Run all tests.
test:
	go test ./... -count=1 -timeout=120s

# Run conformance tests only (invariants + StepsSince ordering).
conformance:
	go test ./internal/deltaqueue/ -v -run "TestInvariant|TestClassifier" -count=1 -timeout=60s

# Run all benchmarks with memory stats.
bench:
	go test ./internal/deltaqueue/ -bench=. -benchmem -benchtime=1s -count=1

# Vet for static issues.
vet:
	go vet ./...

# Evaluation pipeline: run experiments and generate report.
evaluate: run_experiments report

# Run all evaluation experiments.
run_experiments:
	./evaluation/run_experiments.sh

# Generate the evaluation report from experiment data.
report:
	go run ./evaluation/report.go

# Run 2D calibration experiment.
calibrate2d:
	go test -v -run TestOrthogonality ./internal/calibration/ -count=1

# Run language sensing demo.
langdemo:
	go run ./cmd/langdemo/main.go

