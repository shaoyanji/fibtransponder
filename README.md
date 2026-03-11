# fibtransponder

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-draft%20prototype-orange)](#status)
[![Spec](https://img.shields.io/badge/spec-docs%2FSPEC.md-blue)](docs/SPEC.md)

`fibtransponder` is a Fibonacci-radix streaming transponder prototype.

It ingests an unbounded boolean stream and maintains a deterministic, bounded-work state machine over that stream.

**Source of truth:** `docs/SPEC.md`

---

## What it is

A measurement-first stream core that tracks:

- adjacency events (`11`) → retrospective dilation events
- sparse long-zero markers (`8, 16, 32, ...` by default)
- a 6-bit rolling local window
- bounded O(1) (or amortized O(1)) ingest work

Core state entities:

- `r`: global dilation exponent
- `w`: 6-bit rolling hexagram window
- `lastBit`: previous observed bit
- `zeroRun`: current zero-run length

## What it is not

- not a generic LLM wrapper
- not an orchestration framework
- not a finished product with closed semantics

---

## Demo

![fibtransponder TUI demo](docs/media/fibtransponder-tui-hero.gif)

> Live view of bounded streaming ingest with rolling state/probe feedback.

If the preview does not render on your surface, open: `docs/media/fibtransponder-tui-hero.gif`

---

## Quickstart

### Build + test

```bash
go test ./...
```

### Run TUI

```bash
cd cmd/tui
go build -o fibtransponder_tui
./fibtransponder_tui
```

Example:

```bash
echo "010101100101" | ./fibtransponder_tui
```

### Run API

```bash
cd cmd/api
go build -o fibtransponder_api
./fibtransponder_api
```

Default listen address: `http://localhost:8080`.

---

## Spec highlights

- **Indexing:** `bit[i] ↔ F_{i+2}`
- **Canonical Zeckendorf words:** no adjacent `1`s
- **Dilation semantics:** when `11` is observed, increment `r` and interpret retroactively as virtual stuffing
- **Segmentation:** allowed, not forced; sparse candidate boundaries only
- **Safety:** bounded ingest, linear memory growth, budgeted rendering

---

## Repo layout

- `docs/SPEC.md` — authoritative draft spec
- `docs/DESIGN.md` — implementation architecture notes
- `docs/SIGNAL.md` — probe/signal decomposition notes
- `docs/APPLICATIONS.md` — constrained downstream use-cases
- `internal/fsvm` — core streaming state machine
- `internal/bitrope` — append-only bitstream substrate
- `internal/signal` — signal/decomposition layer
- `internal/segauto` — segmentation automaton experiments
- `internal/rosetta` — marker/probe experiments
- `internal/render` — bounded rendering
- `cmd/tui` — terminal UI
- `cmd/api` — API surface
- `test/` — tests (legacy suite is build-tagged)

---

## Publish checklist

- [x] README aligned to `docs/SPEC.md`
- [x] Core docs (`SPEC`, `DESIGN`, `APPLICATIONS`) internally consistent
- [x] Build artifacts removed from version control
- [x] `go test ./...` passing
- [ ] Tag first public release
- [~] Add screenshot/GIF to showcase TUI flow (placeholder added at `docs/media/fibtransponder-tui-hero.gif`)

---

## Status

Draft implementation + research surface around `docs/SPEC.md`.

Open semantic questions remain explicit in the spec (e.g. exact `N(r)` meaning, dilated-probe definitions, marker equations).