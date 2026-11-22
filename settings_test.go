package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := settings{Display: 42, NDI: true, NDIResolution: ndiResolution1080}
	if err := writeSettings(path, want); err != nil {
		t.Fatalf("writeSettings: %v", err)
	}
	if got := readSettings(path); got != want {
		t.Fatalf("readSettings = %+v, want %+v", got, want)
	}
}

// 720p is the default, so it survives a round trip and a missing key.
func TestNDIResolutionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if got := readSettings(path).NDIResolution; got != ndiResolution720 {
		t.Fatalf("default resolution = %d, want 720p", got)
	}
	if err := writeSettings(path, settings{NDIResolution: ndiResolution1080}); err != nil {
		t.Fatalf("writeSettings: %v", err)
	}
	if got := readSettings(path).NDIResolution; got != ndiResolution1080 {
		t.Fatalf("resolution = %d, want 1080p", got)
	}
	a := &app{settingsDir: filepath.Dir(path)}
	a.ndiResolution = ndiResolution1080
	a.saveSettings()
	b := &app{settingsDir: a.settingsDir}
	b.loadSettings()
	if b.ndiResolution != ndiResolution1080 {
		t.Fatalf("loaded resolution = %d, want 1080p", b.ndiResolution)
	}
}

func TestReadSettingsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	if got := readSettings(path); got != (settings{}) {
		t.Fatalf("readSettings = %+v, want defaults", got)
	}
}

func TestReadSettingsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(path); got != (settings{}) {
		t.Fatalf("readSettings = %+v, want defaults", got)
	}
}

// The file written before the display key existed is still read: its
// outputDisplay names the display, and 0 meant "automatic".
func TestReadSettingsLegacyDisplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{\"outputDisplay\": 2, \"ndi\": true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(path); got != (settings{Display: 2, NDI: true}) {
		t.Fatalf("readSettings = %+v, want Display 2 and NDI", got)
	}

	if err := os.WriteFile(path, []byte("{\"outputDisplay\": 0, \"ndi\": false}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(path); got != (settings{}) {
		t.Fatalf("readSettings = %+v, want windowed", got)
	}
}

// The app saves and loads through its settings directory, and never writes
// when there is none.
func TestAppSaveLoadSettings(t *testing.T) {
	a := &app{settingsDir: t.TempDir()}
	a.stageDisplay = 7
	a.ndiOn = true
	a.ndiResolution = ndiResolution1080
	a.saveSettings()

	b := &app{settingsDir: a.settingsDir}
	b.loadSettings()
	if b.stageDisplay != 7 || !b.ndiOn || b.ndiResolution != ndiResolution1080 {
		t.Fatalf("loaded stageDisplay = %d ndiOn = %v resolution = %d, want 7/true/1080p",
			b.stageDisplay, b.ndiOn, b.ndiResolution)
	}
}

func TestSaveSettingsWithoutADirectory(t *testing.T) {
	a := &app{}
	a.stageDisplay = 7
	a.ndiOn = true
	a.saveSettings() // must not panic or touch the real app data dir
}

// A saved display that is no longer attached falls back to a window.
func TestLoadSettingsDropsMissingDisplay(t *testing.T) {
	a := &app{
		settingsDir: t.TempDir(),
		displays:    []mygo.Display{{ID: 1, Label: "Built-in", Primary: true}},
	}
	if err := writeSettings(a.settingsPath(), settings{Display: 99, NDI: true}); err != nil {
		t.Fatal(err)
	}
	a.loadSettings()
	if a.stageDisplay != 0 {
		t.Fatalf("stageDisplay = %d, want windowed", a.stageDisplay)
	}
	if !a.ndiOn {
		t.Fatal("the NDI preference was dropped with the display")
	}
}

func TestLoadSettingsKeepsAttachedDisplay(t *testing.T) {
	a := &app{
		settingsDir: t.TempDir(),
		displays:    []mygo.Display{{ID: 1, Primary: true}, {ID: 2, Label: "Projector"}},
	}
	if err := writeSettings(a.settingsPath(), settings{Display: 2}); err != nil {
		t.Fatal(err)
	}
	a.loadSettings()
	if a.stageDisplay != 2 {
		t.Fatalf("stageDisplay = %d, want 2", a.stageDisplay)
	}
}

func TestDisplayName(t *testing.T) {
	cases := []struct {
		d    mygo.Display
		want string
	}{
		{mygo.Display{Label: "DELL", Primary: true}, "DELL · primary"},
		{mygo.Display{Label: "Projector"}, "Projector · external"},
		{mygo.Display{Label: "Laptop", Internal: true}, "Laptop · built-in"},
		{mygo.Display{}, "Display · external"},
	}
	for _, c := range cases {
		if got := displayName(c.d); got != c.want {
			t.Errorf("displayName(%+v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// stageDisplayFound is where the stage opens fullscreen: only a display the
// operator picked, and only while it is attached.
func TestStageDisplayFound(t *testing.T) {
	displays := []mygo.Display{
		{ID: 1, Label: "Built-in", Primary: true},
		{ID: 2, Label: "Projector"},
	}

	if d, ok := stageDisplayFound(displays, 2); !ok || d.ID != 2 {
		t.Fatalf("picked display = %+v ok = %v", d, ok)
	}
	if _, ok := stageDisplayFound(displays, 99); ok {
		t.Fatal("a missing display was returned")
	}
	if _, ok := stageDisplayFound(displays, 0); ok {
		t.Fatal("windowed was placed on a display")
	}
	if _, ok := stageDisplayFound(nil, 0); ok {
		t.Fatal("windowed was placed on an empty list")
	}
}

func TestSettingsPageListsDisplays(t *testing.T) {
	a := &app{displays: []mygo.Display{
		{ID: 1, Label: "Built-in", Primary: true, Bounds: mygo.Rectangle{Width: 1440, Height: 900}},
		{ID: 2, Label: "Projector", Bounds: mygo.Rectangle{Width: 1920, Height: 1080}},
	}}
	tt := ui.NewTester(a.settingsBody, 900, 800)
	for _, want := range []string{"Stage output", "Windowed on this screen", "Built-in", "Projector", "Publish the stage over NDI", "Stage output size", "720p", "1080p", "NDI stream", "Data"} {
		if !tt.HasText(want) {
			t.Fatalf("settings is missing %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("Automatic") {
		t.Fatal("settings still offers Automatic")
	}
}

// The size radio restarts a running source, and the page says what the feed
// does not need to publish.
func TestNDIResolutionPage(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.settingsBody, 900, 800)
	if !tt.HasText("The feed draws the slide itself, so it does not depend on the stage window or on a second monitor.") {
		t.Fatalf("settings does not explain the windowless feed: %q", tt.Texts())
	}
	// No source is running here: changing the size moves the preference and
	// leaves publishing alone. A running source is stopped by the same
	// call, and stopping one needs a real stream (ndiStream_test.go).
	a.ndiResolution = ndiResolution1080
	a.ndiResolutionChanged()
	if a.ndiResolution != ndiResolution1080 {
		t.Fatal("the size preference did not change")
	}
	if a.ndi != nil {
		t.Fatal("changing the size started a source")
	}
	// Each radio is a clickable row of its own: both say what they set.
	for _, want := range []string{"Stage output size: 720p", "Stage output size: 1080p"} {
		if err := tt.Click(want); err != nil {
			t.Fatalf("clicking %q: %v", want, err)
		}
	}
	if a.ndiResolution != ndiResolution1080 {
		t.Fatalf("clicking 1080p left the size at %d", a.ndiResolution)
	}
}

func TestSettingsPageWithoutDisplays(t *testing.T) {
	a := &app{}
	tt := ui.NewTester(a.settingsBody, 900, 800)
	if !tt.HasText("No displays reported.") {
		t.Fatalf("settings does not explain the empty display list: %q", tt.Texts())
	}
}

// Choosing a display with no stage open saves and redraws, without windows.
func TestStageDisplayChangedWithoutAStage(t *testing.T) {
	a := newApp()
	a.stageDisplayChanged()
}

// A display change that does not touch the stage's display leaves the
// settings alone: nothing here closes and reopens the stage.
func TestDisplayChangeKeepsTheStagePreference(t *testing.T) {
	a := &app{stageDisplay: 2, stageDisplayUsed: 2}
	a.refreshDisplays([]mygo.Display{{ID: 1, Primary: true}, {ID: 2, Label: "Projector"}, {ID: 3, Label: "TV"}})
	if a.stageDisplay != 2 {
		t.Fatalf("stageDisplay = %d, want the unchanged 2", a.stageDisplay)
	}
	if a.stageDisplayUsed != 2 {
		t.Fatalf("stageDisplayUsed = %d, want 2", a.stageDisplayUsed)
	}
}

func TestRefreshDisplaysWithoutAStage(t *testing.T) {
	a := &app{stageDisplay: 2}
	a.refreshDisplays([]mygo.Display{{ID: 1, Primary: true}})
	if a.stageDisplay != 0 {
		t.Fatalf("stageDisplay = %d, want windowed after the display vanished", a.stageDisplay)
	}
}
