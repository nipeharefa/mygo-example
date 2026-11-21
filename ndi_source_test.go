//go:build darwin || linux || windows

package main

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// A frame of one solid color, top row first, BGRA.
func solidSource(c color.RGBA) ndiFrameSource { return solid{c: c} }

type solid struct{ c color.RGBA }

func (s solid) frame(w, h int) (ndiFrame, bool) {
	pix := make([]byte, 4*w*h)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = s.c.B, s.c.G, s.c.R, s.c.A
	}
	return ndiFrame{pix: pix, stride: 4 * w, w: w, h: h}, true
}

func TestRenderBGRA(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	f := renderFrame(img)
	if f.w != 2 || f.h != 2 || len(f.pix) != 16 || f.stride != 8 {
		t.Fatalf("frame = %dx%d stride %d len %d", f.w, f.h, f.stride, len(f.pix))
	}
	if got := f.pix[0:4]; string(got) != "\x33\x22\x11\xff" {
		t.Fatalf("first pixel = % x, want BGRA", got)
	}
}

func TestNDIRenderSize(t *testing.T) {
	for _, tc := range []struct{ wantW, wantH int }{
		{1280, 720}, {1920, 1080},
	} {
		w, h := ndiRenderSize(tc.wantW, tc.wantH)
		if w != tc.wantW || h != tc.wantH {
			t.Fatalf("ndiRenderSize(%d, %d) = %d, %d", tc.wantW, tc.wantH, w, h)
		}
	}
	if w, h := ndiRenderSize(3840, 2160); w != 1920 || h != 1080 {
		t.Fatalf("a 4K request should cap at 1080p, got %d, %d", w, h)
	}
	if _, h := ndiRenderSize(1280, 800); h%2 != 0 {
		t.Fatalf("height %d is odd", h)
	}
}

// A frame waiting on the main thread must not outlive Stop: the loop
// selects on the stream's stop channel, so a source that never answers
// still lets the stream end.
type stuck struct{ stop chan struct{} }

func (s stuck) frame(w, h int) (ndiFrame, bool) {
	<-s.stop
	return ndiFrame{}, false
}

// Review Focus 3: stopping while a frame is in flight. The stream here has
// no runtime and no sender, which also pins that a source reporting false
// never reaches s.rt or s.send.
func TestStreamStopsWithAStuckSource(t *testing.T) {
	s := &ndiStream{stop: make(chan struct{}), done: make(chan struct{})}
	s.src = stuck{s.stop}
	go s.run()
	close(s.stop)
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the loop did not end after stop was closed")
	}
}
