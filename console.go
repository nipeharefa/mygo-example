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
	// deckWidth is the run of show's reading column: rows wider than this
	// leave the eye travelling across empty ground to reach the badge.
	deckWidth = 560
)

// Text sizes, in the theme's points. They are named here because the console
// repeats them: a row title, its second line, a zone heading and a tracked
// label are each one size everywhere.
const (
	textLabel   = 10 // tracked uppercase labels and counters
	textSmall   = 11 // second lines, hints, counters
	textBody    = 12 // rows, buttons' labels
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
		ui.Button(c, "Settings").Label("Settings").OnClick(a.showSettings)
		ui.Button(c, "Check for Updates").OnClick(a.checkForUpdates)
		vruler(c)
		ui.Button(c, "Sign out").OnClick(a.signOut)
	})
}

// statusPill tells the operator what the output is showing right now. With
// no output open it says STANDBY rather than PREVIEW: the monitor row is
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
// sits in a well, so the rows that will go on air are the only raised things
// in the middle of the console.
func (a *app) service(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Background(sunken(t)).Children(func() {
		ui.Row(c).PaddingY(16).PaddingX(24).Gap(8).AlignItems(ui.Center).Children(func() {
			// The title is the column's name; it never truncates. When the
			// window is at its minimum the buttons take what is left instead,
			// so the heading folds to its first word.
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "Run of show").FontSize(textSection).FontWeight(700).SingleLine()
				ui.Text(c, a.deckSummary()).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
			})
			ui.Button(c, "Up").Label("Move slide up").OnClick(func() { a.moveSlide(-1) }).
				Disabled(len(a.deck) == 0 || a.preview == 0)
			ui.Button(c, "Down").Label("Move slide down").OnClick(func() { a.moveSlide(1) }).
				Disabled(len(a.deck) == 0 || a.preview >= len(a.deck)-1)
			ui.Button(c, "Remove").Label("Remove slide").OnClick(a.removeSlide).
				Disabled(len(a.deck) == 0)
		})
		if len(a.deck) == 0 {
			a.deckEmpty(c)
			return
		}
		// The list keeps its own selection state; point it at the app's index
		// and give each row a stable identity every pass.
		a.deckList.Selected = &a.preview
		a.deckList.Key = func(i int) any { return a.deck[i].ID }
		a.deckList.Label = func(i int) string {
			return fmt.Sprintf("Slide %d: %s", i+1, a.deck[i].caption())
		}
		// Dragging a row reorders the deck; the stage stays on whatever it is
		// airing. Move up/down is the same rule from the keyboard.
		ui.Column(c).Grow(1).Padding(8, 0, 16).AlignItems(ui.Center).Children(func() {
			ui.List(c, &a.deckList, len(a.deck), func(i int) {
				a.deckRow(c, i)
			}).Grow(1).MaxWidth(deckWidth).FillWidth().OnSubmit(a.takeSlide)
		})
	})
}

// deckSummary is the line under the run of show's heading: how many slides,
// and which one the monitor is on.
func (a *app) deckSummary() string {
	if len(a.deck) == 0 {
		return "No slides yet"
	}
	// Short by design: the line has to fit the column at the window's
	// minimum width, beside three buttons.
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

func (a *app) deckRow(c *ui.Context, i int) {
	t := c.Theme()
	s := a.deck[i]
	bg := raisedRow(t)
	border := hairlines(t)
	onAir := i == a.stage
	selected := i == a.preview
	if selected {
		bg = rowSelected(t)
		border = rowSelectedEdge(t)
	}
	ui.Row(c).Padding(10, 10).Gap(12).AlignItems(ui.Center).Radius(8).Background(bg).
		Border(1, border).
		Label(fmt.Sprintf("Slide %d: %s", i+1, s.caption())).
		OnClick(func() { a.selectSlide(i) }).Children(func() {
		// The position is a monospaced-ish stamp: a fixed width keeps every
		// title in the list starting on the same line.
		ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(textSmall).FontWeight(600).
			TextColor(t.TextMuted).Width(20).SingleLine()
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			title := ui.Text(c, s.Title).FontSize(textBody).FontWeight(600).SingleLine()
			if selected {
				title.TextColor(t.Text)
			}
			ui.Text(c, deckSubline(s)).FontSize(textSmall).TextColor(t.TextMuted).SingleLine()
		})
		if onAir && a.blackout {
			blackBadge(c)
		} else if onAir {
			onAirBadge(c, "LIVE")
		} else if selected {
			monitorBadge(c)
		}
	})
}

// deckSubline names the part above the first line of a song slide, so the
// run of show reads as title first and part second.
func deckSubline(s slide) string {
	line := s.firstLine()
	if s.Kind == slideSong && s.Section != "" {
		return s.Section + " · " + line
	}
	return line
}

// stagePanel has two cards: the stage, what the room and NDI see, and the
// monitor, the row the operator is working on. Go Live is the only way the
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
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(5, 5).Radius(999).Background(beaconOnStage())
			ui.Text(c, s.eyebrow()).FontSize(8).FontWeight(700).LetterSpacing(1.2).
				TextColor(beaconOnStage()).SingleLine()
		})
		// Two lines, because a thumbnail that drops the rest of the verse
		// makes the operator check the stage window instead of the console.
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

// monitorBadge marks the row the operator is on when it is not the one on
// air: a hairline outline and no fill.
func monitorBadge(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).PaddingY(2).PaddingX(8).Radius(999).Border(1, t.Accent.Alpha(0.5)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "MONITOR").FontSize(9).FontWeight(700).LetterSpacing(0.7).
			TextColor(t.Accent).SingleLine()
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
