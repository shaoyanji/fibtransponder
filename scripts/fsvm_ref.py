#!/usr/bin/env python3
"""Reference implementation (runnable) for the FSVM core.

This is NOT the full project; it's a sanity-check tool you can run immediately
without a Go toolchain.

- Consumes a 0/1 string.
- Emits dilation events when adjacent 1s appear.
- Tracks r, lastBit, zeroRun, and a 6-bit window w.

Example:
  ./scripts/fsvm_ref.py 10011
"""

from dataclasses import dataclass

@dataclass
class State:
    r: int = 0
    w: int = 0
    last: int = 0
    zero_run: int = 0
    dilations: int = 0


def step(st: State, b: int):
    b &= 1
    if b == 0:
        st.zero_run += 1
    else:
        st.zero_run = 0

    dilate = (st.last == 1 and b == 1)
    st.last = b
    st.w = ((st.w << 1) | b) & 0x3F

    if dilate:
        st.r += 1
        st.dilations += 1
        return True
    return False


def run(bits: str):
    st = State()
    for i, ch in enumerate(bits):
        if ch not in '01':
            continue
        b = 1 if ch == '1' else 0
        ev = step(st, b)
        if ev:
            print(f"i={i} DILATE r={st.r} w={st.w:06b} zero_run={st.zero_run}")
    print(f"FINAL r={st.r} dilations={st.dilations} w={st.w:06b} zero_run={st.zero_run}")


if __name__ == '__main__':
    import sys
    if len(sys.argv) < 2:
        print("usage: fsvm_ref.py <01-string>")
        sys.exit(2)
    run(sys.argv[1])
