package main

import (
	"fmt"
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

// addSong appends every section of the song and selects its first slide.
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
	a.index = first
	a.changed()
}

// addVerse appends one scripture slide and selects it.
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
	a.index = len(a.deck) - 1
	a.changed()
}

func (a *app) selectSlide(i int) {
	if i < 0 || i >= len(a.deck) {
		return
	}
	a.index = i
	a.changed()
}

// nextSlide advances without wrapping past the last slide.
func (a *app) nextSlide() {
	if a.index < len(a.deck)-1 {
		a.index++
		a.changed()
	}
}

// prevSlide steps back without wrapping past the first slide.
func (a *app) prevSlide() {
	if a.index > 0 {
		a.index--
		a.changed()
	}
}

// removeSlide drops the selected slide and keeps the index on a real row.
func (a *app) removeSlide() {
	if len(a.deck) == 0 {
		return
	}
	a.deck = append(a.deck[:a.index], a.deck[a.index+1:]...)
	a.clampIndex()
	a.changed()
}

// moveSlide swaps the selected slide with its neighbour.
func (a *app) moveSlide(delta int) {
	target := a.index + delta
	if target < 0 || target >= len(a.deck) {
		return
	}
	a.deck[a.index], a.deck[target] = a.deck[target], a.deck[a.index]
	a.index = target
	a.changed()
}

func (a *app) toggleBlackout() {
	a.blackout = !a.blackout
	a.changed()
}

func (a *app) currentSlide() (slide, bool) {
	if a.index < 0 || a.index >= len(a.deck) {
		return slide{}, false
	}
	return a.deck[a.index], true
}

func (a *app) upcomingSlide() (slide, bool) {
	next := a.index + 1
	if next < 0 || next >= len(a.deck) {
		return slide{}, false
	}
	return a.deck[next], true
}

// clampIndex keeps the selection inside the deck after removals.
func (a *app) clampIndex() {
	if len(a.deck) == 0 {
		a.index = 0
		return
	}
	a.index = clamp(a.index, 0, len(a.deck)-1)
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
