package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// libraryVersion is the shape of library.json. A file from a newer version is
// refused rather than half-read.
const libraryVersion = 1

// libraryFile is what library.json holds: the whole song library, so the
// editor's changes survive a restart. The run of show is still session-only.
type libraryFile struct {
	Version int    `json:"version"`
	Songs   []song `json:"songs"`
}

// readLibrary reads and validates a library file. A missing or malformed file
// is an error; the caller decides whether to seed or keep what it has.
func readLibrary(path string) (libraryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return libraryFile{}, err
	}
	return decodeLibrary(data)
}

// decodeLibrary validates the bytes of a library, wherever they came from: a
// file, or the library half of a backup.
func decodeLibrary(data []byte) (libraryFile, error) {
	var f libraryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return libraryFile{}, fmt.Errorf("library: %w", err)
	}
	if f.Version != libraryVersion {
		return libraryFile{}, fmt.Errorf("library: version %d, want %d", f.Version, libraryVersion)
	}
	seen := make(map[string]bool, len(f.Songs))
	for _, s := range f.Songs {
		if s.ID == "" || strings.TrimSpace(s.Title) == "" {
			return libraryFile{}, fmt.Errorf("library: song with an empty id or title")
		}
		if seen[s.ID] {
			return libraryFile{}, fmt.Errorf("library: duplicate song id %q", s.ID)
		}
		seen[s.ID] = true
	}
	if f.Songs == nil {
		f.Songs = []song{}
	}
	return f, nil
}

// writeLibrary writes the whole file in one step, so a crash mid-write
// cannot leave a half library behind.
func writeLibrary(path string, f libraryFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// marshalLibrary writes the library the way library.json holds it, so a save
// can hash and compare the exact bytes it wrote.
func marshalLibrary(songs []song) ([]byte, error) {
	data, err := json.MarshalIndent(libraryFile{Version: libraryVersion, Songs: songs}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// libraryDir is the folder library.json lives in: the one the operator
// chose, or the app data directory. It is "" before the app is ready.
func (a *app) libraryDir() string {
	if a.libraryFolder != "" {
		return a.libraryFolder
	}
	return a.settingsDir
}

// libraryPath is where the songs live, or "" before the app is ready (and in
// tests, which never touch the real app data directory).
func (a *app) libraryPath() string {
	dir := a.libraryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "library.json")
}

// errLibraryReloaded says the file on disk won and the in-memory edit was
// dropped, so a caller can say so instead of claiming a save.
var errLibraryReloaded = errors.New("library: reloaded from the folder")

// loadLibrary reads library.json, seeding it from the built-in public-domain
// songs on first launch. A corrupt file is logged and left alone: the
// built-ins stay usable and the next save replaces it, except in a chosen
// folder, where a file that cannot be read is never written over.
func (a *app) loadLibrary() {
	path := a.libraryPath()
	if path == "" {
		if len(a.songs) == 0 {
			a.songs = cloneSongs(builtinSongs)
		}
		a.normalizeLibrary()
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && a.libraryFolder == "" {
			a.songs = cloneSongs(builtinSongs)
			a.normalizeLibrary()
			if err := a.saveLibrary(); err != nil {
				log.Printf("mygo-demo: seed library: %v", err)
			}
			return
		}
		log.Printf("mygo-demo: load library: %v", err)
		if len(a.songs) == 0 {
			a.songs = cloneSongs(builtinSongs)
		}
		a.normalizeLibrary()
		a.blockLibrary("The songs folder could not be read, so the songs are not saved over.")
		return
	}
	f, err := decodeLibrary(data)
	if err != nil {
		log.Printf("mygo-demo: load library: %v", err)
		if len(a.songs) == 0 {
			a.songs = cloneSongs(builtinSongs)
		}
		a.normalizeLibrary()
		a.blockLibrary("The songs file could not be read, so it is not saved over.")
		return
	}
	a.songs = f.Songs
	a.normalizeLibrary()
	a.libraryStamp = hashBytes(data)
	a.libraryBlocked = false
	a.libraryNote = ""
}

// blockLibrary stops every later write, so a file the app does not
// understand is not replaced by a seed.
func (a *app) blockLibrary(note string) {
	a.libraryBlocked = true
	a.libraryNote = note
}

// saveLibrary writes the whole library and reports a failure, so the editor
// can say the words are not on disk instead of claiming they are. A missing
// path is a headless test or a window before the app is ready: nothing to do.
// In a chosen folder the file is checked first: if another machine or a sync
// client wrote it since this app read it, the operator decides which copy
// wins before anything is replaced.
func (a *app) saveLibrary() error {
	path := a.libraryPath()
	if path == "" {
		return nil
	}
	if a.libraryBlocked {
		return errors.New(a.libraryNote)
	}
	if a.libraryFolder != "" {
		if err := a.checkForeignLibrary(); err != nil {
			return err
		}
	}
	data, err := marshalLibrary(a.songs)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(path, data); err != nil {
		return err
	}
	a.libraryStamp = hashBytes(data)
	a.snapshotDaily()
	return nil
}

// checkForeignLibrary compares the file with what this app last read or
// wrote. A file it cannot read is a conflict too: writing over it blindly is
// how a synced library disappears.
func (a *app) checkForeignLibrary() error {
	data, err := os.ReadFile(a.libraryPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if a.libraryStamp == "" || hashBytes(data) == a.libraryStamp {
		return nil
	}
	return a.resolveLibraryConflict(data)
}

// resolveLibraryConflict asks which copy to keep. Reloading adopts the other
// file and reports errLibraryReloaded; keeping this app's songs saves the
// other copy in backups/ and lets the write continue.
func (a *app) resolveLibraryConflict(data []byte) error {
	ask := a.askMessage
	if ask == nil {
		ask = mygo.Dialog.Message
	}
	res, err := ask(mygo.MessageOptions{
		Type:         mygo.MessageWarning,
		Title:        "The songs folder changed",
		Message:      "Another program or machine changed the songs file.",
		Detail:       "Reload it to use those songs, or keep the songs here and save the other copy in the backups folder.",
		Buttons:      []string{"Reload the file", "Keep my songs"},
		CancelButton: -1,
	})
	if err != nil {
		return err
	}
	if res.Button == 0 {
		f, err := decodeLibrary(data)
		if err != nil {
			return fmt.Errorf("the changed file could not be read: %w", err)
		}
		a.songs = f.Songs
		a.normalizeLibrary()
		a.editQuery = ""
		a.editSong, a.editPart, a.editSlide = 0, 0, 0
		a.clampEditor()
		a.libraryStamp = hashBytes(data)
		a.libraryNote = "Reloaded " + songCount(len(a.songs)) + " from the songs folder."
		return errLibraryReloaded
	}
	if _, err := a.snapshotBytes("conflict", data); err != nil {
		log.Printf("mygo-demo: keep the other songs copy: %v", err)
	}
	// The write that follows replaces the other copy, so its stamp is the
	// one to beat.
	a.libraryStamp = hashBytes(data)
	a.libraryNote = ""
	return nil
}

// normalizeLibrary drops songs that cannot be addressed and pulls every size
// back into the slider's range, so the editor never shows a value it cannot
// produce.
func (a *app) normalizeLibrary() {
	seen := make(map[string]bool, len(a.songs))
	out := make([]song, 0, len(a.songs))
	for _, s := range a.songs {
		if s.ID == "" || strings.TrimSpace(s.Title) == "" || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		s.TextSize = s.textSize()
		// The editor always needs one part with one slide to stand on; a
		// hand-edited file may leave either empty.
		if len(s.Parts) == 0 {
			s.Parts = []songPart{{Label: "Verse 1", Slides: []string{""}}}
		}
		for j := range s.Parts {
			if len(s.Parts[j].Slides) == 0 {
				s.Parts[j].Slides = []string{""}
			}
		}
		out = append(out, s)
	}
	a.songs = out
}

// cloneSongs copies the parts and slides too, so editing a song cannot reach
// through to the built-ins.
func cloneSongs(list []song) []song {
	out := make([]song, len(list))
	for i, s := range list {
		parts := make([]songPart, len(s.Parts))
		for j, p := range s.Parts {
			parts[j] = songPart{Label: p.Label, Slides: append([]string(nil), p.Slides...)}
		}
		s.Parts = parts
		out[i] = s
	}
	return out
}

// newSongID makes a stable, readable id from a title: lower-case letters,
// digits and single dashes, with -2, -3… when the name is taken.
func newSongID(title string, existing []song) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		base = "song"
	}
	id := base
	for n := 2; ; n++ {
		taken := false
		for _, s := range existing {
			if s.ID == id {
				taken = true
				break
			}
		}
		if !taken {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}
