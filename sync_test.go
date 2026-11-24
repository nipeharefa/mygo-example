package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func writeLibraryAt(t *testing.T, path string, songs []song) {
	t.Helper()
	if err := writeLibrary(path, libraryFile{Version: libraryVersion, Songs: songs}); err != nil {
		t.Fatal(err)
	}
}

func readLibraryAt(t *testing.T, path string) libraryFile {
	t.Helper()
	f, err := readLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// The chosen folder is remembered in settings.json and the library follows
// it on the next launch.
func TestSongsFolderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	folder := t.TempDir()
	a := newApp()
	a.settingsDir = dir
	a.libraryFolder = folder
	a.saveSettings()
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, "library.json")); err != nil {
		t.Fatalf("the library is not in the folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "library.json")); err == nil {
		t.Error("the library was also written to the app data directory")
	}

	b := newApp()
	b.settingsDir = dir
	b.loadSettings()
	if b.libraryFolder != folder {
		t.Fatalf("folder = %q, want %q", b.libraryFolder, folder)
	}
	b.loadLibrary()
	if len(b.songs) != len(a.songs) {
		t.Errorf("songs = %d, want %d", len(b.songs), len(a.songs))
	}
}

func TestChooseSongsFolderCopiesMine(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{folder}, nil }
	a.chooseSongsFolder()
	if a.folderWaiting == nil || a.folderWaiting.Existing != nil {
		t.Fatalf("plan = %+v, want an empty folder", a.folderWaiting)
	}
	a.confirmFolder()
	if a.libraryFolder != folder {
		t.Fatalf("folder = %q, want %q", a.libraryFolder, folder)
	}
	if a.folderWaiting != nil {
		t.Error("the question outlived the answer")
	}
	if !strings.Contains(a.folderNote, folder) || a.folderNoteErr {
		t.Errorf("note = %q", a.folderNote)
	}
	f := readLibraryAt(t, filepath.Join(folder, "library.json"))
	if len(f.Songs) != len(a.songs) {
		t.Errorf("folder library = %d songs, want %d", len(f.Songs), len(a.songs))
	}
	if s := readSettings(a.settingsPath()); s.LibraryFolder != folder {
		t.Errorf("saved folder = %q, want %q", s.LibraryFolder, folder)
	}
	if len(backupNames(t, a, "before-folder-")) != 1 {
		t.Error("the state before the move was not copied into backups")
	}
}

func TestChooseSongsFolderAdoptsTheFolder(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	writeLibraryAt(t, filepath.Join(folder, "library.json"), cloneSongs(builtinSongs[:2]))
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{folder}, nil }
	a.chooseSongsFolder()
	if a.folderWaiting == nil || a.folderWaiting.Existing == nil {
		t.Fatalf("plan = %+v, want the folder's library", a.folderWaiting)
	}
	if len(a.songs) != len(builtinSongs) {
		t.Fatal("songs changed before the answer")
	}
	a.confirmFolder()
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want the folder's 2", len(a.songs))
	}
	if !a.validEditorSlide() {
		t.Error("nothing is selected after adopting the folder")
	}
}

func TestChooseSongsFolderCancel(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{folder}, nil }
	a.chooseSongsFolder()
	a.cancelFolder()
	if a.folderWaiting != nil {
		t.Fatal("cancel kept the plan")
	}
	if a.libraryFolder != "" {
		t.Errorf("folder = %q, want the app's own", a.libraryFolder)
	}
	if _, err := os.Stat(filepath.Join(folder, "library.json")); err == nil {
		t.Error("cancel wrote a library")
	}
}

func TestChooseSongsFolderRefusesABadLibrary(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "library.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{folder}, nil }
	a.chooseSongsFolder()
	if a.folderWaiting != nil {
		t.Fatal("a bad library was queued")
	}
	if !strings.Contains(a.folderNote, "could not be read") || !a.folderNoteErr {
		t.Errorf("note = %q (error %v)", a.folderNote, a.folderNoteErr)
	}
}

// A folder that went away, or a file that cannot be read, opens read-only:
// the seed is usable and nothing is written over what might be the real
// library.
func TestUnavailableFolderBlocksSaving(t *testing.T) {
	a := backupApp(t)
	a.libraryFolder = filepath.Join(t.TempDir(), "gone")
	a.loadLibrary()
	if len(a.songs) != len(builtinSongs) {
		t.Fatalf("songs = %d, want the seed", len(a.songs))
	}
	if !a.libraryBlocked || a.libraryNote == "" {
		t.Fatal("an unavailable folder did not block saving")
	}
	if err := a.saveLibrary(); err == nil {
		t.Fatal("saveLibrary wrote into an unavailable folder")
	}
	if _, err := os.Stat(filepath.Join(a.libraryFolder, "library.json")); err == nil {
		t.Error("a library appeared in the missing folder")
	}
}

func TestUseAppFolder(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.libraryFolder = folder
	a.libraryStamp = ""
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	a.useAppFolder()
	if a.libraryFolder != "" {
		t.Fatalf("folder = %q, want the app's own", a.libraryFolder)
	}
	if _, err := os.Stat(filepath.Join(a.settingsDir, "library.json")); err != nil {
		t.Fatalf("the library did not come back: %v", err)
	}
	if s := readSettings(a.settingsPath()); s.LibraryFolder != "" {
		t.Errorf("saved folder = %q, want none", s.LibraryFolder)
	}
}

// A file another machine wrote is noticed before it is replaced. Reloading
// adopts it and tells the caller the edit in hand was dropped.
func TestConflictReloadsTheOtherFile(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.libraryFolder = folder
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	writeLibraryAt(t, filepath.Join(folder, "library.json"), cloneSongs(builtinSongs[:3]))
	a.askMessage = func(opts mygo.MessageOptions) (mygo.MessageResult, error) {
		if len(opts.Buttons) != 2 || opts.Buttons[0] != "Reload the file" {
			t.Errorf("buttons = %v", opts.Buttons)
		}
		return mygo.MessageResult{Button: 0}, nil
	}
	err := a.saveLibrary()
	if !errors.Is(err, errLibraryReloaded) {
		t.Fatalf("err = %v, want errLibraryReloaded", err)
	}
	if len(a.songs) != 3 {
		t.Errorf("songs = %d, want the other file's 3", len(a.songs))
	}
	if !strings.Contains(a.libraryNote, "Reloaded 3 songs") {
		t.Errorf("note = %q", a.libraryNote)
	}
}

func TestConflictKeepsMineAndSavesTheirs(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.libraryFolder = folder
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	writeLibraryAt(t, filepath.Join(folder, "library.json"), cloneSongs(builtinSongs[:3]))
	a.songs[0].Title = "Kept Here"
	a.askMessage = func(mygo.MessageOptions) (mygo.MessageResult, error) {
		return mygo.MessageResult{Button: 1}, nil
	}
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	f := readLibraryAt(t, filepath.Join(folder, "library.json"))
	if f.Songs[0].Title != "Kept Here" {
		t.Errorf("folder title = %q, want this app's", f.Songs[0].Title)
	}
	copies := backupNames(t, a, "conflict-")
	if len(copies) != 1 {
		t.Fatalf("conflict copies = %v, want one", copies)
	}
	data, err := os.ReadFile(filepath.Join(a.backupDir(), copies[0]))
	if err != nil {
		t.Fatal(err)
	}
	other, err := decodeLibrary(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Songs) != 3 {
		t.Errorf("conflict copy = %d songs, want the other file's 3", len(other.Songs))
	}
}

func TestNoConflictWhenTheFileIsOurs(t *testing.T) {
	a := backupApp(t)
	a.libraryFolder = t.TempDir()
	a.askMessage = func(mygo.MessageOptions) (mygo.MessageResult, error) {
		t.Error("the app asked about its own file")
		return mygo.MessageResult{}, nil
	}
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	a.songs[0].Title = "Changed"
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
}

func TestReloadLibraryAction(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.libraryFolder = folder
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	writeLibraryAt(t, filepath.Join(folder, "library.json"), cloneSongs(builtinSongs[:2]))
	a.reloadLibrary()
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want 2", len(a.songs))
	}
	if !strings.Contains(a.folderNote, "Reloaded 2 songs") || a.folderNoteErr {
		t.Errorf("note = %q", a.folderNote)
	}
}

func TestSettingsPageShowsTheSongsFolder(t *testing.T) {
	a := backupApp(t)
	tt := ui.NewTester(a.settingsBody, 900, 1400)
	for _, want := range []string{"Songs folder", "Choose a folder…", "Edit on one machine at a time."} {
		if !tt.HasText(want) {
			t.Errorf("settings is missing %q", want)
		}
	}

	a.libraryFolder = t.TempDir()
	tt = ui.NewTester(a.settingsBody, 900, 1400)
	for _, want := range []string{"Change folder…", "Use the app's folder", "Reload"} {
		if !tt.HasText(want) {
			t.Errorf("settings with a folder is missing %q", want)
		}
	}

	a.folderWaiting = &folderPlan{Dir: a.libraryFolder, Existing: &libraryFile{Songs: cloneSongs(builtinSongs[:2])}}
	tt = ui.NewTester(a.settingsBody, 900, 1400)
	if !tt.HasText("Use the 2 songs in") || !tt.HasText("Use those songs") {
		t.Errorf("the folder question is missing: %q", tt.Texts())
	}
}

// The folder question is answered on the page, like the restore question.
func TestSettingsPageAnswersTheFolderQuestion(t *testing.T) {
	a := backupApp(t)
	folder := t.TempDir()
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{folder}, nil }
	a.chooseSongsFolder()
	tt := ui.NewTester(a.settingsBody, 900, 1400)
	if !tt.HasText("Keep the 5 songs here in " + folder + "?") {
		t.Fatalf("the question is missing: %q", tt.Texts())
	}
	if err := tt.Click("Use this folder"); err != nil {
		t.Fatal(err)
	}
	if a.libraryFolder != folder {
		t.Fatalf("folder = %q, want %q", a.libraryFolder, folder)
	}
}

func TestSettingsRoundTripKeepsTheFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := settings{LibraryFolder: "/tmp/songs"}
	if err := writeSettings(path, want); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(path); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["libraryFolder"]; !ok {
		t.Error("settings.json has no libraryFolder")
	}
}
