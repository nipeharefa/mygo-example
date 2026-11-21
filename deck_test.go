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
	if a.index != 0 {
		t.Fatalf("index = %d, want 0", a.index)
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
}

func TestNextPrevClamp(t *testing.T) {
	a := &app{}
	a.addSong("amazing-grace")
	a.prevSlide()
	if a.index != 0 {
		t.Fatalf("prev at 0 moved to %d", a.index)
	}
	a.nextSlide()
	if a.index != 1 {
		t.Fatalf("next moved to %d, want 1", a.index)
	}
	a.selectSlide(2)
	a.nextSlide()
	if a.index != 2 {
		t.Fatalf("next past the end moved to %d", a.index)
	}
}

func TestSelectSlideClamps(t *testing.T) {
	a := &app{}
	a.addSong("amazing-grace")
	a.selectSlide(1)
	if a.index != 1 {
		t.Fatalf("index = %d, want 1", a.index)
	}
	a.selectSlide(99)
	a.selectSlide(-1)
	if a.index != 1 {
		t.Fatalf("out-of-range select changed index to %d", a.index)
	}
}

func TestRemoveSlideClampsIndex(t *testing.T) {
	a := &app{}
	a.addSong("amazing-grace")
	a.selectSlide(2)
	a.removeSlide()
	if len(a.deck) != 2 || a.index != 1 {
		t.Fatalf("deck = %d index = %d, want 2/1", len(a.deck), a.index)
	}
	a.removeSlide()
	a.removeSlide()
	if len(a.deck) != 0 || a.index != 0 {
		t.Fatalf("deck = %d index = %d, want 0/0", len(a.deck), a.index)
	}
	a.removeSlide()
	if a.index != 0 {
		t.Fatalf("remove on empty moved index to %d", a.index)
	}
}

func TestMoveSlideReorders(t *testing.T) {
	a := &app{}
	a.addSong("amazing-grace")
	first, second := a.deck[0].ID, a.deck[1].ID
	a.selectSlide(1)
	a.moveSlide(-1)
	if a.index != 0 {
		t.Fatalf("index = %d, want 0", a.index)
	}
	if a.deck[0].ID != second || a.deck[1].ID != first {
		t.Fatalf("order not swapped: %q %q", a.deck[0].ID, a.deck[1].ID)
	}
	a.moveSlide(-1)
	if a.deck[0].ID != second {
		t.Fatal("move up past the top reordered the deck")
	}
	a.selectSlide(2)
	a.moveSlide(1)
	if a.index != 2 {
		t.Fatalf("move down past the end moved index to %d", a.index)
	}
}

func TestToggleBlackout(t *testing.T) {
	a := &app{}
	if _, ok := a.currentSlide(); ok {
		t.Fatal("empty deck reported a current slide")
	}
	a.toggleBlackout()
	if !a.blackout {
		t.Fatal("blackout not enabled")
	}
	a.toggleBlackout()
	if a.blackout {
		t.Fatal("blackout not disabled")
	}
}

func TestCurrentAndUpcomingSlide(t *testing.T) {
	a := &app{}
	a.addSong("amazing-grace")
	cur, ok := a.currentSlide()
	if !ok || cur.Section != "Verse 1" {
		t.Fatalf("current = %+v ok = %v", cur, ok)
	}
	up, ok := a.upcomingSlide()
	if !ok || up.Section != "Verse 2" {
		t.Fatalf("upcoming = %+v ok = %v", up, ok)
	}
	a.selectSlide(2)
	if _, ok := a.upcomingSlide(); ok {
		t.Fatal("last slide reported an upcoming slide")
	}
}

func TestChangedWithoutWindows(t *testing.T) {
	a := &app{}
	a.changed()
}
