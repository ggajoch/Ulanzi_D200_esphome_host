package main

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"ulanzi-d200/pkg/state"
	"ulanzi-d200/pkg/ui"
)

// parseForecast splits a forecast widget's value (docs/configuration.md): rows of
// "time|icon|temperature|rain|wind" separated by ";", then "#" and the box, "line 1|line 2|#colour"
func parseForecast(v string) (rows [][]string, box []string) {
	rowsPart, boxPart, _ := strings.Cut(v, "#")
	for _, r := range strings.Split(rowsPart, ";") {
		if f := strings.Split(r, "|"); len(f) == 5 {
			rows = append(rows, f)
		}
	}
	if boxPart != "" {
		box = strings.Split(boxPart, "|")
	}
	return rows, box
}

func drawTextRight(img *image.RGBA, x, y int, size float64, text string, col color.RGBA) {
	ui.DrawTextLeft(img, x-ui.MeasureTextWidth(text, size), y, size, text, col)
}

// drawForecast draws the forecast widget, laid out for the big key turned tall (about 200x460):
// the icon and text side by side on top (now), text2 under them (all three empty: no header),
// value's rows, then its box.
// shortcut: other key shapes get the same pixel layout, clipped; scale it when one needs them
func drawForecast(img *image.RGBA, rect image.Rectangle, bg color.RGBA, s state.ButtonState, iconCol, textCol, text2Col color.RGBA) {
	rows, box := parseForecast(s.Value)
	x0, y0, x1, w := rect.Min.X, rect.Min.Y, rect.Max.X, rect.Dx()
	pad := w / 16
	grey := color.RGBA{150, 156, 168, 255}
	rainCol := color.RGBA{102, 187, 255, 255}

	top := y0 + 4
	if s.IconID != "" || s.Text != "" || s.Text2 != "" { // the header is optional
		ui.DrawIcon(img, x0+w/4, y0+52, 68, s.IconID, iconCol)
		ui.DrawText(img, x0+w*68/100, y0+50, ui.FitSize(s.Text, 48, w*45/100), s.Text, textCol)
		ui.DrawText(img, x0+w/2, y0+110, ui.FitSize(s.Text2, 30, w-2*pad), s.Text2, text2Col)
		ui.DrawHLine(img, x0+pad, x1-1-pad, y0+136, color.RGBA{60, 64, 76, 255})
		top = y0 + 140
	}

	boxH := 0
	if len(box) > 0 {
		boxH = 64
	}
	bottom := rect.Max.Y - boxH - 8
	for i, f := range rows { // time, icon, temperature, rain (empty: dry), wind/gusts
		rh := (bottom - top) / len(rows)
		y := top + i*rh
		if i%2 == 1 {
			draw.Draw(img, image.Rect(x0, y, x1, y+rh), &image.Uniform{color.RGBA{bg.R / 2, bg.G / 2, bg.B / 2, 255}}, image.Point{}, draw.Src)
		}
		l1, l2 := y+rh*3/10, y+rh*3/4
		ui.DrawTextLeft(img, x0+pad, l1, 26, f[0], grey)
		ui.DrawIcon(img, x0+w/2, l1, 36, f[1], color.RGBA{120, 180, 255, 255}) // leaves "-12°" room
		drawTextRight(img, x1-pad, l1, 32, f[2], textCol)
		if f[3] != "" {
			ui.DrawIcon(img, x0+pad+9, l2, 22, "water", rainCol)
			ui.DrawTextLeft(img, x0+pad+22, l2, 22, f[3], rainCol)
		}
		drawTextRight(img, x1-pad, l2, 22, f[4], grey)
		ui.DrawIcon(img, x1-pad-ui.MeasureTextWidth(f[4], 22)-16, l2, 24, "weather-windy", grey)
	}

	if len(box) > 0 {
		bc := color.RGBA{102, 102, 119, 255}
		if len(box) > 2 {
			bc = parseHexColor(box[2])
		}
		r := image.Rect(x0+pad/2, rect.Max.Y-boxH-4, x1-pad/2, rect.Max.Y-4)
		draw.Draw(img, r, &image.Uniform{color.RGBA{bc.R / 5, bc.G / 5, bc.B / 5, 255}}, image.Point{}, draw.Src)
		for t := 0; t < 2; t++ { // a 2 px frame
			ui.DrawHLine(img, r.Min.X, r.Max.X-1, r.Min.Y+t, bc)
			ui.DrawHLine(img, r.Min.X, r.Max.X-1, r.Max.Y-1-t, bc)
			ui.DrawVLine(img, r.Min.X+t, r.Min.Y, r.Max.Y-1, bc)
			ui.DrawVLine(img, r.Max.X-1-t, r.Min.Y, r.Max.Y-1, bc)
		}
		cx, maxW := r.Min.X+r.Dx()/2, r.Dx()-12
		ui.DrawText(img, cx, r.Min.Y+boxH*3/10, ui.FitSize(box[0], 24, maxW), box[0], color.RGBA{255, 255, 255, 255})
		if len(box) > 1 {
			ui.DrawText(img, cx, r.Min.Y+boxH*7/10, ui.FitSize(box[1], 22, maxW), box[1], color.RGBA{215, 220, 230, 255})
		}
	}
}
