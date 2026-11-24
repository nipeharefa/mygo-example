package main

import (
	"fmt"
	"strings"
	"testing"
)

// deckApp gives the model tests the built-in library without a window.
func deckApp() *app {
	return &app{songs: cloneSongs(builtinSongs)}
}

func TestDataIsReachable(t *testing.T) {
	if len(builtinSongs) < 4 {
		t.Fatalf("builtinSongs = %d, want at least 4", len(builtinSongs))
	}
	seenSongs := map[string]bool{}
	slides := 0
	for _, s := range builtinSongs {
		if s.ID == "" || s.Title == "" {
			t.Errorf("song missing id or title: %+v", s)
		}
		if seenSongs[s.ID] {
			t.Errorf("duplicate song id %q", s.ID)
		}
		seenSongs[s.ID] = true
		if len(s.Parts) == 0 {
			t.Errorf("song %q has no parts", s.ID)
		}
		for _, part := range s.Parts {
			if part.Label == "" {
				t.Errorf("song %q has an empty part label", s.ID)
			}
			if len(part.Slides) == 0 {
				t.Errorf("song %q part %q has no slides", s.ID, part.Label)
			}
			for _, text := range part.Slides {
				if strings.TrimSpace(text) == "" {
					t.Errorf("song %q part %q has a blank slide", s.ID, part.Label)
				}
				slides++
			}
		}
	}
	if slides < 8 {
		t.Fatalf("slides = %d, want at least 8", slides)
	}
	if _, ok := findSong(builtinSongs, builtinSongs[0].ID); !ok {
		t.Errorf("findSong(%q) not found", builtinSongs[0].ID)
	}
	if _, ok := findSong(builtinSongs, "no-such-song"); ok {
		t.Error("findSong accepted an unknown id")
	}

	if len(packs) < 3 {
		t.Fatalf("packs = %d, want at least 3", len(packs))
	}
	seenVerses := map[string]bool{}
	verses := 0
	for _, p := range packs {
		if p.Name == "" {
			t.Error("verse pack has no name")
		}
		if len(p.Verses) == 0 {
			t.Errorf("pack %q has no verses", p.Name)
		}
		for _, v := range p.Verses {
			if v.Ref == "" || v.Text == "" {
				t.Errorf("pack %q has an empty verse", p.Name)
			}
			if seenVerses[v.Ref] {
				t.Errorf("duplicate verse ref %q", v.Ref)
			}
			seenVerses[v.Ref] = true
			verses++
		}
	}
	if verses < 12 {
		t.Fatalf("verses = %d, want at least 12", verses)
	}
	if _, ok := verseByRef(packs[0].Verses[0].Ref); !ok {
		t.Errorf("verseByRef(%q) not found", packs[0].Verses[0].Ref)
	}
	if _, ok := verseByRef("No Such 1:1"); ok {
		t.Error("verseByRef accepted an unknown ref")
	}
}

func TestAddSongAppendsSections(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	if len(a.deck) != 3 {
		t.Fatalf("deck = %d slides, want 3", len(a.deck))
	}
	if a.preview != 0 {
		t.Fatalf("preview = %d, want 0", a.preview)
	}
	if a.stage != 0 {
		t.Fatalf("stage = %d, want 0", a.stage)
	}
	if got := a.deck[0].Title; got != "Amazing Grace" {
		t.Errorf("title = %q", got)
	}
	if got := a.deck[0].Section; got != "Verse 1" {
		t.Errorf("section = %q", got)
	}
	if a.deck[0].Text == "" {
		t.Error("slide text is empty")
	}
	if a.deck[0].Kind != slideSong {
		t.Errorf("kind = %v, want slideSong", a.deck[0].Kind)
	}
	if a.deck[0].ID == a.deck[1].ID || a.deck[1].ID == a.deck[2].ID {
		t.Errorf("duplicate slide ids: %q %q %q", a.deck[0].ID, a.deck[1].ID, a.deck[2].ID)
	}
}

func TestAddSongMovesOnlyTheMonitor(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.nextStage()
	onAir := a.deck[a.stage].ID

	a.addSong("amazing-grace")
	if a.stage != 1 {
		t.Fatalf("adding a song changed the stage to %d", a.stage)
	}
	if got := a.deck[a.stage].ID; got != onAir {
		t.Fatalf("the slide on air changed from %q to %q", onAir, got)
	}
	if a.preview != 3 {
		t.Fatalf("preview = %d, want the first new slide (3)", a.preview)
	}
}

func TestAddSongTwiceKeepsIDsUnique(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.addSong("amazing-grace")
	seen := map[string]bool{}
	for _, s := range a.deck {
		if seen[s.ID] {
			t.Fatalf("duplicate slide id %q", s.ID)
		}
		seen[s.ID] = true
	}
	if len(a.deck) != 6 {
		t.Fatalf("deck = %d, want 6", len(a.deck))
	}
}

func TestAddSongIgnoresUnknownID(t *testing.T) {
	a := deckApp()
	a.addSong("no-such-song")
	if len(a.deck) != 0 {
		t.Fatalf("deck = %d, want empty", len(a.deck))
	}
}

func TestAddVerseAppendsOneSlide(t *testing.T) {
	a := deckApp()
	a.addVerse("Psalm 23:1")
	if len(a.deck) != 1 {
		t.Fatalf("deck = %d, want 1", len(a.deck))
	}
	s := a.deck[0]
	if s.Title != "Psalm 23:1" || s.Section != "Scripture" || s.Kind != slideVerse {
		t.Errorf("verse slide = %+v", s)
	}
	if s.caption() != "Psalm 23:1" {
		t.Errorf("caption = %q", s.caption())
	}
	if a.preview != 0 {
		t.Errorf("preview = %d, want the new slide", a.preview)
	}
}

func TestMonitorNavigationClamps(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.prevSlide()
	if a.preview != 0 {
		t.Fatalf("prev at 0 moved to %d", a.preview)
	}
	a.nextSlide()
	if a.preview != 1 {
		t.Fatalf("next moved to %d, want 1", a.preview)
	}
	a.selectSlide(2)
	a.nextSlide()
	if a.preview != 2 {
		t.Fatalf("next past the end moved to %d", a.preview)
	}
	if a.stage != 0 {
		t.Fatalf("monitor navigation changed the stage to %d", a.stage)
	}
}

func TestSelectSlideClamps(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(1)
	if a.preview != 1 {
		t.Fatalf("preview = %d, want 1", a.preview)
	}
	a.selectSlide(99)
	a.selectSlide(-1)
	if a.preview != 1 {
		t.Fatalf("out-of-range select changed preview to %d", a.preview)
	}
	if a.stage != 0 {
		t.Fatalf("selecting changed the stage to %d", a.stage)
	}
}

func TestTakeSlide(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(2)
	a.takeSlide()
	if a.stage != 2 || a.preview != 2 {
		t.Fatalf("stage = %d preview = %d, want 2/2", a.stage, a.preview)
	}
	a.selectSlide(1)
	if a.stage != 2 {
		t.Fatalf("choosing another monitor row changed the stage to %d", a.stage)
	}
	a.takeSlide()
	if a.stage != 1 {
		t.Fatalf("second take left the stage at %d", a.stage)
	}
}

func TestTakeSlideWithoutADeck(t *testing.T) {
	a := deckApp()
	a.takeSlide()
	if a.stage != 0 || a.preview != 0 {
		t.Fatalf("empty take moved to stage=%d preview=%d", a.stage, a.preview)
	}
}

func TestNextStagePullsTheMonitor(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.nextStage()
	if a.stage != 1 || a.preview != 1 {
		t.Fatalf("next stage = %d preview = %d, want 1/1", a.stage, a.preview)
	}
	a.nextStage()
	a.nextStage()
	if a.stage != 2 {
		t.Fatalf("next past the end moved the stage to %d", a.stage)
	}
	a.prevStage()
	if a.stage != 1 || a.preview != 1 {
		t.Fatalf("prev stage = %d preview = %d, want 1/1", a.stage, a.preview)
	}
	a.prevStage()
	a.prevStage()
	if a.stage != 0 {
		t.Fatalf("prev past the start moved the stage to %d", a.stage)
	}
}

func TestRemoveMonitorRowKeepsStage(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	onAir := a.deck[0].ID
	a.selectSlide(2)
	a.removeSlide()
	if len(a.deck) != 2 || a.preview != 1 || a.stage != 0 {
		t.Fatalf("deck = %d preview = %d stage = %d, want 2/1/0", len(a.deck), a.preview, a.stage)
	}
	if got := a.deck[a.stage].ID; got != onAir {
		t.Fatalf("removing a monitor row changed the slide on air to %q", got)
	}
}

func TestRemoveStageRowAdvancesStage(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(1)
	a.takeSlide()
	a.removeSlide()
	if len(a.deck) != 2 || a.stage != 1 || a.preview != 1 {
		t.Fatalf("deck = %d stage = %d preview = %d, want 2/1/1", len(a.deck), a.stage, a.preview)
	}
	if got := a.deck[a.stage].Section; got != "Verse 3" {
		t.Fatalf("the row after the removed live row is not on air: %q", got)
	}
}

func TestRemoveStageRowAtTheEnd(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(2)
	a.takeSlide()
	a.removeSlide()
	if len(a.deck) != 2 || a.stage != 1 {
		t.Fatalf("deck = %d stage = %d, want 2/1", len(a.deck), a.stage)
	}
}

func TestRemoveAboveStageShiftsStage(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(2)
	a.takeSlide()
	a.selectSlide(0)
	a.removeSlide()
	if a.stage != 1 {
		t.Fatalf("removing above the stage left it at %d, want 1", a.stage)
	}
	if got := a.deck[a.stage].Section; got != "Verse 3" {
		t.Fatalf("the live slide changed to %q", got)
	}
}

func TestRemoveToEmpty(t *testing.T) {
	a := deckApp()
	a.addVerse("Psalm 23:1")
	a.removeSlide()
	if len(a.deck) != 0 || a.preview != 0 || a.stage != 0 {
		t.Fatalf("deck = %d preview = %d stage = %d, want 0/0/0", len(a.deck), a.preview, a.stage)
	}
	a.removeSlide()
	if a.preview != 0 || a.stage != 0 {
		t.Fatalf("remove on empty moved preview=%d stage=%d", a.preview, a.stage)
	}
}

func TestMoveSlideKeepsTheStageContent(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(1)
	a.takeSlide()
	onAir := a.deck[a.stage].ID

	a.selectSlide(0)
	a.moveSlide(1)
	if a.preview != 1 {
		t.Fatalf("preview = %d, want 1", a.preview)
	}
	if a.stage != 0 {
		t.Fatalf("stage = %d, want 0 after the swap", a.stage)
	}
	if got := a.deck[a.stage].ID; got != onAir {
		t.Fatalf("the slide on air changed from %q to %q", onAir, got)
	}
}

func TestMoveSlideMovesTheStageWithIt(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.selectSlide(1)
	a.takeSlide()
	a.moveSlide(-1)
	if a.preview != 0 || a.stage != 0 {
		t.Fatalf("preview = %d stage = %d, want 0/0", a.preview, a.stage)
	}
	if got := a.deck[a.stage].Section; got != "Verse 2" {
		t.Fatalf("the live row changed to %q", got)
	}
}

func TestMoveSlideStaysInBounds(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	last := a.deck[0].ID
	a.selectSlide(0)
	a.moveSlide(-1)
	if a.deck[0].ID != last {
		t.Fatal("move up past the top reordered the deck")
	}
	a.selectSlide(2)
	a.moveSlide(1)
	if a.preview != 2 {
		t.Fatalf("move down past the end moved preview to %d", a.preview)
	}
}

func TestStageAndPreviewSlides(t *testing.T) {
	a := deckApp()
	if _, ok := a.stageSlide(); ok {
		t.Fatal("empty deck reported a stage slide")
	}
	if _, ok := a.previewSlide(); ok {
		t.Fatal("empty deck reported a preview slide")
	}
	a.addSong("amazing-grace")
	a.selectSlide(2)
	stage, ok := a.stageSlide()
	if !ok || stage.Section != "Verse 1" {
		t.Fatalf("stage = %+v ok = %v", stage, ok)
	}
	preview, ok := a.previewSlide()
	if !ok || preview.Section != "Verse 3" {
		t.Fatalf("preview = %+v ok = %v", preview, ok)
	}
}

func TestUpcomingSlide(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	up, ok := a.upcomingSlide()
	if !ok || up.Section != "Verse 2" {
		t.Fatalf("upcoming = %+v ok = %v", up, ok)
	}
	a.nextStage()
	a.nextStage()
	if _, ok := a.upcomingSlide(); ok {
		t.Fatal("last slide reported an upcoming slide")
	}
}

func TestToggleBlackout(t *testing.T) {
	a := deckApp()
	a.toggleBlackout()
	if !a.blackout {
		t.Fatal("blackout not enabled")
	}
	a.toggleBlackout()
	if a.blackout {
		t.Fatal("blackout not disabled")
	}
}

func TestChangedWithoutWindows(t *testing.T) {
	a := deckApp()
	a.changed()
}

// seededApp returns the app's seed deck for the reorder tests.
func seededApp() *app {
	return newApp()
}

func deckIDs(a *app) []string {
	ids := make([]string, len(a.deck))
	for i, s := range a.deck {
		ids[i] = s.ID
	}
	return ids
}

func TestNormalizeRows(t *testing.T) {
	cases := []struct {
		rows []int
		n    int
		want []int
	}{
		{[]int{2, 0}, 5, []int{0, 2}},
		{[]int{1, 1}, 3, []int{1}},
		{[]int{-1, 9}, 3, nil},
		{nil, 3, nil},
	}
	for _, c := range cases {
		got := normalizeRows(c.rows, c.n)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("normalizeRows(%v, %d) = %v, want %v", c.rows, c.n, got, c.want)
		}
	}
}

// Dragging the first slide to the end leaves the same slide on air.
func TestReorderMovesSlidesAndKeepsTheStage(t *testing.T) {
	a := seededApp()
	a.stage, a.preview = 0, 0
	onAir := a.deck[0].ID
	want := deckIDs(a)

	a.reorderSlides([]int{0}, len(a.deck))

	if a.deck[len(a.deck)-1].ID != onAir {
		t.Fatalf("the dragged slide landed at %v", deckIDs(a))
	}
	if got := a.deck[a.stage].ID; got != onAir {
		t.Fatalf("stage shows %s, want the same slide %s", got, onAir)
	}
	if a.preview != len(a.deck)-1 {
		t.Fatalf("preview = %d, want the dragged row at the end", a.preview)
	}
	// Every slide survives, only the order changed.
	if len(deckIDs(a)) != len(want) {
		t.Fatalf("deck lost a slide: %v", deckIDs(a))
	}
}

// Dragging a later row upwards, over the row on air, keeps that row on air.
func TestReorderOverTheStageKeepsItOnAir(t *testing.T) {
	a := seededApp()
	a.addVerse("Psalm 23:2")
	a.stage, a.preview = 1, len(a.deck)-1
	onAir := a.deck[1].ID
	moved := a.deck[len(a.deck)-1].ID

	a.reorderSlides([]int{len(a.deck) - 1}, 0)

	if a.deck[0].ID != moved {
		t.Fatalf("deck = %v, want %s first", deckIDs(a), moved)
	}
	if a.deck[a.stage].ID != onAir {
		t.Fatalf("stage shows %s, want %s", a.deck[a.stage].ID, onAir)
	}
	if a.preview != 0 {
		t.Fatalf("preview = %d, want 0", a.preview)
	}
}

func TestReorderWithoutMovingDoesNothing(t *testing.T) {
	a := seededApp()
	before := deckIDs(a)
	a.reorderSlides([]int{1}, 1)
	if fmt.Sprint(deckIDs(a)) != fmt.Sprint(before) {
		t.Fatalf("a no-op drag reordered the deck: %v", deckIDs(a))
	}
}

func TestReorderIgnoresBadRows(t *testing.T) {
	a := seededApp()
	before := deckIDs(a)
	a.reorderSlides(nil, 0)
	a.reorderSlides([]int{99}, 0)
	a.reorderSlides([]int{-3}, 2)
	if fmt.Sprint(deckIDs(a)) != fmt.Sprint(before) {
		t.Fatalf("bad rows reordered the deck: %v", deckIDs(a))
	}
}

// Move down keeps the moved slide on air and puts its neighbour on air with
// it, like a drag that carries the stage along.
func TestMoveSlideUsesTheReorder(t *testing.T) {
	a := seededApp()
	a.stage, a.preview = 0, 0
	second := a.deck[1].ID
	a.moveSlide(1)
	if a.preview != 1 {
		t.Fatalf("preview = %d, want 1", a.preview)
	}
	if a.stage != 1 {
		t.Fatalf("stage = %d, want the slide that moved to 1", a.stage)
	}

	// Moving a row that is not on air leaves the stage alone.
	a.stage, a.preview = 0, 2
	b := seededApp()
	_ = b
	onAir := a.deck[0].ID
	a.moveSlide(1)
	if got := a.deck[a.stage].ID; got != onAir {
		t.Fatalf("stage shows %s, want %s", got, onAir)
	}
	if a.deck[a.preview].ID == "" || a.preview != 3 {
		t.Fatalf("preview = %d for %s, want the moved row", a.preview, second)
	}
}

func TestAddSongSnapshotsTextSize(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	for i, s := range a.deck {
		if s.TextSize != defaultTextSize {
			t.Errorf("slide %d size = %v, want %v", i, s.TextSize, defaultTextSize)
		}
	}
}

func TestAddSongAllCreaturesHasTwoSlides(t *testing.T) {
	a := deckApp()
	a.addSong("all-creatures")
	if len(a.deck) != 2 {
		t.Fatalf("deck = %d slides, want 2", len(a.deck))
	}
	for i, s := range a.deck {
		if s.Section != "Verse 1" {
			t.Errorf("slide %d section = %q, want Verse 1", i, s.Section)
		}
	}
	if a.preview != 0 {
		t.Errorf("preview = %d, want 0", a.preview)
	}
}

func TestAddSongSkipsBlankSlides(t *testing.T) {
	a := deckApp()
	draft := song{
		Title: "Draft",
		Parts: []songPart{{Label: "Verse 1", Slides: []string{"", "   ", "real text"}}},
	}
	if n := a.addSongParts(draft); n != 1 {
		t.Fatalf("added = %d, want 1", n)
	}
	if len(a.deck) != 1 || a.deck[0].Text != "real text" {
		t.Fatalf("deck = %+v", a.deck)
	}
	if a.preview != 0 {
		t.Errorf("preview = %d, want 0", a.preview)
	}
}

func TestAddSongWithNoTextLeavesDeckAndPreview(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.nextStage()
	before := len(a.deck)
	draft := song{Parts: []songPart{{Label: "Verse 1", Slides: []string{" ", ""}}}}
	if n := a.addSongParts(draft); n != 0 {
		t.Fatalf("added = %d, want 0", n)
	}
	if len(a.deck) != before || a.preview != 1 {
		t.Errorf("deck = %d, preview = %d; want %d, 1", len(a.deck), a.preview, before)
	}
}

func TestAddPartAppendsOnePart(t *testing.T) {
	a := deckApp()
	a.addSong("amazing-grace")
	a.nextStage()
	onAir := a.deck[a.stage].ID

	if !a.addPart("amazing-grace", 1) {
		t.Fatal("addPart returned false")
	}
	if len(a.deck) != 4 {
		t.Fatalf("deck = %d, want 4", len(a.deck))
	}
	if a.deck[3].Section != "Verse 2" {
		t.Errorf("section = %q, want Verse 2", a.deck[3].Section)
	}
	if a.preview != 3 {
		t.Errorf("preview = %d, want 3", a.preview)
	}
	if a.stage != 1 || a.deck[a.stage].ID != onAir {
		t.Errorf("addPart moved the stage")
	}
}

func TestAddPartIgnoresUnknown(t *testing.T) {
	a := deckApp()
	if a.addPart("no-such-song", 0) {
		t.Error("accepted an unknown song")
	}
	if a.addPart("amazing-grace", 9) {
		t.Error("accepted an out-of-range part")
	}
	if a.addPart("amazing-grace", -1) {
		t.Error("accepted a negative part")
	}
	if len(a.deck) != 0 {
		t.Errorf("deck = %d, want 0", len(a.deck))
	}
}

func TestAddPartSkipsBlankSlides(t *testing.T) {
	a := deckApp()
	draft := song{Title: "Draft"}
	part := songPart{Label: "Chorus", Slides: []string{"", "line"}}
	if n := a.addPartSlides(draft, part); n != 1 {
		t.Fatalf("added = %d, want 1", n)
	}
	if len(a.deck) != 1 || a.deck[0].Section != "Chorus" {
		t.Fatalf("deck = %+v", a.deck)
	}
}
