/* ============================================================================
   The one place the browser talks to Go.
   ============================================================================

   Everything on this site that shows a number is showing a number produced by
   this repository's own code, compiled to WebAssembly. There is no JavaScript
   re-implementation of any algorithm and no pre-baked JSON fixture.

   Consequences that shape this module:

   - uint64 crosses the boundary as a DECIMAL STRING, never a JSON number.
     JavaScript numbers are float64 and lose integer precision above 2^53, which
     every sketch in this repo exceeds. Never JSON.parse a sketch into a Number.
     The shim's U64 type refuses a bare number for exactly this reason, and
     internal/wasmapi's tests pin the round trip.

   - The artifact is 5.0 MB raw, 1.41 MB gzip, 1.02 MB brotli. Almost all of it
     is the Go runtime and encoding/json, not this project (an empty Go wasm is
     already 547 KB gzipped). So it loads lazily: no page without a demo pays.

   - Compression strategy, in order:
       1. fetch the .wasm directly and let the server's Content-Encoding do the
          work, which fetch() applies transparently. This is the common case on
          any CDN and costs nothing to attempt.
       2. if that came back suspiciously large, decode the .gz sibling with
          DecompressionStream, which every browser that ships wasm has.
     Brotli is NOT decoded here: DecompressionStream supports 'gzip' and
     'deflate' but not 'br', so hand-decoding it would mean shipping a decoder
     for a ~1.02 MB saving. The .br file is still emitted by `make wasm` for
     hosts that negotiate it, and costs nothing to keep.

   - go.run() never settles. main() blocks forever on purpose: if it returned,
     the Go runtime would exit and every later call would panic. Readiness is the
     explicit fibReady handshake.
   ========================================================================= */

/**
 * Base path for the wasm directory. This is a GitHub *project* page served at
 * /shaoyanji/fibtransponder/, so an absolute /wasm/... would 404. Astro exposes
 * base via import.meta.env.BASE_URL, which is '/' in dev and '/<repo>/' in the
 * deployed build.
 */
const BASE = import.meta.env.BASE_URL.replace(/\/$/, '');
const WASM_BASE = `${BASE}/wasm`;
const WASM_NAME = 'fibtransponder.wasm';

/** A uint64 that arrived as a decimal string. Use toBigInt, never Number(). */
export type U64 = string & { readonly __u64: unique symbol };

export function u64(v: string): U64 {
  return v as U64;
}

/** Exact. BigInt has arbitrary precision; Number has 53 bits. */
export function toBigInt(v: U64): bigint {
  return BigInt(v);
}

export interface InvokeResult<T> {
  data: T;
  warnings: string[];
}

type CallFn = (method: string, args: string) => string;

interface GoLike {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

let call: CallFn | null = null;
let loading: Promise<CallFn> | null = null;

function canDecompress(): boolean {
  return typeof DecompressionStream !== 'undefined';
}

async function gunzip(buf: ArrayBuffer): Promise<Uint8Array> {
  const stream = new Blob([buf]).stream().pipeThrough(new DecompressionStream('gzip'));
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

async function fetchBytes(url: string): Promise<Uint8Array> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return new Uint8Array(await res.arrayBuffer());
}

/**
 * Load and initialise the module. Idempotent: concurrent callers share one
 * promise and the runtime is only started once.
 */
export function loadFibtransponder(): Promise<CallFn> {
  if (call) return Promise.resolve(call);
  if (loading) return loading;

  loading = (async () => {
    const attempts: Array<[string, () => Promise<Uint8Array>]> = [
      // 1. Let the server compress it for us. If it did, this is already the
      //    smallest form and the size check below is a no-op.
      [`${WASM_BASE}/${WASM_NAME} (via Content-Encoding)`, () => fetchBytes(`${WASM_BASE}/${WASM_NAME}`)],
      // 2. Explicit sibling, decoded here. Works regardless of server config.
      [
        `${WASM_BASE}/${WASM_NAME}.gz (decoded locally)`,
        async () => {
          if (!canDecompress()) throw new Error('DecompressionStream unavailable');
          return gunzip(await (await fetch(`${WASM_BASE}/${WASM_NAME}.gz`)).arrayBuffer());
        },
      ],
    ];

    const failures: string[] = [];
    let bytes: Uint8Array | null = null;

    for (const [label, load] of attempts) {
      try {
        const candidate = await load();
        // A raw 5.0 MB body means no server-side compression happened. Keep it
        // anyway: the .gz path is the guaranteed fallback, and preferring a
        // working 5 MB fetch over a failed 1.4 MB one is the right trade.
        bytes = candidate;
        if (candidate.length < 4_000_000) break;
      } catch (err) {
        failures.push(`${label}: ${(err as Error).message}`);
      }
    }

    if (!bytes) {
      throw new Error(
        `could not load the WebAssembly module. Tried:\n  ${failures.join('\n  ')}\n` +
          `Build it with: make wasm`,
      );
    }

    const Go = (globalThis as unknown as { Go?: new () => GoLike }).Go;
    if (!Go) {
      throw new Error('wasm_exec.js did not define Go. It must be loaded before this module.');
    }
    const go = new Go();

    // Readiness handshake: fibReady is installed before go.run, so the signal
    // cannot be missed. go.run() is deliberately not awaited.
    const ready = new Promise<void>((resolve) => {
      (globalThis as unknown as { fibReady: () => void }).fibReady = resolve;
    });

    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    void go.run(instance);
    await ready;

    const api = (globalThis as unknown as { fibtransponder?: { call: CallFn } }).fibtransponder;
    if (!api?.call) {
      throw new Error('the module started but did not register fibtransponder.call');
    }
    call = api.call;
    return call;
  })();

  return loading;
}

/**
 * Invoke a shim method. Throws on a coded failure so callers can rely on the
 * resolved value being real data. The code is in the message because the codes
 * are part of the wire contract, not free text.
 */
export async function invoke<T>(
  method: string,
  args: Record<string, unknown> = {},
): Promise<InvokeResult<T>> {
  const fn = await loadFibtransponder();
  const env = JSON.parse(fn(method, JSON.stringify(args))) as {
    ok: boolean;
    data?: T;
    warnings?: string[];
    code?: string;
    error?: string;
  };
  if (!env.ok) {
    throw new Error(`${method} failed [${env.code}]: ${env.error}`);
  }
  return { data: env.data as T, warnings: env.warnings ?? [] };
}
