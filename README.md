# fibtransponder

`fibtransponder` is a deterministic streaming primitive for correcting and stabilizing common LLM mistake patterns in flight.

The core idea is simple: instead of paying for another API call whenever generation drifts into a recurring local failure mode, use a bounded local transducer to detect and respond to those patterns as the stream is forming. The project currently explores this through a Fibonacci / Zeckendorf-oriented state machine, streaming signal analysis, and lightweight visual tooling.

This makes `fibtransponder` part of the same family as schema-first ingress and token-efficient parsing tools: not a bigger model, not another agent hop, but a smaller corrective layer that can run locally, cheaply, and continuously.

## Why it exists

A lot of LLM systems pay premium cost to fix cheap errors.

Typical pattern:
- generation drifts
- a second pass or second model is invoked
- more context is replayed
- latency and token cost go up
- the correction path becomes harder to inspect

`fibtransponder` explores a different path:
- keep correction local
- keep the update rule deterministic
- keep the execution surface small
- preserve streaming behavior instead of pausing for another roundtrip

The ambition here is not to replace model judgment. It is to reduce the number of times a predictable local mistake has to be repaired by an expensive remote loop.

## Current shape

Right now this repo is an experimental systems project with three main layers:

1. **Streaming transducer core**
   - deterministic state updates over a bitstream
   - Fibonacci / Zeckendorf-oriented rewrite and dilation logic
   - bounded per-symbol processing

2. **Signal / analysis layer**
   - streaming probes and decomposition ideas
   - transform-oriented analysis over windows
   - exploratory modules for segmentation, signal, entropy, and image-derived streams

3. **Operator surface**
   - a Go TUI for real-time visualization
   - an API service for programmatic access
   - docs/spec work defining the execution model and constraints

## Project status

This is still an active experiment, not a finished product.

What exists today:
- a substantial written spec and design corpus in `docs/`
- a working Go codebase with TUI and API entrypoints
- core streaming/state-machine infrastructure
- extension-oriented analysis modules
- tests around compression and integration surfaces

What is still in motion:
- the exact correction model and strongest use-cases
- how the Fibonacci-oriented transducer maps onto practical LLM error classes
- which probes and transforms belong in core vs outer layers
- how to benchmark the local correction path against ordinary reprompt / second-pass behavior

## Repo layout

- `docs/` — specifications, design notes, benchmarks, applications, and open explorations
- `internal/fsvm` — core streaming state-machine work
- `internal/bitrope` — append-only bit storage / streaming substrate
- `internal/signal` — signal and decomposition layer
- `internal/segauto` — segmentation automaton experiments
- `internal/rosetta` — marker and probe experiments
- `internal/render` — bounded rendering strategy
- `internal/image_hilbert` / `cmd/hilbert_gen` — image-to-bitstream utilities via Hilbert traversal
- `cmd/tui` — terminal visualization surface
- `cmd/api` — API entrypoint
- `test/` — integration and compression tests

## Key architectural idea

The system treats the stream as something you can shape while it is still forming.

Rather than waiting for a whole output artifact and then repairing it with another model call, the project explores whether a deterministic transducer can:
- detect recurring local failure patterns
- preserve correction pressure in flight
- maintain cheap bounded updates
- keep the correction path inspectable

That is the real thesis of the repo.

The Fibonacci / Zeckendorf machinery is part of the mechanism, not the entire point.

## Build and run

Run tests from repo root:

```bash
go test ./...
```

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

By default the API listens on `http://localhost:8080`.

## Suggested reading order

If you want the conceptual shape first:

1. `docs/SPEC.md`
2. `docs/SIGNAL.md`
3. `docs/DESIGN.md`
4. `docs/APPLICATIONS.md`

If you want implementation status and gaps:

1. `IMPLEMENTATION_GAPS.md`
2. `CONFORMANCE_TARGETS.md`
3. `docs/TODO.md`

## What this repo is not

- not a generic LLM wrapper
- not another agent orchestration layer
- not a second-pass API repair loop
- not a finished product claiming solved grounding or universal correction

It is a focused experiment in whether some recurring model errors can be corrected or stabilized locally, with deterministic streaming machinery, before they need to become another expensive model interaction.
