/* ============================================================================
   Small DOM and canvas helpers shared by the demo pages.
   ========================================================================= */

/** Query a required element, loudly. A silent null here becomes a blank demo. */
export function need<T extends Element = HTMLElement>(sel: string, root: ParentNode = document): T {
  const el = root.querySelector<T>(sel);
  if (!el) throw new Error(`required element not found: ${sel}`);
  return el;
}

export function needAll<T extends Element = HTMLElement>(
  sel: string,
  root: ParentNode = document,
): T[] {
  return Array.from(root.querySelectorAll<T>(sel));
}

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Record<string, string> = {},
  ...children: Array<Node | string>
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === 'class') node.className = v;
    else node.setAttribute(k, v);
  }
  for (const c of children) node.append(c);
  return node;
}

export function setText(node: Element, text: string): void {
  if (node.textContent !== text) node.textContent = text;
}

/* -------------------------------------------------------------------------- */
/* Formatting                                                                 */
/* -------------------------------------------------------------------------- */

export function fmtInt(v: number): string {
  return v.toLocaleString('en-US');
}

/**
 * Format a rate to a fixed number of decimals, without ever rendering "-0".
 */
export function fmtRate(v: number, places = 4): string {
  const s = v.toFixed(places);
  return s === `-${(0).toFixed(places)}` ? (0).toFixed(places) : s;
}

/** Group a 64-bit value with thin separators so its magnitude is readable. */
export function fmtU64(v: string): string {
  try {
    return BigInt(v).toLocaleString('en-US');
  } catch {
    return v;
  }
}

/** Hex form, for the places the repo itself reports in hex. */
export function fmtHex(v: string): string {
  try {
    return `0x${BigInt(v).toString(16).padStart(16, '0')}`;
  } catch {
    return v;
  }
}

export function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(2)} MB`;
}

/* -------------------------------------------------------------------------- */
/* Motion preference                                                          */
/* -------------------------------------------------------------------------- */

const motionQuery =
  typeof matchMedia === 'function' ? matchMedia('(prefers-reduced-motion: reduce)') : null;

export function prefersReducedMotion(): boolean {
  return motionQuery?.matches ?? false;
}

/** True when the visitor has asked for reduced motion, re-read on every call. */
export function shouldAnimate(): boolean {
  return !prefersReducedMotion();
}

/* -------------------------------------------------------------------------- */
/* Rendering loop                                                              */
/* -------------------------------------------------------------------------- */

/**
 * A cancellable animation loop. Every demo's playback is driven through this so
 * there is exactly one place that knows about reduced motion.
 */
export class Ticker {
  private raf = 0;
  private last = 0;
  private running = false;

  constructor(
    private readonly step: (dtMs: number) => void,
    private readonly fps = 60,
  ) {}

  start(): void {
    if (this.running) return;
    this.running = true;
    this.last = performance.now();
    const loop = (now: number) => {
      if (!this.running) return;
      // Clamp dt so a backgrounded tab does not resume with a giant jump.
      const dt = Math.min(now - this.last, 100);
      this.last = now;
      this.step(dt);
      this.raf = requestAnimationFrame(loop);
    };
    this.raf = requestAnimationFrame(loop);
  }

  stop(): void {
    this.running = false;
    cancelAnimationFrame(this.raf);
  }

  get isRunning(): boolean {
    return this.running;
  }
}

/** Yield to the event loop so a long Go/WASM call cannot freeze the tab. */
export function nextFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/* -------------------------------------------------------------------------- */
/* Canvas                                                                      */
/* -------------------------------------------------------------------------- */

/**
 * Size a canvas for the device pixel ratio and return a context scaled to CSS
 * pixels, so nothing is drawn at half resolution on a HiDPI display.
 */
export function setupCanvas(
  canvas: HTMLCanvasElement,
  cssWidth: number,
  cssHeight: number,
): CanvasRenderingContext2D {
  const dpr = Math.min(globalThis.devicePixelRatio || 1, 2.5);
  canvas.width = Math.round(cssWidth * dpr);
  canvas.height = Math.round(cssHeight * dpr);
  canvas.style.width = `${cssWidth}px`;
  canvas.style.height = `${cssHeight}px`;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('2d canvas context unavailable');
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  return ctx;
}

/** Read a design token so canvas drawing matches the CSS theme exactly. */
export function token(name: string, fallback = '#888'): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

export function withAlpha(color: string, alpha: number): string {
  const c = color.trim();
  if (c.startsWith('#')) {
    const hex = c.slice(1);
    const full =
      hex.length === 3
        ? hex
            .split('')
            .map((ch) => ch + ch)
            .join('')
        : hex;
    const r = parseInt(full.slice(0, 2), 16);
    const g = parseInt(full.slice(2, 4), 16);
    const b = parseInt(full.slice(4, 6), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  return c;
}

/* -------------------------------------------------------------------------- */
/* Misc                                                                        */
/* -------------------------------------------------------------------------- */

/** Render a status region, preferring an assertive role for errors. */
export function setStatus(node: HTMLElement, message: string, kind: 'info' | 'error' | 'ok' = 'info'): void {
  setText(node, message);
  node.dataset.kind = kind;
}

export function debounce<T extends unknown[]>(
  fn: (...args: T) => void,
  ms: number,
): (...args: T) => void {
  let handle = 0;
  return (...args: T) => {
    clearTimeout(handle);
    handle = setTimeout(() => fn(...args), ms) as unknown as number;
  };
}
