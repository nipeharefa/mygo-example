package main

import (
	"strings"

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
	editorPreviewMin = 320
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

// editorView is the song editor: the library on the left, and the selected
// song's preview and parts on the right. It is a pure function of a.songs and
// the edit indices, like every other view.
func (a *app) editorView(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(editorPadding).Gap(14).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Song editor").FontSize(18).FontWeight(600).SingleLine()
			ui.Spacer(c)
			ui.Button(c, "Close").OnClick(a.closeEditor)
		})
		ui.Row(c).Fill().Grow(1).Gap(18).AlignItems(ui.Stretch).Children(func() {
			a.editorList(c)
			a.editorMain(c)
		})
	})
}

// editorList is the searchable song library. A row keeps its real index, so a
// filter can never remap the selection.
func (a *app) editorList(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(editorListWidth).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Songs").FontSize(12).FontWeight(600).TextColor(t.TextMuted).SingleLine()
			ui.Spacer(c)
			ui.Button(c, "New song")
			ui.Button(c, "Restore samples")
		})
		ui.SearchField(c, &a.editQuery).Label("Search songs").Placeholder("Search songs")
		ui.Scroll(c).Fill().Grow(1).Children(func() {
			if len(a.songs) == 0 {
				ui.Text(c, "No songs yet").FontSize(11).TextColor(t.TextMuted).Padding(6)
				return
			}
			query := strings.ToLower(strings.TrimSpace(a.editQuery))
			shown := 0
			for i, s := range a.songs {
				if query != "" && !strings.Contains(strings.ToLower(s.Title), query) {
					continue
				}
				shown++
				i := i
				row := ui.Button(c, "Edit "+s.Title).OnClick(func() { a.selectSong(i) })
				if i == a.editSong {
					row = row.Background(t.Selection)
				}
			}
			if shown == 0 {
				ui.Text(c, "No songs match.").FontSize(11).TextColor(t.TextMuted).Padding(6)
			}
		})
	})
}

// editorMain is the right column: the selected song and its preview. The
// metadata and parts editors hang off this same column.
func (a *app) editorMain(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Fill().Grow(1).Children(func() {
		s, ok := a.selectedSong()
		if !ok {
			ui.Text(c, "Select a song to edit.").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, s.Title).FontSize(18).FontWeight(600).SingleLine()
			width, _ := c.Size()
			a.editorPreview(c, previewWidth(width))
		})
	})
}

// previewWidth fits the 16:9 preview into the right column but never below
// the smallest size the text can still be read at.
func previewWidth(windowWidth float32) float32 {
	w := windowWidth - editorListWidth - 3*editorPadding
	if w > editorPreviewMax {
		w = editorPreviewMax
	}
	if w < editorPreviewMin {
		w = editorPreviewMin
	}
	return w
}

// editorPreview draws the selected slide exactly as the stage would at this
// width, through the same slideBody.
func (a *app) editorPreview(c *ui.Context, width float32) {
	st := stageTheme()
	c.SetTheme(st)
	s, ok := a.songSlide()
	if !ok {
		ui.Column(c).Width(width).Height(width * 9 / 16).Background(st.Background).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Select a slide").FontSize(12).TextColor(st.TextMuted)
		})
		return
	}
	m := metricsFor(width, s.TextSize)
	ui.Column(c).Width(width).Height(width*9/16).Background(st.Background).Padding(m.PadX, m.PadY).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		slideBody(c, s, m)
	})
}

// selectSong moves the editor to a song and its first part and slide.
func (a *app) selectSong(i int) {
	if i < 0 || i >= len(a.songs) {
		return
	}
	a.editSong = i
	a.editPart = 0
	a.editSlide = 0
	a.clampEditor()
	a.changed()
}

// clampEditor keeps the three indices inside the library after any edit or
// delete; -1 means "nothing selected".
func (a *app) clampEditor() {
	a.editSong = a.clampSelection(len(a.songs), a.editSong)
	if a.editSong < 0 {
		a.editPart, a.editSlide = -1, -1
		return
	}
	parts := a.songs[a.editSong].Parts
	a.editPart = a.clampSelection(len(parts), a.editPart)
	if a.editPart < 0 {
		a.editSlide = -1
		return
	}
	a.editSlide = a.clampSelection(len(parts[a.editPart].Slides), a.editSlide)
}

// clampSelection pulls an index into [0,n), or -1 when there is nothing to
// select. -1 in stays -1: closing a selection is a valid state.
func (a *app) clampSelection(n, i int) int {
	if n == 0 {
		return -1
	}
	if i < 0 {
		return -1
	}
	if i >= n {
		return n - 1
	}
	return i
}

// selectedSong is the song the right column edits.
func (a *app) selectedSong() (song, bool) {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return song{}, false
	}
	return a.songs[a.editSong], true
}

// songSlide is the selected part and slide as a slide, for the preview.
func (a *app) songSlide() (slide, bool) {
	s, ok := a.selectedSong()
	if !ok || a.editPart < 0 || a.editPart >= len(s.Parts) {
		return slide{}, false
	}
	part := s.Parts[a.editPart]
	if a.editSlide < 0 || a.editSlide >= len(part.Slides) {
		return slide{}, false
	}
	return slide{
		Title:    s.Title,
		Section:  part.Label,
		Text:     part.Slides[a.editSlide],
		Kind:     slideSong,
		TextSize: s.textSize(),
	}, true
}
