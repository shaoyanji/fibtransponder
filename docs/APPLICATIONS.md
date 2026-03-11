# APPLICATIONS — spec-compatible exploration map

This file lists plausible downstream uses that remain consistent with `docs/SPEC.md`.

Guardrail: applications must not change core ingest semantics.

## Core signals available to applications

Reliable low-cost outputs from the core path:

- dilation exponent `r(t)`
- dilation event rate
- `zeroRun` lengths and sparse marker crossings (`8,16,32,...` default)
- local 6-bit window statistics (`w`-derived)

These are the primary portable signals. Anything heavier should be optional/lazy.

## 1) Channel stress fingerprinting

Use event-derived summaries as a robust channel signature:

- dilation burst patterns
- zero-run distribution
- marker cadence
- window histogram drift over time

This is an operational fingerprinting use-case, not a claim of full semantic decoding.

## 2) Boolean stream analysis adapters

Treat observed bits as a time series and build bounded adapters:

- windowed autocorrelation
- Walsh–Hadamard summaries on fixed windows
- optional FFT views on mapped bipolar forms

Important: any frequency-domain interpretation should be annotated with `r`/marker context due to retrospective dilation semantics.

## 3) Segmentation-assisted interpretation

At sparse candidate cut points, downstream tooling may evaluate cut/no-cut alternatives.

Constraints:

- candidate points remain deterministic and sparse
- ambiguity represented symbolically (automata), not full enumeration
- outputs remain bounded under worst-case streams

## 4) Visual operator tooling

UI/API surfaces can expose:

- live `r`, zero-run, marker metrics
- bounded summaries of segment hypotheses
- probe snapshots for debugging and tuning

Rendering stays budgeted and non-blocking for ingest.

## 5) Sync/fingerprint probes (Rosetta layer)

Optional research probes:

- modular fingerprints (`N mod p_i`)
- magnitude/log2 bounds via asymptotics

These remain outside core correctness until dilation semantics for each probe are fully locked.

## What is intentionally out of scope (for now)

- broad claims about universal LLM correction or orchestration
- application narratives that require semantics not defined in SPEC
- expensive probes promoted to mandatory ingest behavior

## Compatibility rule

A candidate application is valid only if it preserves:

- O(1)/amortized O(1) ingest updates
- no stuffed-zero materialization
- linear memory growth in observed bits
- budgeted rendering with graceful degradation