package main

import (
	"fmt"
	"sort"
	"strings"
)

// A slide is one screen of the run of show: one section of a song, or one
// verse. Slides are built from the library in data.go and live only in the
// app's memory; the process owns no file.
type slideKind int

const (
	slideSong slideKind = iota
	slideVerse
)

type slide struct {
	ID      string
	Title   string
	Section string
	Text    string
	Kind    slideKind
}

// caption names the slide under the stage preview.
func (s slide) caption() string {
	if s.Kind == slideVerse {
		return s.Title
	}
	return s.Title + " · " + s.Section
}

// eyebrow is the small uppercase line above the slide text.
func (s slide) eyebrow() string {
	if s.Kind == slideVerse {
		return "SCRIPTURE"
	}
	return strings.ToUpper(s.Section)
}

// firstLine is the slide's opening line, used to preview a deck row.
func (s slide) firstLine() string {
	if i := strings.IndexByte(s.Text, '\n'); i >= 0 {
		return s.Text[:i]
	}
	return s.Text
}

// newSlide stamps the next unique id on a slide. The id, not the position,
// is the list's identity, so the same song can appear twice.
func (a *app) newSlide(s slide) slide {
	s.ID = fmt.Sprintf("s%d", a.nextID)
	a.nextID++
	return s
}

// addSong appends every section of the song and moves the monitor to its
// first slide. The stage is untouched until the operator takes it.
func (a *app) addSong(id string) {
	s, ok := songByID(id)
	if !ok {
		return
	}
	first := len(a.deck)
	for _, section := range s.Sections {
		a.deck = append(a.deck, a.newSlide(slide{
			Title:   s.Title,
			Section: section.Label,
			Text:    section.Text,
			Kind:    slideSong,
		}))
	}
	a.preview = first
	a.changed()
}

// addVerse appends one scripture slide and moves the monitor to it.
func (a *app) addVerse(ref string) {
	v, ok := verseByRef(ref)
	if !ok {
		return
	}
	a.deck = append(a.deck, a.newSlide(slide{
		Title:   v.Ref,
		Section: "Scripture",
		Text:    v.Text,
		Kind:    slideVerse,
	}))
	a.preview = len(a.deck) - 1
	a.changed()
}

// selectSlide chooses the monitor row without touching the stage.
func (a *app) selectSlide(i int) {
	if i < 0 || i >= len(a.deck) {
		return
	}
	a.preview = i
	a.changed()
}

// nextSlide advances the monitor without wrapping past the last slide.
func (a *app) nextSlide() {
	if a.preview < len(a.deck)-1 {
		a.preview++
		a.changed()
	}
}

// prevSlide steps the monitor back without wrapping past the first slide.
func (a *app) prevSlide() {
	if a.preview > 0 {
		a.preview--
		a.changed()
	}
}

// takeSlide sends the monitor row to the stage. It is the Go Live action.
func (a *app) takeSlide() {
	if a.preview < 0 || a.preview >= len(a.deck) || a.preview == a.stage {
		return
	}
	a.stage = a.preview
	a.changed()
}

// nextStage advances the stage and pulls the monitor along, so the console
// highlight keeps up with what the room sees.
func (a *app) nextStage() {
	if a.stage < len(a.deck)-1 {
		a.stage++
		a.preview = a.stage
		a.changed()
	}
}

// prevStage steps the stage back and pulls the monitor along.
func (a *app) prevStage() {
	if a.stage > 0 {
		a.stage--
		a.preview = a.stage
		a.changed()
	}
}

// removeSlide drops the monitor row and keeps both indices on real rows.
// The row after the removed one stays on air when the removed row was live.
func (a *app) removeSlide() {
	r := a.preview
	if len(a.deck) == 0 || r < 0 || r >= len(a.deck) {
		return
	}
	s := a.stage
	a.deck = append(a.deck[:r], a.deck[r+1:]...)
	if len(a.deck) == 0 {
		a.preview, a.stage = 0, 0
		a.changed()
		return
	}
	switch {
	case s == r:
		a.stage = clamp(r, 0, len(a.deck)-1)
	case s > r:
		a.stage = s - 1
	}
	a.preview = clamp(r, 0, len(a.deck)-1)
	a.clampSlides()
	a.changed()
}

// moveSlide swaps the monitor row with its neighbour. The slide on air stays
// on air: the reorder carries the stage along with the row it moves.
func (a *app) moveSlide(delta int) {
	p := a.preview + delta
	if a.preview < 0 || a.preview >= len(a.deck) || p < 0 || p >= len(a.deck) {
		return
	}
	a.reorderSlides([]int{a.preview}, p)
}

// reorderSlides moves the rows chosen to before the row that is at to now,
// len(deck) meaning the end of the deck. The moved rows keep their order, the
// slide on air stays on air wherever its row lands, and the monitor follows
// the first moved row. The list calls it with the gap a dropped row lands in;
// Move up/down calls it with the row it lands on, so both paths agree.
func (a *app) reorderSlides(rows []int, to int) {
	n := len(a.deck)
	moved := normalizeRows(rows, n)
	if len(moved) == 0 {
		return
	}
	if len(moved) == 1 && to == moved[0] {
		// A row dropped onto its own gap stays put.
		return
	}
	if to < 0 {
		to = 0
	}
	if to > n {
		to = n
	}
	onAir, hasAir := a.stageSlide()
	// to is the gap the rows land in, counted in the deck without them
	// (len(deck) means the end), so it needs no adjustment.
	at := clamp(to, 0, n-len(moved))
	rest := make([]slide, 0, n-len(moved))
	block := make([]slide, 0, len(moved))
	for i, s := range a.deck {
		if containsRow(moved, i) {
			block = append(block, s)
		} else {
			rest = append(rest, s)
		}
	}
	at = clamp(at, 0, len(rest))
	deck := make([]slide, 0, n)
	deck = append(deck, rest[:at]...)
	deck = append(deck, block...)
	deck = append(deck, rest[at:]...)
	a.deck = deck
	a.stage = 0
	if hasAir {
		if i := deckIndex(deck, onAir.ID); i >= 0 {
			a.stage = i
		}
	}
	a.preview = clamp(at, 0, len(deck)-1)
	a.clampSlides()
	a.changed()
}

// containsRow reports whether row is one of the sorted rows.
func containsRow(rows []int, row int) bool {
	for _, r := range rows {
		if r == row {
			return true
		}
	}
	return false
}

// deckIndex is the row holding a slide id, or -1.
func deckIndex(deck []slide, id string) int {
	for i, s := range deck {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// normalizeRows sorts and dedupes row numbers that name a row of n.
func normalizeRows(rows []int, n int) []int {
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		if r < 0 || r >= n {
			continue
		}
		out = append(out, r)
	}
	sort.Ints(out)
	dedup := out[:0]
	for i, r := range out {
		if i == 0 || out[i-1] != r {
			dedup = append(dedup, r)
		}
	}
	return dedup
}

func (a *app) toggleBlackout() {
	a.blackout = !a.blackout
	a.changed()
}

// stageSlide is the slide the output window and NDI show.
func (a *app) stageSlide() (slide, bool) {
	if a.stage < 0 || a.stage >= len(a.deck) {
		return slide{}, false
	}
	return a.deck[a.stage], true
}

// previewSlide is the slide the console monitor shows.
func (a *app) previewSlide() (slide, bool) {
	if a.preview < 0 || a.preview >= len(a.deck) {
		return slide{}, false
	}
	return a.deck[a.preview], true
}

// upcomingSlide is the slide after the one on air, for the "Next up" line.
func (a *app) upcomingSlide() (slide, bool) {
	next := a.stage + 1
	if next < 0 || next >= len(a.deck) {
		return slide{}, false
	}
	return a.deck[next], true
}

// clampSlides keeps both indices inside the deck after removals.
func (a *app) clampSlides() {
	if len(a.deck) == 0 {
		a.preview, a.stage = 0, 0
		return
	}
	a.preview = clamp(a.preview, 0, len(a.deck)-1)
	a.stage = clamp(a.stage, 0, len(a.deck)-1)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
