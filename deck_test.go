package main

import "testing"

func TestDataIsReachable(t *testing.T) {
	if len(songs) < 4 {
		t.Fatalf("songs = %d, want at least 4", len(songs))
	}
	seenSongs := map[string]bool{}
	sections := 0
	for _, s := range songs {
		if s.ID == "" || s.Title == "" {
			t.Errorf("song missing id or title: %+v", s)
		}
		if seenSongs[s.ID] {
			t.Errorf("duplicate song id %q", s.ID)
		}
		seenSongs[s.ID] = true
		if len(s.Sections) == 0 {
			t.Errorf("song %q has no sections", s.ID)
		}
		for _, sec := range s.Sections {
			if sec.Label == "" || sec.Text == "" {
				t.Errorf("song %q has an empty section", s.ID)
			}
			sections++
		}
	}
	if sections < 8 {
		t.Fatalf("sections = %d, want at least 8", sections)
	}
	if _, ok := songByID(songs[0].ID); !ok {
		t.Errorf("songByID(%q) not found", songs[0].ID)
	}
	if _, ok := songByID("no-such-song"); ok {
		t.Error("songByID accepted an unknown id")
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
	a.addSong("no-such-song")
	if len(a.deck) != 0 {
		t.Fatalf("deck = %d, want empty", len(a.deck))
	}
}

func TestAddVerseAppendsOneSlide(t *testing.T) {
	a := &app{}
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
	if s.eyebrow() != "SCRIPTURE" {
		t.Errorf("eyebrow = %q", s.eyebrow())
	}
	if a.preview != 0 {
		t.Errorf("preview = %d, want the new slide", a.preview)
	}
}

func TestMonitorNavigationClamps(t *testing.T) {
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
	a.takeSlide()
	if a.stage != 0 || a.preview != 0 {
		t.Fatalf("empty take moved to stage=%d preview=%d", a.stage, a.preview)
	}
}

func TestNextStagePullsTheMonitor(t *testing.T) {
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
	a.addSong("amazing-grace")
	a.selectSlide(2)
	a.takeSlide()
	a.removeSlide()
	if len(a.deck) != 2 || a.stage != 1 {
		t.Fatalf("deck = %d stage = %d, want 2/1", len(a.deck), a.stage)
	}
}

func TestRemoveAboveStageShiftsStage(t *testing.T) {
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
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
	a := &app{}
	a.changed()
}
