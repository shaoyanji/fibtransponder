/**
 * The retraction log.
 *
 * Every entry is a claim this repository made and then withdrew, with the
 * measurement that withdrew it. These are the most load-bearing content on the
 * site, so each one is written to be checkable: a reader should be able to open
 * the linked test and confirm the claim no longer holds.
 */

export type Tone = 'claim' | 'verified' | 'retract';

export interface Retraction {
  /** The claim as it was originally made, quoted. */
  claim: string;
  /** What the evidence was. */
  evidence: string;
  /** Why the claim was withdrawn, and what replaced it. */
  withdrawnBecause: string;
  /** The superseding position, if there is one. */
  nowClaim: string;
  /** Where in the repository this is pinned by a test. */
  pinnedAt?: string;
  /** Repo-relative link, if there is a document to read. */
  href?: string;
}

export const retractions: Retraction[] = [
  {
    claim: 'The sketch has a 2⁻⁶⁴ collision probability.',
    evidence:
      'The sketch is a parity function over a 128-symbol alphabet. Measured over random streams of length 1 to 200, about 1.5% collide, with the first collision observed near stream 350.',
    withdrawnBecause:
      'There is no collision bound at all. The figure was a property of a random function, and this is not one.',
    nowClaim:
      'The sketch is a state fingerprint for coherence and divergence tracking, not a stable identifier. Distinct inputs can and do collide.',
    pinnedAt: 'internal/fsvm — TestSketchCollisionRate',
    href: 'https://github.com/shaoyanji/fibtransponder/blob/master/docs/SPEC.md',
  },
  {
    claim: 'Adjacency width and marker threshold are two independent calibration axes, spanning a 2-dimensional parameter space.',
    evidence:
      'Widths 1, 2 and 3 test for a 1-run of length at least 2, 3 and 4, so the event sets are nested: {run≥4} ⊆ {run≥3} ⊆ {run≥2}. Dilation counts must therefore be non-increasing in width for every possible input.',
    withdrawnBecause:
      'Width is an ordered threshold on one latent variable, not a free parameter. There is no gain-versus-selectivity trade-off to calibrate over. The threshold axis does not even affect adjacency detection, because the dilation branch never reads it.',
    nowClaim:
      'Width is one scalar read at three thresholds. The threshold axis is untested on these corpora, because no marker fires on any of them.',
    pinnedAt: 'internal/transponder — TestStructuralCalibration, TestSecondAxisCalibration',
  },
  {
    claim: 'A transponder array is a multi-detector sensor rather than a multi-fingerprint wrapper.',
    evidence:
      'Within each class, all differently-seeded transponders produced identical dilation and marker counts. Seeds changed the fingerprint, not the event structure.',
    withdrawnBecause:
      'The detectors saw identical event structure. Only their fingerprints differed.',
    nowClaim:
      'Seed tables label trajectories. They do not create detector diversity.',
    pinnedAt: 'internal/transponder — TestCorpusExperiment',
  },
  {
    claim: 'A text classifier achieved 100% accuracy on the transponder features.',
    evidence:
      'The classifier program had no train/test split: train and test accuracy were assigned the same variable, so they were identical by construction. Its scoring function was a hand-tuned byte-ratio heuristic whose constants had been adjusted against the very errors it reported. It imported no package from this repository at all.',
    withdrawnBecause:
      'The number measured a heuristic on its own tuning data. It said nothing about the transponder.',
    nowClaim:
      'The generated output is not tracked and must not be cited. The honest experiment is in internal/transponder, and it currently reports byte features beating FSVM features.',
    pinnedAt: 'cmd/classify/main.go — the header says so',
  },
  {
    claim: 'The v1 sketch entropy is 2⁻⁶⁴ per stream; distinct sketches track stream count.',
    evidence:
      'The fold added the 6-bit window directly to the seed, so the upper 56 bits were a function of bit counts alone. 20,000 random streams produced only 256 distinct sketches, colliding after about 18 streams.',
    withdrawnBecause:
      'A nominal 64-bit sketch was carrying about 8 bits of real entropy. The 6-bit window was too small to perturb a 64-bit word without being spread across it first.',
    nowClaim:
      'Multiplying the window by an odd constant first gives ~19,713 distinct over the same 20,000 streams.',
    pinnedAt: 'internal/fsvm — TestSketchEntropy, TestSketchHighBitsCarryEntropy',
  },
  {
    claim: 'Benchmark results as originally published.',
    evidence:
      'Earlier figures cited an AMD EPYC 7763 and also an unnamed Celeron N3010 and Pentium N4200, without attributing individual results to a particular host.',
    withdrawnBecause:
      'Attributing a number to the wrong machine is indistinguishable from inventing it. The unattributed figures were deleted.',
    nowClaim:
      'Every figure names its host, and was re-measured after the sketch fold fix.',
    pinnedAt: 'docs/BENCHMARKS.md',
  },
];

/**
 * Non-retracted claims, i.e. what the project still stands behind. Kept
 * alongside the retractions so the log reads as an audit rather than a list of
 * apologies.
 */
export interface StandingClaim {
  claim: string;
  basis: string;
  pinnedAt: string;
}

export const standingClaims: StandingClaim[] = [
  {
    claim: 'A deterministic state machine ingests a bitstream at O(1) per bit with bounded heap allocation.',
    basis: '~30 ns/bit with 0 counted allocations; a 64-bit word-level path runs at ~14 ns/bit.',
    pinnedAt: 'internal/fsvm — BenchmarkStep, BenchmarkStepWord64V2',
  },
  {
    claim: 'State size is fixed at 96 bytes and does not grow with stream length.',
    basis: 'Pinned with unsafe.Sizeof for both sketch versions, which share one state type.',
    pinnedAt: 'internal/fsvm — TestStateSize',
  },
  {
    claim: 'Varying adjacency width changes which input class the detector is most sensitive to.',
    basis:
      'Width 1 favours prose; widths 2 and 3 favour code. This survives the retraction — it is a consequence of the nesting, not evidence of independent axes.',
    pinnedAt: 'internal/transponder — TestStructuralCalibration',
  },
  {
    claim: 'Regular synthetic input is fully suppressed at width 2 and above.',
    basis:
      'The repeating Fibonacci byte pattern never produces three consecutive 1-bits, so its dilation rate is exactly 0.0000 at both wider settings.',
    pinnedAt: 'internal/transponder — TestStructuralCalibration',
  },
  {
    claim: 'Code is burstier in time than prose: about 14x the windowed dilation variance at width 1.',
    basis: 'Windowed rate variance over 2048-bit windows.',
    pinnedAt: 'REPORT_STRUCTURAL.md §3',
  },
];
