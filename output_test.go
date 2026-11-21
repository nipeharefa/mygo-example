package main

import (
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

// NDI needs the live window; without it the console shows why.
func TestStartNDIWithoutOutput(t *testing.T) {
	if !ndiSupported() {
		t.Skip("no NDI sender on this platform")
	}
	a := newApp()
	if err := a.startNDI(); err == nil {
		t.Fatal("starting NDI without an output window should fail")
	}
	if a.ndi != nil {
		t.Fatal("a sender was kept without an output window")
	}
	if a.ndiErr == "" {
		t.Fatal("no error was recorded for the console")
	}
}

// The NDI toggle is not offered until there is a window to publish.
func TestNDIHiddenWithoutOutput(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)
	signIn(t, a, tt)

	if tt.HasText("Start NDI") {
		t.Fatalf("the console offers NDI before the output is open: %q", tt.Texts())
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

// Turning NDI on with no stage open records the preference and waits.
func TestSetNDIWaitsForTheStage(t *testing.T) {
	if !ndiSupported() {
		t.Skip("NDI is not built in")
	}
	a := &app{}
	a.setNDI(true)
	if !a.ndiOn {
		t.Fatal("the NDI preference was not recorded")
	}
	if a.ndi != nil {
		t.Fatal("NDI started without a stage window")
	}
	if a.ndiErr != "" {
		t.Fatalf("waiting for the stage set an error: %q", a.ndiErr)
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
