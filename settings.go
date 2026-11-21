package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settings is the small part of the app that survives a restart. The run of
// show does not: the deck is a session, the preferences are not.
type settings struct {
	// Display is where the stage opens fullscreen; 0 means a window on the
	// screen the console is on.
	Display int64 `json:"display"`
	NDI     bool  `json:"ndi"`
}

// legacySettings is the file written before the display key was named
// plainly. It is only read when the current key is absent, and 0 meant
// "automatic", which this app no longer offers: 0 is a window now.
type legacySettings struct {
	OutputDisplay *int64 `json:"outputDisplay"`
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
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return settings{}
	}
	if _, ok := keys["display"]; !ok {
		var old legacySettings
		if err := json.Unmarshal(data, &old); err == nil && old.OutputDisplay != nil {
			s.Display = *old.OutputDisplay
		}
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
// attached falls back to a window.
func (a *app) loadSettings() {
	s := readSettings(a.settingsPath())
	a.stageDisplay = s.Display
	a.ndiOn = s.NDI
	// Only drop the saved display when the list is known: startup fills it
	// before loading, while a headless test may leave it empty.
	if len(a.displays) > 0 && a.stageDisplay != 0 && !a.hasDisplay(a.stageDisplay) {
		a.stageDisplay = 0
	}
}

// saveSettings writes the preferences. Failures only cost the next launch's
// settings, so they are logged rather than shown.
func (a *app) saveSettings() {
	path := a.settingsPath()
	if path == "" {
		return
	}
	err := writeSettings(path, settings{Display: a.stageDisplay, NDI: a.ndiOn})
	if err != nil {
		log.Printf("mygo-demo: save settings: %v", err)
	}
}

// hasDisplay reports whether a saved display is still reported.
func (a *app) hasDisplay(id int64) bool {
	_, ok := stageDisplayFound(a.displays, id)
	return ok
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

// displayOption is the radio label for a display: name, role and size.
func displayOption(d mygo.Display) string {
	return fmt.Sprintf("%s · %d×%d", displayName(d), int(d.Bounds.Width), int(d.Bounds.Height))
}

// stageDisplayFound returns the display the stage opens on. Only a display
// the operator picked counts, and only while it is attached: 0, a missing
// display or an empty list means the stage is a plain window.
func stageDisplayFound(displays []mygo.Display, id int64) (mygo.Display, bool) {
	if id == 0 {
		return mygo.Display{}, false
	}
	for _, d := range displays {
		if d.ID == id {
			return d, true
		}
	}
	return mygo.Display{}, false
}

// showSettings brings the Settings page of the console forward.
func (a *app) showSettings() {
	a.page = pageSettings
	if a.win != nil {
		a.win.Focus()
	}
	a.changed()
}

// settingsBody is the Settings page of the console: where the stage opens,
// whether NDI publishes and how that stream is doing, and what is saved.
func (a *app) settingsBody(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Fill().Padding(24, 28).Children(func() {
		ui.Column(c).Gap(16).MaxWidth(c.Theme().Rem(72)).Children(func() {
			ui.Text(c, "Settings").FontSize(20).Bold()

			settingsHeading(c, "Stage output")
			ui.Text(c, "A display opens the stage fullscreen there. A window stays on this screen; drag it to a projector and press Present.").
				FontSize(11).TextColor(t.TextMuted)
			ui.Radio(c, &a.stageDisplay, int64(0), "Windowed on this screen").OnChange(a.stageDisplayChanged)
			for _, d := range a.displays {
				ui.Radio(c, &a.stageDisplay, d.ID, displayOption(d)).OnChange(a.stageDisplayChanged)
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
			a.ndiStatsSection(c)

			ui.Divider(c)
			settingsHeading(c, "Data")
			ui.Text(c, "The run of show is not saved. The display and NDI preferences are saved in the app data directory.").
				FontSize(11).TextColor(t.TextMuted)
			if a.settingsDir == "" {
				ui.Text(c, "Preferences are not saved in this session.").FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
			ui.Text(c, "The hymns and the King James verses are public domain.").FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// ndiStatsSection is the stream's own report. NDI tells a receiver nothing
// about who is watching, so the sender's numbers are the only answer to "is
// it actually publishing, and in HD?".
func (a *app) ndiStatsSection(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "NDI stream").FontSize(12).FontWeight(600).SingleLine()
	ui.Text(c, "Sender stats: NDI 5 and 6 have no receiver tally.").FontSize(10).TextColor(t.TextMuted)
	if a.ndi == nil {
		ui.Text(c, "Not publishing, so there is nothing to measure.").FontSize(11).TextColor(t.TextMuted)
		return
	}
	// Ask the console for the stats timer: the numbers only move while this
	// page draws them.
	a.statsWanted = true
	s := a.ndi.snapshot()
	ui.Textf(c, "%d × %d · %.1f fps · %s", s.Width, s.Height, s.FPS, formatUptime(s.Uptime)).FontSize(13).FontWeight(600).SingleLine()
	rows := [][2]string{
		{"Frames sent", formatCount(s.Frames)},
		{"Skipped", formatCount(s.Dropped)},
		{"State", "Publishing as “" + stageSourceName + "”"},
	}
	if s.Err != "" {
		rows = append(rows, [2]string{"Last error", s.Err})
	}
	for _, row := range rows {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, row[0]).FontSize(11).TextColor(t.TextMuted).Width(110).SingleLine()
			ui.Text(c, row[1]).FontSize(11).SingleLine()
		})
	}
}

// settingsHeading is the small uppercase heading the page repeats.
func settingsHeading(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(10).FontWeight(600).LetterSpacing(0.8).SingleLine()
}

// stageDisplayChanged runs when a display radio is picked: the preference is
// saved and an open stage moves to the new display.
func (a *app) stageDisplayChanged() {
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
		return "On. Starts when the stage opens.", t.TextMuted
	default:
		return "Off.", t.TextMuted
	}
}

// formatCount prints a frame count with thousands separators.
func formatCount(n int64) string {
	s := fmt.Sprint(n)
	neg := false
	if n < 0 {
		neg, s = true, s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}

// formatUptime prints an elapsed time as h:mm:ss.
func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int(d/time.Minute) % 60
	s := int(d/time.Second) % 60
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

// displaysChanged refreshes the display list after a monitor is plugged in or
// removed.
func (a *app) displaysChanged() {
	a.refreshDisplays(a.mainScreen())
}

// refreshDisplays applies a new display list. It only moves the stage when
// the display the stage is using vanished: closing and reopening it on every
// notification is what used to loop.
func (a *app) refreshDisplays(displays []mygo.Display) {
	a.displays = displays
	if a.stageDisplay != 0 && !a.hasDisplay(a.stageDisplay) {
		a.stageDisplay = 0
		a.saveSettings()
	}
	if a.live == nil {
		a.changed()
		return
	}
	if a.stageDisplayUsed != 0 && !a.hasDisplay(a.stageDisplayUsed) {
		a.closeOutput()
		a.openOutput()
		return
	}
	a.changed()
}
