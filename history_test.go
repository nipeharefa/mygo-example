package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// fakeHistory is the version list without SQLite, so the app's rules are
// tested without the native module.
type fakeHistory struct {
	rows   []revision
	libs   [][]byte
	pruned []int
	closed bool
	fail   error
}

func (f *fakeHistory) Add(_ context.Context, r revision, library []byte) error {
	if f.fail != nil {
		return f.fail
	}
	if r.ID == 0 {
		r.ID = int64(len(f.rows) + 1)
	}
	f.rows = append(f.rows, r)
	f.libs = append(f.libs, append([]byte(nil), library...))
	return nil
}

func (f *fakeHistory) Recent(_ context.Context, limit int) ([]revision, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	out := make([]revision, 0, len(f.rows))
	for i := len(f.rows) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, f.rows[i])
	}
	return out, nil
}

func (f *fakeHistory) Library(_ context.Context, id int64) ([]byte, error) {
	for i, r := range f.rows {
		if r.ID == id {
			return f.libs[i], nil
		}
	}
	return nil, errors.New("no such version")
}

func (f *fakeHistory) Prune(_ context.Context, keep int) error {
	f.pruned = append(f.pruned, keep)
	return nil
}

func (f *fakeHistory) Close() error {
	f.closed = true
	return nil
}

func historyApp(t *testing.T) (*app, *fakeHistory) {
	t.Helper()
	a := backupApp(t)
	f := &fakeHistory{}
	a.history = f
	t.Cleanup(func() {
		if a.historyTimer != nil {
			a.historyTimer.Stop()
		}
	})
	return a, f
}

// noteAndFlush is what the trailing timer does, without waiting five
// seconds.
func noteAndFlush(a *app, what string) {
	a.noteHistory(what)
	if a.historyTimer != nil {
		a.historyTimer.Stop()
		a.historyTimer = nil
	}
	a.flushHistory()
}

func TestHistoryRecordsTheStartingState(t *testing.T) {
	a, f := historyApp(t)
	a.recordOpened()
	if len(f.rows) != 1 || f.rows[0].Note != "Opened" {
		t.Fatalf("versions = %+v, want the starting state", f.rows)
	}
	a.recordOpened()
	if len(f.rows) != 1 {
		t.Fatalf("versions = %d, want the unchanged launch skipped", len(f.rows))
	}
	a.songs[0].Title = "Changed"
	noteAndFlush(a, "Saved")
	if len(f.rows) != 2 {
		t.Fatalf("versions = %d, want the edit too", len(f.rows))
	}
	// The starting state is the way back before the first edit.
	a.revisionWaiting = f.rows[0].ID
	a.restoreRevision()
	if a.songs[0].Title == "Changed" {
		t.Fatalf("title = %q, want the starting state", a.songs[0].Title)
	}
}

func TestHistoryRecordsAPause(t *testing.T) {
	a, f := historyApp(t)
	a.songs[0].Title = "Edited"
	noteAndFlush(a, "Saved")
	if len(f.rows) != 1 {
		t.Fatalf("versions = %d, want 1", len(f.rows))
	}
	want, err := marshalLibrary(a.songs)
	if err != nil {
		t.Fatal(err)
	}
	if f.rows[0].Note != "Saved" || f.rows[0].Songs != len(a.songs) {
		t.Errorf("version = %+v", f.rows[0])
	}
	if f.rows[0].Hash != hashBytes(want) {
		t.Error("the version's hash does not match the library it stored")
	}
	if len(f.libs[0]) == 0 {
		t.Error("no library was stored")
	}
	if want := historyKeep; len(f.pruned) != 1 || f.pruned[0] != want {
		t.Errorf("pruned = %v, want [%d]", f.pruned, want)
	}
}

func TestHistorySkipsTheSameContent(t *testing.T) {
	a, f := historyApp(t)
	noteAndFlush(a, "Saved")
	noteAndFlush(a, "Saved")
	if len(f.rows) != 1 {
		t.Fatalf("versions = %d, want the unchanged one skipped", len(f.rows))
	}
	a.songs[0].Title = "Again"
	noteAndFlush(a, "Saved")
	if len(f.rows) != 2 {
		t.Fatalf("versions = %d, want 2", len(f.rows))
	}
}

func TestHistoryStaysOffWithoutAStore(t *testing.T) {
	a := backupApp(t)
	a.noteHistory("Saved")
	a.flushHistory()
	if a.historyPending {
		t.Error("history queued work with no store")
	}
}

func TestHistoryRestoreAsksThenApplies(t *testing.T) {
	a, f := historyApp(t)
	old := cloneSongs(builtinSongs[:2])
	data, err := marshalLibrary(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Add(context.Background(), revision{At: time.Now(), Kind: "auto", Note: "Before", Songs: 2, Hash: hashBytes(data)}, data); err != nil {
		t.Fatal(err)
	}
	a.askRevision(1)
	if len(a.songs) != len(builtinSongs) {
		t.Fatal("the songs changed before the answer")
	}
	a.restoreRevision()
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want the version's 2", len(a.songs))
	}
	if a.revisionWaiting != 0 {
		t.Error("the question outlived the restore")
	}
	if !strings.Contains(a.historyNote, "Restored 2 songs") {
		t.Errorf("note = %q", a.historyNote)
	}
	if len(backupNames(t, a, "before-history-")) != 1 {
		t.Error("the state before the restore was not copied into backups")
	}
	if !a.validEditorSlide() {
		t.Error("nothing is selected after the restore")
	}
	noteAndFlush(a, "Restored a version")
	if len(f.rows) != 2 {
		t.Errorf("versions = %d, want the restore recorded too", len(f.rows))
	}
}

func TestHistoryRestoreOfAMissingVersion(t *testing.T) {
	a, _ := historyApp(t)
	before := len(a.songs)
	a.askRevision(99)
	a.restoreRevision()
	if len(a.songs) != before {
		t.Error("a missing version changed the songs")
	}
	if !strings.Contains(a.historyNote, "could not be read") {
		t.Errorf("note = %q", a.historyNote)
	}
}

func TestHistoryPageListsVersions(t *testing.T) {
	a, f := historyApp(t)
	for i, note := range []string{"New song: Test Hymn", "Part added"} {
		data, err := marshalLibrary(a.songs)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Add(context.Background(), revision{
			At:   time.Date(2026, 11, 24, 9, 12+i, 0, 0, time.Local),
			Kind: "auto", Note: note, Songs: len(a.songs), Hash: hashBytes(data),
		}, data); err != nil {
			t.Fatal(err)
		}
	}
	a.showHistory()
	if a.page != pageHistory {
		t.Fatalf("page = %d, want the history page", a.page)
	}
	tt := ui.NewTester(a.historyBody, 900, 1200)
	for _, want := range []string{"Version history", "New song: Test Hymn", "Part added", "5 songs", "Restore"} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q", want)
		}
	}
	if _, ok := tt.Find("Restore version 1"); !ok {
		t.Error("the first version has no restore")
	}
}

func TestHistoryPageEmpty(t *testing.T) {
	a, _ := historyApp(t)
	a.showHistory()
	tt := ui.NewTester(a.historyBody, 900, 900)
	if !tt.HasText("No versions yet.") {
		t.Fatalf("empty history does not say so: %q", tt.Texts())
	}
}

func TestHistoryPageOff(t *testing.T) {
	a := backupApp(t)
	a.historyNote = "Version history is off: the module did not load"
	a.showHistory()
	tt := ui.NewTester(a.historyBody, 900, 900)
	if !tt.HasText("Version history is off on this machine.") {
		t.Fatalf("the page does not explain: %q", tt.Texts())
	}
	if !tt.HasText(a.historyNote) {
		t.Error("the page does not say why")
	}
}

// The page answers the restore question itself.
func TestHistoryRestoreFromThePage(t *testing.T) {
	a, f := historyApp(t)
	old := cloneSongs(builtinSongs[:2])
	data, err := marshalLibrary(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Add(context.Background(), revision{At: time.Now(), Note: "Before", Songs: 2, Hash: hashBytes(data)}, data); err != nil {
		t.Fatal(err)
	}
	a.showHistory()
	tt := ui.NewTester(a.historyBody, 900, 1200)
	if err := tt.Click("Restore version 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Replace the 5 songs here with these 2 songs?") {
		t.Fatalf("the question is missing: %q", tt.Texts())
	}
	if _, ok := tt.Find("Confirm restoring version 1"); !ok {
		t.Error("the confirming button has no name of its own")
	}
	if err := tt.Click("Restore this version"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs) != 2 {
		t.Fatalf("songs = %d, want 2", len(a.songs))
	}
}

func TestSettingsPageOffersHistory(t *testing.T) {
	a := backupApp(t)
	tt := ui.NewTester(a.settingsBody, 900, 1400)
	if !tt.HasText("Version history") {
		t.Error("Settings does not offer the version history")
	}
	if err := tt.Click("Version history"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageHistory {
		t.Errorf("page = %d, want the history page", a.page)
	}
}
