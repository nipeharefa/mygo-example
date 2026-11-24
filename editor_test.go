package main

import (
	"os"
	"path/filepath"
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

// Opening the editor with nothing selected starts on the first song instead
// of an empty pane.
func TestRequestEditorSelectsTheFirstSong(t *testing.T) {
	a := editorApp(t)
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	a.requestEditor()
	if a.editSong != 0 || a.editPart != 0 || a.editSlide != 0 {
		t.Fatalf("selection = %d/%d/%d, want the first song's first slide", a.editSong, a.editPart, a.editSlide)
	}
	if a.editorTab != editorSlideTab {
		t.Errorf("editorTab = %d, want the Slide tab", a.editorTab)
	}
}

// Opening the editor puts the caret in the first slide, so the first key the
// operator presses is a lyric, not a hunt for the box.
func TestEditorOpensWithTheCaretInTheWords(t *testing.T) {
	a := editorApp(t)
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	a.requestEditor()
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Type("typed on open")
	if got := a.songs[0].Parts[0].Slides[0]; !strings.Contains(got, "typed on open") {
		t.Fatalf("slide = %q, want the words typed on open", got)
	}
}

// Picking a song also leaves the caret in its first slide.
func TestChoosingASongFocusesItsFirstSlide(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Holy, Holy, Holy"); err != nil {
		t.Fatal(err)
	}
	tt.Type("typed for Holy")
	if got := a.songs[1].Parts[0].Slides[0]; !strings.Contains(got, "typed for Holy") {
		t.Fatalf("slide = %q, want the words typed after choosing the song", got)
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

func TestEditorPreviewShowsTheSlideOnly(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	line := (slide{Text: a.songs[0].Parts[0].Slides[0]}).firstLine()
	if !tt.HasText(line) {
		t.Errorf("preview has no first line %q", line)
	}
	if tt.HasText("VERSE 1") {
		t.Errorf("preview still shows the part label: %q", tt.Texts())
	}
	if tt.HasText("Amazing Grace · Verse 1") {
		t.Errorf("preview still shows the caption: %q", tt.Texts())
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

func TestEditorTabsSeparateTheWordsFromTheDetails(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if !tt.HasText("Slide text input") {
		t.Fatalf("the Slide tab does not show the words: %q", tt.Texts())
	}
	if tt.HasText("Song title input") {
		t.Error("the Song fields show on the Slide tab")
	}
	if err := tt.Click("Song"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.editorTab != editorSongTab {
		t.Fatalf("editorTab = %d, want the Song tab", a.editorTab)
	}
	if !tt.HasText("Song title input") {
		t.Fatalf("the Song tab does not show the title: %q", tt.Texts())
	}
	if tt.HasText("Slide text input") {
		t.Error("the words show on the Song tab")
	}
	if err := tt.Click("Slide"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.editorTab != editorSlideTab || !tt.HasText("Slide text input") {
		t.Fatal("the Slide tab did not come back")
	}
}

// A slide click shows the Slide tab and takes the keyboard, so the next keys
// are the words, not another control.
func TestChoosingASlideFocusesItsWords(t *testing.T) {
	a := editorApp(t)
	a.editorTab = editorSongTab
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Verse 2 slide 1"); err != nil {
		t.Fatal(err)
	}
	tt.Type("typed")
	if got := a.songs[0].Parts[1].Slides[0]; !strings.Contains(got, "typed") {
		t.Fatalf("slide text = %q, want the typed words", got)
	}
	if a.editorTab != editorSlideTab {
		t.Errorf("editorTab = %d, want the Slide tab", a.editorTab)
	}
}

// The part header names a part; clicking it opens its first slide with the
// caret already in the words.
func TestChoosingAPartFocusesItsFirstSlide(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Verse 3"); err != nil {
		t.Fatal(err)
	}
	if a.editPart != 2 || a.editSlide != 0 {
		t.Fatalf("selection = part %d slide %d, want part 2 slide 0", a.editPart, a.editSlide)
	}
	tt.Type("typed")
	if got := a.songs[0].Parts[2].Slides[0]; !strings.Contains(got, "typed") {
		t.Fatalf("slide text = %q, want the typed words", got)
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
	if err := tt.Click("Song"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Song title input"); err != nil {
		t.Fatal(err)
	}
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
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
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

// Typing repaints the preview on every change, line breaks included, and the
// header says the words reached the file.
func TestTypingUpdatesThePreviewAndSaves(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides[0] = ""
	a.selectEditorSlide(0, 0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Slide text input"); err != nil {
		t.Fatal(err)
	}
	tt.Type("first line")
	tt.Key(0, ui.KeyEnter)
	tt.Type("second line")
	tt.Frame()
	if !tt.HasText("first line\nsecond line") {
		t.Fatalf("the preview does not show the typed words: %q", tt.Texts())
	}
	if a.editorStatus != "Saved" || a.editorStatusErr {
		t.Errorf("status = %q (error %v), want Saved", a.editorStatus, a.editorStatusErr)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Songs[0].Parts[0].Slides[0]; got != "first line\nsecond line" {
		t.Errorf("saved slide = %q, want both lines", got)
	}
}

// A failed write is shown, not hidden behind a "Saved" that is not true.
func TestFailedSaveIsShown(t *testing.T) {
	a := editorApp(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.settingsDir = blocked
	a.slideTextEdited()
	if !strings.Contains(a.editorStatus, "Could not save") {
		t.Fatalf("status = %q, want the save failure", a.editorStatus)
	}
	if !a.editorStatusErr {
		t.Error("the failed save is not marked as an error")
	}
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if !tt.HasText(a.editorStatus) {
		t.Errorf("the header does not show %q: %q", a.editorStatus, tt.Texts())
	}
}

// Switch slides to prove each one keeps its own text and caret, not the last
// box the focus left.
func TestEachSlideKeepsItsOwnWords(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Slide text input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" one")
	if err := tt.Click("Verse 2 slide 1"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" two")
	if !strings.Contains(a.songs[0].Parts[0].Slides[0], " one") {
		t.Errorf("slide 1 lost its words: %q", a.songs[0].Parts[0].Slides[0])
	}
	if !strings.Contains(a.songs[0].Parts[1].Slides[0], " two") {
		t.Errorf("slide 2 lost its words: %q", a.songs[0].Parts[1].Slides[0])
	}
}

// Add slide below inserts after the current slide, selects the new one, and
// leaves the caret there.
func TestAddSlideBelowInsertsAndFocuses(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides = []string{"a", "b"}
	a.selectEditorSlide(0, 0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Add slide below"); err != nil {
		t.Fatal(err)
	}
	if a.editSlide != 1 {
		t.Fatalf("editSlide = %d, want the new slide at 1", a.editSlide)
	}
	tt.Type("new")
	if got := strings.Join(a.songs[0].Parts[0].Slides, "|"); got != "a|new|b" {
		t.Fatalf("slides = %q, want a|new|b", got)
	}
}

func TestDuplicateSlideCopiesBelow(t *testing.T) {
	a := editorApp(t)
	a.selectEditorSlide(0, 0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Duplicate slide"); err != nil {
		t.Fatal(err)
	}
	if a.editSlide != 1 {
		t.Fatalf("editSlide = %d, want the copy", a.editSlide)
	}
	if a.songs[0].Parts[0].Slides[1] != a.songs[0].Parts[0].Slides[0] {
		t.Errorf("copy = %q, want the original", a.songs[0].Parts[0].Slides[1])
	}
}

// The text area's own undo puts the previous words back, and the library
// follows.
func TestUndoRestoresTheWords(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides[0] = ""
	a.selectEditorSlide(0, 0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Slide text input"); err != nil {
		t.Fatal(err)
	}
	tt.Type("typed")
	if got := a.songs[0].Parts[0].Slides[0]; got != "typed" {
		t.Fatalf("slide = %q, want typed", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if got := a.songs[0].Parts[0].Slides[0]; got != "" {
		t.Fatalf("slide after undo = %q, want the empty text", got)
	}
}

func TestSlideArrowsWalkTheSong(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides = []string{"a", "b", "c"}
	a.selectSong(0)
	a.selectEditorSlide(0, 0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Next slide"); err != nil {
		t.Fatal(err)
	}
	if a.editPart != 0 || a.editSlide != 1 {
		t.Fatalf("after next = part %d slide %d, want 0/1", a.editPart, a.editSlide)
	}
	if err := tt.Click("Previous slide"); err != nil {
		t.Fatal(err)
	}
	if a.editPart != 0 || a.editSlide != 0 {
		t.Fatalf("after previous = part %d slide %d, want 0/0", a.editPart, a.editSlide)
	}
	a.selectEditorSlide(0, 2)
	if err := tt.Click("Next slide"); err != nil {
		t.Fatal(err)
	}
	if a.editPart != 1 || a.editSlide != 0 {
		t.Fatalf("crossing = part %d slide %d, want the next part", a.editPart, a.editSlide)
	}
}

func splitCase(t *testing.T, in, want string) {
	t.Helper()
	if got := strings.Join(splitSlides(in), "|"); got != want {
		t.Errorf("splitSlides(%q) = %q, want %q", in, got, want)
	}
}

func TestSplitSlides(t *testing.T) {
	splitCase(t, "one\n\ntwo", "one|two")
	splitCase(t, "one\nline\n\ntwo", "one\nline|two")
	splitCase(t, "one\r\n\r\ntwo\r\n", "one|two")
	splitCase(t, "one\n\n\n\ntwo", "one|two")
	splitCase(t, " \n\n ", "")
	splitCase(t, "", "")
}

// A part that is still one empty slide is replaced by the paste; a part with
// words keeps them and gets the new slides after.
func TestPasteReplacesAnEmptySlide(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides = []string{""}
	a.selectEditorSlide(0, 0)
	a.pasteText = "one\nline\n\ntwo"
	a.applyPaste()
	if got := strings.Join(a.songs[0].Parts[0].Slides, "|"); got != "one\nline|two" {
		t.Fatalf("slides = %q, want one\\nline|two", got)
	}
	if a.editSlide != 0 {
		t.Errorf("editSlide = %d, want the first pasted slide", a.editSlide)
	}
	if a.editorStatus != "Added 2 slides" {
		t.Errorf("status = %q", a.editorStatus)
	}
}

func TestPasteAppendsToAFilledPart(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides = []string{"old"}
	a.selectEditorSlide(0, 0)
	a.pasteText = "one\n\ntwo"
	a.applyPaste()
	if got := strings.Join(a.songs[0].Parts[0].Slides, "|"); got != "old|one|two" {
		t.Fatalf("slides = %q, want old|one|two", got)
	}
	if a.editSlide != 1 {
		t.Errorf("editSlide = %d, want the first pasted slide", a.editSlide)
	}
}

func TestPasteDialogAddsSlides(t *testing.T) {
	a := editorApp(t)
	a.songs[0].Parts[0].Slides = []string{""}
	a.selectEditorSlide(0, 0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Paste words into this part"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.pasteOpen {
		t.Fatal("the paste button did not open the dialog")
	}
	if err := tt.Click("Pasted words"); err != nil {
		t.Fatal(err)
	}
	tt.Type("first\n\nsecond")
	if !tt.HasText("2 slides ready") {
		t.Errorf("the dialog does not count the blocks: %q", tt.Texts())
	}
	if err := tt.Click("Add slides"); err != nil {
		t.Fatal(err)
	}
	if a.pasteOpen {
		t.Error("the dialog stayed open")
	}
	if got := strings.Join(a.songs[0].Parts[0].Slides, "|"); got != "first|second" {
		t.Fatalf("slides = %q, want first|second", got)
	}
}

func TestAddPartAddsOneSlide(t *testing.T) {
	a := editorApp(t)
	before := len(a.songs[0].Parts)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Add part"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs[0].Parts) != before+1 {
		t.Fatalf("parts = %d, want %d", len(a.songs[0].Parts), before+1)
	}
	if a.editPart != before || a.editSlide != 0 {
		t.Errorf("selection = part %d slide %d, want the new part", a.editPart, a.editSlide)
	}
	if got := a.songs[0].Parts[before].Label; got != "Verse 4" {
		t.Errorf("new part = %q, want Verse 4", got)
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

// The smart part name follows the verses already there, and a song without
// verse numbers keeps the plain Part name.
func TestNextPartLabel(t *testing.T) {
	cases := []struct {
		parts []songPart
		want  string
	}{
		{[]songPart{{Label: "Verse 1"}}, "Verse 2"},
		{[]songPart{{Label: "Verse 1"}, {Label: "Chorus"}, {Label: "Verse 2"}}, "Verse 3"},
		{[]songPart{{Label: "Chorus"}}, "Part 2"},
		{[]songPart{{Label: "Intro"}, {Label: "Tag"}}, "Part 3"},
	}
	for _, c := range cases {
		if got := nextPartLabel(c.parts); got != c.want {
			t.Errorf("nextPartLabel(%v) = %q, want %q", c.parts, got, c.want)
		}
	}
}

func TestRemovePartKeepsLastPart(t *testing.T) {
	a := editorApp(t)
	a.selectSong(songIndex(a, "all-creatures"))
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Song"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Remove part"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs[a.editSong].Parts) != 1 {
		t.Fatalf("parts = %d, want the last part kept", len(a.songs[a.editSong].Parts))
	}
	if !strings.Contains(a.editorStatus, "Delete the song") {
		t.Errorf("status = %q, want the note to name the way out", a.editorStatus)
	}
	if !a.editorStatusErr {
		t.Error("the refusal is not marked as one")
	}
}

// The part's own name in the Slide header carries its removal, so a verse is
// deleted from the tab where its words are, without opening Song.
func TestRemovePartFromTheSlideTab(t *testing.T) {
	a := editorApp(t)
	a.selectSong(0)
	a.selectEditorSlide(2, 0)
	deck := len(a.deck)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Remove part"); err != nil {
		t.Fatal(err)
	}
	if got := len(a.songs[0].Parts); got != 2 {
		t.Fatalf("parts = %d, want the third part gone", got)
	}
	if a.editPart != 1 || a.editSlide != 0 {
		t.Errorf("selection = part %d slide %d, want the neighbour", a.editPart, a.editSlide)
	}
	if a.editorStatus != "Part removed" || a.editorStatusErr {
		t.Errorf("status = %q (error %v), want Part removed", a.editorStatus, a.editorStatusErr)
	}
	if len(a.deck) != deck {
		t.Errorf("deck = %d, want the run of show left at %d", len(a.deck), deck)
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(f.Songs[0].Parts); got != 2 {
		t.Errorf("saved parts = %d, want 2", got)
	}
	// The caret moves to the slide that took the part's place.
	tt.Type(" after")
	if got := a.songs[0].Parts[1].Slides[0]; !strings.Contains(got, " after") {
		t.Errorf("slide = %q, want typing to reach the neighbour", got)
	}
}

// A part added by mistake holds one empty slide, which is exactly the state
// where Remove slide can only refuse. The same header action clears it.
func TestRemoveEmptyPartFromTheSlideTab(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Add part"); err != nil {
		t.Fatal(err)
	}
	if a.editPart != 3 {
		t.Fatalf("editPart = %d, want the new part", a.editPart)
	}
	if !tt.HasText("Empty slide") {
		t.Fatalf("the new part does not show its empty slide: %q", tt.Texts())
	}
	if err := tt.Click("Remove part"); err != nil {
		t.Fatal(err)
	}
	if got := len(a.songs[0].Parts); got != 3 {
		t.Fatalf("parts = %d, want the empty part gone", got)
	}
	if a.editorStatus != "Part removed" {
		t.Errorf("status = %q, want Part removed", a.editorStatus)
	}
	if !a.validEditorSlide() {
		t.Error("no slide is selected after removing the empty part")
	}
}

// Restore samples is a quiet action at the foot of the library, and it still
// does what it says.
func TestRestoreSamplesAction(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Restore samples"); err != nil {
		t.Fatal(err)
	}
	if a.editorStatus != "Samples are already here" {
		t.Fatalf("status = %q, want the samples-already-here note", a.editorStatus)
	}
}

func TestRemoveSlideKeepsLastSlide(t *testing.T) {
	a := editorApp(t)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Remove slide"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs[0].Parts[0].Slides) != 1 {
		t.Fatalf("slides = %d, want the last slide kept", len(a.songs[0].Parts[0].Slides))
	}
	if !strings.Contains(a.editorStatus, "Remove part") {
		t.Errorf("status = %q, want the note to name Remove part", a.editorStatus)
	}
	if !a.editorStatusErr {
		t.Error("the refusal is not marked as one")
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
	create := func() string {
		a.newSongTitle = "New Song"
		a.newSongPart = "Verse 1"
		a.newSongWords = ""
		a.createSong()
		return a.songs[len(a.songs)-1].ID
	}
	first := create()
	second := create()
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

func TestCreateSongNeedsATitle(t *testing.T) {
	a := editorApp(t)
	before := len(a.songs)
	a.newSongTitle = "   "
	a.createSong()
	if len(a.songs) != before {
		t.Fatalf("songs = %d, want the blank title refused", len(a.songs))
	}
}

// The New song dialog collects a name and the words, so creating a song is a
// form rather than a row called "New Song" with the name field hidden below a
// fold.
func TestNewSongDialogCreatesTheSong(t *testing.T) {
	a := editorApp(t)
	before := len(a.songs)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("New song"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.newSongOpen {
		t.Fatal("the New song button did not open the dialog")
	}
	for label, text := range map[string]string{
		"New song title":  "Test Hymn",
		"New song author": "A Writer",
		"New song key":    "D",
		"New song words":  "first slide\n\nsecond slide\n\nthird slide",
	} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Type(text)
	}
	if !tt.HasText("This makes 3 slides") {
		t.Errorf("the dialog does not count the split slides: %q", tt.Texts())
	}
	if err := tt.Click("Create song"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs) != before+1 {
		t.Fatalf("songs = %d, want %d", len(a.songs), before+1)
	}
	s := a.songs[len(a.songs)-1]
	if s.Title != "Test Hymn" || s.Author != "A Writer" || s.Key != "D" {
		t.Errorf("new song = %+v", s)
	}
	if len(s.Parts) != 1 || s.Parts[0].Label != "Verse 1" || len(s.Parts[0].Slides) != 3 {
		t.Fatalf("new song parts = %+v", s.Parts)
	}
	if s.Parts[0].Slides[1] != "second slide" {
		t.Errorf("second slide = %q", s.Parts[0].Slides[1])
	}
	if a.editSong != len(a.songs)-1 || a.editPart != 0 || a.editSlide != 0 {
		t.Errorf("selection = %d/%d/%d, want the new song's first slide", a.editSong, a.editPart, a.editSlide)
	}
	if a.newSongOpen {
		t.Error("the dialog stayed open")
	}
	f, err := readLibrary(a.libraryPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findSong(f.Songs, s.ID); !ok {
		t.Error("the new song is not in the saved library")
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

// Deleting a song from the library is permanent, so the Song tab asks first.
func TestRemoveSongAsksBeforeDeleting(t *testing.T) {
	a := editorApp(t)
	before := len(a.songs)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	if err := tt.Click("Song"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Remove song"); err != nil {
		t.Fatal(err)
	}
	if !a.confirmRemoveSong {
		t.Fatal("Remove song deleted without asking")
	}
	tt.Frame()
	if !tt.HasText("Keep song") || !tt.HasText("Delete song") {
		t.Fatalf("the confirmation is missing: %q", tt.Texts())
	}
	if len(a.songs) != before {
		t.Fatal("the song was removed before the confirmation")
	}
	if err := tt.Click("Keep song"); err != nil {
		t.Fatal(err)
	}
	if a.confirmRemoveSong || len(a.songs) != before {
		t.Fatal("Keep song did not keep it")
	}
	if err := tt.Click("Remove song"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Delete song"); err != nil {
		t.Fatal(err)
	}
	if len(a.songs) != before-1 {
		t.Fatalf("songs = %d, want %d", len(a.songs), before-1)
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
