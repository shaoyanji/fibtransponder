# Fibtransponder CI targets.
# Canonical check: make ci

.PHONY: test conformance bench ci vet

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
