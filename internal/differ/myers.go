package differ

// This file finds which lines of two files match, using memory proportional to
// the files' size. The steps are the same as GNU diff's: lines become integers
// so comparing them is cheap, the matching start and end are trimmed, lines that
// exist on only one side are set aside because they can't match, and Myers'
// linear-space algorithm handles what's left.

// maxEditEffort limits how far one search step looks for the shortest diff.
// Past it, the search settles for its best point so far: the diff is still
// correct but may not be the shortest. Without a limit, large files with little
// in common take time proportional to the square of their size.
const maxEditEffort = 4096

// lcsMatches returns the matched (leftIndex, rightIndex) pairs of a longest
// common subsequence of left and right, in order. Past maxEditEffort it may
// match fewer lines than the longest possible.
func lcsMatches(left, right []string) [][2]int {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	// Lines become integers. A line only on one side gets an ID the other side
	// never uses, which the steps below can compare in constant time.
	ids := make(map[string]int32, len(left))
	a := make([]int32, len(left))
	for i, line := range left {
		id, ok := ids[line]
		if !ok {
			id = int32(len(ids))
			ids[line] = id
		}
		a[i] = id
	}
	b := make([]int32, len(right))
	for i, line := range right {
		id, ok := ids[line]
		if !ok {
			id = int32(len(ids))
			ids[line] = id
		}
		b[i] = id
	}

	// The matching start and end.
	start := 0
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}

	matches := make([][2]int, 0, start+len(a)-endA)
	for i := 0; i < start; i++ {
		matches = append(matches, [2]int{i, i})
	}

	// The middle, without lines that exist on only one side.
	inA := make([]bool, len(ids))
	inB := make([]bool, len(ids))
	for _, id := range a[start:endA] {
		inA[id] = true
	}
	for _, id := range b[start:endB] {
		inB[id] = true
	}
	m := &myers{}
	for i := start; i < endA; i++ {
		if inB[a[i]] {
			m.a = append(m.a, a[i])
			m.aIndex = append(m.aIndex, i)
		}
	}
	for i := start; i < endB; i++ {
		if inA[b[i]] {
			m.b = append(m.b, b[i])
			m.bIndex = append(m.bIndex, i)
		}
	}
	if len(m.a) > 0 && len(m.b) > 0 {
		size := len(m.a) + len(m.b) + 3
		m.forward = make([]int32, size)
		m.backward = make([]int32, size)
		m.offset = len(m.b) + 1
		m.matches = matches
		m.compare(0, len(m.a), 0, len(m.b))
		matches = m.matches
	}

	for i, j := endA, endB; i < len(a); i, j = i+1, j+1 {
		matches = append(matches, [2]int{i, j})
	}
	return matches
}

// myers holds one run of Myers' linear-space algorithm.
type myers struct {
	a, b           []int32
	aIndex, bIndex []int // each entry's line number in the original file
	// The furthest point reached on each diagonal, searching from each end. A
	// diagonal is x-y, which can be negative, so it's stored with offset added.
	forward, backward []int32
	offset            int
	matches           [][2]int
}

// compare adds the matches between a[xLow:xHigh] and b[yLow:yHigh], in order.
func (m *myers) compare(xLow, xHigh, yLow, yHigh int) {
	for xLow < xHigh && yLow < yHigh && m.a[xLow] == m.b[yLow] {
		m.matches = append(m.matches, [2]int{m.aIndex[xLow], m.bIndex[yLow]})
		xLow++
		yLow++
	}
	suffix := 0
	for xLow < xHigh && yLow < yHigh && m.a[xHigh-1] == m.b[yHigh-1] {
		xHigh--
		yHigh--
		suffix++
	}
	if xLow < xHigh && yLow < yHigh {
		x, y := m.split(xLow, xHigh, yLow, yHigh)
		m.compare(xLow, x, yLow, y)
		m.compare(x, xHigh, y, yHigh)
	}
	for i := 0; i < suffix; i++ {
		m.matches = append(m.matches, [2]int{m.aIndex[xHigh+i], m.bIndex[yHigh+i]})
	}
}

// split returns a point on a shortest edit path through the given range, by
// searching from both ends until the searches meet. Both pieces it divides the
// range into are smaller than the range.
func (m *myers) split(xLow, xHigh, yLow, yHigh int) (int, int) {
	const unreachedHigh = int32(1<<31 - 1)
	a, b, fd, bd, off := m.a, m.b, m.forward, m.backward, m.offset

	dMin, dMax := xLow-yHigh, xHigh-yLow
	fMid, bMid := xLow-yLow, xHigh-yHigh
	fMin, fMax, bMin, bMax := fMid, fMid, bMid, bMid
	odd := (fMid-bMid)&1 != 0

	fd[off+fMid] = int32(xLow)
	bd[off+bMid] = int32(xHigh)

	for effort := 1; ; effort++ {
		// One step further from the start.
		if fMin > dMin {
			fMin--
			fd[off+fMin-1] = -1
		} else {
			fMin++
		}
		if fMax < dMax {
			fMax++
			fd[off+fMax+1] = -1
		} else {
			fMax--
		}
		for d := fMax; d >= fMin; d -= 2 {
			low, high := fd[off+d-1], fd[off+d+1]
			x := int(low) + 1
			if low < high {
				x = int(high)
			}
			y := x - d
			for x < xHigh && y < yHigh && a[x] == b[y] {
				x++
				y++
			}
			fd[off+d] = int32(x)
			if odd && bMin <= d && d <= bMax && int(bd[off+d]) <= x {
				return x, y
			}
		}

		// One step further from the end.
		if bMin > dMin {
			bMin--
			bd[off+bMin-1] = unreachedHigh
		} else {
			bMin++
		}
		if bMax < dMax {
			bMax++
			bd[off+bMax+1] = unreachedHigh
		} else {
			bMax--
		}
		for d := bMax; d >= bMin; d -= 2 {
			low, high := bd[off+d-1], bd[off+d+1]
			x := int(high) - 1
			if low < high {
				x = int(low)
			}
			y := x - d
			for xLow < x && yLow < y && a[x-1] == b[y-1] {
				x--
				y--
			}
			bd[off+d] = int32(x)
			if !odd && fMin <= d && d <= fMax && x <= int(fd[off+d]) {
				return x, y
			}
		}

		if effort >= maxEditEffort {
			return m.bestSoFar(xLow, xHigh, yLow, yHigh, fMin, fMax, bMin, bMax)
		}
	}
}

// bestSoFar gives up on the shortest path through a range, returning whichever
// search has covered more of it.
func (m *myers) bestSoFar(xLow, xHigh, yLow, yHigh, fMin, fMax, bMin, bMax int) (int, int) {
	fBest, fX := -1, 0
	for d := fMax; d >= fMin; d -= 2 {
		x := min(int(m.forward[m.offset+d]), xHigh)
		y := x - d
		if y > yHigh {
			x, y = yHigh+d, yHigh
		}
		if x+y > fBest {
			fBest, fX = x+y, x
		}
	}
	bBest, bX := xHigh+yHigh+1, 0
	for d := bMax; d >= bMin; d -= 2 {
		x := max(xLow, int(m.backward[m.offset+d]))
		y := x - d
		if y < yLow {
			x, y = yLow+d, yLow
		}
		if x+y < bBest {
			bBest, bX = x+y, x
		}
	}
	if xHigh+yHigh-bBest < fBest-(xLow+yLow) {
		return fX, fBest - fX
	}
	return bX, bBest - bX
}
