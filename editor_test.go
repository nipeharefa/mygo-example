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
