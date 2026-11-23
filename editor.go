package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	editorWidth      = 1200
	editorHeight     = 800
	editorMinWidth   = 980
	editorMinHeight  = 660
	editorListWidth  = 260
	editorPadding    = 20
	editorPreviewMax = 720
)

// requestEditor asks for the editor window; the next frame opens it, so no
// window work happens inside the click that asked.
func (a *app) requestEditor() {
	a.openEditor = true
	a.changed()
}

// openEditorWindow creates the editor once and follows openHelp: a window
// that already exists is brought forward, never replaced.
func (a *app) openEditorWindow() {
	if a.editor != nil {
		a.editor.Focus()
		return
	}
	w := a.newWindow(mygo.WindowOptions{
		Title:     appTitle + " — Song editor",
		Width:     editorWidth,
		Height:    editorHeight,
		MinWidth:  editorMinWidth,
		MinHeight: editorMinHeight,
		StateKey:  "editor",
		Content:   ui.View(a.editorView),
	})
	a.editor = w
	w.OnClosed(func() {
		if a.editor != w {
			return
		}
		a.editor = nil
	})
}

// closeEditor drops the pointer before closing, so a close callback cannot
// clear a newer window.
func (a *app) closeEditor() {
	w := a.editor
	if w == nil {
		return
	}
	a.editor = nil
	w.Close()
}

// editorView is the song editor. The list, preview and parts editor are
// built from a.songs; the shell only owns the frame around them.
func (a *app) editorView(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(editorPadding).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Song editor").FontSize(18).FontWeight(600).SingleLine()
			ui.Spacer(c)
			ui.Button(c, "Close").OnClick(a.closeEditor)
		})
	})
}
