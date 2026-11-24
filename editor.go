package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	editorWidth       = 1280
	editorHeight      = 820
	editorMinWidth    = 1120
	editorMinHeight   = 700
	editorSongsWidth  = 220
	editorSlidesWidth = 260
	editorPadding     = 20
	editorGap         = 16
	editorPreviewMax  = 420
	editorPreviewMin  = 360
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

// editorView is the song editor: the song library, the selected song's slides,
// and an inspector with the preview and the fields for what is selected. It is
// a pure function of a.songs and the edit indices, like every other view.
func (a *app) editorView(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(editorPadding).Gap(12).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Song editor").FontSize(18).FontWeight(600).SingleLine()
			ui.Spacer(c)
			ui.Button(c, "Add song to run of show").OnClick(a.addEditorSongToDeck).
				Disabled(len(a.songs) == 0)
			ui.Button(c, "Close").Label("Close").OnClick(a.closeEditor)
		})
		ui.Row(c).Fill().Grow(1).Gap(editorGap).AlignItems(ui.Stretch).Children(func() {
			a.editorSongs(c)
			a.editorSlides(c)
			a.editorInspector(c)
		})
	})
}

// editorSongs is the library: one row per song, the way the run of show and
// the song books the operator knows are laid out.
func (a *app) editorSongs(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(editorSongsWidth).FillHeight().Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			editorSection(c, "Songs")
			ui.Spacer(c)
			ui.Text(c, fmt.Sprint(len(a.songs))).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
		ui.SearchField(c, &a.editQuery).Label("Search songs")
		ui.Row(c).Gap(6).Children(func() {
			ui.Button(c, "New song").OnClick(a.newSong)
			ui.Button(c, "Restore samples").OnClick(a.restoreSamples)
		})
		if len(a.songs) == 0 {
			ui.Text(c, "No songs yet").FontSize(11).TextColor(t.TextMuted)
			return
		}
		shown := a.matchingSongs()
		if len(shown) == 0 {
			ui.Text(c, "No songs match.").FontSize(11).TextColor(t.TextMuted)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for _, i := range shown {
					i := i
					s := a.songs[i]
					bg := raisedRow(t)
					border := hairlines(t)
					if i == a.editSong {
						bg = rowSelected(t)
						border = rowSelectedEdge(t)
					}
					ui.Column(c).Padding(9, 10).Gap(2).Radius(8).Border(1, border).Background(bg).Label(s.Title).TabStop(true).OnClick(func() { a.selectSong(i) }).Children(func() {
						ui.Text(c, s.Title).FontSize(12).FontWeight(600).SingleLine()
						ui.Text(c, songMeta(s)).FontSize(10).TextColor(t.TextMuted).SingleLine()
					})
				}
			})
		})
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

// editorSlides is the structure of the selected song: a part header followed
// by its slides, so a verse split across slides reads as one block.
func (a *app) editorSlides(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(editorSlidesWidth).FillHeight().Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			editorSection(c, "Slides")
			ui.Spacer(c)
			ui.Text(c, fmt.Sprint(a.slideCount())).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
		})
		if _, ok := a.selectedSong(); !ok {
			ui.Text(c, "Select a song.").FontSize(11).TextColor(t.TextMuted)
			return
		}
		i := a.editSong
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				for j := range a.songs[i].Parts {
					j := j
					part := a.songs[i].Parts[j]
					ui.Column(c).Gap(3).Children(func() {
						head := sunken(t)
						if j == a.editPart {
							head = rowSelected(t)
						}
						ui.Row(c).Padding(6, 8).Radius(6).Background(head).Gap(6).AlignItems(ui.Center).Label(part.Label).TabStop(true).OnClick(func() { a.selectPart(j) }).Children(func() {
							ui.Text(c, part.Label).FontSize(10).FontWeight(600).TextColor(t.TextMuted).Grow(1).SingleLine()
							ui.Text(c, fmt.Sprint(len(part.Slides))).FontSize(10).TextColor(t.TextMuted).SingleLine()
						})
						for k, text := range part.Slides {
							k := k
							line := (slide{Text: text}).firstLine()
							if strings.TrimSpace(line) == "" {
								line = "Empty slide"
							}
							bg := raisedRow(t)
							border := hairlines(t)
							if j == a.editPart && k == a.editSlide {
								bg = rowSelected(t)
								border = rowSelectedEdge(t)
							}
							ui.Row(c).Padding(7, 8).Radius(8).Border(1, border).Gap(8).AlignItems(ui.Center).Background(bg).Label(fmt.Sprintf("%s slide %d", part.Label, k+1)).TabStop(true).OnClick(func() { a.selectEditorSlide(j, k) }).Children(func() {
								ui.Text(c, fmt.Sprint(k+1)).FontSize(10).TextColor(t.TextMuted).Width(14).SingleLine()
								ui.Text(c, line).FontSize(11).Grow(1).SingleLine()
							})
						}
					})
				}
				// The buttons follow the list they add to, inside the
				// scroll, so they sit under the last slide rather than at
				// the foot of the window.
				ui.Row(c).Gap(6).PaddingY(6).Children(func() {
					ui.Button(c, "Add slide").Label("Add slide").OnClick(a.addSlide).Grow(1)
					ui.Button(c, "Add part").Label("Add part").OnClick(a.addEditorPart)
				})
			})
		})
	})
}

// slideCount is the selected song's slide total, the number beside the
// Slides heading.
func (a *app) slideCount() int {
	s, ok := a.selectedSong()
	if !ok {
		return 0
	}
	n := 0
	for _, p := range s.Parts {
		n += len(p.Slides)
	}
	return n
}

// editorInspector edits what is selected: the song's details, the selected
// part's label, and the selected slide's text. Every field is bound to the
// library itself, so a change is saved as it is typed.
func (a *app) editorInspector(c *ui.Context) {
	t := c.Theme()
	i := a.editSong
	if _, ok := a.selectedSong(); !ok {
		ui.Text(c, "Select a song to edit.").FontSize(12).TextColor(t.TextMuted)
		return
	}
	width, _ := c.Size()
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(14).PaddingY(8).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					editorSection(c, "Preview")
					a.editorPreview(c, previewWidth(width))
					if sl, ok := a.songSlide(); ok {
						total := len(a.songs[i].Parts[a.editPart].Slides)
						ui.Text(c, fmt.Sprintf("%s · slide %d of %d", sl.Section, a.editSlide+1, total)).FontSize(11).TextColor(t.TextMuted).SingleLine()
					}
				})
				if a.editPart >= 0 && a.editPart < len(a.songs[i].Parts) && a.editSlide >= 0 && a.editSlide < len(a.songs[i].Parts[a.editPart].Slides) {
					ui.Column(c).Gap(6).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							editorSection(c, "Slide text")
							ui.Spacer(c)
							ui.Button(c, "Remove slide").OnClick(a.removeEditorSlide)
						})
						ui.TextArea(c, &a.songs[i].Parts[a.editPart].Slides[a.editSlide]).Label("Slide text input").Height(130).OnChange(a.songEdited)
					})
				}
				if a.editPart >= 0 && a.editPart < len(a.songs[i].Parts) {
					ui.Column(c).Gap(8).Children(func() {
						a.editorToggle(c, "Part", &a.editorShowPart)
						if a.editorShowPart {
							editorLabel(c, "Part label")
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.TextInput(c, &a.songs[i].Parts[a.editPart].Label).Label("Part label input").Grow(1).OnChange(a.songEdited)
								ui.Button(c, "Remove part").OnClick(a.removePart)
							})
							ui.Button(c, "Add part to run of show").OnClick(a.addEditorPartToDeck)
						}
					})
				}
				ui.Column(c).Gap(10).Children(func() {
					a.editorToggle(c, "Song details", &a.editorShowDetails)
					if !a.editorShowDetails {
						return
					}
					ui.Row(c).Gap(10).Children(func() {
						ui.Column(c).Gap(4).Grow(1).Children(func() {
							editorLabel(c, "Title")
							ui.TextInput(c, &a.songs[i].Title).Label("Song title input").OnChange(a.songEdited)
						})
						ui.Column(c).Gap(4).Width(200).Children(func() {
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
							ui.Text(c, fmt.Sprintf("%.0f pt", a.songs[i].textSize())).FontSize(11).TextColor(t.TextMuted).SingleLine()
						})
						ui.Slider(c, &a.songs[i].TextSize, minTextSize, maxTextSize).Step(1).Label("Text size").OnChange(func() {
							a.setTextSize(a.songs[i].TextSize)
						})
					})
					ui.Divider(c)
					ui.Row(c).Children(func() {
						ui.Button(c, "Remove song").OnClick(a.removeSong)
						ui.Spacer(c)
					})
				})
				if a.editorStatus != "" {
					ui.Text(c, a.editorStatus).FontSize(11).TextColor(t.TextMuted)
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

// editorToggle folds a section's controls away, so the inspector can show
// the preview and the slide text without the other fields.
func (a *app) editorToggle(c *ui.Context, label string, open *bool) {
	hint := "Show"
	if *open {
		hint = "Hide"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Label(label).TabStop(true).OnClick(func() {
		*open = !*open
		a.changed()
	}).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(600).SingleLine()
		ui.Spacer(c)
		ui.Text(c, hint).FontSize(10).TextColor(c.Theme().TextMuted).SingleLine()
	})
}

// editorLabel names the control under it instead of relying on a placeholder
// that disappears once the field has a value.
func editorLabel(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
}

// previewWidth leaves the preview room inside the inspector but never lets it
// squeeze the fields or shrink below the size the text can still be read at.
func previewWidth(windowWidth float32) float32 {
	w := windowWidth - 2*editorPadding - editorSongsWidth - editorSlidesWidth - 2*editorGap
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

// selectedSong is the song the slides column and inspector edit.
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

// selectPart moves the editor to a part and its first slide.
func (a *app) selectPart(j int) {
	if a.editSong < 0 || a.editSong >= len(a.songs) {
		return
	}
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

// newSong appends an empty draft and selects it. It clears the filter, so the
// new row is never hidden by a search the operator forgot about.
func (a *app) newSong() {
	s := song{
		ID:       newSongID("New Song", a.songs),
		Title:    "New Song",
		TextSize: defaultTextSize,
		Parts:    []songPart{{Label: "Verse 1", Slides: []string{""}}},
	}
	a.editQuery = ""
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

// addEditorPart appends an empty part and selects it.
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

// removeEditorSlide never empties a part: the last slide stays with a status
// note.
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
