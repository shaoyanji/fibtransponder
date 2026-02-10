#!/usr/bin/env python3
"""bitrope.py — append-only immutable-ish bit rope (Python reference)

Goal: support unbounded boolean streams with:
- bounded allocation (fixed-size blocks)
- cheap append
- cheap window reads for analysis/transforms

This is a reference implementation for experiments and dashboards.
It is not meant to be the fastest possible Python.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import List, Tuple


@dataclass(frozen=True)
class Cursor:
    block: int
    off: int  # bit offset inside block


class BitRope:
    def __init__(self, block_bits: int = 1 << 16):
        if block_bits <= 0:
            block_bits = 1 << 16
        if block_bits % 64 != 0:
            # keep packing simple
            block_bits = ((block_bits + 63) // 64) * 64
        self.block_bits = block_bits
        self._blocks: List[List[int]] = []  # each block: list of uint64 words
        self._n_bits_in_last = 0
        self._len_bits = 0

    def __len__(self) -> int:
        return self._len_bits

    def _ensure_block(self):
        if not self._blocks or self._n_bits_in_last >= self.block_bits:
            n_words = self.block_bits // 64
            self._blocks.append([0] * n_words)
            self._n_bits_in_last = 0

    def append_bit(self, b: int) -> Cursor:
        b &= 1
        self._ensure_block()
        bi = len(self._blocks) - 1
        off = self._n_bits_in_last
        wi = off >> 6
        bit = off & 63
        if b:
            self._blocks[bi][wi] |= (1 << bit)
        self._n_bits_in_last += 1
        self._len_bits += 1
        return Cursor(bi, off)

    def get(self, i: int) -> int:
        if i < 0 or i >= self._len_bits:
            return 0
        bi = i // self.block_bits
        off = i - bi * self.block_bits
        wi = off >> 6
        bit = off & 63
        return (self._blocks[bi][wi] >> bit) & 1

    def read_bits(self, start: int, n: int) -> List[int]:
        """Read n bits starting at start (out of range -> 0)."""
        return [self.get(start + j) for j in range(max(0, n))]

    def read_u64_window(self, start: int, n: int) -> int:
        """Read up to 64 bits into LSB-first integer."""
        n = min(64, max(0, n))
        x = 0
        for j in range(n):
            x |= (self.get(start + j) & 1) << j
        return x

    def blocks(self) -> Tuple[int, int]:
        """(num_blocks, block_bits)."""
        return (len(self._blocks), self.block_bits)
