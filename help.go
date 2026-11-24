package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openHelp shows the one-page help window, or focuses it when it is already
// open. Menu callbacks and buttons run on the main thread, so creating a
// window here is allowed.
func (a *app) openHelp() {
	if a.help != nil {
		a.help.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     appTitle + " · Help",
		Width:     640,
		Height:    620,
		MinWidth:  520,
		MinHeight: 420,
		StateKey:  "help",
		Content:   ui.View(a.helpView),
	})
	a.help = w
	w.OnClosed(func() {
		if a.help != w {
			return
		}
		a.help = nil
	})
}

// closeHelp drops the help window.
func (a *app) closeHelp() {
	w := a.help
	if w == nil {
		return
	}
	a.help = nil
	w.Close()
}

// helpView is the reference the operator can open mid-service: what the keys
// do, how the NDI feed works, and where the sample data comes from.
func (a *app) helpView(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Padding(22, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			a.brandMark(c, textSection)
			ui.Text(c, "Help").FontSize(textBody).TextColor(t.TextMuted).SingleLine()
			ui.Spacer(c)
			ui.Button(c, "Check for Updates").OnClick(a.checkForUpdates)
			ui.Button(c, "Close").OnClick(a.closeHelp)
		})
		ui.Divider(c)
		ui.Scroll(c).Fill().Padding(22).Children(func() {
			ui.Column(c).Gap(20).Children(func() {
				helpSection(c, "About")
				version := a.version
				if version == "" {
					version = "development build"
				}
				ui.Text(c, appTitle+" · "+version).FontSize(textTitle).FontWeight(600).SingleLine()
				ui.Text(c, "A simple worship presenter: build a run of show from the library, keep the next slide on the monitor, and send the stage to the room and to NDI.").
					FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, "Demo account: "+demoUsername+" / "+demoPassword).FontSize(textBody).TextColor(t.TextMuted).SingleLine()
				ui.Text(c, "Check for Updates asks the release feed for a newer version; only the installers from a release can update themselves.").
					FontSize(11).TextColor(t.TextMuted)

				ui.Divider(c)
				helpSection(c, "Keyboard")
				helpKeys(c,
					[3]string{"Console", "← →", "Move the monitor"},
					[3]string{"", "Enter", "Go live with the selected row"},
					[3]string{"", "⌘/Ctrl + Enter", "Go live from anywhere"},
					[3]string{"", "B", "Black out or clear"},
					[3]string{"", "⌘/Ctrl + ,", "Open Settings"},
					[3]string{"Live window", "→ ↓ Space", "Next slide"},
					[3]string{"", "← ↑", "Previous slide"},
					[3]string{"", "B", "Black out or clear"},
					[3]string{"", "Esc", "Leave full screen, then close"},
				)

				ui.Divider(c)
				helpSection(c, "NDI output")
				for _, line := range []string{
					"The NDI runtime must be installed before the app starts.",
					"The source is called “" + stageSourceName + "”.",
					"The feed is the slide alone, up to 1920 × 1080, so it can go straight into OBS.",
					"The feed is drawn, so it publishes with no window on screen at all.",
					"Settings remembers whether NDI starts with the stage.",
				} {
					ui.Text(c, line).FontSize(textBody).TextColor(t.TextMuted)
				}

				ui.Divider(c)
				helpSection(c, "Sample data")
				ui.Text(c, "The hymns and the King James verses are public domain. The run of show is a session and is not saved; the display and NDI preferences are.").
					FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

// helpSection is the tracked heading the help page repeats, the same one the
// console and Settings use.
func helpSection(c *ui.Context, label string) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).PaddingY(2).Children(func() {
		ui.Text(c, label).FontSize(textLabel).FontWeight(700).LetterSpacing(0.9).
			TextColor(t.TextMuted).SingleLine()
		ui.Box(c).Height(1).Grow(1).Background(hairlines(t))
	})
}

// helpKeys renders one shortcut table. Each row is group, keys, meaning; the
// group only shows on the first row of its block.
func helpKeys(c *ui.Context, rows ...[3]string) {
	t := c.Theme()
	ui.Column(c).Gap(7).Padding(12, 10).Radius(8).Background(innerWell(t)).Border(1, hairlines(t)).Children(func() {
		for _, row := range rows {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, row[0]).FontSize(10).TextColor(t.TextMuted).Width(84).SingleLine()
				ui.Text(c, row[1]).FontSize(11).FontWeight(600).Width(120).SingleLine()
				ui.Text(c, row[2]).FontSize(textBody).TextColor(t.TextMuted).SingleLine()
			})
		}
	})
}
