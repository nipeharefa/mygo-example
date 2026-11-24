package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func backupApp(t *testing.T) *app {
	t.Helper()
	a := newApp()
	a.settingsDir = t.TempDir()
	return a
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeTestBackup writes a bundle the app did not produce, so a restore is
// tested against a file from somewhere else.
func writeTestBackup(t *testing.T, path string, lib libraryFile, show *showFile) {
	t.Helper()
	f := backupFile{
		Version: backupVersion,
		App:     appTitle,
		Saved:   time.Now().Format(time.RFC3339),
		Library: marshal(t, lib),
	}
	if show != nil {
		f.Show = marshal(t, show)
	}
	if err := writeFileAtomic(path, append(marshal(t, f), '\n')); err != nil {
		t.Fatal(err)
	}
}

func backupNames(t *testing.T, a *app, prefix string) []string {
	t.Helper()
	entries, err := os.ReadDir(a.backupDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestBackupBundleRoundTrip(t *testing.T) {
	a := backupApp(t)
	a.preview, a.stage = 1, 2
	data, err := a.backupBytes()
	if err != nil {
		t.Fatal(err)
	}
	lib, show, err := decodeBackup(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Songs) != len(a.songs) {
		t.Fatalf("songs = %d, want %d", len(lib.Songs), len(a.songs))
	}
	if show == nil {
		t.Fatal("the bundle has no run of show")
	}
	if len(show.Slides) != len(a.deck) || show.Preview != 1 || show.Stage != 2 {
		t.Errorf("show = %d slides, cursor %d/%d, want %d and 1/2",
			len(show.Slides), show.Preview, show.Stage, len(a.deck))
	}
}

func TestDecodeBackupAcceptsAPlainLibrary(t *testing.T) {
	data := marshal(t, libraryFile{Version: libraryVersion, Songs: cloneSongs(builtinSongs[:2])})
	lib, show, err := decodeBackup(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Songs) != 2 {
		t.Errorf("songs = %d, want 2", len(lib.Songs))
	}
	if show != nil {
		t.Error("a plain library carries a run of show")
	}
}

func TestDecodeBackupRejectsANewerBundle(t *testing.T) {
	data := []byte(`{"version": 9, "library": {"version": 1, "songs": []}}`)
	if _, _, err := decodeBackup(data); err == nil {
		t.Fatal("a newer bundle was accepted")
	}
}

// The first save of a day keeps a copy; the rest of the day does not.
func TestDailySnapshotOnceADay(t *testing.T) {
	a := backupApp(t)
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	if err := a.saveLibrary(); err != nil {
		t.Fatal(err)
	}
	names := backupNames(t, a, "backup-")
	if len(names) != 1 {
		t.Fatalf("daily backups = %v, want one", names)
	}
	if want := "backup-" + time.Now().Format("2006-01-02") + ".json"; names[0] != want {
		t.Errorf("backup = %q, want %q", names[0], want)
	}
	lib, show, err := readBackupFile(filepath.Join(a.backupDir(), names[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Songs) != len(a.songs) {
		t.Errorf("snapshot songs = %d, want %d", len(lib.Songs), len(a.songs))
	}
	if show == nil || len(show.Slides) != len(a.deck) {
		t.Error("the snapshot does not hold the run of show")
	}
}

func TestDailySnapshotOnAShowSave(t *testing.T) {
	a := backupApp(t)
	a.changed()
	names := backupNames(t, a, "backup-")
	if len(names) != 1 {
		t.Fatalf("daily backups = %v, want one", names)
	}
}

func TestPruneKeepsTheNewest(t *testing.T) {
	a := backupApp(t)
	if err := os.MkdirAll(a.backupDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 20; i++ {
		name := fmt.Sprintf("backup-2026-01-%02d.json", i)
		if err := os.WriteFile(filepath.Join(a.backupDir(), name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 25; i++ {
		name := fmt.Sprintf("before-restore-2026-01-01-0000%02d.json", i)
		if err := os.WriteFile(filepath.Join(a.backupDir(), name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.pruneBackups()
	daily := backupNames(t, a, "backup-")
	if len(daily) != backupKeep {
		t.Fatalf("daily backups = %d, want %d", len(daily), backupKeep)
	}
	for _, name := range daily {
		if name < "backup-2026-01-07.json" {
			t.Errorf("kept %q, want only the newest %d", name, backupKeep)
		}
	}
	other := backupNames(t, a, "before-restore-")
	if len(other) != conflictKeep {
		t.Errorf("safety copies = %d, want %d", len(other), conflictKeep)
	}
}

func TestBackUpWritesTheChosenFile(t *testing.T) {
	a := backupApp(t)
	target := filepath.Join(t.TempDir(), "copy.json")
	a.saveFile = func(opts mygo.SaveDialogOptions) (string, error) {
		if opts.DefaultPath == "" {
			t.Error("the save dialog has no default name")
		}
		return target, nil
	}
	a.backUpSongs()
	if a.backupNoteErr {
		t.Fatalf("backup failed: %s", a.backupNote)
	}
	if !strings.Contains(a.backupNote, "Backed up 5 songs") {
		t.Errorf("note = %q, want the count", a.backupNote)
	}
	lib, show, err := readBackupFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Songs) != len(a.songs) || show == nil {
		t.Errorf("backup = %d songs, show %v", len(lib.Songs), show != nil)
	}
}

func TestBackUpCancelDoesNothing(t *testing.T) {
	a := backupApp(t)
	a.saveFile = func(mygo.SaveDialogOptions) (string, error) { return "", nil }
	a.backUpSongs()
	if a.backupNote != "" || a.backupNoteErr {
		t.Errorf("note = %q (error %v), want none", a.backupNote, a.backupNoteErr)
	}
}

func TestBackUpFailureIsShown(t *testing.T) {
	a := backupApp(t)
	a.saveFile = func(mygo.SaveDialogOptions) (string, error) { return "", errors.New("no dialog") }
	a.backUpSongs()
	if !strings.Contains(a.backupNote, "Could not open the save dialog") || !a.backupNoteErr {
		t.Errorf("note = %q (error %v)", a.backupNote, a.backupNoteErr)
	}
}

// Restoring is a question first: nothing changes until it is answered, and
// the state that is about to go is copied aside.
func TestRestoreAsksThenApplies(t *testing.T) {
	src := backupApp(t)
	show := src.showFile()
	path := filepath.Join(t.TempDir(), "backup.json")
	writeTestBackup(t, path, libraryFile{Version: libraryVersion, Songs: cloneSongs(builtinSongs[:2])}, &show)

	a := backupApp(t)
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	a.chooseRestore()
	if a.restoreWaiting == nil {
		t.Fatal("no confirmation was asked for")
	}
	if len(a.songs) != len(builtinSongs) {
		t.Fatalf("songs changed to %d before the question", len(a.songs))
	}
	a.confirmRestore()
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want 2", len(a.songs))
	}
	if len(a.deck) != len(show.Slides) {
		t.Errorf("deck = %d, want the backup's %d", len(a.deck), len(show.Slides))
	}
	if !strings.Contains(a.backupNote, "Restored 2 songs") || a.backupNoteErr {
		t.Errorf("note = %q (error %v)", a.backupNote, a.backupNoteErr)
	}
	if a.restoreWaiting != nil {
		t.Error("the plan outlived the restore")
	}
	if !a.validEditorSlide() {
		t.Error("no slide is selected after the restore")
	}
	copies := backupNames(t, a, "before-restore-")
	if len(copies) != 1 {
		t.Fatalf("before-restore copies = %v, want one", copies)
	}
	old, _, err := readBackupFile(filepath.Join(a.backupDir(), copies[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Songs) != len(builtinSongs) {
		t.Errorf("the safety copy holds %d songs, want the 5 that were here", len(old.Songs))
	}
}

// A copy of library.json is a backup too; without a show, the deck stays.
func TestRestoreFromAPlainLibraryKeepsTheDeck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	data := marshal(t, libraryFile{Version: libraryVersion, Songs: cloneSongs(builtinSongs[:2])})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	a := backupApp(t)
	deck := len(a.deck)
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	a.chooseRestore()
	a.confirmRestore()
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want 2", len(a.songs))
	}
	if len(a.deck) != deck {
		t.Errorf("deck = %d, want the unchanged %d", len(a.deck), deck)
	}
}

func TestRestoreRejectsABadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := backupApp(t)
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	a.chooseRestore()
	if a.restoreWaiting != nil {
		t.Fatal("a bad file was queued")
	}
	if !strings.Contains(a.backupNote, "not a backup") || !a.backupNoteErr {
		t.Errorf("note = %q (error %v)", a.backupNote, a.backupNoteErr)
	}
}

func TestRestoreCancelKeepsEverything(t *testing.T) {
	src := backupApp(t)
	show := src.showFile()
	path := filepath.Join(t.TempDir(), "backup.json")
	writeTestBackup(t, path, libraryFile{Version: libraryVersion, Songs: cloneSongs(builtinSongs[:1])}, &show)
	a := backupApp(t)
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	a.chooseRestore()
	a.cancelRestore()
	if a.restoreWaiting != nil {
		t.Fatal("cancel kept the plan")
	}
	if len(a.songs) != len(builtinSongs) {
		t.Errorf("songs = %d, want the original %d", len(a.songs), len(builtinSongs))
	}
}

// The Settings page is where the question is answered.
func TestRestoreFromTheSettingsPage(t *testing.T) {
	src := backupApp(t)
	show := src.showFile()
	path := filepath.Join(t.TempDir(), "backup.json")
	writeTestBackup(t, path, libraryFile{Version: libraryVersion, Songs: cloneSongs(builtinSongs[:2])}, &show)
	a := backupApp(t)
	a.signedIn = true
	a.page = pageSettings
	a.openFile = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	tt := ui.NewTester(a.settingsBody, 900, 1400)
	if err := tt.Click("Restore…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Replace 5 songs here with 2 songs from backup.json?") {
		t.Fatalf("the question is missing: %q", tt.Texts())
	}
	if err := tt.Click("Replace"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want 2", len(a.songs))
	}
}

func TestSettingsPageOffersBackups(t *testing.T) {
	a := &app{}
	tt := ui.NewTester(a.settingsBody, 900, 800)
	for _, want := range []string{"Data", "Back up…", "Restore…", "The hymns and the King James verses are public domain."} {
		if !tt.HasText(want) {
			t.Errorf("settings is missing %q", want)
		}
	}
}
