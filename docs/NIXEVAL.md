# Nix eval epoch channels

`internal/nixeval` is a persistent, in-process Go cache for full Nix evaluations
with file-local updates. `internal/nixeval/kgdoc` ports nixconfig's document
projection from `tools/kg-zeck/kg_zeck.exs`; `cmd/nixeval` exposes that channel.
This is separate from the FSVM and does not change deltaqueue replay semantics.

## Model

An `Engine` hosts named `Channel`s. Each channel names a repository, a flake
attribute, an eligibility predicate, an `Evaluator`, and a `Fragmenter`. Each
channel has one owning goroutine and receives requests over a Go channel. Calls
within a channel are serialized; different channels can run in parallel. The API
accepts contexts, and `Close` cancels work and waits for the workers. Callers must
make their evaluator and fragmenter concurrency-safe if they share instances
across channels.

The default evaluator runs `nix eval --json <attr>` in the repository, then tries
`nix eval --impure --json <attr>` if the first attempt fails or returns invalid
JSON. Tests use a fake evaluator and temporary git repositories.

### Epoch and core key

An epoch stores the full evaluation result, the SHA256 of every eligible file,
and a core key. The core key covers the tracked non-eligible files' working-tree
content and index modes, plus the eligible path set. `git ls-files --stage -z`
defines the tracked files. Untracked files do not contribute. Initialized
gitlinks are hashed recursively; uninitialized gitlinks use a sentinel, not the
parent repository's contents. Symlinks contribute their link text.

The epoch identity is a SHA256 digest of the core key and eligible-file hashes.
It is content-addressed, so an explicit leap on unchanged inputs can retain the
same epoch identity even though it runs a new full evaluation.

A persisted `(size, mtime, mode)` stat cache avoids rehashing unchanged regular
files. Full evaluations force fresh hashes before and after evaluation and
reject concurrent input changes. Ordinary reads still rely on stat metadata;
an edit that preserves both size and mtime can evade this cache. The engine does
not provide a filesystem snapshot or a hermetic evaluation of external inputs.

### Ingest log, segments, and quotient

The log records eligible files in first-observed-divergence order. A re-edit
updates that entry's SHA in place. A reverted file keeps its original position
with its current SHA, matching Elixir. Removing it would shift later entries and
change their segment keys. Newly observed changes within one scan use path order
as a deterministic tie-breaker.

`deltaqueue.ZeckendorfTerms` carves the log greedily into Fibonacci-sized blocks,
largest first, with the oldest entries in the largest block. For example,
12 entries become `[8, 3, 1]`. Appending an entry can reuse older blocks, but a
Fibonacci carry can replace them: `[5, 2]` becomes `[8]`.

A segment key is a length-delimited, order-sensitive 64-bit word-mixed hash over
`(epoch, position, ordered (path, SHA) entries)`. Cached segments contain ordered
file fragments relative to the full core result. A quotient key hashes the epoch
and ordered segment keys. On a quotient hit, `Get` returns the stored result
without fragment or merge work. Otherwise it loads or computes each segment's
fragments, merges them in log order, and stores the result under the quotient.
These word-mixed keys are not cryptographic identities and have no collision
guarantee.

Each channel has a cache directory keyed by its name, absolute repository path,
flake attribute, and `CacheVersion`. The manifest holds the base result, log,
hashes, stat cache, and status; segment and result JSON files are separate.
Writes use temporary files and rename. An OS file lock also serializes writers
from independent engines or CLI processes sharing that channel's cache. There
is no cache eviction policy. Change `CacheVersion` when changing a fragmenter's
semantics.

### Leap triggers and status

A leap runs the full evaluator and resets the log when:

- No epoch exists, or the caller explicitly requests `Leap`.
- The core key or eligible path set changes, including tracking/untracking a doc.
- An eligible file is deleted or restored.
- The fragmenter requests a leap.
- Log length reaches `F_E`. The default Fibonacci index is 10, so the threshold
  is 55. Supported indices are 2 through 34.

`Status` reports the last successful call's epoch, log length `n`, terms,
per-segment cache hits/misses, quotient hit, and leap reason. It does not scan
for changes or predict what the next call will do. A reason is absent if that
call did not leap.

## KG document channel

The full target is `.#knowledgeGraph.graph`. The eligible set is the core
graph's `doc` nodes except
`.agents/skills/router/references/playbooks.md`, restricted to tracked paths.
The generic engine's optional `BaseEligibility` interface supplies this set.
A newly tracked file outside that set invalidates the core and gets a full eval.

The fragmenter preserves the Elixir projection:

- The first line starting with `# ` supplies the label, including trailing
  spaces. Without one, the label is `(untitled)`.
- ASCII word tokens match host and skill labels exactly. Mention edges follow
  the core graph's host/skill vocabulary order, not text occurrence order.
- Relative Markdown links resolve against the document directory, omit anchors,
  reject self/external/absolute links, and reference only core doc paths.
- Policy citations use `[[flake/policies.nix#id]]` syntax. Fragments include only
  valid, known policy IDs, in first-occurrence order without duplicates.
- A syntactically valid citation set different from the core node's `cites`
  requires a leap, including newly mentioned IDs absent from the core. This
  allows a policy's last citation to remove its node, and a restored citation to
  restore it through a full eval.

Merge replaces updated nodes and their outgoing `mentions`, `references`, and
`cites` edges. It preserves other edge types and graph fields, then updates
`stats.edges` and `stats.recordedEdges` (edges having an `evidence` key).

### Differences from kg_zeck.exs

- Segment keys bind position and ordered entries. Elixir hashes sorted entries
  within a segment without position. Go uses word mixing for segment/quotient
  keys instead of SHA256 over Erlang terms; cache files are not interchangeable.
- New divergences observed together use path order rather than graph-node order.
- Core keys use working-tree content and tracked modes/path sets, not index blob
  IDs. Staging unchanged content does not itself invalidate the epoch.
- `Status` describes the last successful call rather than Elixir's speculative
  next-leap inspection.
- The Fibonacci index limit is 34, not Elixir's 40, to stay within the existing
  shared Zeckendorf helper's range. The default remains 10 (55 entries).
- The generic engine has persisted stat caching, named goroutine-owned channels,
  context cancellation, and caller-supplied evaluator/fragmenter interfaces.

Revert handling is not a deviation: log positions are retained in both versions.

## CLI

Requires Go 1.25.7, git, Nix, and a nixconfig checkout. The engine's file locking
uses Unix `flock`. Build the binary outside the checkout if it should not appear
in `git status`:

```bash
go build -o "$HOME/nixeval" ./cmd/nixeval
"$HOME/nixeval" -repo "$HOME/repos/nixconfig" graph
"$HOME/nixeval" -repo "$HOME/repos/nixconfig" status
"$HOME/nixeval" -repo "$HOME/repos/nixconfig" leap
"$HOME/nixeval" -repo "$HOME/repos/nixconfig" parity
```

Flags precede the command:

```text
nixeval [-repo DIR] [-cache DIR] [-epoch 10] graph|leap|status|parity
```

`-repo` defaults to the current directory. The default cache root is
`${XDG_CACHE_HOME:-$HOME/.cache}/fibtransponder/nixeval/`.

- `graph` prints only the persistent result JSON path, for a `kg_graph_path`
  caller to read.
- `leap` forces full evaluation and prints the result path.
- `status` prints the last successful status as JSON, or an empty initial status.
- `parity` resolves the engine result, evaluates the target afresh, and compares
  the complete graphs after sorting the node and edge arrays. It preserves
  duplicate counts and compares stats and other fields too. A mismatch exits 1.

Library checks, with no real Nix evaluation:

```bash
go vet ./...
go test -race -count=1 ./internal/nixeval/... ./cmd/nixeval/
make ci
```

## Reproduce the trial

From this repo:

```bash
scripts/nixeval-trial.sh "$HOME/repos/nixconfig" "$HOME/trial"
```

The script requires Bash and Python 3 as well as the CLI build dependencies. It
creates its own clone at the source's HEAD, local copies of initialized
submodules, binary, caches, logs, and report under a new directory in the output
parent. It does not copy uncommitted source edits or edit the source checkout.
It refuses an output parent inside the source checkout.

There are five samples per timing row and five independent 12-doc walks. Each
walk starts with a fresh primed epoch and appends a body-only edit to each of 12
distinct docs. Every walk step runs parity after timing `graph`. Citation and
`.nix` trigger measurements also assert a leap and run parity. Setup, building,
priming, status inspection, and parity are outside the measured intervals.

The baseline runs nixconfig's unmodified default resolver:

```bash
REPO="$clone" bash -c '. scripts/task/lib/kg-graph.sh; kg_graph_path'
```

Each dirty sample changes the working tree; its paired warm sample repeats that
same read. The baseline gets an isolated `XDG_CACHE_HOME`, with tool resolution
and initial priming outside the timing. The script also times five plain
`nix eval --json .#knowledgeGraph.graph` calls. It writes `report.md`,
`samples.json`, `samples.csv`, `metadata.json`, and per-command stdout/stderr logs.

## Intel Xeon VM trial, 2026-11-19

Linux amd64 VM, CPU reported as `Intel(R) Xeon(R) Processor`, Go 1.25.7.
Nixconfig HEAD was `d8f364b1baf76896251b25172ae042295867abae`; the default
baseline resolved to `iets`. All times below are one-shot process wall times in
milliseconds, five samples per row.

| Path | Median ms | Min ms | Max ms | PR #8 Elixir-era range ms |
|---|---:|---:|---:|---:|
| Cold leap | 2282.405 | 2261.589 | 2504.161 | 2618–4701 |
| Warm hit | 43.177 | 42.567 | 44.555 | 761–1101 |
| Incremental doc edit | 78.415 | 72.580 | 94.061 | 589–1153 |
| Cites-change leap | 2430.608 | 2309.317 | 4343.563 | — |
| .nix-edit leap | 2763.950 | 2241.946 | 3693.918 | — |
| HEAD+diff dirty | 533.105 | 525.151 | 536.596 | 562–1286 |
| HEAD+diff warm | 20.528 | 19.957 | 20.910 | 35–271 |
| Plain nix eval | 2068.143 | 2054.196 | 2113.264 | — |

PR #8's ranges are the supplied historical results, not an Elixir rerun on this
VM. The Go incremental median was about 6.8 times faster than this trial's
HEAD+diff dirty median. Its warm hit was about 2.1 times slower than the default
backend's HEAD+diff warm path. Full evaluation still costs roughly two seconds.

### Walk summary

H is a segment hit; M is a miss. Patterns agreed across all five walks. Each
step's parity passed 5/5 times, with 1991 nodes and 1800 edges.

| Step | Document under docs/ | Median ms | Min–max ms | Terms | Segment H/M | Parity |
|---:|---|---:|---:|---|---|---|
| 1 | NUSHELL.md | 78.090 | 73.305–83.590 | [1] | 1:M | PASS 5/5 |
| 2 | agent-contract.md | 84.141 | 83.119–111.168 | [2] | 2:M | PASS 5/5 |
| 3 | agent-turns.md | 89.965 | 88.708–92.459 | [3] | 3:M | PASS 5/5 |
| 4 | appflow-board.md | 74.396 | 72.170–86.233 | [3, 1] | 3:H, 1:M | PASS 5/5 |
| 5 | blog-drafting.md | 102.195 | 99.642–202.424 | [5] | 5:M | PASS 5/5 |
| 6 | ci-pipeline.md | 74.294 | 72.615–78.925 | [5, 1] | 5:H, 1:M | PASS 5/5 |
| 7 | closure-debloat.md | 83.454 | 81.134–85.861 | [5, 2] | 5:H, 2:M | PASS 5/5 |
| 8 | closure-reduction.md | 123.935 | 122.710–144.819 | [8] | 8:M | PASS 5/5 |
| 9 | control-plane-architecture.md | 74.436 | 72.206–79.355 | [8, 1] | 8:H, 1:M | PASS 5/5 |
| 10 | control-plane-vision.md | 82.053 | 80.951–111.641 | [8, 2] | 8:H, 2:M | PASS 5/5 |
| 11 | deepseek-v4-flash.md | 88.593 | 87.985–90.313 | [8, 3] | 8:H, 3:M | PASS 5/5 |
| 12 | delphi-install.md | 76.003 | 72.860–80.033 | [8, 3, 1] | 8:H, 3:H, 1:M | PASS 5/5 |

The citation trigger removed `router-labels-are-routed` from the clone's
`AGENTS.md`; all five calls leaped with `fragmenter requested leap`. Appending a
comment to `lib/knowledge-graph.nix` produced five core-key leaps. Both trigger
groups passed parity after every edit.

### Limits and warm-path cost

Cold here means a fresh engine cache only. OS caches and the Nix store were not
flushed, and the trial bootstrapped the graph before collecting samples. These
are measurements of this checkout and workload, not a general Nix acceleration
claim. This trial did not exercise the default 55-entry epoch threshold.

A warm quotient hit still loads and decodes the manifest/base graph, runs
`git ls-files` and stat scans twice (including initialized gitlinks), parses the
graph for eligibility, reads the stored result, and rewrites/fsyncs the manifest.
One separate `strace -f -c` warm read observed 2779 `newfstatat` calls, seven
`execve` calls, and one manifest fsync. That traced run is not part of the timing
table, and syscall timing under tracing does not apportion the untraced 43 ms.
There is no phase-level wall-time profile yet.

Raw trial artifacts on the measurement VM are under
`/home/ubuntu/trial/nixeval-trial-xfi0djx1/`; they are not committed. The report
above and the trial script are the repository record.
