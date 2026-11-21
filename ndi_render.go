//go:build darwin || linux || windows

package main

import (
	"image"
	"math"
)

// renderFrame turns a rendered image into the BGRA, top-row-first frame the
// frame loop sends. Go's RGBA is already top row first; only the byte order
// changes, one pixel at a time. The stage draws no translucency, so the
// image's alpha byte goes out as it is.
func renderFrame(img *image.RGBA) ndiFrame {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pix := make([]byte, 4*w*h)
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride:]
		out := pix[y*4*w:]
		for x := 0; x < w; x++ {
			in := row[x*4:]
			out[x*4+0] = in[2]
			out[x*4+1] = in[1]
			out[x*4+2] = in[0]
			out[x*4+3] = in[3]
		}
	}
	return ndiFrame{pix: pix, stride: 4 * w, w: w, h: h}
}

// ndiRenderSize is the size the feed renders at: the size asked for, fitted
// into the HD cap by the one size policy, ndiFrameSize.
func ndiRenderSize(w, h int) (int, int) {
	return ndiFrameSize(w, h)
}

// ndiFrameSize fits a requested frame into the HD cap, keeping its aspect
// ratio. A size already at or below the cap is kept as it is.
func ndiFrameSize(w, h int) (int, int) {
	scale := 1.0
	if w > ndiMaxFrameWidth {
		scale = float64(ndiMaxFrameWidth) / float64(w)
	}
	if h > ndiMaxFrameHeight && float64(ndiMaxFrameHeight)/float64(h) < scale {
		scale = float64(ndiMaxFrameHeight) / float64(h)
	}
	if scale >= 1 {
		return w, h
	}
	nw, nh := int(math.Round(float64(w)*scale)), int(math.Round(float64(h)*scale))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	// Keep the dimensions even, as video encoders prefer 4:2:0 frames.
	nw &^= 1
	nh &^= 1
	return nw, nh
}
