package main

import "testing"

// The physical key (landscape numbering) at each logical position, read off the grids
// the viewer sees: left the physical keys, right their logical positions.
func TestKeyOrder(t *testing.T) {
	want := map[int][14]int{
		// [ 0] [ 1] [ 2] [ 3] [ 4]
		// [ 5] [ 6] [ 7] [ 8] [ 9]
		// [10] [11] [12] [13 ═════]
		0: {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13},
		// Turned 90° clockwise: the landscape left edge is on top.
		// [10] [ 5] [ 0]    [ 0] [ 1] [ 2]
		// [11] [ 6] [ 1]    [ 3] [ 4] [ 5]
		// [12] [ 7] [ 2]    [ 6] [ 7] [ 8]
		// [13] [ 8] [ 3]    [13] [ 9] [10]
		// [13] [ 9] [ 4]    [13] [11] [12]
		90: {10, 5, 0, 11, 6, 1, 12, 7, 2, 8, 3, 9, 4, 13},
		// Upside down.
		// [13 ═════] [12] [11] [10]    [13 ═════] [ 0] [ 1] [ 2]
		// [ 9] [ 8] [ 7] [ 6] [ 5]     [ 3] [ 4] [ 5] [ 6] [ 7]
		// [ 4] [ 3] [ 2] [ 1] [ 0]     [ 8] [ 9] [10] [11] [12]
		180: {12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 13},
		// Turned 270° clockwise (90° counter-clockwise): the landscape right edge is on top.
		// [ 4] [ 9] [13]    [ 0] [ 1] [13]
		// [ 3] [ 8] [13]    [ 2] [ 3] [13]
		// [ 2] [ 7] [12]    [ 4] [ 5] [ 6]
		// [ 1] [ 6] [11]    [ 7] [ 8] [ 9]
		// [ 0] [ 5] [10]    [10] [11] [12]
		270: {4, 9, 3, 8, 2, 7, 12, 1, 6, 11, 0, 5, 10, 13},
	}
	for rot, w := range want {
		if got := keyOrder(rot); got != w {
			t.Errorf("keyOrder(%d) = %v, want %v", rot, got, w)
		}
		// Key presses go through the same permutation, backwards
		rotation.Store(int32(rot))
		for logical, phys := range w {
			if got := logicalKey(phys); got != logical {
				t.Errorf("rotation %d: logicalKey(%d) = %d, want %d", rot, phys, got, logical)
			}
		}
	}
	rotation.Store(0)
}
