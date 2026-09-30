# Benchmarks

Host CPU: AMD EPYC 7763 64-Core, Go 1.25. Re-measured after the v1 sketch
fold fix; earlier revisions cited a Celeron N3010 and a Pentium N4200 without
attributing individual figures to either, and those numbers were removed.

## FSVM v1 (Zobrist-in-core, per-instance seeds)
- Benchmark: `internal/fsvm.BenchmarkStep`
- Result: ~30 ns/op, 0 counted allocs/op, ~3 B/op amortized (the returned event slice escapes)
- Zobrist sketch folded into core Step(): `s.Sketch ^= fsvm.SketchTerm(s.Seeds, b, s.W)`,
  i.e. `s.Seeds[b] + uint64(s.W) * 0x9E3779B97F4A7C15` (the multiplier spreads the
  6-bit window across the full 64-bit word)
- Each instance owns its seed table via `NewWithSeeds()`

## FSVM v2 (independent hash families, avalanche mixer)
- Benchmark: `internal/fsvm.BenchmarkStepV2`
- Result: ~35 ns/op (bit-by-bit), 0 counted allocs/op, ~3 B/op amortized
- `mixSketch(sk, a, b, r) = bits.RotateLeft64(sk*a + b, r)`
- Rich folding: zeroRun, R, seeds, event salts
- `SketchDelta` tracks per-step bit changes

## FSVM v2 word-level fast path
- Benchmark: `internal/fsvm.BenchmarkStepWord64V2`
- Result: ~887 ns/op per 64-bit word (~14 ns/op per bit), 0 allocs/op
- ~2.5× faster per bit than bit-by-bit v2 (~35 ns/op) for bulk throughput

## Proprioceptive calibration
- Benchmark: `internal/calibration.BenchmarkStepWord64Adaptive`
- Result: `BenchmarkAdaptiveArrayStepWord64` ~2176 ns/op per 64-bit word across the
  default 7-transponder array, 192 B/op, 1 alloc/op (the Result slice)
- Same as non-adaptive per-transponder cost; calibration fires every 256 bits (configurable)

## Rich feature extraction
- Benchmark: `internal/fsvm.BenchmarkExtractorExtract`
- Result: ~820 ns/op, 0 allocs/op
- 64-bit rolling window, 8 sub-regions

## Rich feature integration
- Benchmark: `internal/fsvm.BenchmarkStepWithExtractor`
- Result: ~97 ns/op, 0 allocs/op
- Overhead only when events fire (sparse)

## Descriptor distance
- Benchmark: `internal/fsvm.BenchmarkDescriptorDistance`
- Result: ~15 ns/op, 0 allocs/op

## Classifier (read-only sketch)
- Benchmark: `internal/deltaqueue.BenchmarkClassify`
- Result: ~55 ns/op, 0 allocs/op
- Sketch read cost: ~0.65 ns/op (negligible)

### What dominates classifier cost (~55ns total)
| Component | ns/op | % of total |
|---|---|---|
| AuxBuckets (classifyAux) | ~33 | 60% |
| StepsSince increment loop | ~9.5 | 17% |
| Flag derivation | ~0.65 | 1% |
| Sketch read (CoreDelta→cls) | ~0.65 | 1% |
| Struct copy + control flow | ~11 | 21% |

**Conclusion:** Sketch handling is no longer a cost factor. The classifier's
dominant cost is AuxBuckets computation (log-scale ordinal mapping + packing).

## Classifier variants
| Variant | ns/op | allocs |
|---|---|---|
| dense_transition_stream | ~51 | 0 |
| checkpoint_stream | ~85 | 0 |
| periodic_checkpoint_stream | ~72 | 0 |
| zero_stream | ~96 | 0 |

## BitRope
- Benchmark: `internal/bitrope.BenchmarkAppendBit`
- Result: ~14.78 ns/op, 0 allocs/op

## WHT
- Benchmark: `internal/signal/wht.BenchmarkFWHT1024`
- Result: ~23.6–25.8 µs/op, allocs/op=1 (benchmark harness copy)

## FFT (baseline)
- Benchmark: `internal/signal/fft.BenchmarkFFT1024`
- Result: ~60.1 µs/op, **0 allocs/op** (after buffer reuse)

Commands:
```bash
go test -bench . -benchmem ./internal/fsvm ./internal/bitrope ./internal/signal/wht
go test -bench . -benchmem ./internal/signal/fft ./internal/signal/wht
go test -bench=BenchmarkClassify -benchmem ./internal/deltaqueue
```
