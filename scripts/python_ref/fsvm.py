#!/usr/bin/env python3
"""fsvm.py — FSVM reference core (Python)

Extends the earlier `fsvm_ref.py` into a reusable module.

Core rules:
- Observe boolean stream.
- If adjacent 1s occur, emit DILATE and increment r.
- Track a 6-bit sliding window (hexagram) and zero-run length.

Marker emission (optional, unDoSable default):
- emit MARKER when zero-run crosses powers of two: 8,16,32,...

This keeps candidate segmentation points sparse.
"""

from __future__ import annotations

from dataclasses import dataclass


def _is_pow2(x: int) -> bool:
    return x > 0 and (x & (x - 1)) == 0


@dataclass
class Event:
    kind: str
    payload: int = 0
    i: int = 0


@dataclass
class State:
    r: int = 0
    w: int = 0      # 6-bit
    last: int = 0
    zero_run: int = 0
    dilations: int = 0
    markers: int = 0


def step(st: State, b: int, i: int) -> list[Event]:
    b &= 1
    evs: list[Event] = []

    # zero run + markers
    if b == 0:
        st.zero_run += 1
        # sparse markers at 8,16,32,... (not at 1,2,4)
        if st.zero_run >= 8 and _is_pow2(st.zero_run):
            st.markers += 1
            evs.append(Event(kind="MARKER", payload=st.zero_run, i=i))
    else:
        st.zero_run = 0

    # adjacency detection
    if st.last == 1 and b == 1:
        st.r += 1
        st.dilations += 1
        evs.append(Event(kind="DILATE", payload=st.r, i=i))

    st.last = b
    st.w = ((st.w << 1) | b) & 0x3F
    return evs
