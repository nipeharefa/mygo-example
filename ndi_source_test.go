//go:build darwin || linux || windows

package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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

// The stage frame is the slide, in the stage theme, at the size asked for,
// and it exists whether or not the stage window is open.
func TestRenderStageFrameFollowsTheStage(t *testing.T) {
	a := newApp()
	a.addSong("amazing-grace")
	a.addVerse("psalm23-1")
	src := a.frameSource(640, 360)

	first, ok := src.frame(640, 360)
	if !ok {
		t.Fatal("the source gave no frame")
	}
	if first.w != 640 || first.h != 360 || first.stride != 2560 {
		t.Fatalf("frame = %dx%d stride %d", first.w, first.h, first.stride)
	}
	if isFill(first, stageTheme().Background) {
		t.Fatal("the frame is blank")
	}
	// The slide text is drawn in the stage's own text colour, which no
	// other theme uses, so the frame is the stage theme and not the
	// console's.
	if want := stageTheme().Text; !hasTextInk(first, want) {
		t.Fatalf("the frame does not draw the slide text in the stage theme (%v)", want)
	}

	// Blacked out, the frame is the stage's background and nothing else:
	// no slide, no text.
	a.blackout = true
	empty, ok := src.frame(640, 360)
	if !ok {
		t.Fatal("the blackout frame gave no frame")
	}
	if want := stageTheme().Background; !isFill(empty, want) {
		t.Fatal("blackout should render the stage background and nothing else")
	}
}

// Review Focus 2: a stage change reaches the next frame, and a blackout
// frame is black at every size.
func TestRenderStageFrameTracksTheStage(t *testing.T) {
	a := newApp()
	a.addSong("amazing-grace")
	a.addVerse("psalm23-1")
	src := a.frameSource(320, 180)
	before, ok := src.frame(320, 180)
	if !ok {
		t.Fatal("no first frame")
	}
	a.nextStage()
	after, ok := src.frame(320, 180)
	if !ok {
		t.Fatal("no second frame")
	}
	if string(before.pix) == string(after.pix) {
		t.Fatal("the feed did not follow the stage")
	}
	for _, size := range [][2]int{{320, 180}, {1920, 1080}} {
		a.blackout = true
		black, ok := src.frame(size[0], size[1])
		if !ok || !isOneColour(black.pix) {
			t.Fatalf("blackout must render a dark empty frame at %dx%d", size[0], size[1])
		}
		a.blackout = false
		lit, ok := src.frame(size[0], size[1])
		if !ok || isOneColour(lit.pix) {
			t.Fatalf("clearing the blackout must restore the slide at %dx%d", size[0], size[1])
		}
		if lit.w != size[0] || lit.h != size[1] {
			t.Fatalf("frame at %dx%d = %dx%d", size[0], size[1], lit.w, lit.h)
		}
	}
}

// Review Focus 3, for the render source: a frame waiting on the main thread
// must not outlive Stop. With the loop shut down RunOnMain drops the
// render, so the source has to answer from the stop channel.
func TestStageSourceGivesUpWhenStopped(t *testing.T) {
	a := newApp()
	a.addSong("amazing-grace")
	a.addVerse("psalm23-1")
	stop := make(chan struct{})
	src := newStageSource(a, 320, 180, stop)
	close(stop)

	done := make(chan bool, 1)
	go func() {
		_, ok := src.frame(320, 180)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("a stopped source returned a frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the source hung after the stream stopped")
	}
}

// A frame larger than the stage is drawn at the frame's size, so a 2K
// stream is the slide at 2K, capped to HD like the capture was.
func TestRenderStageFrameFillsALargerFrame(t *testing.T) {
	a := newApp()
	a.addSong("amazing-grace")
	a.addVerse("psalm23-1")
	src := a.frameSource(2560, 1080)
	f, ok := src.frame(2560, 1080)
	if !ok {
		t.Fatal("no frame")
	}
	if f.w != 1920 || f.h != 810 {
		t.Fatalf("frame = %dx%d, want the HD cap at the frame's aspect, 1920x810", f.w, f.h)
	}
	if !hasTextInk(f, stageTheme().Text) {
		t.Fatal("the capped frame lost the slide text")
	}
}

// isOneColour reports whether the frame is a blacked-out stage: one colour
// all the way across, and not one pixel of text on it.
func isOneColour(pix []byte) bool {
	seen := map[[3]byte]struct{}{}
	for i := 0; i+2 < len(pix); i += 4 {
		seen[[3]byte{pix[i], pix[i+1], pix[i+2]}] = struct{}{}
	}
	return len(seen) == 1
}

// isFill reports whether every pixel of the frame is exactly c.
func isFill(f ndiFrame, c ui.Color) bool {
	for i := 0; i+2 < len(f.pix); i += 4 {
		if f.pix[i] != c.B || f.pix[i+1] != c.G || f.pix[i+2] != c.R {
			return false
		}
	}
	return true
}

// hasTextInk reports whether some pixel carries want's own colour, which a
// run of text drawn in it leaves in its core: antialiasing only lightens
// the edges towards the background.
func hasTextInk(f ndiFrame, want ui.Color) bool {
	// BGRA in the frame, RGBA in the theme.
	c := [3]byte{want.B, want.G, want.R}
	for i := 0; i+2 < len(f.pix); i += 4 {
		if f.pix[i] == c[0] && f.pix[i+1] == c[1] && f.pix[i+2] == c[2] {
			return true
		}
	}
	return false
}

// The conversion names all four bytes of a pixel: under this toolchain a
// group assignment that names only some of them zeroes the rest (see bgrA),
// and a frame that loses its alpha or one colour is a black feed.
func TestRenderFrameSwapsColourAndKeepsAlpha(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	copy(img.Pix, []byte{11, 10, 9, 255, 246, 241, 232, 128})
	f := renderFrame(img)
	want := []byte{9, 10, 11, 255, 232, 241, 246, 128}
	if string(f.pix) != string(want) {
		t.Fatalf("frame = %v, want BGRA %v", f.pix, want)
	}
}
