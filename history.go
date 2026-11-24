package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	// historyEvery is how long a burst of edits waits before it becomes one
	// version, so typing a verse is one row, not one per key.
	historyEvery = 5 * time.Second
	// historyKeep is how many versions the history keeps.
	historyKeep = 200
)

// revision is one saved state of the song library, as the list shows it.
type revision struct {
	ID    int64
	At    time.Time
	Kind  string
	Note  string
	Songs int
	Hash  string
}

// historyStore keeps the library's versions. The app uses the SQLite store;
// the tests use a fake, so the app logic runs without the native module.
type historyStore interface {
	Add(ctx context.Context, r revision, library []byte) error
	Recent(ctx context.Context, limit int) ([]revision, error)
	Library(ctx context.Context, id int64) ([]byte, error)
	Prune(ctx context.Context, keep int) error
	Close() error
}

// historyPath is where the versions live, or "" before the app is ready.
func (a *app) historyPath() string {
	if a.settingsDir == "" {
		return ""
	}
	return filepath.Join(a.settingsDir, "history.db")
}

// openHistory opens the local version history. A native module that cannot
// load leaves history off with a note, and everything else still works.
func (a *app) openHistory() {
	path := a.historyPath()
	if path == "" {
		return
	}
	st, err := openSQLiteHistory(context.Background(), path)
	if err != nil {
		log.Printf("mygo-demo: history: %v", err)
		a.historyNote = "Version history is off: " + err.Error()
		return
	}
	a.history = st
	a.recordOpened()
}

// recordOpened stores the state the session starts from, so the first edit
// of the day has something to go back to. The newest version is compared by
// hash, so a launch that changes nothing adds no row.
func (a *app) recordOpened() {
	if a.history == nil {
		return
	}
	data, err := marshalLibrary(a.songs)
	if err != nil {
		return
	}
	hash := hashBytes(data)
	a.historyHash = hash
	rows, err := a.history.Recent(context.Background(), 1)
	if err == nil && len(rows) == 1 && rows[0].Hash == hash {
		return
	}
	r := revision{
		At:    time.Now(),
		Kind:  "open",
		Note:  "Opened",
		Songs: len(a.songs),
		Hash:  hash,
	}
	if err := a.history.Add(context.Background(), r, data); err != nil {
		log.Printf("mygo-demo: history: %v", err)
		a.historyNote = "History could not be saved: " + err.Error()
		return
	}
	if err := a.history.Prune(context.Background(), historyKeep); err != nil {
		log.Printf("mygo-demo: prune history: %v", err)
	}
}

// closeHistory writes what is pending and closes the store.
func (a *app) closeHistory() {
	if a.historyTimer != nil {
		a.historyTimer.Stop()
		a.historyTimer = nil
	}
	a.flushHistory()
	if a.history == nil {
		return
	}
	if err := a.history.Close(); err != nil {
		log.Printf("mygo-demo: close history: %v", err)
	}
	a.history = nil
}

// noteHistory marks the library changed and arms one trailing version once
// the editing pauses. It is a no-op when history is off, and one timer at a
// time means a burst of keystrokes becomes one row.
func (a *app) noteHistory(what string) {
	if a.history == nil {
		return
	}
	a.historyWhat = what
	a.historyPending = true
	if a.historyTimer != nil {
		return
	}
	a.historyTimer = time.AfterFunc(historyEvery, func() {
		mygo.RunOnMain(func() {
			a.historyTimer = nil
			a.flushHistory()
		})
	})
}

// flushHistory stores the pending state, unless it is the one already
// stored. It runs on the main thread.
func (a *app) flushHistory() {
	if !a.historyPending || a.history == nil {
		return
	}
	a.historyPending = false
	what := a.historyWhat
	a.historyWhat = ""
	data, err := marshalLibrary(a.songs)
	if err != nil {
		log.Printf("mygo-demo: history: %v", err)
		return
	}
	hash := hashBytes(data)
	if hash == a.historyHash {
		return
	}
	r := revision{
		At:    time.Now(),
		Kind:  "auto",
		Note:  what,
		Songs: len(a.songs),
		Hash:  hash,
	}
	if err := a.history.Add(context.Background(), r, data); err != nil {
		log.Printf("mygo-demo: history: %v", err)
		a.historyNote = "History could not be saved: " + err.Error()
		return
	}
	a.historyHash = hash
	a.historyNote = ""
	if err := a.history.Prune(context.Background(), historyKeep); err != nil {
		log.Printf("mygo-demo: prune history: %v", err)
	}
	if a.page == pageHistory {
		a.loadHistoryRows()
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

// loadHistoryRows reads the list the page shows, newest first.
func (a *app) loadHistoryRows() {
	a.historyRows, a.historyRowsErr = nil, ""
	if a.history == nil {
		return
	}
	rows, err := a.history.Recent(context.Background(), historyKeep)
	if err != nil {
		a.historyRowsErr = err.Error()
		return
	}
	a.historyRows = rows
}

// showHistory opens the version list.
func (a *app) showHistory() {
	a.page = pageHistory
	a.restoreWaiting = nil
	a.folderWaiting = nil
	a.revisionWaiting = 0
	a.menuOpen = false
	a.loadHistoryRows()
	a.changed()
}

// askRevision asks before a version replaces the songs here.
func (a *app) askRevision(id int64) {
	a.revisionWaiting = id
	a.changed()
}

// cancelRevision leaves the songs as they are.
func (a *app) cancelRevision() {
	if a.revisionWaiting == 0 {
		return
	}
	a.revisionWaiting = 0
	a.changed()
}

// restoreRevision applies the version the operator picked, backing up the
// state it replaces first.
func (a *app) restoreRevision() {
	id := a.revisionWaiting
	if id == 0 || a.history == nil {
		return
	}
	data, err := a.history.Library(context.Background(), id)
	if err != nil {
		a.historyNote = "That version could not be read: " + err.Error()
		a.revisionWaiting = 0
		a.changed()
		return
	}
	f, err := decodeLibrary(data)
	if err != nil {
		a.historyNote = "That version could not be read: " + err.Error()
		a.revisionWaiting = 0
		a.changed()
		return
	}
	if _, err := a.snapshotNow("before-history"); err != nil {
		log.Printf("mygo-demo: back up before history restore: %v", err)
	}
	a.songs = f.Songs
	a.normalizeLibrary()
	a.editQuery = ""
	a.editSong, a.editPart, a.editSlide = 0, 0, 0
	a.clampEditor()
	a.revisionWaiting = 0
	if err := a.saveLibrary(); err != nil {
		a.historyNote = "Restored the songs, but could not save them: " + err.Error()
		a.changed()
		return
	}
	a.noteHistory("Restored a version")
	a.historyNote = fmt.Sprintf("Restored %s from the version list.", songCount(len(a.songs)))
	a.changed()
}

// historyBody is the version list: the console's column of cards, one row
// per saved version, newest first.
func (a *app) historyBody(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(28, 0, 40).Children(func() {
			ui.Column(c).Gap(18).Width(760).Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).PaddingY(2).Children(func() {
					ui.Column(c).Gap(3).Grow(1).Children(func() {
						ui.Text(c, "Version history").FontSize(24).FontWeight(700).SingleLine()
						ui.Text(c, fmt.Sprintf("Each pause in editing saves a version. The last %d are kept.", historyKeep)).
							FontSize(textBody).TextColor(t.TextMuted).SingleLine()
					})
					ui.Button(c, "Back to settings").OnClick(a.showSettings)
					ui.Button(c, "Back to console").Label("Back to console").OnClick(a.showConsole)
				})
				settingsCard(c, "Saved versions", "Newest first.", func() {
					if a.history == nil {
						ui.Text(c, "Version history is off on this machine.").FontSize(textBody).TextColor(t.TextMuted)
						if a.historyNote != "" {
							ui.Text(c, a.historyNote).FontSize(textSmall).TextColor(t.Danger)
						}
						return
					}
					if a.historyNote != "" {
						ui.Text(c, a.historyNote).FontSize(textSmall).TextColor(t.TextMuted)
					}
					if a.historyRowsErr != "" {
						ui.Text(c, "The list could not be read: "+a.historyRowsErr).FontSize(textSmall).TextColor(t.Danger)
						return
					}
					if len(a.historyRows) == 0 {
						ui.Text(c, "No versions yet. The first edit of a song saves one.").FontSize(textBody).TextColor(t.TextMuted)
						return
					}
					for _, r := range a.historyRows {
						a.historyRow(c, r)
					}
				})
			})
		})
	})
}

// historyRow is one version: when, what changed, how many songs, and the
// restore that asks first.
func (a *app) historyRow(c *ui.Context, r revision) {
	t := c.Theme()
	pending := a.revisionWaiting == r.ID
	bg, border := raisedRow(t), hairlines(t)
	if pending {
		bg, border = rowSelected(t), rowSelectedEdge(t)
	}
	ui.Column(c).FillWidth().Padding(12).Gap(6).Radius(10).Border(1, border).Background(bg).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, r.At.Format("Jan 2, 15:04:05")).FontSize(textBody).FontWeight(600).SingleLine()
				ui.Text(c, revisionNote(r)).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
			})
			ui.Text(c, songCount(r.Songs)).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
			if pending {
				ui.Button(c, "Cancel").Label("Keep the current songs").OnClick(a.cancelRevision)
				ui.PrimaryButton(c, "Restore this version").
					Label(fmt.Sprintf("Confirm restoring version %d", r.ID)).OnClick(a.restoreRevision)
				return
			}
			ui.Button(c, "Restore").
				Label(fmt.Sprintf("Restore version %d", r.ID)).OnClick(func() { a.askRevision(r.ID) })
		})
		if pending {
			ui.Text(c, fmt.Sprintf("Replace the %s here with these %s? The songs and the set are backed up first.",
				songCount(len(a.songs)), songCount(r.Songs))).FontSize(textSmall).TextColor(t.TextMuted)
		}
	})
}

// revisionNote names what was saved, with a fallback for a version stored
// without a note.
func revisionNote(r revision) string {
	if r.Note == "" {
		return "Saved"
	}
	return r.Note
}
