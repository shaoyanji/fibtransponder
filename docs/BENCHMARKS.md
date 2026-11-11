# Benchmarks

Host CPU: AMD EPYC 7763 64-Core, Go 1.25. Re-measured after the v1 sketch
fold fix; earlier revisions cited a Celeron N3010 and a Pentium N4200 without
attributing individual figures to either, and those numbers were removed.

## Re-measured on clanker, 2026-11-12

Host: the fleet's Instabox box `clanker` — 4 vCPU AMD EPYC, 7 GiB, Linux
6.18.60, Go 1.27.1. Command, one run, `-count=1`, tests skipped:

```bash
go test -run=XXX -bench=. -benchmem -benchtime=1s -count=1 ./internal/...
```

All packages reported `ok`. These are VM numbers from a single run, so read them
as indicative of the shape rather than as a citable figure; the sections below are
the earlier measurements on the host above and are kept as history.

| Benchmark | ns/op | B/op | allocs/op | earlier |
|---|---|---|---|---|
| `fsvm.BenchmarkStep` | 16.80 | 3 | 0 | ~30 |
| `fsvm.BenchmarkStepWord64` | 297.6 | 0 | 0 | — |
| `fsvm.BenchmarkStepSequence64/word` | 303.1 | 0 | 0 | — |
| `fsvm.BenchmarkStepSequence64/bit-by-bit` | 1187 | 256 | 16 | — |
| `fsvm.BenchmarkExtractorExtract` | 85.72 | 0 | 0 | ~820 |
| `fsvm.BenchmarkExtractorPush` | 4.033 | 0 | 0 | — |
| `fsvm.BenchmarkStepWithExtractor` | 19.51 | 0 | 0 | ~97 |
| `fsvm.BenchmarkStepWord64WithExtractor` | 1106 | 0 | 0 | — |
| `fsvm.BenchmarkDescriptorDistance` | 19.62 | 0 | 0 | ~15 |
| `deltaqueue.BenchmarkClassify` | 23.32 | 0 | 0 | ~55 |
| `deltaqueue.BenchmarkClassifyBaseline` | 23.12 | 0 | 0 | — |
| `deltaqueue.BenchmarkClassifyAux` | 9.151 | 0 | 0 | ~33 |
| `deltaqueue.BenchmarkClassifyStepsSince` | 1.186 | 0 | 0 | ~9.5 |
| `deltaqueue.BenchmarkClassifySketchOnly` | 0.2224 | 0 | 0 | ~0.65 |
| `bitrope.BenchmarkAppendBit` | 5.249 | 0 | 0 | ~14.78 |
| `calibration.BenchmarkAdaptiveArrayStepWord64` | 971.5 | 192 | 1 | ~2176 |

Classifier variants on this host: `dense_transition_stream` 22.14,
`periodic_checkpoint_stream` 22.75, `checkpoint_stream` 24.14, `zero_stream`
24.40 ns/op, all 0 allocs/op.

### The breakdown changed shape

Aux is 9.151 of the 23.32 ns total (39%), the `StepsSince` loop 1.186 (5%), flag
derivation and the sketch read 0.22 each (1%), which leaves ~12.5 ns — 54% — for
struct copy and control flow. The earlier table put Aux at 60% and the remainder
at 21%, so "AuxBuckets dominates classifier cost" does not reproduce on this
host. The claim that survives is the narrower one: sketch handling is not a cost
factor, on either host.

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
