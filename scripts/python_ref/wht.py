#!/usr/bin/env python3
"""wht.py — Walsh–Hadamard transform helpers (Python reference)

WHT is often a better first transform for boolean transport than FFT:
- operates on +/-1 sequences
- requires only additions/subtractions
- uses power-of-two windows

This module provides:
- boolean -> bipolar conversion
- in-place fast WHT
- simple spectrum-like outputs
"""

from __future__ import annotations

from typing import List


def bool_to_bipolar(bits: List[int]) -> List[int]:
    # 0->-1, 1->+1
    return [1 if (b & 1) else -1 for b in bits]


def fwht(a: List[int]) -> None:
    """In-place fast Walsh-Hadamard transform (unnormalized). len(a) must be pow2."""
    n = len(a)
    h = 1
    while h < n:
        step = h << 1
        for i in range(0, n, step):
            for j in range(i, i + h):
                x = a[j]
                y = a[j + h]
                a[j] = x + y
                a[j + h] = x - y
        h = step


def power_spectrum(a: List[int]) -> List[int]:
    """Return squared magnitudes (since output is real ints, just square)."""
    return [x * x for x in a]
