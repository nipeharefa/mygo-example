package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// signIn opens the console through the login card, failing the test when the
// demo account is rejected.
func signIn(t *testing.T, a *app, tt *ui.Tester) {
	t.Helper()
	a.username, a.password = demoUsername, demoPassword
	if err := tt.Click("Sign in"); err != nil {
		t.Fatal(err)
	}
	if !a.signedIn {
		t.Fatalf("not signed in after the demo account: %q", tt.Texts())
	}
}

// A fresh window is the login card, not the console.
func TestLoginGate(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

	if !tt.HasText("Sign in") {
		t.Fatalf("the login card did not show: %q", tt.Texts())
	}
	if tt.HasText("Run of show") {
		t.Fatalf("the console showed before signing in: %q", tt.Texts())
	}
	if a.signedIn {
		t.Fatal("a fresh app is signed in")
	}
}

// Typing the wrong password keeps the card and shows no console; editing a
// field clears the error.
func TestLoginRejectsWrongPassword(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

	if err := tt.Click("Username input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoUsername)
	if err := tt.Click("Password input"); err != nil {
		t.Fatal(err)
	}
	tt.Type("nope")
	if err := tt.Click("Sign in"); err != nil {
		t.Fatal(err)
	}
	if a.signedIn {
		t.Fatal("signed in with the wrong password")
	}
	if !tt.HasText("Wrong username or password") {
		t.Fatalf("no error for the wrong password: %q", tt.Texts())
	}

	// Typing clears the error, but the card stays.
	if err := tt.Click("Password input"); err != nil {
		t.Fatal(err)
	}
	tt.Type("x")
	if tt.HasText("Wrong username or password") {
		t.Fatalf("the error outlived the edit: %q", tt.Texts())
	}
	if a.signedIn {
		t.Fatal("signed in while correcting the password")
	}
}

// The demo account opens the console and drops the password.
func TestLoginAcceptsDemoAccount(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

	if err := tt.Click("Username input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoUsername)
	if err := tt.Click("Password input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoPassword)
	if err := tt.Click("Sign in"); err != nil {
		t.Fatal(err)
	}

	if !a.signedIn {
		t.Fatalf("not signed in with the demo account: %q", tt.Texts())
	}
	if a.password != "" {
		t.Errorf("password kept after signing in: %q", a.password)
	}
	if !tt.HasText("Run of show") {
		t.Fatalf("the console did not show: %q", tt.Texts())
	}
}

// Enter submits from the password field, after Tab from the username field.
func TestEnterSubmitsLogin(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

	if err := tt.Click("Username input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoUsername)
	tt.Key(0, ui.KeyTab)
	tt.Type(demoPassword)
	tt.Key(0, ui.KeyEnter)

	if !a.signedIn {
		t.Fatalf("Enter did not submit the login: %q", tt.Texts())
	}
}

// openAccountMenu opens the pane that holds Settings, Check for Updates and
// Sign out, the way the console reaches them.
func openAccountMenu(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("Account menu"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}

// The app-level actions live in one pane, not loose in the transport bar, and
// Escape closes it without doing anything.
func TestAccountPaneHoldsAppActions(t *testing.T) {
	_, tt := signedInApp(t)
	for _, loose := range []string{"Settings", "Check for Updates", "Sign out"} {
		if tt.HasText(loose) {
			t.Fatalf("%q is still loose in the bar: %q", loose, tt.Texts())
		}
	}
	openAccountMenu(t, tt)
	for _, want := range []string{"Settings", "Check for Updates", "Sign out"} {
		if !tt.HasText(want) {
			t.Fatalf("the account pane is missing %q: %q", want, tt.Texts())
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText("Sign out") {
		t.Fatalf("Escape did not close the account pane: %q", tt.Texts())
	}
}

// Signing out returns to the card, drops the password and clears the error.
func TestSignOut(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)
	signIn(t, a, tt)

	openAccountMenu(t, tt)
	if err := tt.Click("Sign out"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.signedIn {
		t.Fatal("still signed in after signing out")
	}
	if a.password != "" || a.authErr != "" {
		t.Fatalf("password = %q authErr = %q after signing out", a.password, a.authErr)
	}
	if !tt.HasText("Sign in") {
		t.Fatalf("the login card did not come back: %q", tt.Texts())
	}
}

func signedInApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)
	signIn(t, a, tt)
	return a, tt
}

// The song library lists the songs and offers one Add button per song.
func TestLibraryShowsSongs(t *testing.T) {
	_, tt := signedInApp(t)

	if !tt.HasText("Amazing Grace") {
		t.Fatalf("the song library is empty: %q", tt.Texts())
	}
	if !tt.HasText("All Creatures of Our God and King") {
		t.Fatalf("a song is missing from the library: %q", tt.Texts())
	}
	for _, want := range []string{"Open output", "Run of show", "STAGE · LIVE", "MONITOR · PREVIEW"} {
		if !tt.HasText(want) {
			t.Fatalf("the console is missing %q: %q", want, tt.Texts())
		}
	}
}

// Adding a song appends every section and selects the first one.
func TestAddSongFromLibrary(t *testing.T) {
	a, tt := signedInApp(t)
	a.deck, a.preview, a.stage = nil, 0, 0
	tt.Frame()

	if err := tt.Click("Add Amazing Grace"); err != nil {
		t.Fatal(err)
	}
	if len(a.deck) != 3 {
		t.Fatalf("deck = %d slides, want 3", len(a.deck))
	}
	if !tt.HasText("Verse 1") {
		t.Fatalf("the deck did not show the song: %q", tt.Texts())
	}
}

// A tile names its part under the picture, so a grid of one song's slides
// reads as Verse 1, Verse 2, Verse 3 rather than repeating the song title.
func TestDeckTileShowsItsPart(t *testing.T) {
	a, tt := signedInApp(t)
	a.deck, a.preview, a.stage = nil, 0, 0
	tt.Frame()
	if err := tt.Click("Add Amazing Grace"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	part := a.deck[0].Section
	if part == "" {
		t.Fatal("the first slide has no part")
	}
	if !tt.HasText(part) {
		t.Fatalf("the tile is missing its part %q: %q", part, tt.Texts())
	}
	if !tt.HasText("Amazing Grace · 3 slides") {
		t.Fatalf("the song header is missing: %q", tt.Texts())
	}
}

// The Bible tab swaps in verse packs and adds one verse slide.
func TestBibleTabAddsVerse(t *testing.T) {
	a, tt := signedInApp(t)
	a.deck, a.preview, a.stage = nil, 0, 0
	tt.Frame()

	if err := tt.Click("Bible"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add Psalm 23:1"); err != nil {
		t.Fatal(err)
	}
	if len(a.deck) != 1 {
		t.Fatalf("deck = %d slides, want 1", len(a.deck))
	}
	if got := a.deck[0].Title; got != "Psalm 23:1" {
		t.Fatalf("added %q, want Psalm 23:1", got)
	}
}

// Searching the library narrows the list without touching the deck.
func TestSongSearchFilters(t *testing.T) {
	a, tt := signedInApp(t)
	a.songQuery = "grace"
	tt.Frame()

	if !tt.HasText("Amazing Grace") {
		t.Fatalf("the matching song disappeared: %q", tt.Texts())
	}
	if tt.HasText("Holy, Holy, Holy") {
		t.Fatalf("a non-matching song is still listed: %q", tt.Texts())
	}
}

// Arrow keys move the deck; typing in a field does not.
func TestDeckArrowShortcuts(t *testing.T) {
	a, tt := signedInApp(t)
	if a.preview != 0 {
		t.Fatalf("preview = %d, want 0", a.preview)
	}
	tt.Key(0, ui.KeyRight)
	if a.preview != 1 {
		t.Fatalf("right arrow moved to %d, want 1", a.preview)
	}
	tt.Key(0, ui.KeyLeft)
	if a.preview != 0 {
		t.Fatalf("left arrow moved to %d, want 0", a.preview)
	}

	if err := tt.Click("Search songs"); err != nil {
		t.Fatal(err)
	}
	tt.Type("verse")
	tt.Key(0, ui.KeyRight)
	if a.preview != 0 {
		t.Fatalf("the arrow in the search field moved the monitor to %d", a.preview)
	}
}

// Black out flips the status pill and the transport label.
func TestBlackoutButton(t *testing.T) {
	a, tt := signedInApp(t)

	if err := tt.Click("Black out"); err != nil {
		t.Fatal(err)
	}
	if !a.blackout {
		t.Fatal("blackout not enabled")
	}
	if !tt.HasText("BLACK") {
		t.Fatalf("the status pill does not say BLACK: %q", tt.Texts())
	}
	if err := tt.Click("Clear black out"); err != nil {
		t.Fatal(err)
	}
	if a.blackout {
		t.Fatal("blackout not cleared")
	}
}

// An emptied deck shows its empty states instead of a blank panel.
func TestEmptyDeckState(t *testing.T) {
	a, tt := signedInApp(t)
	a.deck, a.preview, a.stage = nil, 0, 0
	tt.Frame()

	if !tt.HasText("Nothing on stage yet.") {
		t.Fatalf("no empty state in the deck: %q", tt.Texts())
	}
	if !tt.HasText("Nothing on stage") {
		t.Fatalf("no empty state on the stage card: %q", tt.Texts())
	}
	if !tt.HasText("Nothing selected") {
		t.Fatalf("no empty state on the monitor card: %q", tt.Texts())
	}
}

// Choosing a row moves the monitor only; the stage keeps what the room sees.
func TestDeckClickMovesOnlyTheMonitor(t *testing.T) {
	a, tt := signedInApp(t)
	if err := tt.Click("Slide 2: Amazing Grace · Verse 2"); err != nil {
		t.Fatal(err)
	}
	if a.preview != 1 {
		t.Fatalf("preview = %d, want 1", a.preview)
	}
	if a.stage != 0 {
		t.Fatalf("clicking a row moved the stage to %d", a.stage)
	}
	if !tt.HasText("2 / 4") {
		t.Fatalf("the monitor counter did not follow the selection: %q", tt.Texts())
	}
	if !tt.HasText("LIVE") {
		t.Fatalf("the live row lost its badge: %q", tt.Texts())
	}
}

// Every slide is its own thumbnail: three across at the console's width, and
// two once the window reaches its 1040 minimum.
func TestDeckTilesWrapWithTheWindow(t *testing.T) {
	a, tt := signedInApp(t)
	if len(a.deck) != 4 {
		t.Fatalf("deck = %d slides, want the seeded 4", len(a.deck))
	}
	at := func(i int) ui.Rect {
		t.Helper()
		r, ok := tt.Find(fmt.Sprintf("Slide %d: %s", i+1, a.deck[i].caption()))
		if !ok {
			t.Fatalf("the run of show has no tile %d: %q", i+1, tt.Texts())
		}
		return r
	}
	one, two, three, four := at(0), at(1), at(2), at(3)
	if one.Y != two.Y || one.Y != three.Y {
		t.Fatalf("three tiles did not share a row at %d wide: y = %v %v %v", consoleWidth, one.Y, two.Y, three.Y)
	}
	if four.Y <= one.Y {
		t.Fatalf("the fourth tile did not wrap to a second row")
	}
	if three.X+three.W < 850 {
		t.Fatalf("the tile row ends at %.0f, not at the well's edge", three.X+three.W)
	}

	tt.SetSize(consoleMinWidth, consoleHeight)
	tt.Frame()
	one, two, three = at(0), at(1), at(2)
	if one.Y != two.Y {
		t.Fatalf("two tiles did not share a row at %d wide: y = %v %v", consoleMinWidth, one.Y, two.Y)
	}
	if three.Y <= one.Y {
		t.Fatalf("the third tile did not wrap at the minimum width")
	}
}

// The run of show groups tiles under the song they came from, once per
// contiguous run, so a song that comes back later heads its own group again.
func TestDeckGroupsTilesBySong(t *testing.T) {
	a, tt := signedInApp(t)
	a.deck, a.preview, a.stage = nil, 0, 0
	a.addSong("amazing-grace")
	a.addVerse("Psalm 23:1")
	a.addSong("amazing-grace")
	tt.Frame()

	count := func(s string) int {
		n := 0
		for _, text := range tt.Texts() {
			if text == s {
				n++
			}
		}
		return n
	}
	if got := count("Amazing Grace · 3 slides"); got != 2 {
		t.Fatalf("Amazing Grace has %d headers, want 2 (one per run): %q", got, tt.Texts())
	}
	if got := count("Psalm 23:1 · 1 slide"); got != 1 {
		t.Fatalf("Psalm 23:1 has %d headers, want 1: %q", got, tt.Texts())
	}
}

// Enter takes the monitor row without a list to submit for it.
func TestEnterTakesTheSlideWithoutAList(t *testing.T) {
	a, tt := signedInApp(t)
	if err := tt.Click("Slide 2: Amazing Grace · Verse 2"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	if a.stage != 1 {
		t.Fatalf("Enter left the stage at %d, want 1", a.stage)
	}
}

// Each tile carries its own go-live, so the operator does not have to select
// a tile first and then reach for a button elsewhere.
func TestDeckTileGoLiveSendsThatSlide(t *testing.T) {
	a, tt := signedInApp(t)
	if err := tt.Click("Go live slide 3"); err != nil {
		t.Fatal(err)
	}
	if a.stage != 2 {
		t.Fatalf("stage = %d, want 2", a.stage)
	}
	if a.preview != 2 {
		t.Fatalf("preview = %d, want the row that went live", a.preview)
	}
	if !tt.HasText("3 / 4") {
		t.Fatalf("the stage counter did not follow the row: %q", tt.Texts())
	}
}

// Moving a tile from its own button moves that tile, not the monitor tile,
// and leaves the slide on air on air.
func TestDeckTileMoveActsOnThatTile(t *testing.T) {
	a, tt := signedInApp(t)
	if a.stage != 0 || a.preview != 0 {
		t.Fatalf("stage = %d preview = %d, want both at 0", a.stage, a.preview)
	}
	moved := a.deck[2].ID
	live := a.deck[0].ID
	if err := tt.Click("Move slide 3 up"); err != nil {
		t.Fatal(err)
	}
	if len(a.deck) != 4 || a.deck[1].ID != moved {
		t.Fatalf("the third row did not move up one place")
	}
	if a.deck[a.stage].ID != live {
		t.Fatalf("moving a row moved the slide on air")
	}
	if a.preview != 1 {
		t.Fatalf("preview = %d, want the moved row", a.preview)
	}
}

// Removing a tile drops that tile and keeps the stage on the slide it shows.
func TestDeckTileRemoveDropsThatTile(t *testing.T) {
	a, tt := signedInApp(t)
	dropped := a.deck[1].ID
	if err := tt.Click("Remove slide 2"); err != nil {
		t.Fatal(err)
	}
	if len(a.deck) != 3 {
		t.Fatalf("deck = %d slides, want 3", len(a.deck))
	}
	for _, s := range a.deck {
		if s.ID == dropped {
			t.Fatalf("the removed slide is still in the run of show")
		}
	}
	if a.stage != 0 || a.deck[a.stage].Title != "Amazing Grace" {
		t.Fatalf("removing a row moved the slide on air to %d", a.stage)
	}
}

// A run-of-show tile, found by the label the tests and accessibility see.
func tileRect(t *testing.T, tt *ui.Tester, a *app, i int) ui.Rect {
	t.Helper()
	r, ok := tt.Find(fmt.Sprintf("Slide %d: %s", i+1, a.deck[i].caption()))
	if !ok {
		t.Fatalf("no tile %d: %q", i+1, tt.Texts())
	}
	return r
}

// dragTile presses the picture of one tile, moves it over a point, and
// releases there, the way a pointer drags.
func dragTile(tt *ui.Tester, from ui.Rect, toX, toY float32) {
	x := from.X + from.W/2
	y := from.Y + from.H/2
	tt.Press(x, y)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(x+(toX-x)*f, y+(toY-y)*f)
	}
	tt.Release(toX, toY)
	tt.Frame()
}

// Dragging a tile by its picture and dropping it past another tile's middle
// moves the slide there, and the stage stays on the slide it was showing.
func TestDeckTileDragLandsAfter(t *testing.T) {
	a, tt := signedInApp(t)
	live := a.deck[a.stage].ID
	ids := deckIDs(a)
	from, to := tileRect(t, tt, a, 0), tileRect(t, tt, a, 2)
	dragTile(tt, from, to.X+to.W-6, to.Y+to.H/2)

	want := []string{ids[1], ids[2], ids[0], ids[3]}
	for i, id := range want {
		if a.deck[i].ID != id {
			t.Fatalf("deck after the drag = %v, want %v", deckIDs(a), want)
		}
	}
	if a.deck[a.stage].ID != live {
		t.Fatalf("the drag moved the slide on air off the stage")
	}
	if a.preview != 2 {
		t.Fatalf("preview = %d, want the moved slide", a.preview)
	}
}

// Dropping on a tile's first half lands before it.
func TestDeckTileDragLandsBefore(t *testing.T) {
	a, tt := signedInApp(t)
	first := a.deck[0].ID
	from, to := tileRect(t, tt, a, 0), tileRect(t, tt, a, 2)
	dragTile(tt, from, to.X+6, to.Y+to.H/2)

	if a.deck[1].ID != first {
		t.Fatalf("deck after the drag = %v, want the first slide at 2", deckIDs(a))
	}
}

// Dropping a tile on its own halves changes nothing.
func TestDeckTileDragOntoItselfDoesNothing(t *testing.T) {
	a, tt := signedInApp(t)
	before := deckIDs(a)
	own := tileRect(t, tt, a, 1)
	dragTile(tt, own, own.X+own.W-6, own.Y+own.H/2)
	dragTile(tt, own, own.X+6, own.Y+own.H/2)
	if got := deckIDs(a); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatalf("deck after dropping on itself = %v, want %v", got, before)
	}
}

// The picture is the handle, so a press dragged from the caption selects but
// never moves the slide.
func TestDeckTileDragOnlyFromThePicture(t *testing.T) {
	a, tt := signedInApp(t)
	before := deckIDs(a)
	from, to := tileRect(t, tt, a, 0), tileRect(t, tt, a, 2)
	x := from.X + from.W/2
	y := from.Y + from.H + 12
	tt.Press(x, y)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(x+(to.X+to.W/2-x)*f, y+(to.Y+to.H/2-y)*f)
	}
	tt.Release(to.X+to.W/2, to.Y+to.H/2)
	tt.Frame()
	if got := deckIDs(a); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatalf("a drag from the caption reordered the deck: %v", got)
	}
}

// A slide dragged into another song's run heads its own group there, and the
// headers' counts follow the new order.
func TestDeckTileDragRegroups(t *testing.T) {
	a, tt := signedInApp(t)
	verse, target := tileRect(t, tt, a, 3), tileRect(t, tt, a, 1)
	dragTile(tt, verse, target.X+6, target.Y+target.H/2)

	count := func(s string) int {
		n := 0
		for _, text := range tt.Texts() {
			if text == s {
				n++
			}
		}
		return n
	}
	for _, want := range []string{"Amazing Grace · 1 slide", "Psalm 23:1 · 1 slide", "Amazing Grace · 2 slides"} {
		if count(want) != 1 {
			t.Fatalf("missing header %q after the drag: %q", want, tt.Texts())
		}
	}
}

// Go Live sends the monitor row to the stage.
func TestGoLiveTakesTheSlide(t *testing.T) {
	a, tt := signedInApp(t)
	if err := tt.Click("Slide 3: Amazing Grace · Verse 3"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Go Live"); err != nil {
		t.Fatal(err)
	}
	if a.stage != 2 {
		t.Fatalf("stage = %d, want 2", a.stage)
	}
	if !tt.HasText("3 / 4") {
		t.Fatalf("the stage counter did not follow Go Live: %q", tt.Texts())
	}
}

// Cmd/Ctrl+Enter takes the selected row from anywhere in the console.
func TestCommandEnterTakesTheSlide(t *testing.T) {
	a, tt := signedInApp(t)
	if err := tt.Click("Slide 2: Amazing Grace · Verse 2"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyEnter)
	if a.stage != 1 {
		t.Fatalf("Cmd+Enter left the stage at %d, want 1", a.stage)
	}
}

func TestSettingsPageOpensAndCloses(t *testing.T) {
	a, tt := signedInApp(t)
	openAccountMenu(t, tt)
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != pageSettings {
		t.Fatalf("page = %d, want the Settings page", a.page)
	}
	if !tt.HasText("Stage output") {
		t.Fatal("the Settings page should show the stage output section")
	}
	if err := tt.Click("Back to console"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != pageConsole {
		t.Fatalf("page = %d, want the console", a.page)
	}
	if !tt.HasText("Run of show") {
		t.Fatal("the console should be back")
	}
}

func TestSettingsPageIgnoresDeckShortcuts(t *testing.T) {
	a, tt := signedInApp(t)
	a.showSettings()
	tt.Frame()
	before := a.preview
	tt.Key(0, ui.KeyRight)
	if a.preview != before {
		t.Fatalf("preview moved to %d on the Settings page", a.preview)
	}
}

// Check for Updates lives in the account pane too, because the application
// menu is macOS-only: a Windows install shows no menu bar at all.
func TestCheckForUpdatesInTheAccountPane(t *testing.T) {
	a, tt := signedInApp(t)
	a.version = "0.2.0"
	feed := newFeed(&mygo.Update{Version: "0.2.0"}, nil)
	p := updaterFor(t, a, feed)

	openAccountMenu(t, tt)
	if !tt.HasText("Check for Updates") {
		t.Fatalf("the account pane has no Check for Updates: %q", tt.Texts())
	}
	if err := tt.Click("Check for Updates"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-feed.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the button never asked the feed")
	}
	if feed.calls != 1 {
		t.Fatalf("checked %d times, want 1", feed.calls)
	}
	if got := p.dialogs(); len(got) != 1 {
		t.Fatalf("asked %d dialogs, want 1", len(got))
	}
}

// The help window carries the same button, for an operator who opens Help to
// find out how to update.
func TestHelpWindowOffersCheckForUpdates(t *testing.T) {
	a := &app{}
	tt := ui.NewTester(a.helpView, 640, 620)
	if !tt.HasText("Check for Updates") {
		t.Fatalf("the help window has no Check for Updates button: %q", tt.Texts())
	}
}
