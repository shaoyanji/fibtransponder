# fibtransponder

`fibtransponder` is a Fibonacci-radix streaming transponder prototype.

It ingests an unbounded boolean stream and maintains a deterministic, bounded-work state machine over that stream. The current source of truth is `docs/SPEC.md`.

## Scope (current draft)

The project implements and explores the **measurement-first** model from the spec:

- track adjacency (`11`) and emit retrospective **dilation events**
- track sparse long-zero markers (`8, 16, 32, ...` by default)
- maintain a small rolling window for local probes
- keep ingestion O(1) (or amortized O(1)) per observed bit

Core spec entities:

- `r`: global dilation exponent
- `w`: 6-bit rolling hexagram window
- `lastBit`: previous observed bit
- `zeroRun`: current zero-run length

## Canonical representation

The spec uses Zeckendorf indexing:

- `bit[i] ↔ F_{i+2}`
- canonical words have **no adjacent 1s**

When `11` is observed, `r` increments and interpretation is retroactively rescaled as if the dilation operator had been applied globally. This is modeled semantically (virtual stuffing), not by materializing inserted zeros.

## Segmentation model

There is no mandatory EOF boundary.

Segmentation is an interpretation layer:

- long zero runs suggest candidate cut points
- cuts are optional, never required
- candidate points are sparse/deterministic to preserve DoS resistance

Ambiguity is represented as a regular-language view over cut/no-cut choices.

## Safety targets

From the current spec:

- bounded work per input symbol
- linear memory growth in observed bits (immutable block allocation strategy)
- budgeted rendering with graceful summaries when over budget

## Repository layout

- `docs/SPEC.md` — authoritative draft spec
- `docs/DESIGN.md` — implementation architecture notes
- `docs/SIGNAL.md` — probe/signal decomposition notes
- `docs/APPLICATIONS.md` — potential downstream use-cases
- `internal/fsvm` — core streaming state machine
- `internal/bitrope` — append-only bitstream substrate
- `internal/signal` — signal/decomposition layer
- `internal/segauto` — segmentation automaton experiments
- `internal/rosetta` — marker/probe experiments
- `internal/render` — bounded rendering
- `cmd/tui` — terminal UI
- `cmd/api` — API surface
- `test/` — integration/compression tests

## Build and test

From repo root:

```bash
go test ./...
```

## Run

### TUI

```bash
cd cmd/tui
go build -o fibtransponder_tui
./fibtransponder_tui
```

Example:

```bash
echo "010101100101" | ./fibtransponder_tui
```

### API

```bash
cd cmd/api
go build -o fibtransponder_api
./fibtransponder_api
```

Default listen address: `http://localhost:8080`.

## Status

This repository is a draft implementation + research surface around `docs/SPEC.md`.

Open questions are tracked in the spec (semantic value under dilation, probe semantics under retroactive scaling, and marker equations).