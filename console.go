package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	// libraryWidth and stagePanelWidth are the console's fixed side columns;
	// the run of show takes whatever the window width leaves.
	libraryWidth    = 296
	stagePanelWidth = 344
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
	c.OnShortcut(0, ui.KeyRight, a.nextSlide)
	c.OnShortcut(0, ui.KeyLeft, a.prevSlide)
	a.shell(c)
}

// login is the demo gate: one card, two fields, one account.
func (a *app) login(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Column(c).Width(380).Padding(30, 28).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(9, 9).Radius(999).Background(t.Accent)
				ui.Text(c, appTitle).FontSize(12).FontWeight(600).TextColor(t.TextMuted).SingleLine()
			})
			ui.Text(c, "Sign in to lead the service.").FontSize(22).Bold()
			ui.Text(c, "Your set list and the live output stay on this machine.").FontSize(11).TextColor(t.TextMuted)

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Username").FontSize(11).TextColor(t.TextMuted).SingleLine()
				ui.TextInput(c, &a.username).Label("Username input").Placeholder(demoUsername).
					OnChange(a.clearAuthError).OnSubmit(a.signIn)
			})
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Password").FontSize(11).TextColor(t.TextMuted).SingleLine()
				ui.TextInput(c, &a.password).Label("Password input").Password().Placeholder("Password").
					OnChange(a.clearAuthError).OnSubmit(a.signIn)
			})

			if a.authErr != "" {
				ui.Text(c, a.authErr).FontSize(11).TextColor(t.Danger).SingleLine()
			}
			ui.PrimaryButton(c, "Sign in").OnClick(a.signIn)
			ui.Text(c, "Demo account: "+demoUsername+" / "+demoPassword).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
	})
}

// shell is the console frame: a top bar and three columns, fixeds on the
// sides and the run of show between them. Every mutating control is wired
// to an app method that changes state and invalidates both windows.
func (a *app) shell(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.topBar(c)
		if a.ndiErr != "" {
			ui.Row(c).Padding(16, 10).Children(func() {
				ui.Text(c, "NDI: "+a.ndiErr).FontSize(11).TextColor(t.Danger).SingleLine()
			})
		}
		ui.Divider(c)
		// Stretch, because a Row centers its children vertically by default:
		// the panels are content-height otherwise.
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.library(c)
			ui.Divider(c)
			a.service(c)
			ui.Divider(c)
			a.stagePanel(c)
		})
	})
}

func (a *app) topBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(16, 12).Gap(10).AlignItems(ui.Center).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(9, 9).Radius(999).Background(t.Accent)
			ui.Text(c, appTitle).FontSize(15).Bold().SingleLine()
		})
		ui.Text(c, appSubtitle).FontSize(11).TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
		a.statusPill(c)
		if a.live == nil {
			ui.Button(c, "Open output").OnClick(a.openOutput)
		} else {
			if a.live.IsFullScreen() {
				ui.Button(c, "Exit full screen").OnClick(a.togglePresent)
			} else {
				ui.Button(c, "Present").OnClick(a.togglePresent)
			}
			ui.Button(c, "Close output").OnClick(a.closeOutput)
			if ndiSupported() {
				ui.Button(c, a.ndiLabel()).OnClick(a.toggleNDI)
			}
		}
		ui.Button(c, "Sign out").OnClick(a.signOut)
	})
}

// statusPill tells the operator what the output is showing right now.
func (a *app) statusPill(c *ui.Context) {
	t := c.Theme()
	label, color := "PREVIEW", t.TextMuted
	if a.blackout {
		label, color = "BLACK", t.Warning
	} else if a.live != nil {
		label, color = "LIVE", t.Success
	}
	ui.Row(c).Padding(9, 5).Gap(6).Radius(999).Background(t.SurfaceHover).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(7, 7).Radius(999).Background(color)
		ui.Text(c, label).FontSize(10).FontWeight(600).TextColor(color).SingleLine()
	})
}

func (a *app) library(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(libraryWidth).FillHeight().Background(t.Surface).Children(func() {
		ui.Tabs(c, &a.tab, "Songs", "Bible").Padding(14, 10)
		if a.tab == 0 {
			a.songLibrary(c)
		} else {
			a.verseLibrary(c)
		}
	})
}

func (a *app) songLibrary(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Gap(8).Children(func() {
		ui.Row(c).Padding(0, 12).Children(func() {
			ui.SearchField(c, &a.songQuery).Label("Search songs").Placeholder("Search songs").Grow(1)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Padding(0, 12, 12).Children(func() {
				found := false
				for _, s := range songs {
					if !songMatches(s, a.songQuery) {
						continue
					}
					found = true
					a.songRow(c, s)
				}
				if !found {
					ui.Text(c, "No songs match.").FontSize(11).TextColor(t.TextMuted).Padding(6)
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

func (a *app) songRow(c *ui.Context, s song) {
	t := c.Theme()
	ui.Row(c).Padding(9, 10).Gap(8).AlignItems(ui.Center).Radius(8).Background(t.Background).Children(func() {
		ui.Column(c).Gap(2).Grow(1).Children(func() {
			ui.Text(c, s.Title).FontSize(12).FontWeight(600).SingleLine()
			ui.Text(c, s.Author+" · "+s.Key).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
		ui.Button(c, "Add").Label("Add " + s.Title).OnClick(func() { a.addSong(s.ID) })
	})
}

func (a *app) verseLibrary(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Gap(8).Children(func() {
		ui.Row(c).Padding(0, 12).Children(func() {
			ui.SearchField(c, &a.verseQuery).Label("Search verses").Placeholder("Search verses").Grow(1)
		})
		ui.Row(c).Padding(0, 12).Children(func() {
			ui.Select(c, &a.pack, packNames()).Grow(1)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Padding(0, 12, 12).Children(func() {
				found := false
				for _, v := range packVerses(a.pack) {
					if !verseMatches(v, a.verseQuery) {
						continue
					}
					found = true
					a.verseRow(c, v)
				}
				if !found {
					ui.Text(c, "No verses match.").FontSize(11).TextColor(t.TextMuted).Padding(6)
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
	ui.Row(c).Padding(9, 10).Gap(8).AlignItems(ui.Center).Radius(8).Background(t.Background).Children(func() {
		ui.Column(c).Gap(2).Grow(1).Children(func() {
			ui.Text(c, v.Ref).FontSize(12).FontWeight(600).SingleLine()
			ui.Text(c, v.Text).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
		ui.Button(c, "Add").Label("Add " + v.Ref).OnClick(func() { a.addVerse(v.Ref) })
	})
}

// service is the run of show: the deck, with the controls that shape it.
func (a *app) service(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		ui.Row(c).Padding(18, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "Run of show").FontSize(16).Bold().SingleLine()
				ui.Text(c, fmt.Sprintf("%d slides", len(a.deck))).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			ui.Button(c, "Move up").Label("Move slide up").OnClick(func() { a.moveSlide(-1) }).
				Disabled(len(a.deck) == 0 || a.index == 0)
			ui.Button(c, "Move down").Label("Move slide down").OnClick(func() { a.moveSlide(1) }).
				Disabled(len(a.deck) == 0 || a.index >= len(a.deck)-1)
			ui.Button(c, "Remove").Label("Remove slide").OnClick(a.removeSlide).
				Disabled(len(a.deck) == 0)
		})
		ui.Divider(c)
		if len(a.deck) == 0 {
			ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "Nothing on stage yet.").FontSize(13).TextColor(t.TextMuted)
				ui.Text(c, "Add a song or verse from the library.").FontSize(11).TextColor(t.TextMuted)
			})
			return
		}
		// The list keeps its own selection state; point it at the app's index
		// and give each row a stable identity every pass.
		a.deckList.Selected = &a.index
		a.deckList.Key = func(i int) any { return a.deck[i].ID }
		a.deckList.Label = func(i int) string {
			return fmt.Sprintf("Slide %d: %s", i+1, a.deck[i].caption())
		}
		ui.List(c, &a.deckList, len(a.deck), func(i int) {
			a.deckRow(c, i)
		}).Grow(1)
	})
}

func (a *app) deckRow(c *ui.Context, i int) {
	t := c.Theme()
	s := a.deck[i]
	bg := t.Background
	if i == a.index {
		bg = t.SurfaceHover
	}
	ui.Row(c).Padding(9, 11).Gap(10).AlignItems(ui.Center).Radius(8).Background(bg).
		Label(fmt.Sprintf("Slide %d: %s", i+1, s.caption())).
		OnClick(func() { a.selectSlide(i) }).Children(func() {
		ui.Text(c, fmt.Sprint(i+1)).FontSize(10).TextColor(t.TextMuted).Width(18).SingleLine()
		ui.Column(c).Gap(2).Grow(1).Children(func() {
			ui.Text(c, s.caption()).FontSize(12).FontWeight(600).SingleLine()
			ui.Text(c, s.firstLine()).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
	})
}

func (a *app) stagePanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(stagePanelWidth).FillHeight().Background(t.Surface).Padding(16).Gap(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "On stage").FontSize(10).FontWeight(600).LetterSpacing(0.8).TextColor(t.TextMuted).SingleLine()
			ui.Spacer(c)
			ui.Text(c, a.stageCounter()).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		a.stageCard(c)
		if up, ok := a.upcomingSlide(); ok {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "Next up").FontSize(10).FontWeight(600).TextColor(t.TextMuted).SingleLine()
				ui.Text(c, up.caption()).FontSize(11).SingleLine()
			})
		} else {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "Next up").FontSize(10).FontWeight(600).TextColor(t.TextMuted).SingleLine()
				ui.Text(c, "End of set").FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
		}
		ui.Spacer(c)
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "Prev").Label("Previous slide").OnClick(a.prevSlide).Grow(1)
			ui.Button(c, "Next").Label("Next slide").OnClick(a.nextSlide).Grow(1)
		})
		if a.blackout {
			ui.Button(c, "Clear black out").OnClick(a.toggleBlackout)
		} else {
			ui.Button(c, "Black out").OnClick(a.toggleBlackout)
		}
		ui.Text(c, "← → to move · B to black out").FontSize(10).TextColor(t.TextMuted).SingleLine()
	})
}

// stageCard previews the slide in the output's own colors, so the operator
// sees roughly what the room sees.
func (a *app) stageCard(c *ui.Context) {
	st := stageTheme()
	ui.Column(c).WidthPercent(100).AspectRatio(16.0/9.0).Radius(10).
		Background(st.Background).Border(1, st.Border).Padding(18).Justify(ui.Center).Children(func() {
		s, ok := a.currentSlide()
		if !ok {
			ui.Text(c, "No slide").FontSize(12).TextColor(st.TextMuted).SingleLine()
			return
		}
		ui.Text(c, s.eyebrow()).FontSize(9).FontWeight(600).LetterSpacing(1.2).TextColor(st.Accent).SingleLine()
		ui.Text(c, s.Text).FontSize(14).LineHeight(1.35).TextColor(st.Text)
		ui.Text(c, s.caption()).FontSize(9).TextColor(st.TextMuted).SingleLine()
	})
}

func (a *app) stageCounter() string {
	if len(a.deck) == 0 {
		return "0 / 0"
	}
	return fmt.Sprintf("%d / %d", a.index+1, len(a.deck))
}
