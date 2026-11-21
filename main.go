package main

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	appTitle    = "mygo-demo"
	appSubtitle = "Worship presenter"

	consoleWidth     = 1280
	consoleHeight    = 800
	consoleMinWidth  = 1040
	consoleMinHeight = 660
)

// The demo account. The app has no backend, no password store and no
// session: signing in only gates the console.
const (
	demoUsername = "admin"
	demoPassword = "admin"
)

// app is the whole state of the console. Views are rebuilt from it on every
// pass, so it only holds plain data and widgets' persistent state, never an
// element, a widget part or a *ui.Context.
type app struct {
	signedIn bool
	username string
	password string
	authErr  string

	tab        int
	songQuery  string
	verseQuery string
	pack       string

	deck     []slide
	preview  int // the row selected in the monitor
	stage    int // the row on air
	blackout bool
	nextID   int
	deckList ui.ListState

	win     *mygo.Window
	live    *mygo.Window
	autoNDI bool

	ndi    *ndiStream
	ndiErr string
}

// newApp builds the console with a small seed set list so a fresh install
// has something on stage.
func newApp() *app {
	a := &app{pack: packs[0].Name}
	a.addSong("amazing-grace")
	a.addVerse("Psalm 23:1")
	a.preview, a.stage = 0, 0
	return a
}

// signIn accepts the demo account and switches the next frame to the
// console. Wrong credentials keep the card and show one generic error.
func (a *app) signIn() {
	if strings.TrimSpace(a.username) != demoUsername || a.password != demoPassword {
		a.authErr = "Wrong username or password."
		return
	}
	a.authErr = ""
	a.password = ""
	a.signedIn = true
	a.preview, a.stage = 0, 0
}

// signOut returns to the login card and drops everything that belonged to
// the session. The username stays for the next attempt.
func (a *app) signOut() {
	a.closeOutput()
	a.stopNDI()
	a.signedIn = false
	a.password = ""
	a.authErr = ""
	a.blackout = false
	a.preview, a.stage = 0, 0
}

// clearAuthError hides a stale error as soon as either field changes.
func (a *app) clearAuthError() {
	a.authErr = ""
}

// changed redraws both windows from the new state. It is called from
// main-thread callbacks after every deck mutation, never from the build
// pass.
func (a *app) changed() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.live != nil {
		a.live.Invalidate()
	}
}

// startNDI publishes the live window as an NDI source. There is nothing to
// publish before the output window is open, so that is a clear error rather
// than a silent no-op.
func (a *app) startNDI() error {
	if !ndiSupported() {
		return nil
	}
	if a.ndi != nil {
		return nil
	}
	if a.live == nil {
		a.ndiErr = "NDI output needs the live window"
		return errors.New(a.ndiErr)
	}
	s, err := startNDISender(a.live)
	if err != nil {
		a.ndiErr = err.Error()
		return err
	}
	a.ndi, a.ndiErr = s, ""
	return nil
}

// stopNDI removes the source; it is safe when nothing is streaming.
func (a *app) stopNDI() {
	if a.ndi == nil {
		return
	}
	a.ndi.Stop()
	a.ndi = nil
}

// toggleNDI is the header button's action.
func (a *app) toggleNDI() {
	if a.ndi != nil {
		a.stopNDI()
		return
	}
	_ = a.startNDI()
}

// ndiLabel names the button after what clicking it does.
func (a *app) ndiLabel() string {
	if a.ndi != nil {
		return "Stop NDI"
	}
	return "Start NDI"
}

func main() {
	a := newApp()
	a.autoNDI = os.Getenv("MYGO_NDI") != ""
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     appTitle,
			Width:     consoleWidth,
			Height:    consoleHeight,
			MinWidth:  consoleMinWidth,
			MinHeight: consoleMinHeight,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
	})
	mygo.App.OnQuit(a.stopNDI)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
