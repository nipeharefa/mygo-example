package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	editorWidth        = 1280
	editorHeight       = 820
	editorMinWidth     = 1120
	editorMinHeight    = 700
	editorSongsWidth   = 248
	editorOutlineWidth = 300
	editorPadding      = 20
	editorGap          = 16
	editorPreviewMax   = 420
	editorPreviewMin   = 280
	editorCardPad      = 12
	editorDeskPad      = 16
	editorTextMin      = 150

	// The desk's two tabs: one slide at a time, or the song's settings.
	editorSlideTab = 0
	editorSongTab  = 1
)

// requestEditor asks for the editor window; the next frame opens it, so no
// window work happens inside the click that asked.
func (a *app) requestEditor() {
	a.openEditor = true
	if a.editSong < 0 && len(a.songs) > 0 {
		a.editSong, a.editPart, a.editSlide = 0, 0, 0
		a.clampEditor()
	}
	a.focusEditorText()
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
		Title:     appTitle + " · Song editor",
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
		a.editorDialogsClosed()
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
	a.editorDialogsClosed()
	w.Close()
}

// editorDialogsClosed forgets dialog state with the window: a dialog belongs
// to the editor, not to the library.
func (a *app) editorDialogsClosed() {
	a.newSongOpen, a.pasteOpen, a.confirmRemoveSong = false, false, false
}

// editorView is the song editor: the library, the selected song's slides, and
// one slide's desk. It is a pure function of a.songs and the edit indices, like
// every other view.
func (a *app) editorView(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(editorPadding).Gap(12).Children(func() {
		a.editorHeader(c)
		ui.Row(c).Fill().Grow(1).Gap(editorGap).AlignItems(ui.Stretch).Children(func() {
			a.editorSongs(c)
			a.editorOutline(c)
			a.editorDesk(c)
		})
	})
	a.newSongDialog(c)
	a.pasteDialog(c)
}

// editorHeader names the song being edited and carries the save state, so the
// proof that the words are on disk is not at the bottom of a scroll.
func (a *app) editorHeader(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, "Song editor").FontSize(18).FontWeight(600).SingleLine()
			if s, ok := a.selectedSong(); ok {
				meta := s.Title
				if m := songMeta(s); m != "" {
					meta += " · " + m
				}
				ui.Text(c, meta).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
			}
		})
		if a.editorStatus != "" {
			color := t.TextMuted
			if a.editorStatusErr {
				color = t.Danger
			}
			ui.Text(c, a.editorStatus).FontSize(textSmall).TextColor(color).SingleLine().MaxWidth(360)
		}
		_, hasSong := a.selectedSong()
		ui.Button(c, "Add song to run of show").Disabled(!hasSong).OnClick(a.addEditorSongToDeck)
		ui.Button(c, "Close").Label("Close").OnClick(a.closeEditor)
	})
}

// editorSongs is the library: one row per song, the way the run of show and
// the song books the operator knows are laid out.
func (a *app) editorSongs(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(editorSongsWidth).FillHeight().Padding(editorCardPad).Gap(8).
		Background(t.Surface).Radius(10).Border(1, hairlines(t)).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			editorSection(c, "Songs")
			ui.Spacer(c)
			ui.Text(c, songCount(len(a.songs))).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
		})
		ui.SearchField(c, &a.editQuery).Label("Search songs")
		ui.Button(c, "New song").Label("New song").FillWidth().OnClick(a.openNewSong)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				if len(a.songs) == 0 {
					ui.Text(c, "No songs yet").FontSize(textSmall).TextColor(t.TextMuted)
					ui.Text(c, "New song adds one with a blank verse.").FontSize(textLabel).TextColor(t.TextMuted)
					return
				}
				shown := a.matchingSongs()
				if len(shown) == 0 {
					ui.Text(c, "No songs match.").FontSize(textSmall).TextColor(t.TextMuted)
					ui.Text(c, "Try a title or an author.").FontSize(textLabel).TextColor(t.TextMuted)
					return
				}
				for _, i := range shown {
					a.editorSongRow(c, i)
				}
			})
		})
		editorQuiet(c, "Restore samples", false, a.restoreSamples)
	})
}

// editorSongRow is one library row. The selected song carries the console's
// selected tint and edge, so the editor and the console agree on selection.
func (a *app) editorSongRow(c *ui.Context, i int) {
	t := c.Theme()
	s := a.songs[i]
	bg, border := raisedRow(t), hairlines(t)
	if i == a.editSong {
		bg, border = rowSelected(t), rowSelectedEdge(t)
	}
	ui.Column(c).FillWidth().Padding(8, 10).Gap(2).Radius(8).Border(1, border).Background(bg).
		Label(s.Title).Key(fmt.Sprintf("editor-song-%d", i)).TabStop(true).
		OnClick(func() { a.selectSong(i) }).Children(func() {
		ui.Text(c, s.Title).FontSize(textBody).FontWeight(600).SingleLine()
		ui.Text(c, songMeta(s)).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
	})
}

// matchingSongs keeps the real library indices, so a filter can never remap
// the selection to a different song.
func (a *app) matchingSongs() []int {
	q := strings.ToLower(strings.TrimSpace(a.editQuery))
	out := make([]int, 0, len(a.songs))
	for i, s := range a.songs {
		if q == "" || strings.Contains(strings.ToLower(s.Title), q) || strings.Contains(strings.ToLower(s.Author), q) {
			out = append(out, i)
		}
	}
	return out
}

// songMeta is the muted second line of a library row: who wrote it, and the
// key when the library knows one.
func songMeta(s song) string {
	meta := strings.TrimSpace(s.Author)
	if s.Key != "" {
		if meta == "" {
			return s.Key
		}
		return meta + " · " + s.Key
	}
	if meta == "" {
		return "No author"
	}
	return meta
}

// songCount names the library size with its unit, so a bare number cannot be
// mistaken for a selection index.
func songCount(n int) string {
	if n == 1 {
		return "1 song"
	}
	return fmt.Sprintf("%d songs", n)
}

// editorOutline is the structure of the selected song: each part heads its
// slides, so a verse split across slides reads as one block and a part never
// looks like a slide.
func (a *app) editorOutline(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(editorOutlineWidth).FillHeight().Padding(editorCardPad).Gap(8).
		Background(sunken(t)).Radius(10).Border(1, hairlines(t)).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			editorSection(c, "Slides")
			ui.Spacer(c)
			if s, ok := a.selectedSong(); ok {
				ui.Text(c, slideCount(s.slideCount())).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
			}
		})
		if _, ok := a.selectedSong(); !ok {
			ui.Text(c, "Select a song to see its slides.").FontSize(textSmall).TextColor(t.TextMuted)
			return
		}
		i := a.editSong
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(14).Children(func() {
				for j := range a.songs[i].Parts {
					a.editorPart(c, i, j)
				}
			})
		})
		ui.Button(c, "Add part").Label("Add part").FillWidth().OnClick(a.addEditorPart)
	})
}

// editorPart is one part heading and the slides under it. The heading is a
// label, not a selected row: the selected slide alone carries the tint.
func (a *app) editorPart(c *ui.Context, i, j int) {
	t := c.Theme()
	part := a.songs[i].Parts[j]
	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).FillWidth().Padding(2, 4).Gap(8).AlignItems(ui.Center).Radius(6).
			Label(part.Label).Key(fmt.Sprintf("editor-part-%d-%d", i, j)).TabStop(true).
			OnClick(func() { a.selectPart(j) }).Children(func() {
			ui.Text(c, part.Label).FontSize(textLabel).FontWeight(700).LetterSpacing(0.8).
				TextColor(t.Text).Grow(1).SingleLine()
			ui.Text(c, slideCount(len(part.Slides))).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
		})
		for k := range part.Slides {
			a.editorSlideRow(c, i, j, k)
		}
	})
}

// editorSlideRow is one slide in the outline: its number and first line, with
// the accessible name the tests and assistive technology use.
func (a *app) editorSlideRow(c *ui.Context, i, j, k int) {
	t := c.Theme()
	line := (slide{Text: a.songs[i].Parts[j].Slides[k]}).firstLine()
	if strings.TrimSpace(line) == "" {
		line = "Empty slide"
	}
	bg, border := raisedRow(t), hairlines(t)
	if j == a.editPart && k == a.editSlide {
		bg, border = rowSelected(t), rowSelectedEdge(t)
	}
	ui.Row(c).FillWidth().Padding(6, 8).Gap(8).Radius(8).Border(1, border).Background(bg).
		AlignItems(ui.Center).Label(fmt.Sprintf("%s slide %d", a.songs[i].Parts[j].Label, k+1)).
		Key(fmt.Sprintf("editor-slide-%d-%d-%d", i, j, k)).TabStop(true).
		OnClick(func() { a.selectEditorSlide(j, k) }).Children(func() {
		ui.Text(c, fmt.Sprint(k+1)).FontSize(textLabel).TextColor(t.TextMuted).Width(14).SingleLine()
		ui.Text(c, line).FontSize(textSmall).TextColor(t.Text).Grow(1).SingleLine()
	})
}

// editorDesk is the right pane: the slide being edited, or the song's
// settings. The two are tabs so neither hides the other below a fold.
func (a *app) editorDesk(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Padding(editorDeskPad).Gap(10).
		Background(t.Surface).Radius(10).Border(1, hairlines(t)).Children(func() {
		ui.Tabs(c, &a.editorTab, "Slide", "Song").PaddingY(2)
		if a.editorTab == editorSongTab {
			a.editorSongDesk(c)
			return
		}
		a.editorSlideDesk(c)
	})
}

// editorSlideDesk is the working surface: the slide as the room sees it, the
// words with the caret, and the actions that act on this slide or part. The
// part's own name carries its removal, so an empty verse added by mistake can
// go from the tab where it was typed, not only from Song.
func (a *app) editorSlideDesk(c *ui.Context) {
	t := c.Theme()
	i, j, k := a.editSong, a.editPart, a.editSlide
	if !a.validEditorSlide() {
		ui.Text(c, "Select a slide to edit its words.").FontSize(textSmall).TextColor(t.TextMuted)
		return
	}
	part := a.songs[i].Parts[j]
	width, _ := c.Size()
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, part.Label).FontSize(textBody).FontWeight(600).SingleLine()
		ui.Text(c, fmt.Sprintf("slide %d of %d", k+1, len(part.Slides))).
			FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
		editorQuiet(c, "Remove part", true, a.removePart)
		ui.Button(c, "‹").Label("Previous slide").Disabled(!a.hasPrevSlide()).OnClick(a.prevEditorSlide)
		ui.Button(c, "›").Label("Next slide").Disabled(!a.hasNextSlide()).OnClick(a.nextEditorSlide)
	})
	ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
		a.editorPreview(c, previewWidth(width))
	})
	editorLabel(c, "Slide text")
	ta := ui.TextArea(c.Key(slideFocusKey(i, j, k)), &a.songs[i].Parts[j].Slides[k]).
		Label("Slide text input").
		Placeholder("Type the words for this slide").
		Grow(1).MinHeight(editorTextMin).
		OnChange(a.slideTextEdited)
	ta.FocusBind(&a.editorFocus, slideFocusKey(i, j, k))
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Button(c, "Add slide below").OnClick(a.addSlide)
		ui.Button(c, "Duplicate").Label("Duplicate slide").OnClick(a.duplicateEditorSlide)
		ui.Button(c, "Paste words").Label("Paste words into this part").OnClick(a.openPaste)
		ui.Spacer(c)
		editorQuiet(c, "Remove slide", true, a.removeEditorSlide)
	})
	ui.Text(c, "The run of show keeps a copy; add the song again to send changes.").
		FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
}

// editorSongDesk is the song's settings: identity, text size, the selected
// part, and the one destructive action, kept away from the words.
func (a *app) editorSongDesk(c *ui.Context) {
	t := c.Theme()
	i := a.editSong
	s, ok := a.selectedSong()
	if !ok {
		ui.Text(c, "Select a song to edit its details.").FontSize(textSmall).TextColor(t.TextMuted)
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(14).PaddingY(4).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				editorSection(c, "Song details")
				ui.Row(c).Gap(10).Children(func() {
					ui.Column(c).Gap(4).Grow(1).Children(func() {
						editorLabel(c, "Title")
						ui.TextInput(c, &a.songs[i].Title).Label("Song title input").OnChange(a.songEdited)
					})
					ui.Column(c).Gap(4).Width(110).Children(func() {
						editorLabel(c, "Key")
						ui.TextInput(c, &a.songs[i].Key).Label("Song key input").OnChange(a.songEdited)
					})
				})
				ui.Column(c).Gap(4).Children(func() {
					editorLabel(c, "Author")
					ui.TextInput(c, &a.songs[i].Author).Label("Song author input").OnChange(a.songEdited)
				})
			})
			ui.Column(c).Gap(6).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					editorLabel(c, "Text size")
					ui.Spacer(c)
					ui.Text(c, fmt.Sprintf("%.0f pt", s.textSize())).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
				})
				ui.Slider(c, &a.songs[i].TextSize, minTextSize, maxTextSize).Step(1).Label("Text size").OnChange(func() {
					a.setTextSize(a.songs[i].TextSize)
				})
			})
			if a.editPart >= 0 && a.editPart < len(s.Parts) {
				ui.Divider(c)
				ui.Column(c).Gap(6).Children(func() {
					editorSection(c, "Part")
					editorLabel(c, "Part label")
					ui.TextInput(c, &a.songs[i].Parts[a.editPart].Label).Label("Part label input").OnChange(a.songEdited)
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Button(c, "Add part to run of show").OnClick(a.addEditorPartToDeck)
						ui.Spacer(c)
						editorQuiet(c, "Remove part", true, a.removePart)
					})
					ui.Text(c, "The run of show keeps a copy of the words it was given.").
						FontSize(textLabel).TextColor(t.TextMuted)
				})
			}
			ui.Divider(c)
			ui.Column(c).Gap(8).Children(func() {
				editorSection(c, "Library")
				ui.Text(c, "Every change here saves as you type.").FontSize(textSmall).TextColor(t.TextMuted)
				if a.confirmRemoveSong {
					ui.Column(c).Gap(8).Padding(10).Radius(8).Background(sunken(t)).Children(func() {
						ui.Text(c, fmt.Sprintf("Remove %q and its slides?", s.Title)).FontSize(textBody).FontWeight(600)
						ui.Text(c, "This cannot be undone.").FontSize(textSmall).TextColor(t.TextMuted)
						ui.Row(c).Gap(8).Children(func() {
							ui.Button(c, "Keep song").OnClick(func() {
								a.confirmRemoveSong = false
								a.editorRedraw(false)
							})
							ui.Button(c, "Delete song").Label("Delete song").OnClick(a.removeSong)
						})
					})
				} else {
					editorQuiet(c, "Remove song", true, func() {
						a.confirmRemoveSong = true
						a.editorRedraw(false)
					})
				}
			})
		})
	})
}

// editorSection is a visible group heading inside a column: the console's
// tracked label, so the editor reads as the same app.
func editorSection(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(textLabel).FontWeight(700).LetterSpacing(0.9).
		TextColor(c.Theme().TextMuted).SingleLine()
}

// editorLabel names the control under it instead of relying on a placeholder
// that disappears once the field has a value.
func editorLabel(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(textSmall).TextColor(c.Theme().TextMuted).SingleLine()
}

// editorQuiet is an action that should be available but not compete with the
// primary buttons: a text row with the pane's hover, as a menu item.
func editorQuiet(c *ui.Context, label string, danger bool, fn func()) {
	t := c.Theme()
	color := t.TextMuted
	if danger {
		color = t.Danger
	}
	row := ui.Row(c).Padding(6, 8).Radius(6).AlignItems(ui.Center).Label(label).
		TabStop(true).Cursor(ui.CursorPointer).OnClick(fn)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(textSmall).TextColor(color).SingleLine()
	})
}

// editorPreview draws the selected slide exactly as the stage would at this
// width, through the same slideBody and the stage's colors. It passes those
// colors in; it never switches the window's theme, which would leak into
// every control built after it.
func (a *app) editorPreview(c *ui.Context, width float32) {
	st := stageTheme()
	s, ok := a.songSlide()
	if !ok {
		ui.Column(c).Width(width).Height(width*9/16).Radius(8).
			Background(st.Background).Border(1, stageCardBorder()).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Select a slide").FontSize(textBody).TextColor(st.TextMuted)
		})
		return
	}
	m := metricsFor(width, s.TextSize)
	ui.Column(c).Width(width).Height(width*9/16).Radius(8).
		Background(st.Background).Border(1, stageCardBorder()).
		Padding(m.PadX, m.PadY).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		slideBody(c, s, m, st)
	})
}

// previewWidth leaves the preview room inside the desk but never lets it
// squeeze the words or shrink below the size the text can still be read at.
func previewWidth(windowWidth float32) float32 {
	desk := windowWidth - 2*editorPadding - editorSongsWidth - editorOutlineWidth - 2*editorGap - 2*editorDeskPad
	if desk > editorPreviewMax {
		desk = editorPreviewMax
	}
	if desk < editorPreviewMin {
		desk = editorPreviewMin
	}
	return desk
}

// selectSong moves the editor to a song and its first part and slide, with
// the caret in the first slide: choosing a song and typing takes one click.
func (a *app) selectSong(i int) {
	if i < 0 || i >= len(a.songs) {
		return
	}
	a.editSong = i
	a.editPart = 0
	a.editSlide = 0
	a.confirmRemoveSong = false
	a.clampEditor()
	a.focusEditorText()
	a.editorRedraw(false)
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

// selectedSong is the song the slides column and desk edit.
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

// validEditorSlide reports whether the three indices point at a slide.
func (a *app) validEditorSlide() bool {
	_, ok := a.songSlide()
	return ok
}

// slideFocusKey names the slide whose text should hold the caret.
func slideFocusKey(i, j, k int) string {
	return fmt.Sprintf("slide-%d-%d-%d", i, j, k)
}

// focusEditorText asks the visible text area to take the caret. It goes
// through FocusBind, so a request made before the window exists waits for it,
// and a click elsewhere hands the caret back instead of being stolen again.
func (a *app) focusEditorText() {
	if a.validEditorSlide() {
		a.editorFocus = slideFocusKey(a.editSong, a.editPart, a.editSlide)
	}
}

// currentPartLabel names the part the desk is editing.
func (a *app) currentPartLabel() string {
	s, ok := a.selectedSong()
	if !ok || a.editPart < 0 || a.editPart >= len(s.Parts) {
		return "this part"
	}
	return s.Parts[a.editPart].Label
}

// selectPart moves the editor to a part and its first slide.
func (a *app) selectPart(j int) {
	s, ok := a.selectedSong()
	if !ok || j < 0 || j >= len(s.Parts) || len(s.Parts[j].Slides) == 0 {
		return
	}
	a.selectEditorSlide(j, 0)
}

// selectEditorSlide moves the editor to one slide of one part, shows the slide
// tab, and puts the caret in the words so typing starts at once.
func (a *app) selectEditorSlide(j, k int) {
	s, ok := a.selectedSong()
	if !ok || j < 0 || j >= len(s.Parts) || k < 0 || k >= len(s.Parts[j].Slides) {
		return
	}
	a.editPart = j
	a.editSlide = k
	a.clampEditor()
	a.editorTab = editorSlideTab
	a.focusEditorText()
	a.editorRedraw(false)
}

// stepEditorSlide walks the song's slides in reading order, crossing part
// boundaries, so the desk's arrows need no knowledge of the outline.
func (a *app) stepEditorSlide(delta int) {
	s, ok := a.selectedSong()
	if !ok || a.editPart < 0 || a.editPart >= len(s.Parts) {
		return
	}
	j, k := a.editPart, a.editSlide
	if delta < 0 {
		switch {
		case k > 0:
			k--
		case j > 0:
			j--
			k = len(s.Parts[j].Slides) - 1
		default:
			return
		}
	} else {
		switch {
		case k+1 < len(s.Parts[j].Slides):
			k++
		case j+1 < len(s.Parts):
			j++
			k = 0
		default:
			return
		}
	}
	a.selectEditorSlide(j, k)
}

func (a *app) prevEditorSlide() { a.stepEditorSlide(-1) }
func (a *app) nextEditorSlide() { a.stepEditorSlide(1) }

// hasPrevSlide and hasNextSlide tell the arrows when they are at an end.
func (a *app) hasPrevSlide() bool {
	return a.editPart > 0 || a.editSlide > 0
}

func (a *app) hasNextSlide() bool {
	s, ok := a.selectedSong()
	if !ok || a.editPart < 0 || a.editPart >= len(s.Parts) {
		return false
	}
	return a.editSlide+1 < len(s.Parts[a.editPart].Slides) || a.editPart+1 < len(s.Parts)
}

// slideTextEdited is the text area's change: the words are already in the
// model, so it saves and repaints the editor alone. The console shows the
// deck, which keeps its own copy, so redrawing it here would be wasted work.
func (a *app) slideTextEdited() {
	if !a.validEditorSlide() {
		return
	}
	a.saveNote("Saved", false)
}

// songEdited saves a song-level edit. Title, author and key show in the
// console's library, so that edit repaints the console too.
func (a *app) songEdited() {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
	a.songs[a.editSong].TextSize = clampTextSize(a.songs[a.editSong].TextSize)
	a.saveNote("Saved", true)
}

// setTextSize is the slider's landing point: a size the slider cannot produce
// is pulled back into its range before it is saved.
func (a *app) setTextSize(v float64) {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
	a.songs[a.editSong].TextSize = clampTextSize(v)
	a.saveNote("Saved", false)
}

// saveNote writes the library and reports the truth in the header: the note
// when it worked, the error when it did not. The console is repainted only
// when the edit can show there.
func (a *app) saveNote(note string, console bool) {
	if err := a.saveLibrary(); err != nil {
		a.editorStatusErr = true
		if errors.Is(err, errLibraryReloaded) {
			a.editorStatus = "Reloaded the songs folder; this change was not kept."
		} else {
			a.editorStatus = "Could not save: " + err.Error()
		}
	} else {
		a.editorStatus = note
		a.editorStatusErr = false
		a.noteHistory(note)
	}
	a.editorRedraw(console)
}

// editorRefused shows why an action did nothing, in the danger color.
func (a *app) editorRefused(note string) {
	a.editorStatus = note
	a.editorStatusErr = true
	a.editorRedraw(false)
}

// editorRedraw repaints the editor, and the console only when the change can
// be seen there.
func (a *app) editorRedraw(console bool) {
	if console && a.win != nil {
		a.win.Invalidate()
	}
	if a.editor != nil {
		a.editor.Invalidate()
	}
}

// openNewSong clears the dialog's fields and shows it. A new song is named
// before it exists, so it never appears as "New Song" waiting to be found.
func (a *app) openNewSong() {
	a.newSongTitle = ""
	a.newSongAuthor = ""
	a.newSongKey = ""
	a.newSongPart = "Verse 1"
	a.newSongWords = ""
	a.newSongOpen = true
	a.editorRedraw(false)
}

// createSong adds the dialog's song, selects it, and puts the caret in its
// first slide.
func (a *app) createSong() {
	title := strings.TrimSpace(a.newSongTitle)
	if title == "" {
		return
	}
	part := strings.TrimSpace(a.newSongPart)
	if part == "" {
		part = "Verse 1"
	}
	slides := splitSlides(a.newSongWords)
	if len(slides) == 0 {
		slides = []string{""}
	}
	a.songs = append(a.songs, song{
		ID:       newSongID(title, a.songs),
		Title:    title,
		Author:   strings.TrimSpace(a.newSongAuthor),
		Key:      strings.TrimSpace(a.newSongKey),
		TextSize: defaultTextSize,
		Parts:    []songPart{{Label: part, Slides: slides}},
	})
	a.editQuery = ""
	a.editSong = len(a.songs) - 1
	a.editPart, a.editSlide = 0, 0
	a.editorTab = editorSlideTab
	a.newSongOpen = false
	a.confirmRemoveSong = false
	a.focusEditorText()
	a.saveNote("New song added", true)
}

// newSongDialog collects a song's name and first words. The words split at
// blank lines, so a whole verse becomes its slides in one paste.
func (a *app) newSongDialog(c *ui.Context) {
	if !a.newSongOpen {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.newSongOpen, func() {
		ui.Column(c).Width(480).Gap(10).Children(func() {
			ui.Text(c, "New song").FontSize(textBody + 3).FontWeight(600).SingleLine()
			ui.Text(c, "One blank line starts a new slide.").FontSize(textSmall).TextColor(t.TextMuted)
			ui.Column(c).Gap(4).Children(func() {
				editorLabel(c, "Title")
				ui.TextInput(c, &a.newSongTitle).Label("New song title").Placeholder("Amazing Grace").AutoFocus()
			})
			ui.Row(c).Gap(10).Children(func() {
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					editorLabel(c, "Author")
					ui.TextInput(c, &a.newSongAuthor).Label("New song author").Placeholder("John Newton")
				})
				ui.Column(c).Gap(4).Width(90).Children(func() {
					editorLabel(c, "Key")
					ui.TextInput(c, &a.newSongKey).Label("New song key").Placeholder("G")
				})
			})
			ui.Column(c).Gap(4).Children(func() {
				editorLabel(c, "First part")
				ui.TextInput(c, &a.newSongPart).Label("New song part").Placeholder("Verse 1")
			})
			ui.Column(c).Gap(4).Children(func() {
				editorLabel(c, "Words (optional)")
				ui.TextArea(c, &a.newSongWords).Label("New song words").
					Height(140).Placeholder("Paste the verse here, one slide per blank line.")
			})
			if n := len(splitSlides(a.newSongWords)); n > 0 {
				ui.Text(c, "This makes "+slideCount(n)).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
			}
			if strings.TrimSpace(a.newSongTitle) == "" {
				ui.Text(c, "Give the song a title to create it.").FontSize(textSmall).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				ui.Button(c, "Cancel").OnClick(func() {
					a.newSongOpen = false
					a.editorRedraw(false)
				})
				ui.PrimaryButton(c, "Create song").
					Disabled(strings.TrimSpace(a.newSongTitle) == "").OnClick(a.createSong)
			})
		})
	})
}

// openPaste shows the paste dialog for the selected part.
func (a *app) openPaste() {
	if !a.validEditorSlide() {
		return
	}
	a.pasteText = ""
	a.pasteOpen = true
	a.editorRedraw(false)
}

// applyPaste turns the pasted blocks into slides. A part that is still one
// empty slide is replaced, so a fresh verse does not begin with a blank; any
// other part gets the blocks after its last slide.
func (a *app) applyPaste() {
	i, j := a.editSong, a.editPart
	s, ok := a.selectedSong()
	if !ok || j < 0 || j >= len(s.Parts) {
		return
	}
	slides := splitSlides(a.pasteText)
	if len(slides) == 0 {
		return
	}
	if len(s.Parts[j].Slides) == 1 && strings.TrimSpace(s.Parts[j].Slides[0]) == "" {
		a.songs[i].Parts[j].Slides = slides
		a.editSlide = 0
	} else {
		a.editSlide = len(a.songs[i].Parts[j].Slides)
		a.songs[i].Parts[j].Slides = append(a.songs[i].Parts[j].Slides, slides...)
	}
	a.pasteOpen = false
	a.pasteText = ""
	a.confirmRemoveSong = false
	a.editorTab = editorSlideTab
	a.focusEditorText()
	a.saveNote(addedSlidesNote(len(slides)), false)
}

// pasteDialog takes a whole verse and splits it at blank lines.
func (a *app) pasteDialog(c *ui.Context) {
	if !a.pasteOpen {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.pasteOpen, func() {
		ui.Column(c).Width(520).Gap(10).Children(func() {
			ui.Text(c, "Paste words").FontSize(textBody + 3).FontWeight(600).SingleLine()
			ui.Text(c, "Into "+a.currentPartLabel()+". One blank line starts a new slide.").
				FontSize(textSmall).TextColor(t.TextMuted)
			ui.TextArea(c, &a.pasteText).Label("Pasted words").Height(220).AutoFocus().
				Placeholder("Amazing grace! how sweet the sound\nThat saved a wretch like me!")
			if n := len(splitSlides(a.pasteText)); n > 0 {
				ui.Text(c, slideCount(n)+" ready").FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				ui.Button(c, "Cancel").OnClick(func() {
					a.pasteOpen = false
					a.editorRedraw(false)
				})
				ui.PrimaryButton(c, "Add slides").
					Disabled(len(splitSlides(a.pasteText)) == 0).OnClick(a.applyPaste)
			})
		})
	})
}

// splitSlides divides pasted words into slides at blank lines, dropping the
// empty blocks and keeping the line breaks inside each slide.
func splitSlides(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var out []string
	for _, block := range strings.Split(text, "\n\n") {
		if b := strings.TrimSpace(block); b != "" {
			out = append(out, b)
		}
	}
	return out
}

// addEditorPart appends a part named for the structure it joins, so a hymn
// gets Verse 2 rather than Part 2.
func (a *app) addEditorPart() {
	i := a.editSong
	if i < 0 || i >= len(a.songs) {
		return
	}
	part := songPart{Label: nextPartLabel(a.songs[i].Parts), Slides: []string{""}}
	a.songs[i].Parts = append(a.songs[i].Parts, part)
	a.editPart = len(a.songs[i].Parts) - 1
	a.editSlide = 0
	a.clampEditor()
	a.editorTab = editorSlideTab
	a.focusEditorText()
	a.saveNote("Part added", false)
}

// removePart never empties a song: the last part stays with a status note.
func (a *app) removePart() {
	i, j := a.editSong, a.editPart
	if i < 0 || i >= len(a.songs) || j < 0 || j >= len(a.songs[i].Parts) {
		return
	}
	if len(a.songs[i].Parts) <= 1 {
		a.editorRefused("A song needs one part. Delete the song to remove it.")
		return
	}
	a.songs[i].Parts = append(a.songs[i].Parts[:j], a.songs[i].Parts[j+1:]...)
	a.clampEditor()
	a.focusEditorText()
	a.saveNote("Part removed", false)
}

// addSlide inserts a blank slide after the current one, selects it, and puts
// the caret there: writing the next slide does not need another click.
func (a *app) addSlide() {
	i, j, k := a.editSong, a.editPart, a.editSlide
	if !a.validEditorSlide() {
		return
	}
	slides := a.songs[i].Parts[j].Slides
	out := make([]string, 0, len(slides)+1)
	out = append(out, slides[:k+1]...)
	out = append(out, "")
	out = append(out, slides[k+1:]...)
	a.songs[i].Parts[j].Slides = out
	a.editSlide = k + 1
	a.editorTab = editorSlideTab
	a.focusEditorText()
	a.saveNote("Slide added", false)
}

// duplicateEditorSlide copies the current words below them, for a refrain
// that repeats with one changed line.
func (a *app) duplicateEditorSlide() {
	i, j, k := a.editSong, a.editPart, a.editSlide
	if !a.validEditorSlide() {
		return
	}
	slides := a.songs[i].Parts[j].Slides
	out := make([]string, 0, len(slides)+1)
	out = append(out, slides[:k+1]...)
	out = append(out, slides[k])
	out = append(out, slides[k+1:]...)
	a.songs[i].Parts[j].Slides = out
	a.editSlide = k + 1
	a.editorTab = editorSlideTab
	a.focusEditorText()
	a.saveNote("Slide duplicated", false)
}

// removeEditorSlide never empties a part: the last slide stays with a note
// that names the action which does clear an empty part away.
func (a *app) removeEditorSlide() {
	i, j, k := a.editSong, a.editPart, a.editSlide
	if !a.validEditorSlide() {
		return
	}
	if len(a.songs[i].Parts[j].Slides) <= 1 {
		a.editorRefused("A part needs a slide. Remove part deletes the part.")
		return
	}
	a.songs[i].Parts[j].Slides = append(a.songs[i].Parts[j].Slides[:k], a.songs[i].Parts[j].Slides[k+1:]...)
	a.clampEditor()
	a.focusEditorText()
	a.saveNote("Slide removed", false)
}

// removeSong deletes the selected song and pulls the selection to a neighbor.
// The Song tab asks before it reaches here.
func (a *app) removeSong() {
	i := a.editSong
	if i < 0 || i >= len(a.songs) {
		return
	}
	a.songs = append(a.songs[:i], a.songs[i+1:]...)
	a.confirmRemoveSong = false
	a.clampEditor()
	a.focusEditorText()
	a.saveNote("Song removed", true)
}

// restoreSamples adds any built-in whose id the library lost and never
// overwrites a song the operator edited.
func (a *app) restoreSamples() {
	first := -1
	for _, b := range builtinSongs {
		if _, ok := findSong(a.songs, b.ID); ok {
			continue
		}
		a.songs = append(a.songs, cloneSongs([]song{b})[0])
		if first < 0 {
			first = len(a.songs) - 1
		}
	}
	if first >= 0 {
		a.editSong = first
		a.editPart, a.editSlide = 0, 0
		a.clampEditor()
		a.saveNote("Samples restored", true)
		return
	}
	a.editorStatus = "Samples are already here"
	a.editorStatusErr = false
	a.editorRedraw(true)
}

// nextPartLabel names a new part after the verses already there: Verse 1 gets
// Verse 2, while a song without verse numbers gets a plain Part.
func nextPartLabel(parts []songPart) string {
	highest := 0
	for _, p := range parts {
		if n, ok := verseNumber(p.Label); ok && n > highest {
			highest = n
		}
	}
	if highest > 0 {
		return fmt.Sprintf("Verse %d", highest+1)
	}
	return fmt.Sprintf("Part %d", len(parts)+1)
}

// verseNumber reads "Verse 3" as 3, the only label shape this names for.
func verseNumber(label string) (int, bool) {
	label = strings.TrimSpace(label)
	if len(label) < len("verse ") || !strings.EqualFold(label[:len("verse ")], "verse ") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(label[len("verse "):]))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// addEditorSongToDeck sends every part of the selected song to the run of
// show, in part order, and moves the monitor to the first added slide.
func (a *app) addEditorSongToDeck() {
	s, ok := a.selectedSong()
	if !ok {
		return
	}
	n := a.addSongParts(s)
	if n == 0 {
		a.editorRefused("This song has no text yet")
		return
	}
	a.editorStatus, a.editorStatusErr = addedSlidesNote(n), false
	a.changed()
}

// addEditorPartToDeck sends only the selected part, so a chorus can follow
// the verse it belongs to without adding the whole song again.
func (a *app) addEditorPartToDeck() {
	s, ok := a.selectedSong()
	if !ok || a.editPart < 0 || a.editPart >= len(s.Parts) {
		return
	}
	n := a.addPartSlides(s, s.Parts[a.editPart])
	if n == 0 {
		a.editorRefused("This part has no text yet")
		return
	}
	a.preview = len(a.deck) - n
	a.editorStatus, a.editorStatusErr = addedSlidesNote(n), false
	a.changed()
}

// addedSlidesNote says what the last add put in the run of show.
func addedSlidesNote(n int) string {
	if n == 1 {
		return "Added 1 slide"
	}
	return fmt.Sprintf("Added %d slides", n)
}
