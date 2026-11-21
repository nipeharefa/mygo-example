//go:build darwin || linux || windows

package main

import (
	"errors"
	"testing"
	"time"
)

// fakeSource stands in for the stage renderer, so the frame loop's counters
// can be tested without a window or an NDI runtime.
type fakeSource struct {
	shot ndiFrame
	ok   bool
}

func (f *fakeSource) frame(w, h int) (ndiFrame, bool) { return f.shot, f.ok }

func TestNDIStatsSnapshot(t *testing.T) {
	s := &ndiStream{}
	if got := s.snapshot(); got.Frames != 0 || got.Width != 0 || got.FPS != 0 {
		t.Fatalf("empty snapshot = %+v, want zero", got)
	}

	s.sent(1920, 1080)
	s.sent(1920, 1080)
	s.skipped()
	s.failed(errors.New("capture failed"))
	got := s.snapshot()
	if got.Frames != 2 || got.Dropped != 1 {
		t.Fatalf("snapshot = %+v, want 2 frames and 1 skipped", got)
	}
	if got.Width != 1920 || got.Height != 1080 {
		t.Fatalf("resolution = %dx%d, want 1920x1080", got.Width, got.Height)
	}
	if got.Err != "capture failed" {
		t.Fatalf("error = %q", got.Err)
	}
	if got.Uptime < 0 {
		t.Fatalf("uptime = %v", got.Uptime)
	}
}

// A frame rate needs a window of at least a second before it means anything.
func TestNDIStatsRateNeedsASecond(t *testing.T) {
	s := &ndiStream{}
	s.sent(1280, 720)
	if got := s.snapshot().FPS; got != 0 {
		t.Fatalf("fps = %v after one frame, want 0", got)
	}
	s.mu.Lock()
	s.lastFrame = s.first.Add(2 * time.Second)
	s.mu.Unlock()
	if got := s.snapshot().FPS; got != 0 {
		t.Fatalf("fps = %v with one frame over two seconds, want 0", got)
	}
	for i := 0; i < 59; i++ {
		s.sent(1280, 720)
	}
	s.mu.Lock()
	s.lastFrame = s.first.Add(2 * time.Second)
	s.mu.Unlock()
	if got := s.snapshot().FPS; got < 29 || got > 30 {
		t.Fatalf("fps = %v for 60 frames over two seconds, want about 29.5", got)
	}
}
