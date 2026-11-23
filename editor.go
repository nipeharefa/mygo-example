package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	editorWidth      = 1280
	editorHeight     = 820
	editorMinWidth   = 1120
	editorMinHeight  = 700
	editorListWidth  = 240
	editorPadding    = 20
	editorGap        = 16
	editorPreviewMax = 560
	editorPreviewMin = 380
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

// editorMain is the right column: the selected song's header and the two
// working columns below it.
func (a *app) editorMain(c *ui.Context) {
	t := c.Theme()
	s, ok := a.selectedSong()
	if !ok {
		ui.Text(c, "Select a song to edit.").FontSize(12).TextColor(t.TextMuted)
		return
	}
	width, _ := c.Size()
	ui.Column(c).Grow(1).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.Title).FontSize(20).FontWeight(600).SingleLine()
			ui.Spacer(c)
			ui.Button(c, "Add song to run of show").OnClick(a.addEditorSongToDeck)
			ui.Button(c, "Remove song").OnClick(a.removeSong)
		})
		ui.Row(c).Fill().Grow(1).Gap(editorGap).AlignItems(ui.Stretch).Children(func() {
			a.editorForm(c)
			a.editorSide(c, width)
		})
	})
}

// editorForm is the left working column: details and parts, scrolled on its
// own so the preview stays in place while the operator edits below it.
func (a *app) editorForm(c *ui.Context) {
	i := a.editSong
	ui.Scroll(c).Fill().Grow(1).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			ui.Column(c).Gap(10).Children(func() {
				editorSection(c, "Details")
				editorLabel(c, "Title")
				ui.TextInput(c, &a.songs[i].Title).Label("Song title input").OnChange(a.songEdited)
				ui.Row(c).Gap(10).Children(func() {
					ui.Column(c).Gap(4).Grow(1).Children(func() {
						editorLabel(c, "Author")
						ui.TextInput(c, &a.songs[i].Author).Label("Song author input").OnChange(a.songEdited)
					})
					ui.Column(c).Gap(4).Width(80).Children(func() {
						editorLabel(c, "Key")
						ui.TextInput(c, &a.songs[i].Key).Label("Song key input").OnChange(a.songEdited)
					})
				})
				ui.Column(c).Gap(6).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						editorLabel(c, "Text size")
						ui.Spacer(c)
						ui.Text(c, fmt.Sprintf("%.0f pt", a.songs[i].textSize())).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
					})
					ui.Slider(c, &a.songs[i].TextSize, minTextSize, maxTextSize).Step(1).Label("Text size").OnChange(func() {
						a.setTextSize(a.songs[i].TextSize)
					})
				})
			})
			a.editorParts(c)
		})
	})
}

// editorSide is the right working column: the preview and the text of the
// slide the preview shows.
func (a *app) editorSide(c *ui.Context, windowWidth float32) {
	t := c.Theme()
	w := previewWidth(windowWidth)
	ui.Column(c).Width(w).Gap(10).Children(func() {
		editorSection(c, "Preview")
		a.editorPreview(c, w)
		if s, ok := a.songSlide(); ok {
			total := 0
			if a.editSong >= 0 && a.editPart >= 0 && a.editPart < len(a.songs[a.editSong].Parts) {
				total = len(a.songs[a.editSong].Parts[a.editPart].Slides)
			}
			ui.Text(c, fmt.Sprintf("%s · slide %d of %d", s.Section, a.editSlide+1, total)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		}
		a.editorSlideText(c)
		if a.editorStatus != "" {
			ui.Text(c, a.editorStatus).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

// editorSection is a visible group heading inside the form.
func editorSection(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(12).FontWeight(600).SingleLine()
}

// editorLabel names the control under it instead of relying on a placeholder
// that disappears once the field has a value.
func editorLabel(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
}

// previewWidth fits the 16:9 preview and the form into the space beside the
// library: the preview takes 46% and the form gets the rest.
func previewWidth(windowWidth float32) float32 {
	main := windowWidth - 2*editorPadding - editorListWidth - editorGap
	w := main * 0.46
	if w > editorPreviewMax {
		w = editorPreviewMax
	}
	if w < editorPreviewMin {
		w = editorPreviewMin
	}
	return w
}

// editorPreview draws the selected slide exactly as the stage would at this
// width, through the same slideBody and the stage's colors. It passes those
// colors in; it never switches the window's theme, which would leak into
// every control built after it.
func (a *app) editorPreview(c *ui.Context, width float32) {
	st := stageTheme()
	s, ok := a.songSlide()
	if !ok {
		ui.Column(c).Width(width).Height(width * 9 / 16).Radius(8).Background(st.Background).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Select a slide").FontSize(12).TextColor(st.TextMuted)
		})
		return
	}
	m := metricsFor(width, s.TextSize)
	ui.Column(c).Width(width).Height(width*9/16).Radius(8).Background(st.Background).Padding(m.PadX, m.PadY).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		slideBody(c, s, m, st)
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

// editorParts lists the song's parts and their slides. Each part is a card;
// its buttons select the row they sit in before acting, so a click always
// lands on what the operator saw.
func (a *app) editorParts(c *ui.Context) {
	t := c.Theme()
	s, ok := a.selectedSong()
	if !ok {
		return
	}
	i := a.editSong
	ui.Column(c).Gap(8).Children(func() {
		editorSection(c, "Parts")
		for j := range s.Parts {
			j := j
			part := s.Parts[j]
			ui.Column(c).Gap(8).Padding(10).Radius(8).Background(t.Surface).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &a.songs[i].Parts[j].Label).Label(fmt.Sprintf("Part label %d", j+1)).Grow(1).OnChange(a.songEdited)
					ui.Button(c, "Remove part").OnClick(func() { a.selectPart(j); a.removePart() })
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Button(c, "Add slide").OnClick(func() { a.selectPart(j); a.addSlide() })
					ui.Button(c, "Add part to run of show").OnClick(func() { a.selectPart(j); a.addEditorPartToDeck() })
				})
				if len(part.Slides) == 0 {
					ui.Text(c, "No slides yet.").FontSize(11).TextColor(t.TextMuted)
				}
				for k, text := range a.songs[i].Parts[j].Slides {
					k := k
					selected := j == a.editPart && k == a.editSlide
					line := (slide{Text: text}).firstLine()
					if strings.TrimSpace(line) == "" {
						line = "Empty slide"
					}
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						row := ui.Button(c, fmt.Sprintf("Slide %d", k+1)).OnClick(func() { a.selectEditorSlide(j, k) })
						if selected {
							row = row.Background(t.Selection)
						}
						ui.Text(c, line).FontSize(11).TextColor(t.TextMuted).Grow(1).SingleLine()
						if selected {
							ui.Button(c, "Remove slide").OnClick(func() { a.selectEditorSlide(j, k); a.removeEditorSlide() })
						}
					})
				}
			})
		}
		ui.Button(c, "Add part").OnClick(a.addEditorPart)
	})
}

// editorSlideText edits the selected slide's text. The area is bound to the
// library string itself, so what is typed is what a preview and a save see.
func (a *app) editorSlideText(c *ui.Context) {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
	s := a.songs[a.editSong]
	if a.editPart < 0 || a.editPart >= len(s.Parts) || a.editSlide < 0 || a.editSlide >= len(s.Parts[a.editPart].Slides) {
		return
	}
	ui.Column(c).Gap(4).Children(func() {
		editorSection(c, "Slide text")
		ui.TextArea(c, &a.songs[a.editSong].Parts[a.editPart].Slides[a.editSlide]).Label("Slide text input").Height(170).OnChange(a.songEdited)
	})
}

// selectPart moves the editor to a part and its first slide.
func (a *app) selectPart(j int) {
	if j < 0 || j >= len(a.songs[a.editSong].Parts) {
		return
	}
	a.editPart = j
	a.editSlide = 0
	a.clampEditor()
	a.changed()
}

// selectEditorSlide moves the editor to one slide of one part.
func (a *app) selectEditorSlide(j, k int) {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
	if j < 0 || j >= len(a.songs[a.editSong].Parts) {
		return
	}
	if k < 0 || k >= len(a.songs[a.editSong].Parts[j].Slides) {
		return
	}
	a.editPart = j
	a.editSlide = k
	a.clampEditor()
	a.changed()
}

// songEdited clamps the size, saves the whole library and says so. Every text
// field calls it, so an edit never waits for a Save button.
func (a *app) songEdited() {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
	a.songs[a.editSong].TextSize = clampTextSize(a.songs[a.editSong].TextSize)
	a.saveLibrary()
	a.editorStatus = "Saved"
	a.changed()
}

// setTextSize is the slider's landing point: a size the slider cannot produce
// is pulled back into its range before it is saved.
func (a *app) setTextSize(v float64) {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
	a.songs[a.editSong].TextSize = clampTextSize(v)
	a.saveLibrary()
	a.editorStatus = "Saved"
	a.changed()
}

// newSong appends an empty draft and selects it.
func (a *app) newSong() {
	s := song{
		ID:       newSongID("New Song", a.songs),
		Title:    "New Song",
		TextSize: defaultTextSize,
		Parts:    []songPart{{Label: "Verse 1", Slides: []string{""}}},
	}
	a.songs = append(a.songs, s)
	a.selectSong(len(a.songs) - 1)
	a.saveLibrary()
	a.editorStatus = "New song added"
	a.changed()
}

// removeSong deletes the selected song and pulls the selection to a neighbor.
func (a *app) removeSong() {
	i := a.editSong
	if i < 0 || i >= len(a.songs) {
		return
	}
	a.songs = append(a.songs[:i], a.songs[i+1:]...)
	a.clampEditor()
	a.saveLibrary()
	a.editorStatus = "Song removed"
	a.changed()
}

// addPart appends an empty part and selects it.
func (a *app) addEditorPart() {
	i := a.editSong
	if i < 0 || i >= len(a.songs) {
		return
	}
	part := songPart{Label: fmt.Sprintf("Part %d", len(a.songs[i].Parts)+1), Slides: []string{""}}
	a.songs[i].Parts = append(a.songs[i].Parts, part)
	a.editPart = len(a.songs[i].Parts) - 1
	a.editSlide = 0
	a.clampEditor()
	a.saveLibrary()
	a.editorStatus = "Part added"
	a.changed()
}

// removePart never empties a song: the last part stays with a status note.
func (a *app) removePart() {
	i, j := a.editSong, a.editPart
	if i < 0 || i >= len(a.songs) || j < 0 || j >= len(a.songs[i].Parts) {
		return
	}
	if len(a.songs[i].Parts) <= 1 {
		a.editorStatus = "A song needs at least one part"
		a.changed()
		return
	}
	a.songs[i].Parts = append(a.songs[i].Parts[:j], a.songs[i].Parts[j+1:]...)
	a.clampEditor()
	a.saveLibrary()
	a.editorStatus = "Part removed"
	a.changed()
}

// addSlide appends an empty slide to the selected part.
func (a *app) addSlide() {
	i, j := a.editSong, a.editPart
	if i < 0 || i >= len(a.songs) || j < 0 || j >= len(a.songs[i].Parts) {
		return
	}
	a.songs[i].Parts[j].Slides = append(a.songs[i].Parts[j].Slides, "")
	a.editSlide = len(a.songs[i].Parts[j].Slides) - 1
	a.clampEditor()
	a.saveLibrary()
	a.editorStatus = "Slide added"
	a.changed()
}

// removeSlide never empties a part: the last slide stays with a status note.
func (a *app) removeEditorSlide() {
	i, j, k := a.editSong, a.editPart, a.editSlide
	if i < 0 || i >= len(a.songs) || j < 0 || j >= len(a.songs[i].Parts) {
		return
	}
	if k < 0 || k >= len(a.songs[i].Parts[j].Slides) {
		return
	}
	if len(a.songs[i].Parts[j].Slides) <= 1 {
		a.editorStatus = "A part needs at least one slide"
		a.changed()
		return
	}
	a.songs[i].Parts[j].Slides = append(a.songs[i].Parts[j].Slides[:k], a.songs[i].Parts[j].Slides[k+1:]...)
	a.clampEditor()
	a.saveLibrary()
	a.editorStatus = "Slide removed"
	a.changed()
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
		a.selectSong(first)
		a.editorStatus = "Samples restored"
	} else {
		a.editorStatus = "Samples are already here"
	}
	a.saveLibrary()
	a.changed()
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
		a.editorStatus = "This song has no text yet"
		a.changed()
		return
	}
	a.editorStatus = addedSlidesNote(n)
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
		a.editorStatus = "This part has no text yet"
		a.changed()
		return
	}
	a.preview = len(a.deck) - n
	a.editorStatus = addedSlidesNote(n)
	a.changed()
}

// addedSlidesNote says what the last add put in the run of show.
func addedSlidesNote(n int) string {
	if n == 1 {
		return "Added 1 slide"
	}
	return fmt.Sprintf("Added %d slides", n)
}
