package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// folderPlan is a songs folder the operator picked and the Settings page is
// asking about: a folder that already holds a library, or an empty one that
// will take the songs here.
type folderPlan struct {
	Dir      string
	Existing *libraryFile // nil when the folder holds no library yet
}

// chooseSongsFolder asks for a folder and, when it holds a library, asks
// what to do with it. Nothing moves until the question is answered.
func (a *app) chooseSongsFolder() {
	paths, err := a.askOpen(mygo.OpenDialogOptions{
		Title:       "Choose the songs folder",
		ButtonLabel: "Use folder",
		Message:     "A folder your cloud app syncs, such as iCloud Drive or Dropbox, moves the songs between machines.",
		Directory:   true,
	})
	if err != nil {
		a.folderRefused("Could not open the folder dialog: " + err.Error())
		return
	}
	if len(paths) == 0 {
		return
	}
	dir := paths[0]
	if samePath(dir, a.libraryDir()) {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, "library.json"))
	switch {
	case err == nil:
		f, derr := decodeLibrary(data)
		if derr != nil {
			a.folderRefused("That folder's library.json could not be read: " + derr.Error())
			return
		}
		a.folderWaiting = &folderPlan{Dir: dir, Existing: &f}
	case errors.Is(err, fs.ErrNotExist):
		a.folderWaiting = &folderPlan{Dir: dir}
	default:
		a.folderRefused("Could not read that folder: " + err.Error())
		return
	}
	a.restoreWaiting = nil
	a.folderNote, a.folderNoteErr = "", false
	a.changed()
}

// confirmFolder moves the library to the chosen folder: adopting the songs
// already there, or writing the songs here into it. The state that is about
// to be left behind is copied into backups/ first, and a folder that cannot
// be written is refused rather than silently kept.
func (a *app) confirmFolder() {
	p := a.folderWaiting
	if p == nil {
		return
	}
	oldFolder, oldStamp := a.libraryFolder, a.libraryStamp
	if _, err := a.snapshotNow("before-folder"); err != nil {
		a.folderRefused("Could not save a copy before moving: " + err.Error())
		return
	}
	a.libraryFolder = p.Dir
	a.libraryStamp = ""
	a.libraryBlocked = false
	a.libraryNote = ""
	if p.Existing != nil {
		a.songs = p.Existing.Songs
		a.normalizeLibrary()
		a.editQuery = ""
		a.editSong, a.editPart, a.editSlide = 0, 0, 0
		a.clampEditor()
	}
	if err := a.saveLibrary(); err != nil {
		a.libraryFolder, a.libraryStamp = oldFolder, oldStamp
		a.folderWaiting = nil
		a.folderRefused("The songs folder could not be written: " + err.Error())
		return
	}
	a.folderWaiting = nil
	a.folderNote = "Songs are saved in " + p.Dir + "."
	a.folderNoteErr = false
	a.noteHistory("Made a songs folder")
	a.saveSettings()
	a.changed()
}

// cancelFolder leaves the library where it was.
func (a *app) cancelFolder() {
	if a.folderWaiting == nil {
		return
	}
	a.folderWaiting = nil
	a.changed()
}

// useAppFolder brings the library back to the app data directory, writing
// the songs here into it.
func (a *app) useAppFolder() {
	if a.libraryFolder == "" {
		return
	}
	oldFolder, oldStamp := a.libraryFolder, a.libraryStamp
	if _, err := a.snapshotNow("before-folder"); err != nil {
		a.folderRefused("Could not save a copy before moving: " + err.Error())
		return
	}
	a.libraryFolder = ""
	a.libraryStamp = ""
	if err := a.saveLibrary(); err != nil {
		a.libraryFolder, a.libraryStamp = oldFolder, oldStamp
		a.folderRefused("The app's folder could not be written: " + err.Error())
		return
	}
	a.folderNote = "Songs are saved in the app's own folder."
	a.folderNoteErr = false
	a.noteHistory("Moved the songs back")
	a.saveSettings()
	a.changed()
}

// reloadLibrary reads the songs file again, for when another machine saved
// while this one had the folder open.
func (a *app) reloadLibrary() {
	path := a.libraryPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		a.folderRefused("Could not read the songs file: " + err.Error())
		return
	}
	f, err := decodeLibrary(data)
	if err != nil {
		a.folderRefused("The songs file could not be read: " + err.Error())
		return
	}
	a.songs = f.Songs
	a.normalizeLibrary()
	a.editQuery = ""
	a.editSong, a.editPart, a.editSlide = 0, 0, 0
	a.clampEditor()
	a.libraryStamp = hashBytes(data)
	a.libraryBlocked = false
	a.folderNote = "Reloaded " + songCount(len(a.songs)) + " from the songs folder."
	a.folderNoteErr = false
	a.noteHistory("Reloaded the songs folder")
	a.changed()
}

// folderRefused says why a folder action did nothing, in the danger color.
func (a *app) folderRefused(note string) {
	a.folderNote = note
	a.folderNoteErr = true
	a.changed()
}

// samePath reports whether two paths name the same folder, through the
// symlinks macOS puts in front of /tmp and a cloud folder.
func samePath(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	ca, ea := filepath.Abs(a)
	cb, eb := filepath.Abs(b)
	return ea == nil && eb == nil && ca == cb
}
