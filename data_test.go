package main

import (
	"strings"
	"testing"
)

// Every built-in is authorable in the editor: at least one labeled part, and
// every part holds at least one non-blank slide.
func TestBuiltinSongsHaveParts(t *testing.T) {
	if len(builtinSongs) < 4 {
		t.Fatalf("builtinSongs = %d, want at least 4", len(builtinSongs))
	}
	seen := map[string]bool{}
	slides := 0
	for _, s := range builtinSongs {
		if s.ID == "" || s.Title == "" {
			t.Errorf("song missing id or title: %+v", s)
		}
		if seen[s.ID] {
			t.Errorf("duplicate song id %q", s.ID)
		}
		seen[s.ID] = true
		if len(s.Parts) == 0 {
			t.Errorf("song %q has no parts", s.ID)
		}
		for _, part := range s.Parts {
			if part.Label == "" {
				t.Errorf("song %q has a part without a label", s.ID)
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
}

// A long verse is one labeled part spanning several slides, which is the
// reason parts own slides at all.
func TestAllCreaturesVerseOneHasTwoSlides(t *testing.T) {
	s, ok := findSong(builtinSongs, "all-creatures")
	if !ok {
		t.Fatal("all-creatures not found")
	}
	if len(s.Parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(s.Parts))
	}
	part := s.Parts[0]
	if part.Label != "Verse 1" {
		t.Errorf("label = %q, want Verse 1", part.Label)
	}
	if len(part.Slides) != 2 {
		t.Fatalf("slides = %d, want 2", len(part.Slides))
	}
	if got := strings.Count(strings.Join(part.Slides, "\n"), "\n"); got != 6 {
		t.Errorf("lines = %d, want 7", got+1)
	}
	if !strings.Contains(part.Slides[1], "Alleluia! Alleluia! Alleluia!") {
		t.Errorf("second slide lost the last line: %q", part.Slides[1])
	}
}

func TestSongTextSizeDefaultsAndClamps(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0, 40},
		{10, 24},
		{99, 72},
		{55, 55},
	} {
		if got := (song{TextSize: tc.in}).textSize(); got != tc.want {
			t.Errorf("textSize(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSongSlideCount(t *testing.T) {
	s := song{Parts: []songPart{
		{Label: "Verse 1", Slides: []string{"a", "b"}},
		{Label: "Chorus", Slides: []string{"c"}},
	}}
	if got := s.slideCount(); got != 3 {
		t.Errorf("slideCount = %d, want 3", got)
	}
}

func TestFindSong(t *testing.T) {
	if _, ok := findSong(builtinSongs, "amazing-grace"); !ok {
		t.Error("findSong rejected a built-in id")
	}
	if _, ok := findSong(builtinSongs, "no-such-song"); ok {
		t.Error("findSong accepted an unknown id")
	}
}
