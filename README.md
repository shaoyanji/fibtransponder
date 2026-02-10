# fibtransponder

A deterministic, streaming, **Zeckendorf/Fibonacci-radix** transponder experiment with:

- **Canonical constraint:** Zeckendorf digits over **F2-based indexing** (bit 0 ↔ F2=1) and **no adjacent 1s** in canonical form.
- **Dilation protocol:** Adjacent `11` in the *observed* stream triggers a **global retrospective dilation event** `r++` interpreted as **virtual zero-stuffing** between all digits (upsample-by-2). This is done **without materializing stuffed zeros**.
- **Segmentation:** Optional/allowed message segmentation induced by long runs of zeros; treated as an **interpretation layer** (regular-language/NFA), not a mutation.
- **Markers (future):** “Rosetta stone” checkpoints bridging (a) fib radix legality/rewrites, (b) Binet/log2 magnitude bounds, and (c) modular/binary fingerprints.

## Status
Deliverables in this folder are:
- a written spec + design notes (`docs/`)
- a functional Go TUI application that processes and visualizes bitstreams in real-time.
- an API service providing programmatic access to the state machine.
- a Go package for core components (`internal/fsvm`, `internal/bitrope`).
- **Multiple analysis modules (`internal/segauto`, `internal/rosetta`, `internal/signal`, `internal/typing_analyzer`, `internal/entropy_estimator`) implemented as pluggable `extension.Extension`s.**
- **Python reference implementation** (FSVM + bit rope + WHT + 2D embed demo) for comparison and further exploration.

## Folder structure
- `docs/` — specs and design
- `internal/bitrope` — append-only bit rope (immutable blocks)
- `internal/extension` — defines the `Extension` interface for pluggable analysis modules.
- `internal/fsvm` — core streaming state machine (dilation, hexagram window, counters)
- `internal/render` — bounded-budget rendering strategy (exemplars, summaries)
- `internal/rosetta` — marker/probe plan (log2/Binet + residues), now an `Extension`.
- `internal/segauto` — segmentation automaton (NFA), now an `Extension`.
- `internal/signal` — applications layer: boolean transport → windowing, transforms (FFT/WHT), decomposition, now an `Extension` with feature extraction.
- `internal/typing_analyzer` — analysis of typing patterns, now an `Extension`.
- `internal/entropy_estimator` — bitstream entropy estimation, now an `Extension`.
- `cmd/fibtransponder` — CLI skeleton (currently not actively developed, TUI/API are primary interfaces)
- `cmd/tui` — TUI application using `charmbracelet/bubbletea` for real-time bitstream visualization, dynamically loads `Extension`s.
- `cmd/api` — RESTful API service for programmatic access to the state machine, dynamically loads `Extension`s.

## Go toolchain
The Go code is functional and can be built and run.

### TUI Application (`cmd/tui`)

The TUI application provides a real-time, interactive visualization of the bitstream processing, FSVM state, and various analysis modules (all `Extension`s). It's built using `charmbracelet/bubbletea`.

To build and run the TUI application:

```bash
cd ~/.openclaw/workspace/projects/fibtransponder/cmd/tui
go build -o fibtransponder_tui
./fibtransponder_tui
```

You can pipe a stream of '0's and '1's into it, e.g.:
```bash
echo "010101100101" | ~/.openclaw/workspace/projects/fibtransponder/cmd/tui/fibtransponder_tui
```

### API Service (`cmd/api`)

The API service provides a RESTful interface to interact with the state machine and its analysis modules programmatically. This allows for integration with other applications, automated testing, or building custom frontends.

**To run the API service:**

```bash
cd ~/.openclaw/workspace/projects/fibtransponder/cmd/api
go build -o fibtransponder_api
./fibtransponder_api
```

The API service will start on `http://localhost:8080`.

**Endpoints:**

*   **`POST /api/sessions`**: Create a new state machine session.
    *   **Request:** `{"initial_bits": "0101..."}` (optional)
    *   **Response:** `{"sessionId": "uuid", "fsvmState": {...}, "extensionOutputs": [...]}`
    *   **Example:**
        ```bash
        curl -X POST http://localhost:8080/api/sessions -H "Content-Type: application/json" -d '{"initial_bits": "01101"}'
        ```

*   **`POST /api/sessions/{session_id}/process`**: Process a sequence of bits for a given session.
    *   **Request:** `{"bits": "0101..."}`
    *   **Response:** `{"sessionId": "uuid", "fsvmState": {...}, "extensionOutputs": [...]}` (updated state)
    *   **Example:**
        ```bash
        SESSION_ID="<get-from-create-response>" # e.g., "a1b2c3d4-e5f6-7890-1234-567890abcdef"
        curl -X POST http://localhost:8080/api/sessions/$SESSION_ID/process -H "Content-Type: application/json" -d '{"bits": "11001"}'
        ```

*   **`GET /api/sessions/{session_id}`**: Retrieve the current state and all analysis results for a session.
    *   **Response:** `{"sessionId": "uuid", "fsvmState": {...}, "extensionOutputs": [...]}`
    *   **Example:**
        ```bash
        SESSION_ID="<get-from-create-response>"
        curl http://localhost:8080/api/sessions/$SESSION_ID
        ```

## Key idea (one paragraph)
Maintain a streaming measurement transducer with O(1) update cost per input bit: track a small sliding window (“hexagram”, 6 bits), a global dilation counter `r`, and cheap summary probes. When `11` appears, increment `r` (retrospective dilation) rather than rewriting history. Segmentation is allowed and represented symbolically as a regular language over cut/no-cut choices at sparse “candidate markers” (e.g., zero-run power-of-two crossings), enabling unDoSable rendering of a few representative interpretations.

To run tests for the Go packages:

```bash
cd ~/.openclaw/workspace/projects/fibtransponder
go test ./...
```