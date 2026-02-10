#!/usr/bin/env python3
"""demo_dashboard.py — minimal 'dashboard' demo (text)

Shows:
- dilation events
- marker events (zero-run pow2 crossings)
- WHT spectrum for a fixed window
- 2D embedding + box-count sketch

This is a proof-of-deliverable, not the final UI.
"""

from __future__ import annotations

import sys

from bitrope import BitRope
from fsvm import State, step
from wht import bool_to_bipolar, fwht, power_spectrum
from embed2d import embed, multiscale_box_counts


def main(bits: str, window: int = 64, width2d: int = 16):
    rope = BitRope(block_bits=1 << 12)  # small for demo
    st = State()

    events = []
    for i, ch in enumerate(bits):
        if ch not in '01':
            continue
        b = 1 if ch == '1' else 0
        rope.append_bit(b)
        events.extend(step(st, b, i))

    print("=== FSVM summary ===")
    print(f"len_bits={len(rope)} r={st.r} dilations={st.dilations} markers={st.markers} zero_run={st.zero_run} w={st.w:06b}")
    if events:
        print("events:")
        for e in events[:40]:
            print(f"  i={e.i} {e.kind} {e.payload}")
        if len(events) > 40:
            print(f"  ... ({len(events)-40} more)")

    # WHT window
    n = min(window, len(rope))
    # enforce pow2
    pow2 = 1
    while pow2 * 2 <= n:
        pow2 *= 2
    win = rope.read_bits(max(0, len(rope) - pow2), pow2)
    a = bool_to_bipolar(win)
    fwht(a)
    ps = power_spectrum(a)
    top = sorted(enumerate(ps), key=lambda kv: kv[1], reverse=True)[:8]

    print("\n=== WHT (last pow2 window) ===")
    print(f"window={pow2} top_components(index, power):")
    for idx, p in top:
        print(f"  {idx:4d} {p}")

    # 2D embedding
    take = min(256, len(rope))
    bits2 = rope.read_bits(max(0, len(rope) - take), take)
    grid = embed(bits2, width2d)
    counts = multiscale_box_counts(grid, [1, 2, 4, 8])

    print("\n=== 2D embed + box counts ===")
    print(f"embedded last {take} bits into W={width2d}, H={len(grid)}")
    for b, c in counts:
        print(f"  box={b:2d} count={c}")


if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("usage: demo_dashboard.py <01-string>")
        sys.exit(2)
    main(sys.argv[1])
