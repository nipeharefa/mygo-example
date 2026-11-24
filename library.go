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

// libraryPath is where the songs live, or "" before the app is ready (and in
// tests, which never touch the real app data directory).
func (a *app) libraryPath() string {
	if a.settingsDir == "" {
		return ""
	}
	return filepath.Join(a.settingsDir, "library.json")
}

// loadLibrary reads library.json, seeding it from the built-in public-domain
// songs on first launch. A corrupt file is logged and left alone: the
// built-ins stay usable and the next save replaces it.
func (a *app) loadLibrary() {
	path := a.libraryPath()
	if path == "" {
		if len(a.songs) == 0 {
			a.songs = cloneSongs(builtinSongs)
		}
		a.normalizeLibrary()
		return
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		a.songs = cloneSongs(builtinSongs)
		a.normalizeLibrary()
		if err := writeLibrary(path, libraryFile{Version: libraryVersion, Songs: a.songs}); err != nil {
			log.Printf("mygo-demo: seed library: %v", err)
		}
		return
	}
	f, err := readLibrary(path)
	if err != nil {
		log.Printf("mygo-demo: load library: %v", err)
		if len(a.songs) == 0 {
			a.songs = cloneSongs(builtinSongs)
		}
		a.normalizeLibrary()
		return
	}
	a.songs = f.Songs
	a.normalizeLibrary()
}

// saveLibrary writes the whole library and reports a failure, so the editor
// can say the words are not on disk instead of claiming they are. A missing
// path is a headless test or a window before the app is ready: nothing to do.
func (a *app) saveLibrary() error {
	path := a.libraryPath()
	if path == "" {
		return nil
	}
	return writeLibrary(path, libraryFile{Version: libraryVersion, Songs: a.songs})
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
