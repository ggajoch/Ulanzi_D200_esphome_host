package ui

import (
	"reflect"
	"testing"
)

func TestWrapTwo(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	const maxW = 160 // inner width of a ~200 px key
	cases := []struct {
		text  string
		lines []string
	}{
		{"Drzwi", []string{"Drzwi"}},                          // fits on one line at full size
		{"Wszystkie", []string{"Wszystkie"}},                  // too long but one word: shrinks
		{"Prąd warsztat", []string{"Prąd", "warsztat"}},       // too long, two words: wraps
		{"Światło w kuchni", []string{"Światło", "w kuchni"}}, // most even split
	}
	for _, c := range cases {
		lines, size := WrapTwo(c.text, 44, 40, maxW)
		if !reflect.DeepEqual(lines, c.lines) {
			t.Errorf("WrapTwo(%q) lines = %q, want %q", c.text, lines, c.lines)
		}
		for _, l := range lines {
			if w := MeasureTextWidth(l, size); w > maxW {
				t.Errorf("WrapTwo(%q): %q is %d px at %.0f, wider than %d", c.text, l, w, size, maxW)
			}
		}
	}
}
