# fibtransponder

A deterministic, streaming, **Zeckendorf/Fibonacci-radix** transponder experiment with:

- **Canonical constraint:** Zeckendorf digits over **F2-based indexing** (bit 0 ↔ F2=1) and **no adjacent 1s** in canonical form.
- **Dilation protocol:** Adjacent `11` in the *observed* stream triggers a **global retrospective dilation event** `r++` interpreted as **virtual zero-stuffing** between all digits (upsample-by-2). This is done **without materializing stuffed zeros**.
- **Segmentation:** Optional/allowed message segmentation induced by long runs of zeros; treated as an **interpretation layer** (regular-language/NFA), not a mutation.
- **Markers (future):** “Rosetta stone” checkpoints bridging (a) fib radix legality/rewrites, (b) Binet/log2 magnitude bounds, and (c) modular/binary fingerprints.

## Status
Deliverables in this folder are:
- a written spec + design notes (`docs/`)
- a Go package skeleton (no Go toolchain on this host, so not compiled here)
- **Python reference implementation** that runs now (FSVM + bit rope + WHT + 2D embed demo)

## Folder structure
- `docs/` — specs and design
- `internal/bitrope` — append-only bit rope (immutable blocks)
- `internal/fsvm` — core streaming state machine (dilation, hexagram window, counters)
- `internal/render` — bounded-budget rendering strategy (exemplars, summaries)
- `internal/rosetta` — marker/probe plan (log2/Binet + residues) (mostly TODO)
- `internal/signal` — applications layer: boolean transport → windowing, transforms (FFT/WHT), decomposition
- `cmd/fibtransponder` — CLI skeleton

## Go toolchain
This host currently does **not** have `go` installed, so the Go code here is a scaffold. To build on a machine with Go installed:

```bash
cd projects/fibtransponder
go test ./...
```

## Key idea (one paragraph)
Maintain a streaming measurement transducer with O(1) update cost per input bit: track a small sliding window (“hexagram”, 6 bits), a global dilation counter `r`, and cheap summary probes. When `11` appears, increment `r` (retrospective dilation) rather than rewriting history. Segmentation is allowed and represented symbolically as a regular language over cut/no-cut choices at sparse “candidate markers” (e.g., zero-run power-of-two crossings), enabling unDoSable rendering of a few representative interpretations.
