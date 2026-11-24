package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openStageDelay is how long after a frame that asked for the stage the
// console asks the main loop for another one. Any later frame would do; a
// short wait keeps the output opening as the click finishes.
const openStageDelay = 10 * time.Millisecond

// bringStageForward shows an open stage and places it where it belongs: the
// window already exists, so there is no window to create on the main
// thread, but the placement may still take a fullscreen transition.
func (a *app) bringStageForward() {
	if a.live == nil {
		return
	}
	a.openOutput()
}

// liveView draws the output window: the selected slide on the stage theme,
// or a plain black frame while the console is blacked out. It also owns the
// window's shortcuts, so the operator can drive the service from the live
// window when the console is on another screen.
func (a *app) liveView(c *ui.Context) {
	c.SetTheme(stageTheme())
	t := c.Theme()

	// The window has no title bar (WindowOptions.Frameless), so the slide
	// itself is the drag handle when the output is windowed. Dragging over
	// the text still moves the window: only interactive elements take the
	// press, and a slide has none.
	if a.blackout {
		ui.Column(c).Fill().Background(ui.Hex(stageBackgroundHex)).DragWindow()
	} else {
		s, ok := a.stageSlide()
		width, _ := c.Size()
		m := metricsFor(width, s.TextSize)
		ui.Column(c).Fill().Background(t.Background).Padding(m.PadX, m.PadY).Justify(ui.Center).AlignItems(ui.Center).DragWindow().Children(func() {
			if !ok {
				ui.Text(c, "Nothing on stage").FontSize(24).TextColor(t.TextMuted)
				return
			}
			slideBody(c, s, m, t)
		})
	}

	c.OnShortcut(0, ui.KeyRight, a.nextStage)
	c.OnShortcut(0, ui.KeyDown, a.nextStage)
	c.OnShortcut(0, ui.KeySpace, a.nextStage)
	c.OnShortcut(0, ui.KeyLeft, a.prevStage)
	c.OnShortcut(0, ui.KeyUp, a.prevStage)
	c.OnShortcut(0, ui.KeyB, a.toggleBlackout)
	c.OnShortcut(0, ui.KeyEscape, func() {
		switch {
		case a.live == nil:
		case a.live.IsFullScreen():
			a.live.SetFullScreen(false)
		default:
			// Windowed: Esc is the only way out of a chromeless window.
			a.closeOutput()
		}
	})
}

// openOutput creates the stage window, or brings it back when it already
// exists, and places it. It creates the window only once: an open stage is
// moved by placeStage, never recreated, since a fullscreen window never
// reports the size a new one would ask for.
//
// A window's platform work belongs on the main loop, outside the event that
// asked for it. Creating the stage from a click draws its first frame
// before the console frame that event already asked for, and each platform
// call on a window that is still being shown waits for the window server;
// done from the click itself, that is a console that stops responding. So
// the ask is recorded (requestStage) and acted on in a later frame.
func (a *app) openOutput() {
	live := a.live
	switch {
	case live == nil:
		// Frameless, so the stage is the slide alone: no title bar,
		// traffic lights or resize edges. The feed does not need it; it
		// is the operator's view of the room.
		live = a.newWindow(mygo.WindowOptions{
			Title:           appTitle + " · Live",
			Width:           1280,
			Height:          720,
			MinWidth:        640,
			MinHeight:       360,
			Frameless:       true,
			BackgroundColor: stageBackgroundHex,
			Content:         ui.View(a.liveView),
		})
		if live == nil {
			return
		}
		a.live = live
		// Both callbacks are guarded by identity: a close or a fullscreen
		// event from a stage that has since been replaced must not touch
		// the new one.
		live.OnClosed(func() { a.shutStage(live) })
		live.OnEnterFullScreen(func() { a.placeStage(live) })
	default:
		live.Show()
		live.Focus()
	}
	a.placeStage(live)
}

// placeStage puts the open stage where the preference says: fullscreen on
// the picked display, or a window on the console's screen. It runs after
// the window exists, so the platform calls see a real window, and again
// when the stage enters fullscreen. Every platform call is skipped when the
// stage is already there: on macOS each one is answered by the window
// server, and answering them from a fullscreen event rings that server at
// itself.
func (a *app) placeStage(live *mygo.Window) bool {
	if live == nil || a.live != live {
		return false
	}
	if d, ok := stageDisplayFound(a.displays, a.stageDisplay); ok {
		if a.stageDisplayUsed != d.ID || !live.IsFullScreen() {
			live.SetPosition(int(d.WorkArea.X), int(d.WorkArea.Y))
			live.SetFullScreen(true)
		}
		a.stageDisplayUsed = d.ID
	} else if live.IsFullScreen() {
		live.SetFullScreen(false)
		a.stageDisplayUsed = 0
	} else {
		a.stageDisplayUsed = 0
	}
	if a.ndiOn {
		_ = a.startNDI()
	}
	a.changed()
	return true
}

// shutStage runs once when the stage window closes, whether from the
// console, its own Esc, or the window's close button. NDI keeps publishing:
// the feed draws the slide, so the room does not go dark because the
// operator closed the window it was previewing on.
func (a *app) shutStage(live *mygo.Window) {
	if live == nil || a.live != live {
		return
	}
	a.live = nil
	a.stageDisplayUsed = 0
	a.changed()
}

// closeOutput drops the stage window. The window is forgotten first, so its
// own close event finds nothing to do.
func (a *app) closeOutput() {
	live := a.live
	if live == nil {
		return
	}
	a.shutStage(live)
	if live.IsFullScreen() {
		live.SetFullScreen(false)
	}
	live.Close()
}

// togglePresent switches the stage between windowed and fullscreen.
func (a *app) togglePresent() {
	live := a.live
	if live == nil {
		return
	}
	if live.IsFullScreen() {
		live.SetFullScreen(false)
		a.stageDisplayUsed = 0
	} else if d, ok := stageDisplayFound(a.displays, a.stageDisplay); ok {
		live.SetPosition(int(d.WorkArea.X), int(d.WorkArea.Y))
		live.SetFullScreen(true)
		a.stageDisplayUsed = d.ID
	} else {
		live.SetFullScreen(true)
	}
	a.changed()
}
