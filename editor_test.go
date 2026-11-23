package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestRequestEditorSetsOpenFlag(t *testing.T) {
	a := &app{}
	a.requestEditor()
	if !a.openEditor {
		t.Error("requestEditor did not set openEditor")
	}
	if a.editor != nil {
		t.Error("requestEditor created a window by itself")
	}
}

func TestSettleWithoutWindowDoesNotOpen(t *testing.T) {
	a := &app{openEditor: true}
	a.settle()
	if a.editor != nil {
		t.Error("settle opened an editor without a console window")
	}
	if a.openEditor {
		t.Error("settle left the editor request pending")
	}
}

func TestCloseEditorNilSafe(t *testing.T) {
	a := &app{}
	a.closeEditor()
}

func TestChangedKeepsEditorNilSafe(t *testing.T) {
	a := &app{}
	a.changed()
}

func TestSignOutClosesEditorState(t *testing.T) {
	a := &app{signedIn: true, openEditor: true}
	a.signOut()
	if a.openEditor {
		t.Error("sign-out left the editor request pending")
	}
	if a.editor != nil {
		t.Error("sign-out left the editor pointer set")
	}
}

func TestConsoleSongEditorButton(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)
	signIn(t, a, tt)
	tt.Click("Song editor")
	if a.editor != nil {
		t.Error("a headless test opened an editor window")
	}
	if !a.signedIn {
		t.Error("the editor button signed the operator out")
	}
}

func TestEditorEmptyLibrary(t *testing.T) {
	a := newApp()
	a.songs = nil
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if !tt.HasText("No songs yet") {
		t.Errorf("empty editor does not say No songs yet: %q", tt.Texts())
	}
	if !tt.HasText("New song") {
		t.Error("empty editor has no New song button")
	}
}

func TestEditorSelectsSong(t *testing.T) {
	a := newApp()
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Amazing Grace")
	if a.editSong != 0 {
		t.Fatalf("editSong = %d, want 0", a.editSong)
	}
	line := (slide{Text: a.songs[0].Parts[0].Slides[0]}).firstLine()
	if !tt.HasText(line) {
		t.Fatalf("preview does not show %q: %q", line, tt.Texts())
	}
}

func TestEditorPreviewShowsSlideParts(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	line := (slide{Text: a.songs[0].Parts[0].Slides[0]}).firstLine()
	if !tt.HasText("VERSE 1") {
		t.Errorf("preview has no eyebrow: %q", tt.Texts())
	}
	if !tt.HasText(line) {
		t.Errorf("preview has no first line %q", line)
	}
	if !tt.HasText("Amazing Grace · Verse 1") {
		t.Errorf("preview has no caption: %q", tt.Texts())
	}
}

func TestEditorPartListSelectsPart(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Verse 2")
	if a.editPart != 1 || a.editSlide != 0 {
		t.Fatalf("selection = part %d slide %d, want part 1 slide 0", a.editPart, a.editSlide)
	}
}

func TestEditorSlideListSelectsSlide(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Verse 3 slide 1")
	if a.editPart != 2 || a.editSlide != 0 {
		t.Fatalf("selection = part %d slide %d, want part 2 slide 0", a.editPart, a.editSlide)
	}
}

func TestEditorPartSectionFolds(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Part"); err != nil {
		t.Fatal(err)
	}
	if a.editorShowPart {
		t.Fatal("clicking Part did not fold the section")
	}
	if tt.HasText("Add part to run of show") {
		t.Error("the folded Part section still shows its controls")
	}
	if err := tt.Click("Part"); err != nil {
		t.Fatal(err)
	}
	if !a.editorShowPart {
		t.Fatal("clicking Part again did not unfold the section")
	}
}

func TestEditorDetailsSectionFolds(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Song details"); err != nil {
		t.Fatal(err)
	}
	if a.editorShowDetails {
		t.Fatal("clicking Song details did not fold the section")
	}
	if tt.HasText("Text size") {
		t.Error("the folded Song details section still shows its controls")
	}
	if err := tt.Click("Song details"); err != nil {
		t.Fatal(err)
	}
	if !a.editorShowDetails {
		t.Fatal("clicking Song details again did not unfold the section")
	}
}

func TestEditorPreviewWidthClamped(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Frame()
	if got := previewWidth(editorWidth); got != editorPreviewMax {
		t.Errorf("preview at %d = %v, want the cap %v", editorWidth, got, editorPreviewMax)
	}
	if got := previewWidth(900); got != editorPreviewMin {
		t.Errorf("preview at 900 = %v, want the floor %v", got, editorPreviewMin)
	}
	if got := previewWidth(2000); got != editorPreviewMax {
		t.Errorf("preview at 2000 = %v, want the cap %v", got, editorPreviewMax)
	}
}

func TestEditorPreviewKeepsTheConsoleTheme(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	leaked := false
	tt := ui.NewTester(func(c *ui.Context) {
		a.editorView(c)
		if got, want := c.Theme().Background, consoleTheme(c).Background; got != want {
			leaked = true
		}
	}, editorWidth, editorHeight)
	tt.Frame()
	if leaked {
		t.Fatal("editorView changed the window theme; the preview must pass stage colors explicitly")
	}
}

func TestEditorSearchKeepsSelectionStable(t *testing.T) {
	a := newApp()
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Search songs")
	tt.Type("holy")
	tt.Click("Holy, Holy, Holy")
	if a.editSong != 1 {
		t.Fatalf("editSong = %d, want the real index 1", a.editSong)
	}
}

// songIndex is where a song sits in the library, or -1.
func songIndex(a *app, id string) int {
	for i := range a.songs {
		if a.songs[i].ID == id {
			return i
		}
	}
	return -1
}

// editorApp is an app with a saved library in a temp dir and the first song
// selected, the starting point for the editing tests.
func editorApp(t *testing.T) *app {
	t.Helper()
	a := newApp()
	a.settingsDir = t.TempDir()
	a.loadLibrary()
	a.selectSong(0)
	return a
}

func TestEditingTitleSavesLibrary(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Song title input")
	tt.Type(" Grace")
	want := a.songs[0].Title
	if !strings.Contains(want, "Grace") {
		t.Fatalf("title = %q, want the typed text", want)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if f.Songs[0].Title != want {
		t.Errorf("saved title = %q, want %q", f.Songs[0].Title, want)
	}
}

func TestEditingSlideTextSavesLibrary(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, 1600)
	tt.Click("Slide text input")
	tt.Type("new line")
	want := a.songs[0].Parts[0].Slides[0]
	if !strings.Contains(want, "new line") {
		t.Fatalf("slide text = %q, want the typed text", want)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if f.Songs[0].Parts[0].Slides[0] != want {
		t.Errorf("saved slide = %q, want %q", f.Songs[0].Parts[0].Slides[0], want)
	}
}

func TestAddPartAddsOneSlide(t *testing.T) {
	a := editorApp(t)
	before := len(a.songs[0].Parts)
	tt := ui.NewTester(a.editorView, editorWidth, 1600)
	tt.Click("Add part")
	if len(a.songs[0].Parts) != before+1 {
		t.Fatalf("parts = %d, want %d", len(a.songs[0].Parts), before+1)
	}
	if a.editPart != before || a.editSlide != 0 {
		t.Errorf("selection = part %d slide %d, want the new part", a.editPart, a.editSlide)
	}
	if len(a.songs[0].Parts[before].Slides) != 1 {
		t.Errorf("new part has %d slides, want 1", len(a.songs[0].Parts[before].Slides))
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Songs[0].Parts) != before+1 {
		t.Errorf("saved parts = %d, want %d", len(f.Songs[0].Parts), before+1)
	}
}

func TestRemovePartKeepsLastPart(t *testing.T) {
	a := editorApp(t)
	a.selectSong(songIndex(a, "all-creatures"))
	tt := ui.NewTester(a.editorView, editorWidth, 1600)
	tt.Click("Remove part")
	if len(a.songs[a.editSong].Parts) != 1 {
		t.Fatalf("parts = %d, want the last part kept", len(a.songs[a.editSong].Parts))
	}
	if !strings.Contains(strings.ToLower(a.editorStatus), "part") {
		t.Errorf("status = %q, want a note about needing a part", a.editorStatus)
	}
}

func TestRemoveSlideKeepsLastSlide(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Remove slide")
	if len(a.songs[0].Parts[0].Slides) != 1 {
		t.Fatalf("slides = %d, want the last slide kept", len(a.songs[0].Parts[0].Slides))
	}
	if !strings.Contains(strings.ToLower(a.editorStatus), "slide") {
		t.Errorf("status = %q, want a note about needing a slide", a.editorStatus)
	}
}

func TestSetTextSizeClampsAndSaves(t *testing.T) {
	a := editorApp(t)
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if f.Songs[0].TextSize != defaultTextSize {
		t.Errorf("seeded size = %v, want %v", f.Songs[0].TextSize, defaultTextSize)
	}
	a.setTextSize(99)
	if got := a.songs[0].TextSize; got != maxTextSize {
		t.Errorf("size 99 = %v, want %v", got, maxTextSize)
	}
	a.setTextSize(10)
	if got := a.songs[0].TextSize; got != minTextSize {
		t.Errorf("size 10 = %v, want %v", got, minTextSize)
	}
	f, err = readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if f.Songs[0].TextSize != minTextSize {
		t.Errorf("saved size = %v, want %v", f.Songs[0].TextSize, minTextSize)
	}
}

func TestNewSongIDIsUnique(t *testing.T) {
	a := editorApp(t)
	a.newSong()
	first := a.songs[len(a.songs)-1].ID
	a.newSong()
	second := a.songs[len(a.songs)-1].ID
	if first == second {
		t.Fatalf("both new songs got id %q", first)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findSong(f.Songs, first); !ok {
		t.Error("the first new song is not in the file")
	}
	if _, ok := findSong(f.Songs, second); !ok {
		t.Error("the second new song is not in the file")
	}
}

func TestRestoreSamplesOnlyAddsMissing(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Title = "My Grace"
	a.selectSong(0)
	a.songEdited()
	a.selectSong(1)
	gone := a.songs[1].ID
	a.removeSong()

	a.restoreSamples()
	if len(a.songs) != len(builtinSongs) {
		t.Fatalf("songs = %d, want %d", len(a.songs), len(builtinSongs))
	}
	if _, ok := findSong(a.songs, gone); !ok {
		t.Errorf("restore did not bring back %q", gone)
	}
	if got := a.songs[0].Title; got != "My Grace" {
		t.Errorf("restore overwrote the edited title with %q", got)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Songs) != len(builtinSongs) {
		t.Errorf("saved songs = %d, want %d", len(f.Songs), len(builtinSongs))
	}
}

func TestRemoveSong(t *testing.T) {
	a := editorApp(t)
	gone := a.songs[0].ID
	a.removeSong()
	if len(a.songs) != len(builtinSongs)-1 {
		t.Fatalf("songs = %d, want one fewer", len(a.songs))
	}
	if _, ok := findSong(a.songs, gone); ok {
		t.Errorf("song %q is still in the model", gone)
	}
	if a.editSong != 0 {
		t.Errorf("editSong = %d, want 0", a.editSong)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findSong(f.Songs, gone); ok {
		t.Errorf("song %q is still in the file", gone)
	}
}

func TestAddSongToRunOfShow(t *testing.T) {
	a := editorApp(t)
	before := len(a.deck)
	tt := ui.NewTester(a.editorView, editorWidth, 1600)
	tt.Click("Add song to run of show")
	if len(a.deck) != before+3 {
		t.Fatalf("deck = %d, want %d", len(a.deck), before+3)
	}
	if a.preview != before {
		t.Errorf("preview = %d, want the first added slide %d", a.preview, before)
	}
	if a.editorStatus != "Added 3 slides" {
		t.Errorf("status = %q, want Added 3 slides", a.editorStatus)
	}
}

func TestAddSongWithNoTextDoesNothing(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts = []songPart{{Label: "Verse 1", Slides: []string{"", "  "}}}
	a.selectSong(0)
	before := len(a.deck)
	a.addEditorSongToDeck()
	if len(a.deck) != before {
		t.Fatalf("deck = %d, want the blank song to add nothing", len(a.deck))
	}
	if !strings.Contains(strings.ToLower(a.editorStatus), "no text") {
		t.Errorf("status = %q, want a note about missing text", a.editorStatus)
	}
}

func TestAddPartToRunOfShow(t *testing.T) {
	a := editorApp(t)
	stage := a.stage
	a.selectSong(songIndex(a, "amazing-grace"))
	a.selectPart(1)
	before := len(a.deck)
	a.addEditorPartToDeck()
	if len(a.deck) != before+1 {
		t.Fatalf("deck = %d, want one slide added", len(a.deck))
	}
	if got := a.deck[len(a.deck)-1].Section; got != "Verse 2" {
		t.Errorf("section = %q, want Verse 2", got)
	}
	if a.stage != stage {
		t.Errorf("stage = %d, want it untouched at %d", a.stage, stage)
	}
}

func TestEditingSongLeavesDeckUntouched(t *testing.T) {
	a := editorApp(t)
	a.selectSong(songIndex(a, "amazing-grace"))
	a.addEditorSongToDeck()
	added := a.deck[len(a.deck)-3]
	oldText, oldSize := added.Text, added.TextSize
	a.songs[a.editSong].Parts[0].Slides[0] = "rewritten"
	a.setTextSize(60)
	if got := a.deck[len(a.deck)-3].Text; got != oldText {
		t.Errorf("deck text = %q, want the snapshot %q", got, oldText)
	}
	if got := a.deck[len(a.deck)-3].TextSize; got != oldSize {
		t.Errorf("deck size = %v, want the snapshot %v", got, oldSize)
	}
}
