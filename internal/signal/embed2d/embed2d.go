package embed2d

// 1D boolean stream -> 2D embedding + simple box-counting sketch.

func Embed(bits []uint8, width int) [][]uint8 {
	if width <= 0 {
		width = 1
	}
	h := (len(bits) + width - 1) / width
	grid := make([][]uint8, h)
	for y := 0; y < h; y++ {
		grid[y] = make([]uint8, width)
	}
	for t, b := range bits {
		y := t / width
		x := t % width
		grid[y][x] = b & 1
	}
	return grid
}

func BoxCount(grid [][]uint8, box int) int {
	if box <= 0 || len(grid) == 0 {
		return 0
	}
	h := len(grid)
	w := len(grid[0])
	cnt := 0
	for y0 := 0; y0 < h; y0 += box {
		for x0 := 0; x0 < w; x0 += box {
			hit := false
			for y := y0; y < h && y < y0+box && !hit; y++ {
				row := grid[y]
				for x := x0; x < w && x < x0+box; x++ {
					if row[x] == 1 {
						hit = true
						break
					}
				}
			}
			if hit {
				cnt++
			}
		}
	}
	return cnt
}

type BoxCountPair struct {
	Box   int
	Count int
}

func MultiScaleBoxCounts(grid [][]uint8, boxes []int) []BoxCountPair {
	out := make([]BoxCountPair, 0, len(boxes))
	for _, b := range boxes {
		out = append(out, BoxCountPair{Box: b, Count: BoxCount(grid, b)})
	}
	return out
}
