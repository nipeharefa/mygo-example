package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestWriteShots draws each surface with ui.Render and writes it as a PNG.
// It is opt-in: MYGO_SHOTS names the output directory, and nothing opens a
// window, because a headless tester has no application.
//
//	MYGO_SHOTS=tmp CGO_ENABLED=0 go test -run TestWriteShots .
func TestWriteShots(t *testing.T) {
	dir := os.Getenv("MYGO_SHOTS")
	if dir == "" {
		t.Skip("set MYGO_SHOTS to write surface snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	shots := []struct {
		name string
		w, h int
		view func(*ui.Context)
	}{
		{"console-runofshow", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.view(c)
		}},
		{"console-blackout", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.blackout = true
			a.view(c)
		}},
		// The monitor on a row that is not on air: selection has to read
		// without the LIVE badge, since the row no longer carries one.
		{"console-monitor-away", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.preview = 2
			a.view(c)
		}},
		// A song that comes back later heads its own group again.
		{"console-repeated-song", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.addSong("amazing-grace")
			a.preview = 0
			a.view(c)
		}},
		// The account pane: app actions out of the bar, in one card.
		{"console-account-pane", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.menuOpen = true
			a.view(c)
		}},
		{"console-settings", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.page = pageSettings
			a.view(c)
		}},
		{"login", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.view(c)
		}},

		{"stage", 1280, 720, func(c *ui.Context) {
			a := newApp()
			a.liveView(c)
		}},
		{"console-bible", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.tab = 1
			a.view(c)
		}},
		{"console-empty", 1280, 800, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.deck = nil
			a.stage = 0
			a.preview = 0
			a.view(c)
		}},
		{"editor", 1280, 900, func(c *ui.Context) {
			a := newApp()
			a.editSong, a.editPart, a.editSlide = 0, 0, 0
			a.editorView(c)
		}},
		{"console-narrow", 1040, 720, func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.view(c)
		}},
		{"help", 640, 620, func(c *ui.Context) {
			a := newApp()
			a.helpView(c)
		}},
	}
	for _, s := range shots {
		writeShot(t, filepath.Join(dir, s.name+".png"), ui.Render(s.view, s.w, s.h, 2))
	}

	// The dark appearance comes from a Tester, which ui.Render has no
	// option for: the console must hold up on a dim desk at night.
	for _, s := range []struct {
		name string
		view func(*ui.Context)
	}{
		{"console-dark", func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.view(c)
		}},
		{"console-dark-settings", func(c *ui.Context) {
			a := newApp()
			a.signedIn = true
			a.page = pageSettings
			a.view(c)
		}},
		{"console-dark-editor", func(c *ui.Context) {
			a := newApp()
			a.editSong, a.editPart, a.editSlide = 0, 0, 0
			a.editorView(c)
		}},
	} {
		tt := ui.NewTesterWith(s.view, 1280, 800, ui.TesterOptions{Dark: true})
		writeShot(t, filepath.Join(dir, s.name+".png"), tt.Image())
	}

}

// writeShot encodes one rendered surface at scale 2: the shots are compared
// side by side in a review, so the type has to stay legible at full size.
func writeShot(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
