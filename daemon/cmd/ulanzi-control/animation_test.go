package main

import (
	"image"
	"math/rand"
	"testing"

	xdraw "golang.org/x/image/draw"

	"ulanzi-d200/pkg/ui"
)

// The crop keeps the target's aspect ratio and stays centred inside the source
func TestCoverRect(t *testing.T) {
	cases := []struct {
		src  image.Rectangle
		dst  image.Point
		want image.Rectangle
	}{
		{image.Rect(0, 0, 480, 270), image.Pt(720, 1280), image.Rect(164, 0, 315, 270)}, // landscape GIF, portrait screen
		{image.Rect(0, 0, 270, 480), image.Pt(1280, 720), image.Rect(0, 164, 270, 315)}, // portrait GIF, landscape screen
		{image.Rect(0, 0, 400, 300), image.Pt(800, 600), image.Rect(0, 0, 400, 300)},    // same ratio: all of it
	}
	for _, c := range cases {
		if got := coverRect(c.src, c.dst); got != c.want {
			t.Errorf("coverRect(%v, %v) = %v, want %v", c.src, c.dst, got, c.want)
		}
	}
}

// fbMap's table puts every pixel where scaling, RotateInto and blit put it
func TestFBMap(t *testing.T) {
	fbStride = LOG_HEIGHT * 4 // the D200's 720x1280 framebuffer
	const w, h = 64, 48
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	rand.New(rand.NewSource(1)).Read(src.Pix)
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = 0xff
	}
	for _, rot := range []int{0, 90, 180, 270} {
		view := image.NewRGBA(image.Rect(0, 0, LOG_WIDTH, LOG_HEIGHT))
		if rot == 90 || rot == 270 {
			view = image.NewRGBA(image.Rect(0, 0, LOG_HEIGHT, LOG_WIDTH))
		}
		xdraw.NearestNeighbor.Scale(view, view.Rect, src, coverRect(src.Rect, view.Rect.Size()), xdraw.Src, nil)
		logical := image.NewRGBA(image.Rect(0, 0, LOG_WIDTH, LOG_HEIGHT))
		ui.RotateInto(logical, logical.Rect, view, rot)
		want := make([]byte, fbStride*LOG_WIDTH)
		blit(want, logical)

		table := fbMap(len(want), rot, w, h)
		got := make([]byte, len(want))
		g, s := words(got), words(src.Pix)
		for i, si := range table {
			if si >= 0 {
				p := s[si] // R, G, B, A in memory: to the framebuffer's B, G, R, A
				g[i] = p&0xff00ff00 | (p&0xff)<<16 | (p>>16)&0xff
			}
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("rotation %d: byte %d is %d, want %d", rot, i, got[i], want[i])
			}
		}
	}
}
