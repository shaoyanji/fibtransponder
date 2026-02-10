# Benchmarks

Host CPU: Intel(R) Pentium(R) CPU N4200 @ 1.10GHz

## FSVM
- Benchmark: `internal/fsvm.BenchmarkStep`
- Result: ~46.55 ns/op, 0 allocs/op

## BitRope
- Benchmark: `internal/bitrope.BenchmarkAppendBit`
- Result: ~14.78 ns/op, 0 allocs/op

## WHT
- Benchmark: `internal/signal/wht.BenchmarkFWHT1024`
- Result: ~23.6–25.8 µs/op, allocs/op=1 (benchmark harness copy)

## FFT (baseline)
- Benchmark: `internal/signal/fft.BenchmarkFFT1024`
- Result: ~60.1 µs/op, **0 allocs/op** (after buffer reuse)

Notes:
- WHT and FFT benches are now **0 alloc**.
- Next: make CLI reuse buffers too (currently allocates per window); or accept CLI as a demo and keep the library alloc-free.

Commands:
```bash
go test -bench . -benchmem ./internal/fsvm ./internal/bitrope ./internal/signal/wht
go test -bench . -benchmem ./internal/signal/fft ./internal/signal/wht
```
