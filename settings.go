package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
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
