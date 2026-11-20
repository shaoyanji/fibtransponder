#!/usr/bin/env bash
set -euo pipefail

exec python3 - "$0" "$@" <<'PY'
import csv
import hashlib
import json
import os
from pathlib import Path
import shutil
import statistics
import subprocess
import sys
import tempfile
import time


def main():
    if len(sys.argv) != 4:
        raise RuntimeError("usage: scripts/nixeval-trial.sh NIXCONFIG_SOURCE OUTPUT_PARENT")
    script, source, parent = map(lambda s: Path(s).resolve(), sys.argv[1:])
    if not (source / ".git").exists():
        raise RuntimeError("source must be a nixconfig git checkout")
    if parent == source or source in parent.parents:
        raise RuntimeError("output must not be inside the source checkout")
    parent.mkdir(parents=True, exist_ok=True)
    root = Path(tempfile.mkdtemp(prefix="nixeval-trial-", dir=parent))
    repo, logs = root / "repo", root / "logs"
    logs.mkdir()
    env = os.environ.copy()
    for key in ("KG_GRAPH", "KG_BACKEND", "IETS_BIN", "IETS_CACHE", "REPO"):
        env.pop(key, None)
    env["XDG_CACHE_HOME"] = str(root / "xdg-cache")
    commands, samples = [], []
    print(f"Artifacts: {root}", flush=True)

    def run(args, label, cwd=None, check=True):
        args = list(map(str, args))
        start = time.perf_counter_ns()
        result = subprocess.run(args, cwd=cwd, env=env, capture_output=True, timeout=600)
        ms = (time.perf_counter_ns() - start) / 1_000_000
        prefix = logs / f"{len(commands):04d}-{label}"
        prefix.with_suffix(".stdout").write_bytes(result.stdout)
        prefix.with_suffix(".stderr").write_bytes(result.stderr)
        commands.append(dict(args=args, cwd=str(cwd or Path.cwd()), ms=ms,
                             exit=result.returncode, log=str(prefix)))
        (root / "commands.json").write_text(json.dumps(commands, indent=2) + "\n")
        if check and result.returncode:
            raise RuntimeError(f"{label}: exit {result.returncode}: "
                               f"{result.stderr.decode(errors='replace')[-4000:]} (logs: {prefix})")
        return result.stdout, ms, result.returncode

    def clone(src, dest, revision=None):
        run(["git", "clone", "--no-hardlinks", "--no-checkout", src, dest], "clone")
        if revision is None:
            revision = run(["git", "-C", src, "rev-parse", "HEAD"], "source-head")[0].decode().strip()
        run(["git", "-C", dest, "checkout", "--detach", revision], "checkout")
        listing = run(["git", "-C", dest, "ls-files", "--stage", "-z"], "gitlinks")[0]
        for entry in listing.split(b"\0"):
            if not entry:
                continue
            meta, file = entry.split(b"\t", 1)
            mode, commit, stage = meta.split()
            if mode != b"160000":
                continue
            local = src / os.fsdecode(file)
            target = dest / os.fsdecode(file)
            if (local / ".git").exists():
                clone(local, target, commit.decode())
            else:
                print(f"Uninitialised source gitlink left uninitialised: {file!r}", flush=True)

    clone(source, repo)
    binary = root / "nixeval"
    run(["go", "build", "-o", binary, "./cmd/nixeval"], "build", script.parent.parent)
    revision = run(["git", "rev-parse", "HEAD"], "revision", repo)[0].decode().strip()
    print(f"Source HEAD: {revision}; runs: 5; clock: perf_counter_ns; units: ms", flush=True)
    print("Cold means fresh engine cache, not a flushed OS/Nix store cache.", flush=True)

    def engine(cache, op):
        return run([binary, "-repo", repo, "-cache", cache, op], op)

    def status(cache):
        return json.loads(engine(cache, "status")[0])

    def graph(cache, op="graph"):
        out, ms, _ = engine(cache, op)
        file = Path(os.fsdecode(out).strip())
        data = json.loads(file.read_bytes())
        if not isinstance(data.get("nodes"), list) or not isinstance(data.get("edges"), list):
            raise RuntimeError("cached artifact is not a graph")
        return data, ms

    def parity(cache):
        out = engine(cache, "parity")[0].decode().strip()
        if not out.startswith("parity PASS "):
            raise RuntimeError(f"parity did not pass: {out}")
        return out

    def sample(phase, number, ms, s=None, file="", step=0, verified=False):
        samples.append(dict(phase=phase, run=number, step=step, ms=ms,
                            file=file, status=s, parity=verified))
        (root / "samples.json").write_text(json.dumps(samples, indent=2) + "\n")

    bootstrap = root / "cache-bootstrap"
    core, _ = graph(bootstrap, "leap")
    tracked = set(os.fsdecode(p) for p in run(["git", "ls-files", "-z"], "tracked", repo)[0].split(b"\0") if p)
    docs = sorted({n["path"] for n in core["nodes"] if n.get("type") == "doc"
                   and n.get("path", "").startswith("docs/") and n["path"].endswith(".md")
                   and n["path"] in tracked and (repo / n["path"]).is_file()
                   and n["path"] != ".agents/skills/router/references/playbooks.md"})
    if len(docs) < 12:
        raise RuntimeError(f"need 12 eligible docs under docs/, found {len(docs)}")
    docs = docs[:12]
    originals = {p: (repo / p).read_bytes() for p in docs}
    first = docs[0]

    def edit(file, tag):
        (repo / file).write_bytes(originals[file] + f"\nNixeval trial body sample {tag}.\n".encode())

    def restore():
        for file, content in originals.items():
            (repo / file).write_bytes(content)

    def ensure_incremental(s, epoch, n):
        if s["epoch"] != epoch or s["n"] != n or s.get("leap_reason"):
            raise RuntimeError(f"expected incremental epoch with n={n}, got {s}")

    for i in range(1, 6):
        cache = root / f"cache-cold-{i}"
        _, ms = graph(cache, "leap")
        s = status(cache)
        if s.get("leap_reason") != "explicit leap" or s["n"] != 0:
            raise RuntimeError(f"cold leap failed: {s}")
        sample("cold leap", i, ms, s)

    warm = root / "cache-warm"
    graph(warm, "leap")
    for i in range(1, 6):
        _, ms = graph(warm)
        s = status(warm)
        if not s["quotient_hit"]:
            raise RuntimeError("warm graph missed quotient")
        sample("warm hit", i, ms, s)

    incremental = root / "cache-incremental"
    graph(incremental, "leap")
    epoch = status(incremental)["epoch"]
    for i in range(1, 6):
        edit(first, f"incremental-{i}")
        _, ms = graph(incremental)
        s = status(incremental)
        ensure_incremental(s, epoch, 1)
        parity(incremental)
        sample("incremental doc edit", i, ms, s, first, verified=True)
    restore()

    print("\nIndividual walks (parity time excluded):", flush=True)
    for i in range(1, 6):
        cache = root / f"cache-walk-{i}"
        graph(cache, "leap")
        epoch = status(cache)["epoch"]
        for step, file in enumerate(docs, 1):
            edit(file, f"walk-{i}-{step}")
            _, ms = graph(cache)
            s = status(cache)
            ensure_incremental(s, epoch, step)
            verdict = parity(cache)
            sample("walk", i, ms, s, file, step, verified=True)
            hits = ",".join(f"{b['term']}:{'H' if b['hit'] else 'M'}" for b in s["segments"])
            print(f"walk={i} step={step:02d} {ms:.3f} ms terms={s['terms']} "
                  f"segments=[{hits}] {verdict}", flush=True)
        restore()

    cited = next(((n["path"], cite) for n in core["nodes"] if n.get("type") == "doc"
                  and n.get("path") in tracked and n.get("path") != ".agents/skills/router/references/playbooks.md"
                  and (repo / n["path"]).is_file() for cite in n.get("cites", [])
                  if f"[[flake/policies.nix#{cite}]]".encode() in (repo / n["path"]).read_bytes()), None)
    if cited is None:
        raise RuntimeError("no eligible doc has a removable policy citation")
    cite_file, cite = cited
    cite_original = (repo / cite_file).read_bytes()
    nix_file = "lib/knowledge-graph.nix"
    if nix_file not in tracked:
        nix_file = next(p for p in sorted(tracked) if p.endswith(".nix") and (repo / p).is_file())
    nix_original = (repo / nix_file).read_bytes()
    for phase, file, original in (("cites-change leap", cite_file, cite_original),
                                  (".nix-edit leap", nix_file, nix_original)):
        for i in range(1, 6):
            cache = root / f"cache-trigger-{phase.split()[0]}-{i}"
            graph(cache, "leap")
            before = status(cache)["epoch"]
            if phase == "cites-change leap":
                changed = original.replace(f"[[flake/policies.nix#{cite}]]".encode(), b"")
                reason = "fragmenter requested leap"
            else:
                changed = original + f"\n# nixeval trial core edit {i}\n".encode()
                reason = "core key or eligible path set changed"
            (repo / file).write_bytes(changed)
            _, ms = graph(cache)
            s = status(cache)
            if s.get("leap_reason") != reason or s["n"] != 0 or s["epoch"] == before:
                raise RuntimeError(f"{phase} failed: {s}")
            parity(cache)
            sample(phase, i, ms, s, file, verified=True)
            (repo / file).write_bytes(original)

    baseline = ["bash", "-c", '. scripts/task/lib/kg-graph.sh; kg_graph_path']
    env["REPO"] = str(repo)
    run(baseline, "baseline-prime", repo)
    resolved, _, code = run(["bash", "-c", '. scripts/task/lib/iets.sh; resolve_iets'],
                            "baseline-resolver", repo, check=False)
    backend = resolved.decode().strip() if code == 0 else "nix eval fallback (iets unavailable)"
    print(f"\nBaseline default evaluator: {backend}", flush=True)
    for i in range(1, 6):
        edit(first, f"baseline-{i}")
        for phase in ("HEAD+diff dirty", "HEAD+diff warm"):
            out, ms, _ = run(baseline, "baseline", repo)
            json.loads(Path(os.fsdecode(out).strip()).read_bytes())
            sample(phase, i, ms, file=first)
    restore()
    for i in range(1, 6):
        out, ms, _ = run(["nix", "eval", "--json", ".#knowledgeGraph.graph"], "plain-nix", repo)
        json.loads(out)
        sample("plain nix eval", i, ms)

    dirty = run(["git", "status", "--porcelain", "--untracked-files=no"], "final-status", repo)[0]
    if dirty.strip():
        raise RuntimeError(f"trial failed to restore disposable checkout: {dirty!r}")

    def stats(rows):
        times = [r["ms"] for r in rows]
        return f"{statistics.median(times):.3f} | {min(times):.3f} | {max(times):.3f}"

    report = [f"Artifacts: {root}", f"Source HEAD: {revision}", f"Baseline default evaluator: {backend}",
              "Five runs per row; all times in ms; build/setup/parity excluded.",
              "Cold = fresh engine cache, not flushed OS/Nix caches.", "",
              "| Path | Median ms | Min ms | Max ms |", "|---|---:|---:|---:|"]
    for phase in ("cold leap", "warm hit", "incremental doc edit", "cites-change leap", ".nix-edit leap",
                  "HEAD+diff dirty", "HEAD+diff warm", "plain nix eval"):
        report.append(f"| {phase} | {stats([r for r in samples if r['phase'] == phase])} |")
    report += ["", "Walk: five independent walks; H=segment hit, M=miss; parity PASS after every edit.",
               "| Step | Doc | Median ms | Min ms | Max ms | Terms | Segment H/M | Parity |",
               "|---:|---|---:|---:|---:|---|---|---|"]
    for step, file in enumerate(docs, 1):
        rows = [r for r in samples if r["phase"] == "walk" and r["step"] == step]
        patterns = {(tuple(r["status"]["terms"]), tuple(b["hit"] for b in r["status"]["segments"])) for r in rows}
        if len(patterns) != 1:
            raise RuntimeError(f"inconsistent carry/cache pattern at step {step}: {patterns}")
        s = rows[-1]["status"]
        hits = ", ".join(f"{b['term']}:{'H' if b['hit'] else 'M'}" for b in s["segments"])
        report.append(f"| {step} | {file} | {stats(rows)} | {s['terms']} | {hits} | PASS 5/5 |")
    report += ["", f"Citation trigger: {cite_file} (removed {cite}); .nix trigger: {nix_file}.",
               "PR #8 Elixir: cold leap 2618–4701 ms; incremental 589–1153 ms; warm hit 761–1101 ms;",
               "HEAD+diff dirty 562–1286 ms; HEAD+diff warm 35–271 ms.",
               "Disposable checkout restored; source checkout was only read."]
    (root / "report.md").write_text("\n".join(report) + "\n")
    with (root / "samples.csv").open("w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["phase", "run", "step", "ms", "file", "status", "parity"])
        for r in samples:
            writer.writerow([r[k] if k != "status" else json.dumps(r[k]) for k in
                             ("phase", "run", "step", "ms", "file", "status", "parity")])
    (root / "metadata.json").write_text(json.dumps(dict(source=str(source), revision=revision,
        binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(), backend=backend, runs=5), indent=2) + "\n")
    print("\n" + "\n".join(report), flush=True)


try:
    main()
except (RuntimeError, OSError, subprocess.SubprocessError) as error:
    print(f"nixeval-trial: {error}", file=sys.stderr)
    sys.exit(1)
PY
