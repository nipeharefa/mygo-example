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

	if a.blackout {
		ui.Column(c).Fill().Background(ui.Hex(stageBackgroundHex))
	} else {
		ui.Column(c).Fill().Background(t.Background).Padding(56, 72).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			s, ok := a.currentSlide()
			if !ok {
				ui.Text(c, "Nothing on stage").FontSize(24).TextColor(t.TextMuted)
				return
			}
			ui.Text(c, s.eyebrow()).FontSize(14).FontWeight(600).LetterSpacing(2).TextColor(t.Accent).SingleLine()
			ui.Text(c, s.Text).FontSize(40).LineHeight(1.3).TextAlign(ui.Center).TextColor(t.Text)
			ui.Text(c, s.caption()).FontSize(16).TextColor(t.TextMuted)
		})
	}

	c.OnShortcut(0, ui.KeyRight, a.nextSlide)
	c.OnShortcut(0, ui.KeyDown, a.nextSlide)
	c.OnShortcut(0, ui.KeySpace, a.nextSlide)
	c.OnShortcut(0, ui.KeyLeft, a.prevSlide)
	c.OnShortcut(0, ui.KeyUp, a.prevSlide)
	c.OnShortcut(0, ui.KeyB, a.toggleBlackout)
	c.OnShortcut(0, ui.KeyEscape, func() {
		if a.live != nil && a.live.IsFullScreen() {
			a.live.SetFullScreen(false)
		}
	})
}

// openOutput creates the second window, or brings it back when it already
// exists. On a machine with a second display it opens fullscreen there.
func (a *app) openOutput() {
	if a.live != nil {
		a.live.Show()
		a.live.Focus()
		return
	}
	opts := mygo.WindowOptions{
		Title:           appTitle + " — Live",
		Width:           1280,
		Height:          720,
		MinWidth:        640,
		MinHeight:       360,
		BackgroundColor: stageBackgroundHex,
		Content:         ui.View(a.liveView),
	}
	if d, ok := secondDisplay(); ok {
		opts.X, opts.Y = int(d.Bounds.X), int(d.Bounds.Y)
		opts.Width, opts.Height = int(d.Bounds.Width), int(d.Bounds.Height)
		opts.FullScreen = true
	}
	live := mygo.NewWindow(opts)
	a.live = live
	live.OnClosed(func() {
		// Only clear if this window is still the current one: a close racing
		// a reopen must not drop the new window's state.
		if a.live != live {
			return
		}
		a.stopNDI()
		a.live = nil
		a.changed()
	})
	if a.autoNDI {
		_ = a.startNDI()
	}
	a.changed()
}

// closeOutput drops the live window and the NDI source that was publishing
// it. Closing through OnClosed would do the same, but nil-ing first keeps a
// stale pointer out of the next frame.
func (a *app) closeOutput() {
	live := a.live
	if live == nil {
		return
	}
	a.live = nil
	a.stopNDI()
	a.changed()
	live.Close()
}

// togglePresent switches the live window between windowed and fullscreen.
func (a *app) togglePresent() {
	if a.live == nil {
		return
	}
	a.live.SetFullScreen(!a.live.IsFullScreen())
	a.changed()
}

// secondDisplay returns the first display that is not the primary one, so a
// projector plugged in on the right gets the output fullscreen.
func secondDisplay() (mygo.Display, bool) {
	for _, d := range mygo.Screen.Displays() {
		if !d.Primary {
			return d, true
		}
	}
	return mygo.Display{}, false
}
