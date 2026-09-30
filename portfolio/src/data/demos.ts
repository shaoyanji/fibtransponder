/**
 * Demo catalogue. One source of truth for the card grid, the /demos index, and
 * the navigation inside each demo page.
 *
 * `tier` drives presentation, not importance: "hero" demos get a full-bleed
 * canvas treatment, "light" demos are compact single-purpose tools.
 */
export interface Demo {
  slug: string;
  title: string;
  /** Kicker above the title, set in small caps. */
  kicker: string;
  /** One sentence for the card. */
  blurb: string;
  /** What the demo actually demonstrates, for the demo page. */
  detail: string;
  tier: 'hero' | 'light';
  /** Shimmer methods this demo calls, shown on the card. */
  methods: string[];
  /** Whether the demo needs a working WebAssembly module. */
  needsWasm: boolean;
}

export const demos: Demo[] = [
  {
    slug: 'fsvm-stepper',
    title: 'The state machine, one bit at a time',
    kicker: 'Hero demo',
    blurb:
      'Step the core by hand. Watch dilation events fire on adjacency, markers fire on zero-runs, and the 64-bit sketch evolve.',
    detail:
      'Every bit of input is shown individually, with the two events the machine can emit marked on the ribbon: DILATE when it sees two adjacent ones, MARKER when a zero-run crosses a power of two. The dilation exponent r, the 6-bit window, the zero-run length and the sketch are all live and index-aligned, so you can scrub to any position and read the machine\'s exact state at that bit. Switch sketch version to compare the Zobrist XOR fold against the v2 avalanche mixer, and raise the adjacency width to watch the dilation ticks thin out.',
    tier: 'hero',
    methods: ['fsvm.run', 'fsvm.meta'],
    needsWasm: true,
  },
  {
    slug: 'hilbert-codec',
    title: 'Why a Hilbert curve does not compress images',
    kicker: 'Hero demo',
    blurb:
      'The image codec, measured honestly — including the finding that it mostly expands its input.',
    detail:
      'The curve fills a square with one continuous path, so 2D neighbours land close together in the bitstream. The instinct is that this should make run-length coding work on images. Measurement says otherwise, and this demo shows why: the curve is fractal, so it crosses a screen-space edge far more than once. Runs grow roughly 3.7x for every doubling of image size, and past an average run of about 9 bits the terminator overhead eats the gains. The same codec compresses a plain bit pattern beautifully, so the ratio is a property of the input, not of the codec.',
    tier: 'hero',
    methods: ['hilbert.encode', 'hilbert.path'],
    needsWasm: true,
  },
  {
    slug: 'dilation-tree',
    title: 'Hierarchy emerging from adjacency alone',
    kicker: 'Hero demo',
    blurb:
      'Group dilation events by burstiness and a tree appears. Prose, code and noise separate.',
    detail:
      'No parser, no grammar, no model. Just: take the stream of dilation events, measure the intervals between them, and group where the intervals are shorter than the local mean. That is the whole algorithm. What comes out is a hierarchy whose depth, balance, skew and depth-entropy distinguish natural language from source code from noise — and the shape of that tree is a property of the text, not of the grouping rule.',
    tier: 'hero',
    methods: ['tree.build'],
    needsWasm: true,
  },
  {
    slug: 'zeck-residual',
    title: 'Adjacency density across scales',
    kicker: 'Hero demo',
    blurb:
      'A Zeckendorf word has no adjacent ones, so canonical form has residual zero. Plot it across window sizes.',
    detail:
      'The residual is the density of adjacent 1-bits in a window: zero for a canonical Zeckendorf word, high for noise. Plotted against window size it becomes a structural fingerprint of the input, and the envelope is a usable signal in its own right. Canonical form sits at the floor, structured text sits in the middle, and random data at the ceiling.',
    tier: 'hero',
    methods: ['zeck.profile'],
    needsWasm: true,
  },
  {
    slug: 'width-axis',
    title: 'The nested threshold, and the claim it killed',
    kicker: 'Retraction demo',
    blurb:
      'Run the falsification live: dilation counts are non-increasing in width for every possible input.',
    detail:
      'This repository once claimed that adjacency width was a second, independent detector axis. It is not. Widths 1, 2 and 3 test for a 1-run of length at least 2, 3 and 4 respectively, so the event sets are nested and the dilation counts are forced to be non-increasing in width — for every possible input, not just these corpora. This demo runs that assertion over thousands of random streams and reports the violation count, live. The live run also reproduces the published sensitivity table, so the page cannot quietly disagree with the repository.',
    tier: 'light',
    methods: ['structural.corpus', 'structural.nesting', 'structural.matrix'],
    needsWasm: true,
  },
  {
    slug: 'sketch-entropy',
    title: 'A 64-bit sketch that only held 256 values',
    kicker: 'Bug demo',
    blurb:
      'The window was added to the seed without spreading it. Side by side, the broken fold and the fix.',
    detail:
      'The sketch is meant to be a 64-bit fingerprint, but folding the 6-bit window straight into a seed perturbs only the low bits and leaves the upper 56 a function of bit counts alone. That collapses a nominal 64-bit value to at most 1024 distinct results. Multiplying the window by an odd constant first spreads all 64 window values across the full word. This demo runs both folds over the same random streams and counts distinct sketches, and it also shows the pathological case that the fix does not address: an all-zero stream still yields only 2.',
    tier: 'light',
    methods: ['sketch.entropy'],
    needsWasm: true,
  },
];

export function demoBySlug(slug: string): Demo | undefined {
  return demos.find((d) => d.slug === slug);
}
