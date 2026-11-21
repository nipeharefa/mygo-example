package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settings is the small part of the app that survives a restart. The run of
// show does not: the deck is a session, the preferences are not.
type settings struct {
	OutputDisplay int64 `json:"outputDisplay"`
	NDI           bool  `json:"ndi"`
}

// readSettings returns saved preferences, or the defaults when the file is
// missing or unreadable. A demo app should open even with a broken file.
func readSettings(path string) settings {
	if path == "" {
		return settings{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings{}
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return settings{}
	}
	return s
}

// writeSettings stores the preferences, creating the app data directory on
// first use.
func writeSettings(path string, s settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// settingsPath is where the preferences live, or "" before the app is ready
// (and in tests, which never touch the real app data directory).
func (a *app) settingsPath() string {
	if a.settingsDir == "" {
		return ""
	}
	return filepath.Join(a.settingsDir, "settings.json")
}

// loadSettings reads the saved preferences. A display that is no longer
// attached falls back to automatic placement.
func (a *app) loadSettings() {
	s := readSettings(a.settingsPath())
	a.outputDisplay = s.OutputDisplay
	a.ndiOn = s.NDI
	// Only drop the saved display when the list is known: startup fills it
	// before loading, while a headless test may leave it empty.
	if len(a.displays) > 0 && a.outputDisplay != 0 && !a.hasDisplay(a.outputDisplay) {
		a.outputDisplay = 0
	}
}

// saveSettings writes the preferences. Failures only cost the next launch's
// settings, so they are logged rather than shown.
func (a *app) saveSettings() {
	path := a.settingsPath()
	if path == "" {
		return
	}
	err := writeSettings(path, settings{OutputDisplay: a.outputDisplay, NDI: a.ndiOn})
	if err != nil {
		log.Printf("mygo-demo: save settings: %v", err)
	}
}

// hasDisplay reports whether a saved display is still reported.
func (a *app) hasDisplay(id int64) bool {
	for _, d := range a.displays {
		if d.ID == id {
			return true
		}
	}
	return false
}

// displayName labels a display in Settings.
func displayName(d mygo.Display) string {
	name := d.Label
	if name == "" {
		name = "Display"
	}
	role := "external"
	if d.Primary {
		role = "primary"
	} else if d.Internal {
		role = "built-in"
	}
	return name + " · " + role
}

// openSettings shows the Settings window, or focuses it when it is open.
func (a *app) openSettings() {
	if a.settings != nil {
		a.settings.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     appTitle + " — Settings",
		Width:     560,
		Height:    520,
		MinWidth:  460,
		MinHeight: 420,
		StateKey:  "settings",
		Content:   ui.View(a.settingsView),
	})
	a.settings = w
	w.OnClosed(func() {
		if a.settings != w {
			return
		}
		a.settings = nil
	})
}

// closeSettings drops the Settings window.
func (a *app) closeSettings() {
	w := a.settings
	if w == nil {
		return
	}
	a.settings = nil
	w.Close()
}

// settingsView is the preferences window: where the stage opens, whether
// NDI publishes, and what is and is not saved.
func (a *app) settingsView(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Padding(20, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Settings").FontSize(18).Bold().SingleLine()
			ui.Spacer(c)
			ui.Button(c, "Close").OnClick(a.closeSettings)
		})
		ui.Divider(c)
		ui.Scroll(c).Fill().Padding(20).Children(func() {
			ui.Column(c).Gap(16).Children(func() {
				settingsHeading(c, "Stage output")
				ui.Text(c, "Automatic follows a projector, otherwise the stage opens as a window on this screen.").
					FontSize(11).TextColor(t.TextMuted)
				ui.Radio(c, &a.outputDisplay, int64(0), "Automatic").OnChange(a.outputDisplayChanged)
				for _, d := range a.displays {
					ui.Radio(c, &a.outputDisplay, d.ID, displayOption(d)).OnChange(a.outputDisplayChanged)
				}
				if len(a.displays) == 0 {
					ui.Text(c, "No displays reported.").FontSize(11).TextColor(t.TextMuted).SingleLine()
				}

				ui.Divider(c)
				settingsHeading(c, "NDI")
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Text(c, "Publish the stage over NDI").FontSize(12).FontWeight(600).SingleLine()
						ui.Text(c, "The feed is the slide alone, up to 1920 × 1080.").FontSize(10).TextColor(t.TextMuted)
					})
					ui.Switch(c, &a.ndiOn).Label("Publish the stage over NDI").
						OnChange(a.ndiToggled).Disabled(!ndiSupported())
				})
				status, color := a.ndiStatus(t)
				ui.Text(c, status).FontSize(11).TextColor(color)

				ui.Divider(c)
				settingsHeading(c, "Data")
				ui.Text(c, "The run of show is not saved. The display and NDI preferences are saved in the app data directory.").
					FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, "The hymns and the King James verses are public domain.").FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

// settingsHeading is the small uppercase heading the window repeats.
func settingsHeading(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(10).FontWeight(600).LetterSpacing(0.8).SingleLine()
}

// outputDisplayChanged runs when a display radio is picked: the preference
// is saved and an open stage moves to the new display.
func (a *app) outputDisplayChanged() {
	a.saveSettings()
	if a.live != nil {
		a.closeOutput()
		a.openOutput()
		return
	}
	a.changed()
}

// ndiToggled is the Switch handler: the switch already wrote a.ndiOn.
func (a *app) ndiToggled() {
	a.setNDI(a.ndiOn)
}

// setNDI records the NDI preference and applies it to an open stage. Turning
// it on with no stage open is not an error: the preference waits.
func (a *app) setNDI(on bool) {
	if !ndiSupported() {
		return
	}
	a.ndiOn = on
	a.saveSettings()
	if on {
		if a.live != nil {
			_ = a.startNDI()
		}
	} else {
		a.stopNDI()
		a.ndiErr = ""
	}
	a.changed()
}

// ndiStatus explains the current NDI state under the switch.
func (a *app) ndiStatus(t *ui.Theme) (string, ui.Color) {
	switch {
	case !ndiSupported():
		return "NDI is unavailable in this build.", t.TextMuted
	case a.ndiErr != "":
		return "NDI: " + a.ndiErr, t.Danger
	case a.ndi != nil:
		return "Publishing as “" + stageSourceName + "”.", t.Success
	case a.ndiOn:
		return "Starts when the stage opens.", t.TextMuted
	default:
		return "Not publishing.", t.TextMuted
	}
}

// displayOption is the radio label for a display: name, role and size.
func displayOption(d mygo.Display) string {
	return fmt.Sprintf("%s · %d×%d", displayName(d), int(d.Bounds.Width), int(d.Bounds.Height))
}

// displaysChanged refreshes the display list after a monitor is plugged in or
// removed, and moves an open stage when its display vanished.
func (a *app) displaysChanged() {
	a.refreshDisplays(mygo.Screen.Displays())
}

// refreshDisplays applies a new display list to the open stage and to the
// saved preference; split from the platform call so tests stay headless.
func (a *app) refreshDisplays(displays []mygo.Display) {
	a.displays = displays
	if a.outputDisplay != 0 && !a.hasDisplay(a.outputDisplay) {
		a.outputDisplay = 0
		a.saveSettings()
	}
	if a.live != nil {
		a.closeOutput()
		a.openOutput()
		return
	}
	a.changed()
}
