# Fibtransponder CI targets.
# Canonical check: make ci

.PHONY: test conformance bench ci vet evaluate report run_experiments calibrate2d langdemo wasm wasm-size wasm-compress site site-install site-build site-preview

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

# --- Portfolio site WASM build -------------------------------------------
#
# Compiles internal/wasmapi for the browser. The shim is ordinary Go with no
# syscall/js, so `go test ./internal/wasmapi/` covers it natively and is part of
# `make ci`. This target only adds the js/wasm wrapper in cmd/fibwasm.
#
# Output lands in portfolio/public/wasm/, which is gitignored: the artifact is
# build output, not source. The site loads it lazily, so no page without a demo
# ever pays for it.
WASM_DIR := portfolio/public/wasm
WASM_BIN := $(WASM_DIR)/fibtransponder.wasm

wasm:
	@mkdir -p $(WASM_DIR)
	GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
	  -o $(WASM_BIN) ./cmd/fibwasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/
	@$(MAKE) --no-print-directory wasm-size wasm-compress

# Emit pre-compressed siblings next to the .wasm.
#
# The Go runtime is ~547 KB gzipped before any of this project's code is linked
# in, and encoding/json alone accounts for ~655 KB more, so the artifact is large
# for reasons that have nothing to do with fibtransponder. Pre-compressing
# sidesteps the question of whether the host (GitHub Pages behind Fastly) will
# negotiate gzip or brotli for a .wasm: the files are already on disk and the
# client can just try .br, then .gz, then the raw bytes.
#
# brotli and gzip are optional. CI runners are not guaranteed to have brotli, so
# a missing compressor is a skipped format, not a failed build.
wasm-compress:
	@if command -v brotli >/dev/null 2>&1; then \
	  brotli -q 11 -f -k $(WASM_BIN) && echo "wasm brotli: $$(numfmt --to=iec $$(stat -c%s $(WASM_BIN).br))"; \
	else \
	  echo "wasm brotli: skipped (no brotli on PATH)"; \
	fi
	@gzip -9 -c $(WASM_BIN) > $(WASM_BIN).gz
	@echo "wasm gzip:   $$(numfmt --to=iec $$(stat -c%s $(WASM_BIN).gz))"
	@cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/ 2>/dev/null || true

# Report transfer sizes, since that is what actually matters for a web demo.
# Treat the number as runtime-plus-dependencies rather than a code-size metric.
wasm-size:
	@raw=$$(stat -c%s $(WASM_BIN)); \
	echo "wasm raw:    $$(numfmt --to=iec $$raw)  ($$raw bytes)"

# --- Portfolio site ---------------------------------------------------------
#
# The site's build output is gitignored, so `make site` is the one command that
# produces everything needed to serve it locally. See portfolio/README.md.

.PHONY: site site-install site-build site-preview

site: wasm site-build

site-install:
	cd portfolio && npm ci

site-build:
	cd portfolio && npm run build
	@test -s portfolio/dist/index.html || { echo "site: dist/index.html missing"; exit 1; }
	@test -s portfolio/dist/wasm/fibtransponder.wasm || { echo "site: wasm missing from dist"; exit 1; }
	@echo "site: $$(find portfolio/dist -name '*.html' | wc -l) pages in portfolio/dist"

# Serves dist/ under the same base path the deployed site uses, so a local check
# exercises the real asset URLs rather than only working at the root.
site-preview: site
	cd portfolio && npm run preview

