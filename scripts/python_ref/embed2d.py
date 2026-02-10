#!/usr/bin/env python3
"""embed2d.py — map a 1D boolean stream into a 2D field for spatial analysis.

Mapping:
- choose width W
- bit index t -> (x=t%W, y=t//W)

Provides:
- tile extraction
- box-counting sketch for fractal-ish summaries
"""

from __future__ import annotations

from typing import List, Tuple


def embed(bits: List[int], W: int) -> List[List[int]]:
    if W <= 0:
        raise ValueError("W must be > 0")
    H = (len(bits) + W - 1) // W
    grid = [[0] * W for _ in range(H)]
    for t, b in enumerate(bits):
        grid[t // W][t % W] = b & 1
    return grid


def box_count(grid: List[List[int]], box: int) -> int:
    """Count boxes (box x box) that contain at least one 1."""
    if box <= 0:
        return 0
    H = len(grid)
    W = len(grid[0]) if H else 0
    cnt = 0
    for y0 in range(0, H, box):
        for x0 in range(0, W, box):
            hit = 0
            for y in range(y0, min(H, y0 + box)):
                row = grid[y]
                for x in range(x0, min(W, x0 + box)):
                    if row[x]:
                        hit = 1
                        break
                if hit:
                    break
            cnt += hit
    return cnt


def multiscale_box_counts(grid: List[List[int]], boxes: List[int]) -> List[Tuple[int,int]]:
    return [(b, box_count(grid, b)) for b in boxes]
