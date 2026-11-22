//go:build darwin || linux || windows

package main

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// renderFrame turns a rendered image into the BGRA, top-row-first frame the
// frame loop sends. Go's RGBA is already top row first; only the byte order
// changes, a pixel at a time. The stage draws no translucency, so the
// image's alpha byte goes out as it is.
func renderFrame(img *image.RGBA) ndiFrame {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pix := make([]byte, 4*w*h)
	bgrA(pix, img.Pix, 4*w, h)
	return ndiFrame{pix: pix, stride: 4 * w, w: w, h: h}
}

// bgrA swaps red and blue over h rows of a bitmap stride bytes a row into
// dst, 4×w bytes a row. Each pixel is assigned all four of its bytes, so the
// swap does not depend on what dst held before.
func bgrA(dst, src []byte, stride, h int) {
	for y := 0; y < h; y++ {
		in := src[y*stride : (y+1)*stride]
		out := dst[y*stride : (y+1)*stride]
		for q := 0; q+4 <= len(in); q += 4 {
			out[q], out[q+1], out[q+2], out[q+3] = in[q+2], in[q+1], in[q], in[q+3]
		}
	}
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

// ndiStageWidth and ndiStageHeight are the stage laid out at the size the
// output window opens at (output.go), the size the diagnostics report the
// capture at, and the size the feed draws the slide at. A view laid out much
// narrower does not fit its padding and drops its text, so the feed never
// lays the stage out smaller than this: ui.Render scales text but still
// wraps lines at the layout's width.
const (
	ndiStageWidth  = 1280
	ndiStageHeight = 720
)

// renderStageFrame draws liveView with ui.Render, on the main thread. It
// reads app state the way a view does, so it runs on the main thread and
// only there; the frame it returns belongs to the caller.
func (a *app) renderStageFrame(w, h int) (ndiFrame, bool) {
	w, h = ndiRenderSize(w, h)
	if w <= 0 || h <= 0 {
		return ndiFrame{}, false
	}
	// The stage is laid out at its own size and the frame is scaled to the
	// size wanted, so a frame smaller than the stage is the slide made
	// smaller, never the middle of it. A frame larger than the stage is
	// drawn at the bigger scale, and ui.Render rounds up, so the image
	// covers the frame and shrinking it never has to crop.
	scale := float32(1)
	if w > ndiStageWidth || h > ndiStageHeight {
		scale = float32(math.Max(float64(w)/ndiStageWidth, float64(h)/ndiStageHeight))
	}
	return shrinkFrame(renderFrame(ui.Render(a.liveView, ndiStageWidth, ndiStageHeight, scale)), w, h), true
}

// shrinkFrame fits a frame to w×h by averaging blocks of it down. A frame
// the size wanted is copied, bytes and all, which is the common case. The
// frame drawn at a larger scale covers w×h and is averaged down. A frame
// that does not cover w×h cannot happen after the scale above, and falls
// back to its centre rather than growing. The result is a buffer of its own,
// since the frame loop keeps the frame it was handed while the next one is
// drawn.
func shrinkFrame(f ndiFrame, w, h int) ndiFrame {
	if f.w == w && f.h == h {
		pix := make([]byte, len(f.pix))
		copy(pix, f.pix)
		return ndiFrame{pix: pix, stride: f.stride, w: w, h: h}
	}
	if w > f.w || h > f.h {
		return cropFrame(f, (f.w-min(w, f.w))/2, (f.h-min(h, f.h))/2, w, h)
	}
	// Whole factors when the sides allow them: a frame the engine drew at a
	// scale is already smooth, and averaging it again would only blur it.
	if fx, fy := f.w/w, f.h/h; f.w%w == 0 && f.h%h == 0 && fx == fy && fx > 1 {
		return blockAverage(f, w, h, fx, true)
	}
	return blockAverage(f, w, h, 0, false)
}

// blockAverage averages the source blocks of a frame down to w×h: whole
// k×k blocks when k > 0, and whatever the evenly divided frame gives
// otherwise. The sums run in float32, so a whole frame divides into a block
// without losing a byte and the adds and divides vectorise where integer
// divides do not.
func blockAverage(f ndiFrame, w, h, k int, whole bool) ndiFrame {
	x0 := make([]int, w)
	x1 := make([]int, w)
	for x := 0; x < w; x++ {
		if whole {
			x0[x], x1[x] = x*k, (x+1)*k
		} else {
			x0[x], x1[x] = x*f.w/w, (x+1)*f.w/w
		}
	}
	pix := make([]byte, 4*w*h)
	sum := make([]float32, w*4)
	for y := 0; y < h; y++ {
		rowStart, rowEnd := y*f.h/h, (y+1)*f.h/h
		if whole {
			rowStart, rowEnd = y*k, (y+1)*k
		}
		for sy := rowStart; sy < rowEnd; sy++ {
			row := f.pix[sy*f.stride:]
			for x := 0; x < w; x++ {
				q := x0[x] * 4
				var b0, b1, b2, b3 float32
				for e := x1[x] * 4; q < e; q += 4 {
					b0 += float32(row[q])
					b1 += float32(row[q+1])
					b2 += float32(row[q+2])
					b3 += float32(row[q+3])
				}
				n := float32(x1[x] - x0[x])
				sum[x*4] += b0 / n
				sum[x*4+1] += b1 / n
				sum[x*4+2] += b2 / n
				sum[x*4+3] += b3 / n
			}
		}
		rows := float32(rowEnd - rowStart)
		out := pix[y*4*w:]
		for i, v := range sum {
			out[i] = byte(v/rows + 0.5)
			sum[i] = 0
		}
	}
	return ndiFrame{pix: pix, stride: 4 * w, w: w, h: h}
}

// cropFrame copies the w×h region at (x, y) into a buffer of its own: the
// frame loop keeps the frame it was handed while the next one is drawn.
func cropFrame(f ndiFrame, x, y, w, h int) ndiFrame {
	if w > f.w {
		w = f.w
	}
	if h > f.h {
		h = f.h
	}
	pix := make([]byte, 4*w*h)
	for row := 0; row < h; row++ {
		copy(pix[row*4*w:(row+1)*4*w], f.pix[(y+row)*f.stride+x*4:(y+row)*f.stride+x*4+4*w])
	}
	return ndiFrame{pix: pix, stride: 4 * w, w: w, h: h}
}
