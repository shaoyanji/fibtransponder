# Portfolio site

The public site for **fibtransponder**. Static Astro output, deployed to GitHub
Pages by `.github/workflows/pages.yml`.

## The one thing to understand

**Every number on this site is computed by this repository's own Go, compiled to
WebAssembly at build time.** There is no JavaScript re-implementation of any
algorithm and no recorded JSON fixture anywhere in the project.

That is a deliberate constraint. A portfolio site that hard-codes its numbers
rots: change the code, forget the figure, and the page becomes a lie. Here there
is nothing stored to rot, so if a number on the page disagrees with the
repository, a Go test fails first.

The shim is `../internal/wasmapi` — plain Go, no `syscall/js`, unit tested
natively as part of `make ci`. The js-only wrapper is `../cmd/fibwasm`.

## Layout

```
src/
  pages/
    index.astro              overview, headline numbers, retractions preview
    demos.astro              index of all six demos
    retractions.astro        the full retraction log
    method.astro             spec, complexity, benchmarks, how to reproduce
    demo/*.astro             one page per demo
  layouts/
    Base.astro               document shell, fonts, wasm_exec loader
    Demo.astro               demo page shell with prev/next navigation
  components/                header, footer, metric tile, demo card
  data/
    demos.ts                 demo catalogue
    retractions.ts           retraction log and standing claims
  lib/
    fib.ts                   the only module that talks to Go
    ui.ts                    DOM, canvas and motion helpers
  styles/
    tokens.css               the design system
public/wasm/                 build output, gitignored
```

## Commands

```bash
make wasm            # from the repo root: build the WebAssembly module
npm install          # once
npm run dev          # dev server; the wasm is already in public/
npm run build        # → dist/
npm run preview      # serve dist/ with the deployed base path
```

`npm run build` alone will not build the module. Run `make wasm` first; the
artifact lands in `public/wasm/`, which is gitignored because it is build output.

## Conventions worth knowing before editing

**uint64 crosses the wire as a decimal string, never a JSON number.** JavaScript
numbers are float64 and lose integer precision above 2⁵³, which every sketch in
this repository exceeds. The `U64` type in the shim refuses a bare number for
exactly this reason, and `internal/wasmapi`'s tests pin the round trip. Never
`Number()` a sketch; use `toBigInt()` from `lib/fib.ts`.

**Neumorphism is restricted to non-text chrome.** Panels, buttons, slider tracks
and wells get the soft shadow treatment. All text sits on a flat surface at 7:1
or better. This is not a preference: neumorphism's defining property is that its
edges have no contrast against the background, which is a WCAG 1.4.11 failure by
construction. Every neumorphic surface therefore also carries a real border, and
every accent colour in `tokens.css` was solved against both the surface and the
sunk surface to clear AA. `tokens.css` is the reference for this — the comments
there record the measured ratios.

**Colour never carries meaning alone.** Every semantic state — claim, verified,
retracted, most-sensitive cell — is also labelled in words or marked with a
shape, per WCAG 1.4.1.

**Astro cannot interpolate an array into an `<svg>`.** It renders HTML text
nodes, so a mapped array of SVG elements silently produces nothing. Build the
geometry as a single `<path>` with a computed `d` instead; that is what the hero
sunburst does.

**Go/WASM runs synchronously on the browser's event loop.** A single
20,000-stream call is about eight million steps and would freeze the tab. The
heavy sweeps pass an explicit `count` and yield between calls, and pass a
`session` id so the shim can carry its seen-set across calls — otherwise the
distinct counts are per-chunk and summing them over-counts.

**Nothing autoplays.** The stepper's play button exists, but nothing starts on
its own, in any motion preference. The demo is there when the visitor wants it.

## Verification performed

- 34 Go tests in `internal/wasmapi`, plus the repository's own `make ci`
- All ten pages driven headless: no console errors, no page errors
- Every demo exercised for real values, not just rendering — including the
  u64-as-string path, the live falsification, and the compression ratios
- axe-core against WCAG 2.1 A and AA, in both colour schemes: zero serious or
  critical violations
- Reduced motion: nothing autoplays, transitions neutralised, explicit playback
  still works
- Keyboard: 43 focus stops on a demo page, all visible, all with a focus ring;
  the falsification runs from the keyboard alone
- Mobile: no horizontal overflow at 390px

## Deployment

`.github/workflows/pages.yml` runs the Go tests for the shim, builds the wasm,
checks the artifact is a valid module with the right magic number, builds the
site, checks every page and the wasm made it into `dist/`, then deploys.

It is separate from `ci.yml` on purpose: the Go tests are the correctness gate
and must stay fast and dependency-free.
