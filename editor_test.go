package main

import (
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
	tt.Click("Edit Amazing Grace")
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

func TestEditorPreviewWidthClamped(t *testing.T) {
	a := newApp()
	a.selectSong(0)
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Frame()
	if got := previewWidth(editorWidth); got != editorPreviewMax {
		t.Errorf("preview at %d = %v, want %v", editorWidth, got, editorPreviewMax)
	}
	if got := previewWidth(900); got < 320 {
		t.Errorf("preview at 900 = %v, want at least 320", got)
	}
}

func TestEditorSearchKeepsSelectionStable(t *testing.T) {
	a := newApp()
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	tt := ui.NewTester(a.editorView, editorWidth, editorHeight)
	tt.Click("Search songs")
	tt.Type("holy")
	tt.Click("Edit Holy, Holy, Holy")
	if a.editSong != 1 {
		t.Fatalf("editSong = %d, want the real index 1", a.editSong)
	}
}
