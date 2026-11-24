package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const (
	// backupVersion is the shape of a backup file. A bundle from a newer
	// version is refused rather than half-read.
	backupVersion = 1
	// backupKeep is how many daily snapshots are kept.
	backupKeep = 14
	// conflictKeep is how many before-restore and conflict copies are kept.
	conflictKeep = 20
	// maxBackupBytes caps the file a restore will read, so a wrong pick
	// cannot load a video into memory.
	maxBackupBytes = 8 << 20
)

// backupFile is one file that holds everything worth keeping: the songs and
// the run of show together, with the time it was taken, so one restore puts
// a service back. The halves are the same documents the app writes, so a
// backup can also be read as a plain library file.
type backupFile struct {
	Version int             `json:"version"`
	App     string          `json:"app"`
	Saved   string          `json:"saved"`
	Library json.RawMessage `json:"library"`
	Show    json.RawMessage `json:"show,omitempty"`
}

// restorePlan is a backup the operator picked and the Settings page is
// asking about, so no file replaces the library until the question is
// answered.
type restorePlan struct {
	Path    string
	Library libraryFile
	Show    *showFile
}

// backupDir is where the automatic snapshots and the safety copies live, or
// "" before the app is ready.
func (a *app) backupDir() string {
	if a.settingsDir == "" {
		return ""
	}
	return filepath.Join(a.settingsDir, "backups")
}

// backupBytes is the current library and run of show as one file.
func (a *app) backupBytes() ([]byte, error) {
	lib, err := json.Marshal(libraryFile{Version: libraryVersion, Songs: a.songs})
	if err != nil {
		return nil, err
	}
	show, err := json.Marshal(a.showFile())
	if err != nil {
		return nil, err
	}
	f := backupFile{
		Version: backupVersion,
		App:     appTitle,
		Saved:   time.Now().Format(time.RFC3339),
		Library: lib,
		Show:    show,
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// writeBackup writes one backup file in one step.
func (a *app) writeBackup(path string) error {
	data, err := a.backupBytes()
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// snapshotDaily keeps one copy of a day's work before the day's first save.
// The check is a string compare, so the per-keystroke save pays nothing for
// it; a failed write is logged and not retried until the next launch, so the
// log cannot fill with one line per key.
func (a *app) snapshotDaily() {
	dir := a.backupDir()
	if dir == "" {
		return
	}
	today := time.Now().Format("2006-01-02")
	if a.backupDay == today {
		return
	}
	a.backupDay = today
	path := filepath.Join(dir, "backup-"+today+".json")
	if _, err := os.Stat(path); err == nil {
		return
	}
	if err := a.writeBackup(path); err != nil {
		log.Printf("mygo-demo: daily backup: %v", err)
	}
	a.pruneBackups()
}

// snapshotNow writes a copy under its own name, for a restore or a conflict:
// nothing that is about to be replaced disappears without a trace.
func (a *app) snapshotNow(prefix string) (string, error) {
	dir := a.backupDir()
	if dir == "" {
		return "", nil
	}
	name := fmt.Sprintf("%s-%s.json", prefix, time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(dir, name)
	if err := a.writeBackup(path); err != nil {
		return "", err
	}
	a.pruneBackups()
	return path, nil
}

// snapshotBytes keeps bytes this app is not keeping, such as the losing side
// of a synced-folder conflict, before they are replaced.
func (a *app) snapshotBytes(prefix string, data []byte) (string, error) {
	dir := a.backupDir()
	if dir == "" {
		return "", nil
	}
	name := fmt.Sprintf("%s-%s.json", prefix, time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(dir, name)
	if err := writeFileAtomic(path, data); err != nil {
		return "", err
	}
	a.pruneBackups()
	return path, nil
}

// pruneBackups throws away the oldest daily snapshots past backupKeep and
// the oldest safety copies past conflictKeep. It never fails a save: a
// backup that could not be pruned costs space, not data.
func (a *app) pruneBackups() {
	dir := a.backupDir()
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var daily, other []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasPrefix(name, "backup-2") && len(name) == len("backup-2006-01-02.json") {
			daily = append(daily, name)
		} else {
			other = append(other, name)
		}
	}
	prune := func(names []string, keep int) {
		// The names carry their time, so sorting them sorts them by age.
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
		for _, name := range names[min(keep, len(names)):] {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				log.Printf("mygo-demo: prune backup %s: %v", name, err)
			}
		}
	}
	prune(daily, backupKeep)
	prune(other, conflictKeep)
}

// decodeBackup reads a backup bundle, or a plain library file, so a copy of
// library.json works as a restore too.
func decodeBackup(data []byte) (libraryFile, *showFile, error) {
	var probe struct {
		Version int             `json:"version"`
		Library json.RawMessage `json:"library"`
		Show    json.RawMessage `json:"show"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return libraryFile{}, nil, fmt.Errorf("backup: %w", err)
	}
	if len(probe.Library) == 0 {
		lib, err := decodeLibrary(data)
		return lib, nil, err
	}
	if probe.Version != backupVersion {
		return libraryFile{}, nil, fmt.Errorf("backup: version %d, want %d", probe.Version, backupVersion)
	}
	lib, err := decodeLibrary(probe.Library)
	if err != nil {
		return libraryFile{}, nil, err
	}
	var show *showFile
	if len(probe.Show) > 0 {
		f, err := decodeShow(probe.Show)
		if err != nil {
			return libraryFile{}, nil, err
		}
		show = &f
	}
	return lib, show, nil
}

// readBackupFile reads a chosen backup, with a size cap.
func readBackupFile(path string) (libraryFile, *showFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return libraryFile{}, nil, err
	}
	if info.Size() > maxBackupBytes {
		return libraryFile{}, nil, fmt.Errorf("the file is larger than %d MB", maxBackupBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return libraryFile{}, nil, err
	}
	return decodeBackup(data)
}

// askSave and askOpen are the native dialogs, with a seam so the tests can
// answer without an application.
func (a *app) askSave(opts mygo.SaveDialogOptions) (string, error) {
	if a.saveFile != nil {
		return a.saveFile(opts)
	}
	return mygo.Dialog.Save(opts)
}

func (a *app) askOpen(opts mygo.OpenDialogOptions) ([]string, error) {
	if a.openFile != nil {
		return a.openFile(opts)
	}
	return mygo.Dialog.Open(opts)
}

// backUpSongs writes the library and the run of show wherever the operator
// chooses, as one file they can copy to another machine.
func (a *app) backUpSongs() {
	if a.settingsDir == "" {
		a.backupRefused("There is no app data directory to back up from.")
		return
	}
	name := "mygo-demo-backup-" + time.Now().Format("2006-01-02") + ".json"
	defaultPath := name
	if a.saveFile == nil {
		if dir, err := mygo.App.Path(mygo.PathDocuments); err == nil {
			defaultPath = filepath.Join(dir, name)
		}
	}
	path, err := a.askSave(mygo.SaveDialogOptions{
		Title:       "Back up songs and run of show",
		ButtonLabel: "Back up",
		DefaultPath: defaultPath,
		Filters:     []mygo.FileFilter{{Name: "MyGo demo backup", Extensions: []string{"json"}}},
	})
	if err != nil {
		a.backupRefused("Could not open the save dialog: " + err.Error())
		return
	}
	if path == "" {
		return
	}
	if err := a.writeBackup(path); err != nil {
		a.backupRefused("Could not write the backup: " + err.Error())
		return
	}
	a.backupNote = fmt.Sprintf("Backed up %s and the run of show to %s.",
		songCount(len(a.songs)), filepath.Base(path))
	a.backupNoteErr = false
	a.changed()
}

// chooseRestore reads the file the operator picked and asks before it
// replaces anything.
func (a *app) chooseRestore() {
	paths, err := a.askOpen(mygo.OpenDialogOptions{
		Title:       "Restore from a backup",
		ButtonLabel: "Open",
		Filters:     []mygo.FileFilter{{Name: "MyGo demo backup", Extensions: []string{"json"}}},
	})
	if err != nil {
		a.backupRefused("Could not open the file dialog: " + err.Error())
		return
	}
	if len(paths) == 0 {
		return
	}
	lib, show, err := readBackupFile(paths[0])
	if err != nil {
		a.backupRefused("That file is not a backup: " + err.Error())
		return
	}
	a.restoreWaiting = &restorePlan{Path: paths[0], Library: lib, Show: show}
	a.backupNote = ""
	a.backupNoteErr = false
	a.changed()
}

// confirmRestore applies the plan the Settings page is showing. The library
// and the set here are copied into the backups folder first.
func (a *app) confirmRestore() {
	r := a.restoreWaiting
	if r == nil {
		return
	}
	if _, err := a.snapshotNow("before-restore"); err != nil {
		log.Printf("mygo-demo: back up before restore: %v", err)
	}
	a.songs = r.Library.Songs
	a.normalizeLibrary()
	a.editQuery = ""
	a.editSong, a.editPart, a.editSlide = 0, 0, 0
	a.clampEditor()
	if r.Show != nil {
		a.applyShow(*r.Show)
		// An explicit restore is the way past a file the app refused.
		a.showBlocked = false
		a.showSaved = ""
	}
	a.restoreWaiting = nil
	if err := a.saveLibrary(); err != nil {
		a.backupRefused("Restored the songs, but could not save them: " + err.Error())
		return
	}
	a.saveShow()
	a.backupNote = fmt.Sprintf("Restored %s from %s.", songCount(len(a.songs)), filepath.Base(r.Path))
	a.backupNoteErr = false
	a.changed()
}

// cancelRestore drops the pending plan, leaving everything as it was.
func (a *app) cancelRestore() {
	if a.restoreWaiting == nil {
		return
	}
	a.restoreWaiting = nil
	a.changed()
}

// backupRefused says why a backup action did nothing, in the danger color.
func (a *app) backupRefused(note string) {
	a.backupNote = note
	a.backupNoteErr = true
	a.changed()
}
