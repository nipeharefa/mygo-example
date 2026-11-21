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
	want := settings{OutputDisplay: 42, NDI: true}
	if err := writeSettings(path, want); err != nil {
		t.Fatalf("writeSettings: %v", err)
	}
	if got := readSettings(path); got != want {
		t.Fatalf("readSettings = %+v, want %+v", got, want)
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

// The app saves and loads through its settings directory, and never writes
// when there is none.
func TestAppSaveLoadSettings(t *testing.T) {
	a := &app{settingsDir: t.TempDir()}
	a.outputDisplay = 7
	a.ndiOn = true
	a.saveSettings()

	b := &app{settingsDir: a.settingsDir}
	b.loadSettings()
	if b.outputDisplay != 7 || !b.ndiOn {
		t.Fatalf("loaded outputDisplay = %d ndiOn = %v, want 7/true", b.outputDisplay, b.ndiOn)
	}
}

func TestSaveSettingsWithoutADirectory(t *testing.T) {
	a := &app{}
	a.outputDisplay = 7
	a.ndiOn = true
	a.saveSettings() // must not panic or touch the real app data dir
}

// A saved display that is no longer attached falls back to automatic.
func TestLoadSettingsDropsMissingDisplay(t *testing.T) {
	a := &app{
		settingsDir: t.TempDir(),
		displays:    []mygo.Display{{ID: 1, Label: "Built-in", Primary: true}},
	}
	if err := writeSettings(a.settingsPath(), settings{OutputDisplay: 99, NDI: true}); err != nil {
		t.Fatal(err)
	}
	a.loadSettings()
	if a.outputDisplay != 0 {
		t.Fatalf("outputDisplay = %d, want automatic", a.outputDisplay)
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
	if err := writeSettings(a.settingsPath(), settings{OutputDisplay: 2}); err != nil {
		t.Fatal(err)
	}
	a.loadSettings()
	if a.outputDisplay != 2 {
		t.Fatalf("outputDisplay = %d, want 2", a.outputDisplay)
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

// chooseDisplay is where the stage opens: an explicit display, otherwise the
// first projector, otherwise a plain window.
func TestChooseDisplay(t *testing.T) {
	primary := mygo.Display{ID: 1, Label: "Built-in", Primary: true}
	projector := mygo.Display{ID: 2, Label: "Projector"}
	displays := []mygo.Display{primary, projector}

	if d, ok := chooseDisplay(displays, 2); !ok || d.ID != 2 {
		t.Fatalf("explicit display = %+v ok = %v", d, ok)
	}
	if _, ok := chooseDisplay(displays, 99); ok {
		t.Fatal("a missing explicit display was returned")
	}
	if d, ok := chooseDisplay(displays, 0); !ok || d.ID != 2 {
		t.Fatalf("automatic display = %+v ok = %v", d, ok)
	}
	if _, ok := chooseDisplay([]mygo.Display{primary}, 0); ok {
		t.Fatal("automatic picked the primary display")
	}
	if _, ok := chooseDisplay(nil, 0); ok {
		t.Fatal("automatic picked a display from an empty list")
	}
}

func TestSettingsViewListsDisplays(t *testing.T) {
	a := &app{displays: []mygo.Display{
		{ID: 1, Label: "Built-in", Primary: true, Bounds: mygo.Rectangle{Width: 1440, Height: 900}},
		{ID: 2, Label: "Projector", Bounds: mygo.Rectangle{Width: 1920, Height: 1080}},
	}}
	tt := ui.NewTester(a.settingsView, 560, 520)
	for _, want := range []string{"Settings", "Stage output", "Automatic", "Built-in", "Projector", "Publish the stage over NDI", "Data"} {
		if !tt.HasText(want) {
			t.Fatalf("settings is missing %q: %q", want, tt.Texts())
		}
	}
}

func TestSettingsViewWithoutDisplays(t *testing.T) {
	a := &app{}
	tt := ui.NewTester(a.settingsView, 560, 520)
	if !tt.HasText("No displays reported.") {
		t.Fatalf("settings does not explain the empty display list: %q", tt.Texts())
	}
}

// Choosing a display with no stage open saves and redraws, without windows.
func TestOutputDisplayChangedWithoutAStage(t *testing.T) {
	a := &app{}
	a.outputDisplay = 2
	a.outputDisplayChanged()
}

func TestRefreshDisplaysWithoutAStage(t *testing.T) {
	a := &app{outputDisplay: 2}
	a.refreshDisplays([]mygo.Display{{ID: 1, Primary: true}})
	if a.outputDisplay != 0 {
		t.Fatalf("outputDisplay = %d, want automatic after the display vanished", a.outputDisplay)
	}
}
