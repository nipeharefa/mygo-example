package main

import (
	"testing"

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

// Signing out returns to the card, drops the password and clears the error.
func TestSignOut(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)
	signIn(t, a, tt)

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

// Enter on the focused run of show takes the row without a global handler.
func TestEnterTakesTheSlide(t *testing.T) {
	a, tt := signedInApp(t)
	if err := tt.Click("Slide 2: Amazing Grace · Verse 2"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	if a.stage != 1 {
		t.Fatalf("Enter left the stage at %d, want 1", a.stage)
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
