package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// showApp is an app whose files live in a fresh directory.
func showApp(t *testing.T) *app {
	t.Helper()
	a := newApp()
	a.settingsDir = t.TempDir()
	return a
}

func TestShowRoundTrip(t *testing.T) {
	a := showApp(t)
	a.preview, a.stage = 2, 1
	a.changed()
	if a.showNote != "" {
		t.Fatalf("save note = %q, want none", a.showNote)
	}
	f, err := readShow(a.showPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Slides) != len(a.deck) {
		t.Fatalf("saved slides = %d, want %d", len(f.Slides), len(a.deck))
	}
	if f.Preview != 2 || f.Stage != 1 {
		t.Errorf("saved cursor = %d/%d, want 2/1", f.Preview, f.Stage)
	}
	if f.NextID != a.nextID {
		t.Errorf("saved nextID = %d, want %d", f.NextID, a.nextID)
	}

	b := newApp()
	b.settingsDir = a.settingsDir
	b.loadShow()
	if len(b.deck) != len(a.deck) {
		t.Fatalf("loaded slides = %d, want %d", len(b.deck), len(a.deck))
	}
	for i := range a.deck {
		if b.deck[i] != a.deck[i] {
			t.Fatalf("slide %d = %+v, want %+v", i, b.deck[i], a.deck[i])
		}
	}
	if b.preview != 2 || b.stage != 1 {
		t.Errorf("loaded cursor = %d/%d, want 2/1", b.preview, b.stage)
	}
	if b.nextID < a.nextID {
		t.Errorf("nextID = %d, want at least %d", b.nextID, a.nextID)
	}
	// A restored deck adds new slides without reusing an id.
	s := b.newSlide("New", "Verse 1", "words", slideSong, 40)
	for _, other := range b.deck[:len(b.deck)-1] {
		if other.ID == s.ID {
			t.Fatalf("new slide reused id %q", s.ID)
		}
	}
}

func TestLoadShowKeepsSeedWithoutFile(t *testing.T) {
	a := showApp(t)
	before := len(a.deck)
	a.loadShow()
	if len(a.deck) != before {
		t.Fatalf("deck = %d, want the seed's %d", len(a.deck), before)
	}
	if a.showBlocked {
		t.Error("a missing file blocked saving")
	}
}

func TestLoadShowClampsTheCursor(t *testing.T) {
	a := showApp(t)
	f := a.showFile()
	f.Preview, f.Stage = 99, 99
	if err := writeShow(a.showPath(), f); err != nil {
		t.Fatal(err)
	}
	b := newApp()
	b.settingsDir = a.settingsDir
	b.loadShow()
	if b.preview != len(b.deck)-1 || b.stage != len(b.deck)-1 {
		t.Errorf("cursor = %d/%d, want both at the last slide", b.preview, b.stage)
	}
}

func TestLoadShowRefusesANewerVersion(t *testing.T) {
	a := showApp(t)
	if err := writeShow(a.showPath(), showFile{Version: showVersion + 1}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.showPath())
	if err != nil {
		t.Fatal(err)
	}
	b := newApp()
	b.settingsDir = a.settingsDir
	seed := len(b.deck)
	b.loadShow()
	if !b.showBlocked {
		t.Fatal("a newer show was not refused")
	}
	if len(b.deck) != seed {
		t.Errorf("deck = %d, want the seed's %d", len(b.deck), seed)
	}
	if !strings.Contains(b.showNote, "could not be read") {
		t.Errorf("note = %q, want the refusal", b.showNote)
	}
	// Nothing may overwrite the file it could not read.
	b.addSong("holy-holy-holy")
	after, err := os.ReadFile(b.showPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("the refused file was overwritten")
	}
}

func TestLoadShowRepairsIDs(t *testing.T) {
	a := showApp(t)
	if err := writeShow(a.showPath(), showFile{
		Version: showVersion,
		Slides: []showSlide{
			{Title: "One", Section: "Verse 1", Text: "a", Kind: int(slideSong)},
			{ID: "s7", Title: "Two", Section: "Verse 1", Text: "b", Kind: int(slideSong)},
			{ID: "s7", Title: "Three", Section: "Verse 1", Text: "c", Kind: int(slideSong)},
		},
		NextID: 3,
	}); err != nil {
		t.Fatal(err)
	}
	b := newApp()
	b.settingsDir = a.settingsDir
	b.loadShow()
	if len(b.deck) != 3 {
		t.Fatalf("deck = %d, want 3", len(b.deck))
	}
	seen := map[string]bool{}
	for i, s := range b.deck {
		if s.ID == "" {
			t.Fatalf("slide %d kept an empty id", i)
		}
		if seen[s.ID] {
			t.Fatalf("slide %d reused id %q", i, s.ID)
		}
		seen[s.ID] = true
	}
	if b.nextID < 9 {
		t.Errorf("nextID = %d, want past s7's 8", b.nextID)
	}
}

// A save that changed nothing must not rewrite the file: the deck is rebuilt
// from state on every pass, and only a real edit should touch the disk.
func TestSaveShowSkipsAnUnchangedDeck(t *testing.T) {
	a := showApp(t)
	a.changed()
	marker := []byte("{}\n")
	if err := os.WriteFile(a.showPath(), marker, 0o644); err != nil {
		t.Fatal(err)
	}
	a.changed()
	got, err := os.ReadFile(a.showPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(marker) {
		t.Error("an unchanged deck rewrote the file")
	}
	// A real change writes it again.
	a.preview = 1
	a.changed()
	f, err := readShow(a.showPath())
	if err != nil {
		t.Fatal(err)
	}
	if f.Preview != 1 {
		t.Errorf("saved preview = %d, want 1", f.Preview)
	}
}

func TestSaveShowReportsAFailure(t *testing.T) {
	a := newApp()
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.settingsDir = blocked
	a.changed()
	if !strings.Contains(a.showNote, "not being saved") {
		t.Fatalf("note = %q, want a save failure", a.showNote)
	}
}

func TestReadShowRejectsAMalformedFile(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"not json":   "{",
		"no version": `{"slides": []}`,
		"bad kind":   `{"version": 1, "slides": [{"id": "s0", "kind": 9}]}`,
	}
	for name, body := range cases {
		path := filepath.Join(dir, "show.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readShow(path); err == nil {
			t.Errorf("%s: read succeeded, want an error", name)
		}
	}
}

func TestShowFileIsReadable(t *testing.T) {
	a := showApp(t)
	a.changed()
	data, err := os.ReadFile(a.showPath())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "slides", "preview", "stage", "nextID"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("show.json has no %q", key)
		}
	}
}
