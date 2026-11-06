package main

// A full-screen animation over the keys, e.g. when the intercom rings (display play_animation):
// config/ring.gif, cropped to fill the screen as the viewer sees it; a flashing bell while it
// loads, or instead of it without that file. The display ticker draws it instead of the keys;
// a key press ends it. The GIF keeps its own pace: frames the screen can't keep up with are
// composited, not shown.
//
// Speed (RK3308, Cortex-A35): frames are composited straight into framebuffer pixels (BGRA
// uint32) through a palette table, and one pass through fbMap's table scales, turns and copies
// them; draw.Draw, Scale, RotateInto and blit took ~100 ms each per 720x1280 frame.

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"runtime/debug"
	"sync/atomic"
	"time"
	"unsafe"

	xdraw "golang.org/x/image/draw"

	"ulanzi-d200/pkg/ui"
)

const (
	animFile    = "config/ring.gif"
	animMaxSize = 16 << 20 // decoded frames stay in RAM (128 MiB device)
)

type animation struct {
	start, until time.Time
	shown        int // frames put on the screen, for the log
	next         time.Time

	bells [2][]byte // without a GIF: two framebuffer images, flashing

	g        *gif.GIF
	idx      int      // next GIF frame
	canvas   []uint32 // the GIF's logical screen in framebuffer pixels
	dispose  byte     // the last composited frame's disposal, done before the next one
	disposeR image.Rectangle
	saved    []uint32 // the canvas before that frame (DisposalPrevious)
	table    []int32  // fbMap
	back     []byte   // the next framebuffer contents: one copy per frame, so no tearing
}

var anim atomic.Pointer[animation]

// startAnimation shows the bell at once; the GIF, when there is one, takes over once loaded
func startAnimation(d time.Duration) {
	now := time.Now()
	bell := &animation{start: now, until: now.Add(d), next: now}
	bell.bells[0], bell.bells[1] = bellFrame(false), bellFrame(true)
	anim.Store(bell)
	go func() {
		t := time.Now()
		g, err := loadGIF(animFile)
		if err != nil {
			fmt.Println("Animation:", err, "- the bell instead")
			return
		}
		a := newGIFAnimation(g, d)
		fmt.Printf("Animation: %s, %d frames %dx%d, ready in %.1f s\n", animFile, len(g.Image),
			g.Config.Width, g.Config.Height, time.Since(t).Seconds())
		anim.CompareAndSwap(bell, a) // unless a key press ended it meanwhile
	}()
}

func loadGIF(path string) (*gif.GIF, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > animMaxSize {
		return nil, fmt.Errorf("%s: %d bytes, more than %d", path, st.Size(), animMaxSize)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err == nil && len(g.Image) == 0 {
		err = fmt.Errorf("%s: no frames", path)
	}
	return g, err
}

func newGIFAnimation(g *gif.GIF, d time.Duration) *animation {
	now := time.Now()
	w, h := g.Config.Width, g.Config.Height
	return &animation{start: now, until: now.Add(d), next: now, g: g,
		canvas: make([]uint32, w*h),
		table:  fbMap(fbStride*LOG_WIDTH, int(rotation.Load()), w, h),
		back:   make([]byte, fbStride*LOG_WIDTH),
	}
}

// stopAnimation ends a running animation and redraws the keys; false when none ran
func stopAnimation() bool {
	a := anim.Swap(nil)
	if a == nil {
		return false
	}
	fmt.Printf("Animation: %d frames shown in %.1f s\n", a.shown, time.Since(a.start).Seconds())
	markAllDirty()
	go debug.FreeOSMemory() // the decoded GIF and buffers: tens of MB on a 128 MiB device
	return true
}

// drawAnimation puts the frame due now on the screen; false when no animation runs
func drawAnimation(fb []byte) bool {
	a := anim.Load()
	if a == nil {
		return false
	}
	now := time.Now()
	if now.After(a.until) {
		stopAnimation()
		return false
	}
	if now.Before(a.next) {
		return true
	}
	if a.g == nil {
		copy(fb, a.bells[a.shown%2])
		a.next = now.Add(400 * time.Millisecond)
		a.shown++
		return true
	}
	// Composite every frame due by now, show only the last
	for n := 0; !a.next.After(now); n++ {
		if n == len(a.g.Image) { // a whole loop behind: start the pace from here
			a.next = now
			break
		}
		a.next = a.next.Add(a.composite())
	}
	back := words(a.back)
	for i, s := range a.table {
		if s >= 0 {
			back[i] = a.canvas[s]
		}
	}
	copy(fb, a.back)
	a.shown++
	return true
}

// composite draws the next GIF frame onto the canvas and returns how long it stays
func (a *animation) composite() time.Duration {
	g, i, w := a.g, a.idx, a.g.Config.Width
	switch a.dispose { // what the previous frame asked for, now that it has been shown
	case gif.DisposalBackground:
		for y := a.disposeR.Min.Y; y < a.disposeR.Max.Y; y++ {
			clear(a.canvas[y*w+a.disposeR.Min.X : y*w+a.disposeR.Max.X])
		}
	case gif.DisposalPrevious:
		copy(a.canvas, a.saved)
	}
	if i == 0 {
		clear(a.canvas)
	}
	f := g.Image[i]
	r := f.Bounds().Intersect(image.Rect(0, 0, w, g.Config.Height))
	a.dispose, a.disposeR = 0, r
	if i < len(g.Disposal) {
		a.dispose = g.Disposal[i]
	}
	if a.dispose == gif.DisposalPrevious {
		a.saved = append(a.saved[:0], a.canvas...)
	}
	var lut [256]uint32
	var transparent [256]bool
	for k, c := range f.Palette {
		cr, cg, cb, ca := c.RGBA()
		lut[k] = 0xff000000 | (cr>>8)<<16 | (cg>>8)<<8 | cb>>8 // framebuffer byte order B, G, R, A
		transparent[k] = ca == 0
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		src := f.Pix[f.PixOffset(r.Min.X, y):f.PixOffset(r.Max.X, y)]
		dst := a.canvas[y*w+r.Min.X : y*w+r.Max.X]
		for x, k := range src {
			if !transparent[k] {
				dst[x] = lut[k]
			}
		}
	}
	a.idx = (i + 1) % len(g.Image)

	delay := 0
	if i < len(g.Delay) {
		delay = g.Delay[i]
	}
	if delay < 2 { // 0 or 1 (1/100 s): browsers play these at 10 fps
		return 100 * time.Millisecond
	}
	return time.Duration(delay) * 10 * time.Millisecond
}

// fbMap gives, for every pixel of a framebuffer fbLen bytes long, the index into a w x h
// image cropped and scaled (nearest neighbour) to fill the screen as the viewer sees it,
// or -1. It runs the keys' own path (RotateInto, blit) once on an image of indexes.
func fbMap(fbLen, rot, w, h int) []int32 {
	view := image.NewRGBA(image.Rect(0, 0, LOG_WIDTH, LOG_HEIGHT))
	if rot == 90 || rot == 270 {
		view = image.NewRGBA(image.Rect(0, 0, LOG_HEIGHT, LOG_WIDTH))
	}
	vw, vh := view.Rect.Dx(), view.Rect.Dy()
	crop := coverRect(image.Rect(0, 0, w, h), image.Pt(vw, vh))
	for vy := 0; vy < vh; vy++ {
		sy := crop.Min.Y + (2*vy+1)*crop.Dy()/(2*vh) // pixel centres, as x/image/draw does
		for vx := 0; vx < vw; vx++ {
			sx := crop.Min.X + (2*vx+1)*crop.Dx()/(2*vw)
			s, o := sy*w+sx, view.PixOffset(vx, vy)
			view.Pix[o], view.Pix[o+1], view.Pix[o+2], view.Pix[o+3] = byte(s), byte(s>>8), byte(s>>16), 0xff
		}
	}
	logical := image.NewRGBA(image.Rect(0, 0, LOG_WIDTH, LOG_HEIGHT))
	ui.RotateInto(logical, logical.Rect, view, rot)
	buf := make([]byte, fbLen)
	blit(buf, logical) // swaps R and B
	table := make([]int32, fbLen/4)
	for i := range table {
		b := buf[i*4 : i*4+4]
		table[i] = -1
		if b[3] != 0 {
			table[i] = int32(b[2]) | int32(b[1])<<8 | int32(b[0])<<16
		}
	}
	return table
}

// bellFrame is a ringing bell on the whole screen, its colours swapped when inverted
func bellFrame(inverted bool) []byte {
	view := image.NewRGBA(image.Rect(0, 0, LOG_WIDTH, LOG_HEIGHT))
	if rot := rotation.Load(); rot == 90 || rot == 270 {
		view = image.NewRGBA(image.Rect(0, 0, LOG_HEIGHT, LOG_WIDTH))
	}
	bg, fg := color.RGBA{255, 200, 0, 255}, color.RGBA{0, 0, 0, 255}
	if inverted {
		bg, fg = fg, bg
	}
	xdraw.Draw(view, view.Rect, &image.Uniform{bg}, image.Point{}, xdraw.Src)
	cx, cy := view.Rect.Dx()/2, view.Rect.Dy()/2
	ui.DrawIcon(view, cx, cy-60, 360, "bell-ring", fg)
	ui.DrawText(view, cx, cy+220, 110, "Domofon", fg)
	logical := image.NewRGBA(image.Rect(0, 0, LOG_WIDTH, LOG_HEIGHT))
	ui.RotateInto(logical, logical.Rect, view, int(rotation.Load()))
	buf := make([]byte, fbStride*LOG_WIDTH)
	blit(buf, logical)
	return buf
}

// words views a framebuffer image as one uint32 a pixel
func words(b []byte) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(b))), len(b)/4)
}

// coverRect is the centred part of src with dst's aspect ratio: scaled to dst it fills it
func coverRect(src image.Rectangle, dst image.Point) image.Rectangle {
	w, h := src.Dx(), src.Dy()
	if w*dst.Y > h*dst.X { // src wider than dst: crop the sides
		cw := h * dst.X / dst.Y
		x0 := src.Min.X + (w-cw)/2
		return image.Rect(x0, src.Min.Y, x0+cw, src.Max.Y)
	}
	ch := w * dst.Y / dst.X
	y0 := src.Min.Y + (h-ch)/2
	return image.Rect(src.Min.X, y0, src.Max.X, y0+ch)
}
