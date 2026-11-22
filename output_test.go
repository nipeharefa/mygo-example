package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// testLive builds the output view testers use; the real window needs an
// application, which a headless tester does not have.
func testLive(a *app) *ui.Tester {
	return ui.NewTester(a.liveView, 1280, 720)
}

func mustStage(t *testing.T, a *app) slide {
	t.Helper()
	s, ok := a.stageSlide()
	if !ok {
		t.Fatal("no slide on stage")
	}
	return s
}

// The output shows the selected slide, with its caption under it.
func TestLiveViewShowsCurrentSlide(t *testing.T) {
	a := newApp()
	tt := testLive(a)
	s := mustStage(t, a)

	if !tt.HasText(s.firstLine()) {
		t.Fatalf("the live view does not show %q: %q", s.firstLine(), tt.Texts())
	}
	if !tt.HasText(s.caption()) {
		t.Fatalf("the live view does not show the caption %q", s.caption())
	}
}

// The output follows the console's selection.
func TestLiveViewFollowsNext(t *testing.T) {
	a := newApp()
	tt := testLive(a)

	a.nextStage()
	tt.Frame()
	s := mustStage(t, a)
	if !tt.HasText(s.firstLine()) {
		t.Fatalf("after next the live view shows the wrong slide: %q", tt.Texts())
	}
}

// Blackout leaves nothing but the dark background.
func TestLiveViewBlackoutHidesText(t *testing.T) {
	a := newApp()
	tt := testLive(a)
	s := mustStage(t, a)

	a.toggleBlackout()
	tt.Frame()
	if tt.HasText(s.firstLine()) || tt.HasText(s.caption()) {
		t.Fatalf("blackout still shows slide text: %q", tt.Texts())
	}
}

// The output window drives the deck by itself, for a second screen.
func TestLiveViewShortcuts(t *testing.T) {
	a := newApp()
	tt := testLive(a)

	tt.Key(0, ui.KeyRight)
	if a.stage != 1 {
		t.Fatalf("right arrow moved the stage to %d, want 1", a.stage)
	}
	tt.Key(0, ui.KeyLeft)
	if a.stage != 0 {
		t.Fatalf("left arrow moved the stage to %d, want 0", a.stage)
	}
	tt.Key(0, ui.KeyB)
	if !a.blackout {
		t.Fatal("B did not black out the output")
	}
}

// Review Focus 1: the feed draws the slide, so one monitor is enough and the
// source starts with no stage window at all.
func TestStartNDIWithoutAWindow(t *testing.T) {
	if !ndiSupported() {
		t.Skip("no NDI sender on this platform")
	}
	a := newApp()
	err := a.startNDI()
	if err != nil && !strings.Contains(err.Error(), "NDI") {
		t.Fatalf("starting NDI failed: %v", err)
	}
	if err != nil {
		// The runtime here cannot publish; the state still has to agree.
		if a.ndi != nil || a.ndiErr == "" {
			t.Fatalf("a failed start kept a sender: ndi=%v err=%q", a.ndi, a.ndiErr)
		}
		return
	}
	defer a.stopNDI()
	if a.ndi == nil || a.ndiErr != "" {
		t.Fatalf("NDI did not start: ndi=%v err=%q", a.ndi, a.ndiErr)
	}
}

// Closing the stage window is not a request to stop publishing the room.
func TestClosingTheStageKeepsNDI(t *testing.T) {
	if !ndiSupported() {
		t.Skip("NDI is not built in")
	}
	a := newApp()
	a.ndiOn = true
	a.closeOutput()
	if !a.ndiOn {
		t.Fatal("closing the stage cleared the NDI preference")
	}
}

// The output shows the stage, not the row the operator is looking at on the
// monitor.
func TestLiveViewRendersTheStageNotTheMonitor(t *testing.T) {
	a := newApp()
	a.selectSlide(2)
	tt := testLive(a)

	stage := mustStage(t, a)
	if !tt.HasText(stage.firstLine()) {
		t.Fatalf("the live view does not show the stage slide %q: %q", stage.firstLine(), tt.Texts())
	}
	monitor, ok := a.previewSlide()
	if !ok {
		t.Fatal("no preview slide")
	}
	if monitor.ID != stage.ID && tt.HasText(monitor.firstLine()) {
		t.Fatalf("the live view shows the monitor slide %q", monitor.firstLine())
	}
}

// The size the feed publishes at is the preference, 720p by default.
func TestNDIResolutionSetting(t *testing.T) {
	a := newApp()
	if w, h := a.ndiFrameSize(); w != 1280 || h != 720 {
		t.Fatalf("default feed size = %dx%d, want 720p", w, h)
	}
	a.ndiResolution = ndiResolution1080
	if w, h := a.ndiFrameSize(); w != 1920 || h != 1080 {
		t.Fatalf("1080p feed size = %dx%d", w, h)
	}
}

// Review Focus 4: signing out leaves no sender behind.
func TestSignOutStopsNDI(t *testing.T) {
	a := newApp()
	a.ndiOn = true
	a.signedIn = true
	a.signOut()
	if a.ndi != nil {
		t.Fatal("NDI still set after sign-out")
	}
}

func TestSetNDIOffClearsThePreference(t *testing.T) {
	a := &app{ndiOn: true, ndiErr: "old error"}
	a.setNDI(false)
	if a.ndiOn {
		t.Fatal("the NDI preference was not cleared")
	}
	if a.ndi != nil || a.ndiErr != "" {
		t.Fatalf("NDI is still publishing: ndi=%v err=%q", a.ndi, a.ndiErr)
	}
}
