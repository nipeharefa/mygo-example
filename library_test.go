package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleLibrary() libraryFile {
	return libraryFile{
		Version: libraryVersion,
		Songs: []song{{
			ID:       "a-song",
			Title:    "A Song",
			Author:   "Someone",
			Key:      "G",
			TextSize: 40,
			Parts: []songPart{{
				Label:  "Verse 1",
				Slides: []string{"one\ntwo", "three"},
			}},
		}},
	}
}

func TestLibraryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	want := sampleLibrary()
	if err := writeLibrary(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if data[len(data)-1] != '\n' {
		t.Error("library file does not end with a newline")
	}
	got, err := readLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestReadLibraryRejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	f := sampleLibrary()
	f.Songs = append(f.Songs, f.Songs[0])
	if err := writeLibrary(path, f); err != nil {
		t.Fatal(err)
	}
	if _, err := readLibrary(path); err == nil {
		t.Fatal("readLibrary accepted duplicate ids")
	}
}

func TestLoadLibrarySeedsBuiltinsWhenMissing(t *testing.T) {
	a := &app{settingsDir: t.TempDir()}
	a.loadLibrary()
	if len(a.songs) != len(builtinSongs) {
		t.Fatalf("songs = %d, want %d", len(a.songs), len(builtinSongs))
	}
	for _, s := range a.songs {
		if s.textSize() != defaultTextSize {
			t.Errorf("song %q size = %v, want %v", s.ID, s.textSize(), defaultTextSize)
		}
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatalf("seeded file: %v", err)
	}
	if len(f.Songs) != len(builtinSongs) {
		t.Errorf("seeded songs = %d, want %d", len(f.Songs), len(builtinSongs))
	}
}

func TestLoadLibraryKeepsBuiltinsOnCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	garbage := []byte("this is not json\n")
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{settingsDir: dir}
	a.loadLibrary()
	if len(a.songs) != len(builtinSongs) {
		t.Fatalf("songs = %d, want the built-ins", len(a.songs))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(garbage) {
		t.Error("loadLibrary rewrote a corrupt file")
	}
}

func TestLoadLibraryAcceptsEmptySongs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	if err := os.WriteFile(path, []byte("{\"version\":1,\"songs\":[]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{settingsDir: dir}
	a.loadLibrary()
	if len(a.songs) != 0 {
		t.Fatalf("songs = %d, want 0", len(a.songs))
	}
}

func TestSaveLibraryWithoutDirIsNoop(t *testing.T) {
	a := &app{}
	a.saveLibrary()
	if a.libraryPath() != "" {
		t.Errorf("libraryPath = %q, want empty", a.libraryPath())
	}
}

func TestCloneSongsIsDeep(t *testing.T) {
	got := cloneSongs(builtinSongs)
	got[0].Title = "Changed"
	got[0].Parts[0].Slides[0] = "Changed"
	if builtinSongs[0].Title != "Amazing Grace" {
		t.Errorf("clone changed the title: %q", builtinSongs[0].Title)
	}
	if builtinSongs[0].Parts[0].Slides[0] == "Changed" {
		t.Error("clone shares its slides with the built-ins")
	}
}

func TestNewSongID(t *testing.T) {
	if got := newSongID("New Song", nil); got != "new-song" {
		t.Errorf("id = %q, want new-song", got)
	}
	if got := newSongID("New Song", []song{{ID: "new-song"}}); got != "new-song-2" {
		t.Errorf("id = %q, want new-song-2", got)
	}
	if got := newSongID("Amazing Grace!", nil); got != "amazing-grace" {
		t.Errorf("id = %q, want amazing-grace", got)
	}
	if got := newSongID("!!!", nil); got != "song" {
		t.Errorf("id = %q, want song", got)
	}
}
