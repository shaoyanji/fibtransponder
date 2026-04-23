# Benchmarks

Host CPU: Intel(R) Celeron(R) CPU N3010 @ 1.04GHz

## FSVM v1 (Zobrist-in-core, per-instance seeds)
- Benchmark: `internal/fsvm.BenchmarkStep`
- Result: ~44–48 ns/op, 0 allocs/op
- Zobrist sketch folded into core Step(): `s.Sketch ^= s.Seeds[b] + uint64(s.W)`
- Each instance owns its seed table via `NewWithSeeds()`

## FSVM v2 (independent hash families, avalanche mixer)
- Benchmark: `internal/fsvm.BenchmarkStepV2`
- Result: ~61 ns/op (bit-by-bit), 0 allocs/op
- `mixSketch(sk, a, b, r) = bits.RotateLeft64(sk*a + b, r)`
- Rich folding: zeroRun, R, seeds, event salts
- `SketchDelta` tracks per-step bit changes

## FSVM v2 word-level fast path
- Benchmark: `internal/fsvm.BenchmarkStepWord64V2`
- Result: ~32 ns/op per bit (mixed word), 0 allocs/op
- 3× faster than 64× bit-by-bit for bulk throughput

## Proprioceptive calibration
- Benchmark: `internal/calibration.BenchmarkStepWord64Adaptive`
- Result: ~53 ns/op mixed, 0 allocs/op
- Same as non-adaptive; calibration fires every 256 bits (configurable)

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
