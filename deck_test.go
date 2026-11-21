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
