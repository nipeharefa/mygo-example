package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	// libraryWidth and stagePanelWidth are the console's fixed side columns;
	// the run of show takes whatever the window width leaves.
	libraryWidth    = 300
	stagePanelWidth = 352
	// deckInset keeps the run of show's tiles off the well's edges, and lines
	// them up with the heading above.
	deckInset = 20
	// deckTileMin is the narrowest a slide tile may be before the grid drops a
	// column; deckTileGap spaces them, and tilePad is the tile's own margin
	// around the picture, caption and controls.
	deckTileMin = 160
	deckTileGap = 12
	tilePad     = 8
)

// Text sizes, in the theme's points. They are named here because the console
// repeats them: a tile title, its caption, a zone heading and a tracked
// label are each one size everywhere.
const (
	textLabel   = 10 // tracked uppercase labels and counters
	textSmall   = 11 // second lines, hints, counters
	textBody    = 12 // tiles, buttons' labels
	textTitle   = 13 // a zone heading
	textSection = 15 // the run of show's heading
	textDisplay = 20 // a page title
)

// view draws the login card or the console. It is a pure function of app:
// the same state always produces the same interface, and nothing built here
// survives the pass.
func (a *app) view(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	if !a.signedIn {
		a.login(c)
		return
	}
	// Window shortcuts for the console. Text fields take the arrow keys
	// first, so typing in the search box never moves the deck.
	if a.page == pageSettings {
		// The Settings page keeps only its own way out.
		c.OnWindowShortcut(ui.Cmd, ui.KeyComma, a.showConsole)
		c.OnWindowShortcut(ui.Ctrl, ui.KeyComma, a.showConsole)
		c.OnWindowShortcut(ui.Cmd, ui.KeyEscape, a.showConsole)
		c.OnWindowShortcut(ui.Ctrl, ui.KeyEscape, a.showConsole)
	} else {
		c.OnShortcut(0, ui.KeyRight, a.nextSlide)
		c.OnShortcut(0, ui.KeyLeft, a.prevSlide)
		c.OnShortcut(0, ui.KeyB, a.toggleBlackout)
		// The run of show is a grid of tiles, not a list, so Enter needs a
		// handler of its own: a focused control still takes the key first.
		c.OnWindowShortcut(0, ui.KeyEnter, a.takeSlide)
		c.OnWindowShortcut(ui.Cmd, ui.KeyEnter, a.takeSlide)
		c.OnWindowShortcut(ui.Ctrl, ui.KeyEnter, a.takeSlide)
		c.OnWindowShortcut(ui.Cmd, ui.KeyComma, a.showSettings)
		c.OnWindowShortcut(ui.Ctrl, ui.KeyComma, a.showSettings)
	}
	// An open output waits for a turn of the main loop that is not this
	// frame: asking for it here lets the loop show the console first (see
	// settle and openOutput).
	if a.openStage {
		c.After(openStageDelay)
	}
	a.shell(c)
	a.settle()
}

// login is the demo gate: one card, two fields, one account. The card sits on
// a faint warm wash so it reads as the only lit thing in a dark room, the same
// idea the stage uses.
func (a *app) login(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Column(c).Width(400).PaddingY(30).PaddingX(32).Gap(0).Radius(16).Background(t.Surface).
			Border(1, t.Border).Shadow(0, 18, 48, 0, ui.Color{A: 36}).Children(func() {
			a.brandMark(c, textBody)
			ui.Column(c).Gap(5).PaddingY(18).Children(func() {
				ui.Text(c, "Sign in to lead the service.").FontSize(24).FontWeight(700).SingleLine()
				ui.Text(c, "Your set list and the live output stay on this machine.").
					FontSize(textBody).TextColor(t.TextMuted)
			})
			ui.Column(c).Gap(6).PaddingY(8).Children(func() {
				fieldLabel(c, "Username")
				ui.TextInput(c, &a.username).Label("Username input").Placeholder(demoUsername).
					OnChange(a.clearAuthError).OnSubmit(a.signIn)
			})
			ui.Column(c).Gap(6).PaddingY(8).Children(func() {
				fieldLabel(c, "Password")
				ui.TextInput(c, &a.password).Label("Password input").Password().Placeholder("Password").
					OnChange(a.clearAuthError).OnSubmit(a.signIn)
			})
			if a.authErr != "" {
				ui.Row(c).PaddingY(6).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(999).Background(t.Danger)
					ui.Text(c, a.authErr).FontSize(textSmall).TextColor(t.Danger).SingleLine()
				})
			}
			ui.Column(c).PaddingY(10).Children(func() {
				ui.PrimaryButton(c, "Sign in").OnClick(a.signIn)
			})
			ui.Row(c).PaddingY(4).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Demo account").FontSize(textLabel).FontWeight(600).TextColor(t.TextMuted).SingleLine()
				ui.Text(c, demoUsername+" / "+demoPassword).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
			})
		})
	})
}

// shell is the console frame: a brand bar, a transport bar, and three zones
// under them. Every mutating control is wired to an app method that changes
// state and invalidates both windows.
func (a *app) shell(c *ui.Context) {
	t := c.Theme()
	a.statsWanted = false
	// The bar is drawn on the console's own background, not on a raised
	// surface: the three columns below are the raised parts, so the frame
	// reads as one desk with a well and two lit side panels.
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.topBar(c)
		hrule(c)
		if a.ndiErr != "" {
			a.errorStrip(c, "NDI: "+a.ndiErr)
		}
		if a.page == pageSettings {
			a.settingsBody(c)
		} else {
			// Stretch, because a Row centers its children vertically by
			// default: the panels are content-height otherwise.
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				a.library(c)
				vrule(c)
				a.service(c)
				vrule(c)
				a.stagePanel(c)
			})
		}
	})
}

// topBar is the brand row and the transport row. The brand row holds what
// says where the operator is (the app, the page, what the room sees); the
// transport row holds what does something, in the order an operator reaches
// for it.
func (a *app) topBar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Children(func() {
		ui.Row(c).Padding(18, 12).Gap(10).AlignItems(ui.Center).Children(func() {
			a.brandMark(c, textSection)
			if a.page == pageSettings {
				ui.Text(c, "Settings").FontSize(textBody).TextColor(t.TextMuted).SingleLine()
			} else {
				a.statusPill(c)
			}
			ui.Spacer(c)
			a.transport(c)
		})
	})
}

// brandMark is the beacon dot and the app's name, the one element every
// window carries.
func (a *app) brandMark(c *ui.Context, size float32) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(9, 9).Radius(999).Background(t.Accent)
		ui.Text(c, appTitle).FontSize(size).FontWeight(700).SingleLine()
		if size <= textBody {
			return
		}
		ui.Text(c, appSubtitle).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
	})
}

// transport is the run of buttons that act on the output. Grouped, because
// the stage controls and the moving parts of the app are different jobs: the
// stage group changes the room, the rest changes the console.
func (a *app) transport(c *ui.Context) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if a.live == nil {
			ui.PrimaryButton(c, "Open output").OnClick(a.requestStage)
		} else {
			label := "Present"
			if a.live.IsFullScreen() {
				label = "Exit full screen"
			}
			ui.PrimaryButton(c, label).OnClick(a.togglePresent)
			if ndiSupported() {
				ui.Button(c, a.ndiLabel()).OnClick(a.toggleNDI)
			}
			ui.Button(c, "Close output").OnClick(a.closeOutput)
		}
		vruler(c)
		ui.Button(c, "Song editor").OnClick(a.requestEditor)
		vruler(c)
		a.accountMenu(c)
	})
}

// accountMenu is the bar's one app-level control: a pane holding Settings,
// Check for Updates and Sign out. They are together so the bar stays about
// the service, and they are in the window rather than only in the menu bar,
// which Windows does not have.
func (a *app) accountMenu(c *ui.Context) {
	t := c.Theme()
	btn := ui.Button(c, "Account").Label("Account menu").Key("account-menu")
	btn.OnClick(func() {
		a.menuOpen = !a.menuOpen
		a.changed()
	})
	pane := ui.Popover(c, btn, &a.menuOpen, func() {
		ui.Column(c).Width(216).Gap(2).Children(func() {
			ui.Text(c, "ACCOUNT").FontSize(textLabel).FontWeight(700).LetterSpacing(0.9).
				TextColor(t.TextMuted).Padding(6, 9).SingleLine()
			a.accountItem(c, "Settings", "account-settings", func() {
				a.menuOpen = false
				a.showSettings()
			})
			a.accountItem(c, "Check for Updates", "account-updates", func() {
				a.menuOpen = false
				a.checkForUpdates()
			})
			ui.Column(c).Padding(4, 0).Children(func() {
				ui.Box(c).Height(1).FillWidth().Background(hairlines(t))
			})
			a.accountItem(c, "Sign out", "account-signout", func() {
				a.menuOpen = false
				a.signOut()
			})
		})
	})
	// Escape or a click outside closes the pane; the bar redraws without it.
	if pane.Dismissed() {
		a.changed()
	}
}

// accountItem is one row of the account pane: a clickable, focusable row with
// the pane's hover, like a menu item drawn in the console's own theme.
func (a *app) accountItem(c *ui.Context, label, key string, fn func()) {
	t := c.Theme()
	item := ui.Row(c).FillWidth().Padding(7, 9).Radius(6).AlignItems(ui.Center).
		Label(label).Key(key).TabStop(true).Cursor(ui.CursorPointer).OnClick(fn)
	if item.Hovered() {
		item.Background(t.SurfaceHover)
	}
	item.Children(func() {
		ui.Text(c, label).FontSize(textBody).SingleLine()
	})
}

// statusPill tells the operator what the output is showing right now. With
// no output open it says STANDBY rather than PREVIEW: the monitor tile is
// always a preview, so the pill only claims a state when the room has one.
func (a *app) statusPill(c *ui.Context) {
	t := c.Theme()
	label, color := "STANDBY", t.TextMuted
	if a.blackout {
		label, color = "BLACK", t.Warning
	} else if a.live != nil {
		label, color = "LIVE", onAirColor(t)
	}
	ui.Row(c).PaddingY(5).PaddingX(10).Gap(6).Radius(999).AlignItems(ui.Center).
		Border(1, color.Alpha(0.35)).Background(color.Alpha(0.10)).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(color)
		ui.Text(c, label).FontSize(textLabel).FontWeight(700).LetterSpacing(0.8).TextColor(color).SingleLine()
	})
}

// errorStrip is a full-width note under the bar, for a failure the operator
// has to know about while the service runs.
func (a *app) errorStrip(c *ui.Context, message string) {
	t := c.Theme()
	ui.Row(c).Padding(18, 8).Gap(8).AlignItems(ui.Center).Background(t.Danger.Alpha(0.10)).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(t.Danger)
		ui.Text(c, message).FontSize(textSmall).TextColor(t.Danger).SingleLine()
	})
}

// library is the source column: Songs or Bible, searched and added from.
func (a *app) library(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(libraryWidth).FillHeight().Background(t.Surface).Children(func() {
		ui.Row(c).Padding(16, 12, 0).Children(func() {
			ui.Tabs(c, &a.tab, "Songs", "Bible").PaddingY(2)
		})
		if a.tab == 0 {
			a.songLibrary(c)
		} else {
			a.verseLibrary(c)
		}
	})
}

func (a *app) songLibrary(c *ui.Context) {
	ui.Column(c).Grow(1).Gap(8).Padding(0, 10).Children(func() {
		ui.Row(c).Padding(16, 0, 0).Children(func() {
			ui.SearchField(c, &a.songQuery).Label("Search songs").Placeholder("Search songs").Grow(1)
		})
		count := 0
		for _, s := range a.songs {
			if songMatches(s, a.songQuery) {
				count++
			}
		}
		a.zoneLabel(c, fmt.Sprintf("%d songs", count))
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(4).Padding(12, 0, 16).Children(func() {
				found := false
				for _, s := range a.songs {
					if !songMatches(s, a.songQuery) {
						continue
					}
					found = true
					a.songRow(c, s)
				}
				if !found {
					a.emptyNote(c, "No songs match.", "Try a title or an author.")
				}
			})
		})
	})
}

func songMatches(s song, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s.Title), q) ||
		strings.Contains(strings.ToLower(s.Author), q)
}

// songRow is a library row: the song on the left, the one action it has on
// the right. It is a raised card inside the column so the Add button reads as
// a control, not as part of the row.
func (a *app) songRow(c *ui.Context, s song) {
	t := c.Theme()
	ui.Row(c).Padding(10, 9).Gap(8).AlignItems(ui.Center).Radius(8).
		Background(raisedRow(t)).Border(1, hairlines(t)).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, s.Title).FontSize(textBody).FontWeight(600).SingleLine()
			ui.Text(c, s.Author+" · "+s.Key).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
		})
		ui.Button(c, "Add").Label("Add " + s.Title).OnClick(func() { a.addSong(s.ID) })
	})
}

func (a *app) verseLibrary(c *ui.Context) {
	ui.Column(c).Grow(1).Gap(8).Padding(0, 10).Children(func() {
		ui.Row(c).Padding(16, 0, 0).Children(func() {
			ui.SearchField(c, &a.verseQuery).Label("Search verses").Placeholder("Search verses").Grow(1)
		})
		ui.Row(c).Padding(16, 0, 0).Children(func() {
			ui.Select(c, &a.pack, packNames()).Grow(1)
		})
		a.zoneLabel(c, fmt.Sprintf("%d verses", len(packVerses(a.pack))))
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(4).Padding(12, 0, 16).Children(func() {
				found := false
				for _, v := range packVerses(a.pack) {
					if !verseMatches(v, a.verseQuery) {
						continue
					}
					found = true
					a.verseRow(c, v)
				}
				if !found {
					a.emptyNote(c, "No verses match.", "Try a reference or a few words.")
				}
			})
		})
	})
}

func packNames() []string {
	names := make([]string, len(packs))
	for i, p := range packs {
		names[i] = p.Name
	}
	return names
}

func packVerses(name string) []verse {
	for _, p := range packs {
		if p.Name == name {
			return p.Verses
		}
	}
	return nil
}

func verseMatches(v verse, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(v.Ref), q) ||
		strings.Contains(strings.ToLower(v.Text), q)
}

func (a *app) verseRow(c *ui.Context, v verse) {
	t := c.Theme()
	ui.Row(c).Padding(10, 9).Gap(8).AlignItems(ui.Center).Radius(8).
		Background(raisedRow(t)).Border(1, hairlines(t)).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, v.Ref).FontSize(textBody).FontWeight(600).SingleLine()
			ui.Text(c, v.Text).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
		})
		ui.Button(c, "Add").Label("Add " + v.Ref).OnClick(func() { a.addVerse(v.Ref) })
	})
}

// service is the run of show: the deck, with the controls that shape it. It
// sits in a well, and every slide is a tile, grouped under the song it came
// from, so the operator scans the room's next hour as pictures.
func (a *app) service(c *ui.Context) {
	t := c.Theme()
	cols, tile := deckGrid(c)
	ui.Column(c).Grow(1).FillHeight().Background(sunken(t)).Children(func() {
		ui.Row(c).Padding(16, deckInset, 8, deckInset).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "Run of show").FontSize(textSection).FontWeight(700).SingleLine()
				ui.Text(c, a.deckSummary()).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
			})
		})
		if len(a.deck) == 0 {
			a.deckEmpty(c)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, deckInset, 16, deckInset).Gap(18).Children(func() {
				for _, g := range deckGroups(a.deck) {
					a.deckGroup(c, g, cols, tile)
				}
			})
		})
	})
}

// deckGroup is a run of consecutive slides from one song, or one verse
// reference, in deck order. A song that comes back later starts a new group,
// because the run of show is the service order, not the library's order.
type deckGroup struct {
	title    string
	from, to int // deck rows [from, to)
}

// deckGroups splits the deck into those runs without moving anything.
func deckGroups(deck []slide) []deckGroup {
	out := make([]deckGroup, 0, len(deck))
	for i, s := range deck {
		if i > 0 && deck[i-1].Kind == s.Kind && deck[i-1].Title == s.Title {
			out[len(out)-1].to = i + 1
			continue
		}
		out = append(out, deckGroup{title: s.Title, from: i, to: i + 1})
	}
	return out
}

// deckGrid is how many tiles a row holds and how wide each one is: as many as
// fit at deckTileMin, sharing the well's width. It reads the window because
// the well is what the two side columns leave.
func deckGrid(c *ui.Context) (cols int, tile float32) {
	window, _ := c.Size()
	usable := window - libraryWidth - stagePanelWidth - 2 - 2*deckInset
	if usable < deckTileMin {
		usable = deckTileMin
	}
	cols = int((usable + deckTileGap) / (deckTileMin + deckTileGap))
	cols = max(cols, 1)
	tile = (usable - float32(cols-1)*deckTileGap) / float32(cols)
	return cols, tile
}

// deckGroup draws one song's header and its tiles, filled left to right.
func (a *app) deckGroup(c *ui.Context, g deckGroup, cols int, tile float32) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Label(fmt.Sprintf("%s · %s", g.title, slideCount(g.to-g.from))).Children(func() {
			ui.Text(c, g.title).FontSize(textLabel).FontWeight(700).LetterSpacing(0.9).
				TextColor(t.Text).SingleLine()
			ui.Text(c, slideCount(g.to-g.from)).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
			ui.Box(c).Height(1).Grow(1).Background(hairlines(t))
		})
		for from := g.from; from < g.to; from += cols {
			to := min(from+cols, g.to)
			ui.Row(c).Gap(deckTileGap).AlignItems(ui.Start).Children(func() {
				for i := from; i < to; i++ {
					a.deckTile(c, i, tile)
				}
			})
		}
	})
}

// slideCount is the "3 slides" beside a group's title.
func slideCount(n int) string {
	if n == 1 {
		return "1 slide"
	}
	return fmt.Sprintf("%d slides", n)
}

// deckTile is one slide: the whole slide drawn at the tile's size, the part
// under it, and its own controls. The picture is slideBody through the same
// metrics the stage and the editor use, so it is the room's slide made small
// rather than a different sketch of it.
func (a *app) deckTile(c *ui.Context, i int, width float32) {
	t := c.Theme()
	s := a.deck[i]
	onAir := i == a.stage
	selected := i == a.preview
	bg, border := raisedRow(t), hairlines(t)
	if selected {
		bg, border = rowSelected(t), rowSelectedEdge(t)
	}
	frame := stageCardBorder()
	if onAir {
		frame = onAirColor(t)
	}
	inner := width - 2*tilePad
	m := metricsFor(inner, s.TextSize)
	tile := ui.Column(c).Width(width).Padding(tilePad).Gap(6).Radius(10).Background(bg).
		Border(1, border).
		OnClick(func() { a.selectSlide(i) })
	// The tile takes the dragged picture: the half of it under the pointer
	// decides which side the slide lands on.
	if _, ok := ui.DragOver[deckDrag](tile); ok {
		after := dragLandsAfter(tile, width)
		tile.DrawOver(func(p *ui.Painter, r ui.Rect) {
			x := r.X
			if after {
				x = r.X + r.W - 2
			}
			p.Fill(ui.Rect{X: x, Y: r.Y + 4, W: 2, H: r.H - 8}, t.Accent, 1)
		})
	}
	if d, ok := ui.Drop[deckDrag](tile); ok {
		a.pendingDrop = &deckDrop{from: d.from, at: i, after: dragLandsAfter(tile, width)}
	}
	tile.Children(func() {
		// The picture is the handle and the tile's name: MyGo starts a drag
		// from the element the press is in, so pressing a control beside it
		// never drags the tile, and a plain click still selects.
		ui.Column(c).Width(inner).Height(inner*9/16).Radius(6).
			Background(stageSurface()).Border(1, frame).
			Padding(m.PadX, m.PadY).Justify(ui.Center).AlignItems(ui.Center).
			Label(fmt.Sprintf("Slide %d: %s", i+1, s.caption())).
			Cursor(ui.CursorGrab).Drag(deckDrag{from: i}).
			OnClick(func() { a.selectSlide(i) }).Children(func() {
			slideBody(c, s, m, stageTheme())
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(textLabel).FontWeight(600).
				TextColor(t.TextMuted).Width(16).SingleLine()
			ui.Text(c, tileCaption(s)).FontSize(textSmall).TextColor(t.TextMuted).SingleLine().Grow(1)
			if onAir && a.blackout {
				blackBadge(c)
			} else if onAir {
				onAirBadge(c, "LIVE")
			}
		})
		a.deckActions(c, s, i, onAir)
	})
}

// dragLandsAfter reports whether the pointer has passed the middle of a tile,
// so a dragged slide lands after it rather than before it.
func dragLandsAfter(tile ui.Element, width float32) bool {
	x, _, over := tile.PointerPosition()
	return over && x > width/2
}

// tileCaption names the slide's place inside its group: the part of a song,
// or the kind of verse, since the group header already gives the title.
func tileCaption(s slide) string {
	if s.Kind == slideVerse {
		return "Scripture"
	}
	if s.Section == "" {
		return "Song"
	}
	return s.Section
}

// deckActions are the tile's own controls: send it to the room, move it, or
// drop it. They sit in a toolbar, so they stay faceless glyphs until hovered
// or focused instead of painting four chips on every tile, and the slide id in
// Key lets automation find them without an index.
func (a *app) deckActions(c *ui.Context, s slide, i int, onAir bool) {
	ui.Toolbar(c, func() {
		actionButton(c, "▶", fmt.Sprintf("Go live slide %d", i+1), "deck-take-"+s.ID).
			OnClick(func() { a.takeSlideAt(i) }).Disabled(onAir)
		actionButton(c, "↑", fmt.Sprintf("Move slide %d up", i+1), "deck-up-"+s.ID).
			OnClick(func() { a.moveSlideAt(i, -1) }).Disabled(i == 0)
		actionButton(c, "↓", fmt.Sprintf("Move slide %d down", i+1), "deck-down-"+s.ID).
			OnClick(func() { a.moveSlideAt(i, 1) }).Disabled(i == len(a.deck)-1)
		actionButton(c, "✕", fmt.Sprintf("Remove slide %d", i+1), "deck-remove-"+s.ID).
			OnClick(func() { a.removeSlideAt(i) })
	}).Label(fmt.Sprintf("Slide %d actions", i+1))
}

// actionButton is one of a tile's small controls: a glyph in a 24x24 target,
// the smallest WCAG asks for, and a name for the screen reader and the tests.
func actionButton(c *ui.Context, glyph, label, key string) ui.Element {
	return ui.Button(c, glyph).Label(label).Key(key).FontSize(textSmall).
		Width(24).Height(24).Padding(0)
}

// deckSummary is the line under the run of show's heading: how many slides,
// and which one the monitor is on.
func (a *app) deckSummary() string {
	if len(a.deck) == 0 {
		return "No slides yet"
	}
	// Short by design: the line has to fit the well at the window's minimum
	// width.
	return fmt.Sprintf("%d slides · %d of %d", len(a.deck), a.preview+1, len(a.deck))
}

// deckEmpty is what the run of show says before the first song is added: it
// names the next step instead of only reporting the absence.
func (a *app) deckEmpty(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(8).Padding(0, 0, 40).Children(func() {
		ui.Box(c).Size(34, 3).Radius(999).Background(t.Accent.Alpha(0.35))
		ui.Text(c, "Nothing on stage yet.").FontSize(textTitle).FontWeight(600)
		ui.Text(c, "Add a song or verse from the library.").FontSize(textBody).TextColor(t.TextMuted)
		ui.Row(c).Gap(14).Padding(6, 0, 0).Children(func() {
			a.keyHint(c, "← →", "monitor")
			a.keyHint(c, "↵", "go live")
			a.keyHint(c, "B", "black out")
		})
	})
}

// stagePanel has two cards: the stage, what the room and NDI see, and the
// monitor, the tile the operator is working on. Go Live is the only way the
// stage changes without the live window's own keys.
func (a *app) stagePanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(stagePanelWidth).FillHeight().Background(t.Surface).Children(func() {
		ui.Scroll(c).Fill().Padding(16).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				a.panelHeader(c, "STAGE · LIVE", a.stage, true)
				if a.blackout {
					a.blackCard(c)
				} else {
					a.slideCard(c, a.stageSlide, "Nothing on stage", true)
				}
				a.panelHeader(c, "MONITOR · PREVIEW", a.preview, false)
				a.slideCard(c, a.previewSlide, "Nothing selected", false)

				ui.Column(c).Gap(8).PaddingY(4).Children(func() {
					if a.blackout {
						ui.PrimaryButton(c, "Clear black out").OnClick(a.toggleBlackout)
					} else {
						ui.PrimaryButton(c, "Go Live").OnClick(a.takeSlide).Disabled(len(a.deck) == 0)
					}
					ui.Row(c).Gap(8).Children(func() {
						ui.Button(c, "Prev").Label("Previous slide").OnClick(a.prevSlide).Grow(1)
						ui.Button(c, "Next").Label("Next slide").OnClick(a.nextSlide).Grow(1)
						if !a.blackout {
							ui.Button(c, "Black out").OnClick(a.toggleBlackout)
						}
					})
				})
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					a.keyHint(c, "← →", "monitor")
					a.keyHint(c, "↵", "go live")
					a.keyHint(c, "B", "black out")
				})
			})
		})
	})
}

// panelHeader labels a card with its state and its position in the deck.
func (a *app) panelHeader(c *ui.Context, label string, i int, live bool) {
	t := c.Theme()
	dot := t.TextMuted
	if live {
		dot = onAirColor(t)
	}
	ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(dot)
		ui.Text(c, label).FontSize(textLabel).FontWeight(700).LetterSpacing(0.9).
			TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
		ui.Text(c, a.deckCounter(i)).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
	})
}

// slideCard renders one slide in the output's own colors, so both cards look
// like the room does. The stage card keeps its aura while the monitor card
// stays quiet: only one of them can be on air, so only one may glow.
func (a *app) slideCard(c *ui.Context, get func() (slide, bool), empty string, live bool) {
	ui.Column(c).WidthPercent(100).AspectRatio(16.0/9.0).Radius(10).
		Background(stageSurface()).Border(1, stageCardBorder()).
		Shadow(0, 0, 22, 0, glow(c.Theme())).
		Padding(16).Justify(ui.Center).Gap(5).Children(func() {
		s, ok := get()
		if !ok {
			ui.Text(c, empty).FontSize(textBody).TextColor(mutedOnStage()).SingleLine()
			return
		}
		// The room sees the words alone, so the card shows the words alone:
		// the panel's own header says which card is which.
		ui.Text(c, s.Text).FontSize(11).LineHeight(1.35).TextColor(inkOnStage()).
			MaxWidthPercent(100)
	})
}

// blackCard is the stage card while the room is dark. It keeps the card's
// shape, so the panel does not jump, and says why it is empty.
func (a *app) blackCard(c *ui.Context) {
	ui.Column(c).WidthPercent(100).AspectRatio(16.0/9.0).Radius(10).
		Background(dimOnStage()).Border(1, stageCardBorder()).
		Padding(14).Justify(ui.Center).Children(func() {
		ui.Text(c, "BLACKED OUT").FontSize(8).FontWeight(700).LetterSpacing(1.2).
			TextColor(beaconOnStage()).SingleLine()
		ui.Text(c, "The room is dark; the deck is held.").FontSize(textBody).
			TextColor(mutedOnStage()).SingleLine()
	})
}

// deckCounter is "row / total" for either card, or 0 / 0 on an empty deck.
func (a *app) deckCounter(i int) string {
	if len(a.deck) == 0 {
		return "0 / 0"
	}
	return fmt.Sprintf("%d / %d", i+1, len(a.deck))
}

// zoneLabel is the small tracked line that names what a list holds.
func (a *app) zoneLabel(c *ui.Context, label string) {
	t := c.Theme()
	ui.Text(c, label).FontSize(textLabel).FontWeight(600).LetterSpacing(0.8).
		TextColor(t.TextMuted).Padding(16, 2, 0, 4).SingleLine()
}

// emptyNote is a list's empty state: what happened, then the way out.
func (a *app) emptyNote(c *ui.Context, title, hint string) {
	t := c.Theme()
	ui.Column(c).Gap(2).Padding(8, 12).Children(func() {
		ui.Text(c, title).FontSize(textBody).FontWeight(600).TextColor(t.TextMuted).SingleLine()
		ui.Text(c, hint).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
	})
}

// fieldLabel names an input above it, so the placeholder can disappear
// without taking the label with it.
func fieldLabel(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(textSmall).TextColor(c.Theme().TextMuted).SingleLine()
}

// keyHint is one keyboard shortcut and what it does, the form the console
// uses wherever it teaches a key.
func (a *app) keyHint(c *ui.Context, keys, what string) {
	t := c.Theme()
	ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Text(c, keys).FontSize(textLabel).FontWeight(600).TextColor(t.Text).
			Padding(4, 2).Radius(4).Background(innerWell(t)).SingleLine()
		ui.Text(c, what).FontSize(textLabel).TextColor(t.TextMuted).SingleLine()
	})
}

// onAirBadge marks the one row the room is showing. It is filled, unlike
// every other label on the row, so it cannot be read as a status light.
func onAirBadge(c *ui.Context, label string) {
	t := c.Theme()
	ui.Row(c).PaddingY(3).PaddingX(9).Radius(999).Background(onAirColor(t)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(textLabel).FontWeight(700).LetterSpacing(0.7).
			TextColor(t.Background).SingleLine()
	})
}

// blackBadge takes the place of the LIVE badge while the room is dark: the
// row is still the one on stage, but nothing is on screen.
func blackBadge(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).PaddingY(2).PaddingX(8).Radius(999).Background(t.Warning.Alpha(0.18)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "BLACK").FontSize(9).FontWeight(700).LetterSpacing(0.7).
			TextColor(t.Warning).SingleLine()
	})
}

// hrule is the hairline between the console's rows, and vrule the one
// between its columns. They are lighter than a control's border on purpose.
func hrule(c *ui.Context) {
	ui.Row(c).Height(1).FillWidth().Background(hairlines(c.Theme()))
}

func vrule(c *ui.Context) {
	ui.Column(c).Width(1).FillHeight().Background(hairlines(c.Theme()))
}

// vruler is the short vertical rule that groups the top bar's buttons.
func vruler(c *ui.Context) {
	ui.Column(c).Width(1).Height(18).Background(hairlines(c.Theme()))
}
