package main

// The song and scripture library. Every text here is public domain: the
// hymns are traditional and the verses are King James Version (1611).
// A slide's text carries one line per line break so a slide can render it
// as written.

const (
	// defaultTextSize is the body size at the 1280x720 design, in stage
	// pixels. A song may choose its own, and every canvas scales it by
	// width/1280.
	defaultTextSize = 40.0
	minTextSize     = 24.0
	maxTextSize     = 72.0
)

// songPart is one labeled part of a song. It spans one or more slides, so a
// verse too long for one screen keeps a single name on every screen it
// covers.
type songPart struct {
	Label  string   `json:"label"`
	Slides []string `json:"slides"`
}

type song struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Author   string     `json:"author"`
	Key      string     `json:"key"`
	TextSize float64    `json:"textSize"` // stage px at 1280x720
	Parts    []songPart `json:"parts"`
}

type verse struct {
	Ref  string
	Text string
}

type versePack struct {
	Name   string
	Verses []verse
}

// textSize is the size the output should draw: a zero value from an older
// file means the default, and anything out of range is pulled back in.
func (s song) textSize() float64 {
	return clampTextSize(s.TextSize)
}

// clampTextSize makes any stored size usable by the slider and the renderer.
func clampTextSize(size float64) float64 {
	if size <= 0 {
		return defaultTextSize
	}
	if size < minTextSize {
		return minTextSize
	}
	if size > maxTextSize {
		return maxTextSize
	}
	return size
}

func (s song) slideCount() int {
	n := 0
	for _, part := range s.Parts {
		n += len(part.Slides)
	}
	return n
}

var builtinSongs = []song{
	{
		ID:     "amazing-grace",
		Title:  "Amazing Grace",
		Author: "John Newton",
		Key:    "G",
		Parts: []songPart{
			{
				Label: "Verse 1",
				Slides: []string{
					"Amazing grace! how sweet the sound\n" +
						"That saved a wretch like me!\n" +
						"I once was lost, but now am found;\n" +
						"Was blind, but now I see.",
				},
			},
			{
				Label: "Verse 2",
				Slides: []string{
					"'Twas grace that taught my heart to fear,\n" +
						"And grace my fears relieved;\n" +
						"How precious did that grace appear\n" +
						"The hour I first believed!",
				},
			},
			{
				Label: "Verse 3",
				Slides: []string{
					"Through many dangers, toils, and snares,\n" +
						"I have already come;\n" +
						"'Tis grace hath brought me safe thus far,\n" +
						"And grace will lead me home.",
				},
			},
		},
	},
	{
		ID:     "holy-holy-holy",
		Title:  "Holy, Holy, Holy",
		Author: "Reginald Heber",
		Key:    "D",
		Parts: []songPart{
			{
				Label: "Verse 1",
				Slides: []string{
					"Holy, holy, holy! Lord God Almighty!\n" +
						"Early in the morning our song shall rise to Thee;\n" +
						"Holy, holy, holy! Merciful and mighty!\n" +
						"God in three Persons, blessed Trinity!",
				},
			},
			{
				Label: "Verse 2",
				Slides: []string{
					"Holy, holy, holy! All the saints adore Thee,\n" +
						"Casting down their golden crowns around the glassy sea;\n" +
						"Cherubim and seraphim falling down before Thee,\n" +
						"Which wert, and art, and evermore shalt be.",
				},
			},
		},
	},
	{
		ID:     "be-thou-my-vision",
		Title:  "Be Thou My Vision",
		Author: "Attr. Dallán Forgaill, trans. Mary Byrne",
		Key:    "Eb",
		Parts: []songPart{
			{
				Label: "Verse 1",
				Slides: []string{
					"Be Thou my Vision, O Lord of my heart;\n" +
						"Naught be all else to me, save that Thou art;\n" +
						"Thou my best thought, by day or by night,\n" +
						"Waking or sleeping, Thy presence my light.",
				},
			},
			{
				Label: "Verse 2",
				Slides: []string{
					"Be Thou my Wisdom, and Thou my true Word;\n" +
						"I ever with Thee and Thou with me, Lord;\n" +
						"Thou my great Father, I Thy true son;\n" +
						"Thou in me dwelling, and I with Thee one.",
				},
			},
		},
	},
	{
		ID:     "all-creatures",
		Title:  "All Creatures of Our God and King",
		Author: "St. Francis of Assisi, trans. William H. Draper",
		Key:    "D",
		Parts: []songPart{
			{
				// One part over two slides: the label stays Verse 1 on both.
				Label: "Verse 1",
				Slides: []string{
					"All creatures of our God and King,\n" +
						"Lift up your voice and with us sing,\n" +
						"Alleluia! Alleluia!\n" +
						"Thou burning sun with golden beam,",
					"Thou silver moon with softer gleam,\n" +
						"O praise Him, O praise Him!\n" +
						"Alleluia! Alleluia! Alleluia!",
				},
			},
		},
	},
	{
		ID:     "mighty-fortress",
		Title:  "A Mighty Fortress Is Our God",
		Author: "Martin Luther, trans. Frederic H. Hedge",
		Key:    "C",
		Parts: []songPart{
			{
				Label: "Verse 1",
				Slides: []string{
					"A mighty fortress is our God,\n" +
						"A bulwark never failing;\n" +
						"Our helper He, amid the flood\n" +
						"Of mortal ills prevailing.\n" +
						"For still our ancient foe\n" +
						"Doth seek to work us woe;\n" +
						"His craft and power are great,\n" +
						"And armed with cruel hate,\n" +
						"On earth is not his equal.",
				},
			},
			{
				Label: "Verse 2",
				Slides: []string{
					"Did we in our own strength confide,\n" +
						"Our striving would be losing;\n" +
						"Were not the right Man on our side,\n" +
						"The Man of God's own choosing.\n" +
						"Dost ask who that may be?\n" +
						"Christ Jesus, it is He;\n" +
						"Lord Sabaoth His name,\n" +
						"From age to age the same,\n" +
						"And He must win the battle.",
				},
			},
		},
	},
}

var packs = []versePack{
	{
		Name: "Assurance",
		Verses: []verse{
			{Ref: "Psalm 23:1", Text: "The LORD is my shepherd; I shall not want."},
			{Ref: "John 3:16", Text: "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life."},
			{Ref: "Romans 8:28", Text: "And we know that all things work together for good to them that love God, to them who are the called according to his purpose."},
			{Ref: "Philippians 4:13", Text: "I can do all things through Christ which strengtheneth me."},
		},
	},
	{
		Name: "Comfort",
		Verses: []verse{
			{Ref: "Isaiah 41:10", Text: "Fear thou not; for I am with thee: be not dismayed; for I am thy God: I will strengthen thee; yea, I will help thee; yea, I will uphold thee with the right hand of my righteousness."},
			{Ref: "Matthew 11:28", Text: "Come unto me, all ye that labour and are heavy laden, and I will give you rest."},
			{Ref: "Psalm 34:18", Text: "The LORD is nigh unto them that are of a broken heart; and saveth such as be of a contrite spirit."},
			{Ref: "Psalm 46:1", Text: "God is our refuge and strength, a very present help in trouble."},
		},
	},
	{
		Name: "Praise",
		Verses: []verse{
			{Ref: "Psalm 100:4", Text: "Enter into his gates with thanksgiving, and into his courts with praise: be thankful unto him, and bless his name."},
			{Ref: "Psalm 150:6", Text: "Let every thing that hath breath praise the LORD. Praise ye the LORD."},
			{Ref: "Hebrews 13:15", Text: "By him therefore let us offer the sacrifice of praise to God continually, that is, the fruit of our lips giving thanks to his name."},
			{Ref: "Psalm 103:1", Text: "Bless the LORD, O my soul: and all that is within me, bless his holy name."},
		},
	},
	{
		Name: "Sending",
		Verses: []verse{
			{Ref: "Matthew 28:19", Text: "Go ye therefore, and teach all nations, baptizing them in the name of the Father, and of the Son, and of the Holy Ghost."},
			{Ref: "Numbers 6:24", Text: "The LORD bless thee, and keep thee."},
			{Ref: "2 Corinthians 13:14", Text: "The grace of the Lord Jesus Christ, and the love of God, and the communion of the Holy Ghost, be with you all. Amen."},
			{Ref: "Isaiah 6:8", Text: "Also I heard the voice of the Lord, saying, Whom shall I send, and who will go for us? Then said I, Here am I; send me."},
		},
	},
}

// findSong looks a song up in any library, not just the built-ins: the
// editor works on the active one.
func findSong(list []song, id string) (song, bool) {
	for _, s := range list {
		if s.ID == id {
			return s, true
		}
	}
	return song{}, false
}

func verseByRef(ref string) (verse, bool) {
	for _, p := range packs {
		for _, v := range p.Verses {
			if v.Ref == ref {
				return v, true
			}
		}
	}
	return verse{}, false
}
