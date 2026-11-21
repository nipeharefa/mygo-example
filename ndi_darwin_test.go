//go:build darwin

package main

import "testing"

// TestNDIFrameSize pins the rule that keeps the NDI feed at HD: a window
// already at or below the cap is sent as captured, and a Retina window is
// scaled down without changing its aspect ratio.
func TestNDIFrameSize(t *testing.T) {
	cases := []struct {
		w, h         int
		wantW, wantH int
		name         string
	}{
		{1280, 720, 1280, 720, "windowed 720p is sent as captured"},
		{1920, 1080, 1920, 1080, "fullscreen 1080p is sent as captured"},
		{2560, 1440, 1920, 1080, "Retina window is scaled to 1080p"},
		{3840, 2160, 1920, 1080, "4K is scaled to 1080p"},
		{1080, 1920, 608, 1080, "a tall window is capped by height and stays even"},
	}
	for _, c := range cases {
		gotW, gotH := ndiFrameSize(c.w, c.h)
		if gotW != c.wantW || gotH != c.wantH {
			t.Errorf("%s: ndiFrameSize(%d, %d) = %d×%d, want %d×%d",
				c.name, c.w, c.h, gotW, gotH, c.wantW, c.wantH)
		}
	}
}
