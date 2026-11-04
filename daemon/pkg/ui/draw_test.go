package ui

import (
	"image"
	"image/color"
	"testing"
)

// The viewer's top-left and top-right corners land on the screen corners that face them
// when the screen is turned deg clockwise: at 90 the screen's left edge is the viewer's top.
func TestRotateInto(t *testing.T) {
	red, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	src := image.NewRGBA(image.Rect(0, 0, 3, 2)) // the viewer's tile: 3 wide, 2 tall
	src.SetRGBA(0, 0, red)
	src.SetRGBA(2, 0, blue)
	cases := []struct {
		deg       int
		size      image.Point // the tile on the screen
		red, blue image.Point // where the viewer's top-left and top-right end up on the screen
	}{
		{0, image.Pt(3, 2), image.Pt(0, 0), image.Pt(2, 0)},
		{90, image.Pt(2, 3), image.Pt(0, 2), image.Pt(0, 0)},  // bottom-left, top-left
		{180, image.Pt(3, 2), image.Pt(2, 1), image.Pt(0, 1)}, // bottom-right, bottom-left
		{270, image.Pt(2, 3), image.Pt(1, 0), image.Pt(1, 2)}, // top-right, bottom-right
	}
	for _, c := range cases {
		dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
		r := image.Rectangle{Min: image.Pt(5, 5), Max: image.Pt(5, 5).Add(c.size)}
		RotateInto(dst, r, src, c.deg)
		if got := dst.RGBAAt(r.Min.X+c.red.X, r.Min.Y+c.red.Y); got != red {
			t.Errorf("%d°: viewer's top-left not at screen %v", c.deg, c.red)
		}
		if got := dst.RGBAAt(r.Min.X+c.blue.X, r.Min.Y+c.blue.Y); got != blue {
			t.Errorf("%d°: viewer's top-right not at screen %v", c.deg, c.blue)
		}
	}
}
