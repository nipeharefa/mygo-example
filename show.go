package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// showVersion is the shape of show.json. A file from a newer version is
// refused and never overwritten, so a downgrade cannot mangle a show the
// newer app wrote.
const showVersion = 1

// showFile is what show.json holds: the whole run of show, so the set
// survives a restart, a crash or a quit in the middle of a service. The
// songs themselves live in library.json; a slide carries its words here, so
// a set stays as it was given even if the library changes later.
type showFile struct {
	Version int         `json:"version"`
	Slides  []showSlide `json:"slides"`
	Preview int         `json:"preview"`
	Stage   int         `json:"stage"`
	NextID  int         `json:"nextID"`
}

// showSlide is one slide in the file, written plainly so a person can read
// it and a conflict is easy to see.
type showSlide struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Section  string  `json:"section"`
	Text     string  `json:"text"`
	Kind     int     `json:"kind"`
	TextSize float64 `json:"textSize,omitempty"`
}

// showPath is where the run of show lives, or "" before the app is ready
// (and in tests, which never touch the real app data directory).
func (a *app) showPath() string {
	if a.settingsDir == "" {
		return ""
	}
	return filepath.Join(a.settingsDir, "show.json")
}

// readShow reads and validates a run of show. A missing or malformed file is
// an error; the caller decides whether to seed or keep what it has.
func readShow(path string) (showFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return showFile{}, err
	}
	var f showFile
	if err := json.Unmarshal(data, &f); err != nil {
		return showFile{}, fmt.Errorf("show: %w", err)
	}
	if f.Version != showVersion {
		return showFile{}, fmt.Errorf("show: version %d, want %d", f.Version, showVersion)
	}
	for _, s := range f.Slides {
		if s.Kind < int(slideSong) || s.Kind > int(slideVerse) {
			return showFile{}, fmt.Errorf("show: slide %q has kind %d", s.ID, s.Kind)
		}
	}
	return f, nil
}

// writeShow stores a run of show in one step.
func writeShow(path string, f showFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// loadShow replaces the seeded run of show with the saved one. A missing
// file leaves the seed in place; a file it cannot read is left untouched and
// writing is blocked, so the app cannot overwrite a show it does not
// understand.
func (a *app) loadShow() {
	path := a.showPath()
	if path == "" {
		return
	}
	f, err := readShow(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		log.Printf("mygo-demo: load show: %v", err)
		a.showBlocked = true
		a.showNote = "The saved run of show could not be read, so it is not saved over."
		return
	}
	deck := make([]slide, 0, len(f.Slides))
	seen := make(map[string]bool, len(f.Slides))
	next := f.NextID
	for _, sf := range f.Slides {
		s := slide{
			ID:       sf.ID,
			Title:    sf.Title,
			Section:  sf.Section,
			Text:     sf.Text,
			Kind:     slideKind(sf.Kind),
			TextSize: clampTextSize(sf.TextSize),
		}
		// The id is the slide's identity: a file hand-edited without one,
		// or with a duplicate, gets a fresh one rather than two rows that
		// act as one.
		if s.ID == "" || seen[s.ID] {
			s.ID = fmt.Sprintf("s%d", next)
		}
		if n, ok := slideIDNumber(s.ID); ok && n+1 > next {
			next = n + 1
		}
		seen[s.ID] = true
		deck = append(deck, s)
	}
	a.deck = deck
	a.preview, a.stage = f.Preview, f.Stage
	a.clampSlides()
	a.nextID = max(next, a.nextFreeSlideID())
	a.showSaved = a.showBytes()
	a.showNote = ""
}

// saveShow writes the run of show when it differs from what is on disk. It
// runs from changed(), so every deck action lands without a Save button; the
// equality check keeps a repaint that changed nothing off the disk.
func (a *app) saveShow() {
	path := a.showPath()
	if path == "" || a.showBlocked {
		return
	}
	data := a.showBytes()
	if data == a.showSaved {
		return
	}
	if err := writeShow(path, a.showFile()); err != nil {
		log.Printf("mygo-demo: save show: %v", err)
		a.showNote = "The run of show is not being saved: " + err.Error()
		return
	}
	a.showSaved = data
	a.showNote = ""
}

// showBytes is the serialized form, used to tell whether anything changed.
func (a *app) showBytes() string {
	data, err := json.Marshal(a.showFile())
	if err != nil {
		return ""
	}
	return string(data)
}

// showFile is the run of show as the file holds it.
func (a *app) showFile() showFile {
	f := showFile{
		Version: showVersion,
		Slides:  make([]showSlide, 0, len(a.deck)),
		Preview: a.preview,
		Stage:   a.stage,
		NextID:  a.nextID,
	}
	for _, s := range a.deck {
		f.Slides = append(f.Slides, showSlide{
			ID:       s.ID,
			Title:    s.Title,
			Section:  s.Section,
			Text:     s.Text,
			Kind:     int(s.Kind),
			TextSize: s.TextSize,
		})
	}
	return f
}

// nextFreeSlideID is one past the highest s<n> id in the deck, so a restored
// show never hands out an id it already uses.
func (a *app) nextFreeSlideID() int {
	next := 0
	for _, s := range a.deck {
		if n, ok := slideIDNumber(s.ID); ok && n+1 > next {
			next = n + 1
		}
	}
	return next
}

// slideIDNumber reads s12 as 12, the shape newSlide writes.
func slideIDNumber(id string) (int, bool) {
	if !strings.HasPrefix(id, "s") {
		return 0, false
	}
	n, err := strconv.Atoi(id[1:])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
