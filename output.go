package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

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
		ui.Column(c).Fill().Background(t.Background).Padding(56, 72).Justify(ui.Center).AlignItems(ui.Center).DragWindow().Children(func() {
			s, ok := a.stageSlide()
			if !ok {
				ui.Text(c, "Nothing on stage").FontSize(24).TextColor(t.TextMuted)
				return
			}
			ui.Text(c, s.eyebrow()).FontSize(14).FontWeight(600).LetterSpacing(2).TextColor(t.Accent).SingleLine()
			ui.Text(c, s.Text).FontSize(40).LineHeight(1.3).TextAlign(ui.Center).TextColor(t.Text)
			ui.Text(c, s.caption()).FontSize(16).TextColor(t.TextMuted)
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
// exists. A display picked in Settings places it there, fullscreen.
func (a *app) openOutput() {
	if a.live != nil {
		a.live.Show()
		a.live.Focus()
		return
	}
	// Frameless, so the capture NDI publishes is the slide alone: no title
	// bar, traffic lights or resize edges in the feed.
	live := a.newWindow(mygo.WindowOptions{
		Title:           appTitle + " — Live",
		Width:           1280,
		Height:          720,
		MinWidth:        640,
		MinHeight:       360,
		Frameless:       true,
		BackgroundColor: stageBackgroundHex,
		Content:         ui.View(a.liveView),
	})
	a.live = live
	a.stageDisplayUsed = 0
	// Both callbacks are guarded by identity: a close or a fullscreen event
	// from a stage that has since been replaced must not touch the new one.
	live.OnClosed(func() { a.shutStage(live) })
	live.OnEnterFullScreen(func() { a.placeStage(live) })
	if !a.placeStage(live) {
		a.changed()
	}
}

// placeStage puts the open stage where the preference says: fullscreen on
// the picked display, or a window on the console's screen. Placement runs
// after creation so the platform call sees a real window.
func (a *app) placeStage(live *mygo.Window) bool {
	if live == nil || a.live != live {
		return false
	}
	if d, ok := stageDisplayFound(a.displays, a.stageDisplay); ok {
		live.SetPosition(int(d.WorkArea.X), int(d.WorkArea.Y))
		live.SetFullScreen(true)
		a.stageDisplayUsed = d.ID
	} else {
		live.SetFullScreen(false)
		a.stageDisplayUsed = 0
	}
	if a.ndiOn {
		_ = a.startNDI()
	}
	a.changed()
	return true
}

// shutStage runs once when the stage window closes, whether from the
// console, its own Esc, or the window's close button.
func (a *app) shutStage(live *mygo.Window) {
	if live == nil || a.live != live {
		return
	}
	a.live = nil
	a.stageDisplayUsed = 0
	a.stopNDI()
	a.changed()
}

// closeOutput drops the stage window and the NDI source publishing it. The
// window is forgotten first, so its own close event finds nothing to do.
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
