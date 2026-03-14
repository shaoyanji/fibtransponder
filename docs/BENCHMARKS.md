# Benchmarks

Host CPU: Intel(R) Pentium(R) CPU N4200 @ 1.10GHz

## FSVM (Zobrist-in-core, per-instance seeds)
- Benchmark: `internal/fsvm.BenchmarkStep`
- Result: ~44–48 ns/op, 0 allocs/op (5-run range: 43.95–47.95)
- Zobrist sketch folded into core Step(): `s.Sketch ^= s.Seeds[b] + uint64(s.W)`
- Each instance owns its seed table via `NewWithSeeds()`

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
