package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	appTitle    = "mygo-demo"
	appSubtitle = "Worship presenter"

	// stageSourceName is how the NDI source finds the app in OBS and the NDI
	// tools. It carries the window's name, not the brand: an operator looks
	// for the stage, not for the product.
	stageSourceName = "mygo-demo (Live)"

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

	songs []song // the active library: library.json once loaded

	deck     []slide
	preview  int // the row selected in the monitor
	stage    int // the row on air
	blackout bool
	nextID   int
	// pendingDrop is a tile dropped in the frame just built, applied by
	// settle once the build is done: ui.Drop delivers during the build.
	pendingDrop *deckDrop
	// menuOpen shows the account pane in the top bar.
	menuOpen bool

	// The run of show on disk: what was last written, whether the file may
	// not be overwritten, and the note that says why.
	showSaved   string
	showBlocked bool
	showNote    string

	// Backups: the day's snapshot is taken once, the file dialogs go
	// through seams the tests answer, and the Settings page reads the note
	// and the restore the operator is being asked about.
	saveFile       func(mygo.SaveDialogOptions) (string, error)
	openFile       func(mygo.OpenDialogOptions) ([]string, error)
	askMessage     func(mygo.MessageOptions) (mygo.MessageResult, error)
	backupDay      string
	backupNote     string
	backupNoteErr  bool
	restoreWaiting *restorePlan

	// The songs folder: empty means the app data directory, and a chosen
	// folder carries the stamp of the file this app last read or wrote, so
	// another machine's write is noticed before it is replaced.
	libraryFolder  string
	libraryStamp   string
	libraryBlocked bool
	libraryNote    string
	folderNote     string
	folderNoteErr  bool
	folderWaiting  *folderPlan

	win  *mygo.Window
	live *mygo.Window

	// Seams, so the window and screen calls the tests cannot make stay in
	// one place and the logic around them runs headless.
	newWindow  func(mygo.WindowOptions) *mygo.Window
	mainScreen func() []mygo.Display

	help              *mygo.Window
	editor            *mygo.Window
	openEditor        bool // the frame asked for the editor window
	editSong          int  // selection in a.songs
	editPart          int
	editSlide         int
	editorStatus      string
	editorStatusErr   bool // editorStatus is a refusal or a failed save
	editQuery         string
	editorTab         int  // which desk tab the editor shows
	newSongOpen       bool // the New song dialog is open
	newSongTitle      string
	newSongAuthor     string
	newSongKey        string
	newSongPart       string
	newSongWords      string
	pasteOpen         bool // the Paste words dialog is open
	pasteText         string
	confirmRemoveSong bool // the Song tab is asking before deleting
	// editorFocus is the slide whose text should take the caret, as a key
	// shared with the text area's FocusBind: choosing, adding or opening a
	// slide puts the caret there, and clicking elsewhere hands it back.
	editorFocus string
	settingsDir string // app data dir; empty keeps tests headless
	version     string // mygo.App.Version(); empty under go run

	// Updates, as the Help menu offers them: the context a check runs under
	// (so quitting cancels it), whether this build can replace itself, and
	// the seams a test stands in for, because a test has no release feed and
	// cannot answer a dialog.
	ctx       context.Context
	updatable bool
	// updateChannel follows the saved preference: false offers stable releases
	// only, true offers the newest beta too.
	updateChannel bool
	setBeta       func(on bool) // the seam for mygo.Updater.SetPrereleases
	updateChk     func(ctx context.Context) (*mygo.Update, error)
	updateAsk     func(mygo.MessageOptions) (mygo.MessageResult, error)
	updateDl      func(ctx context.Context, up *mygo.Update, progress func(downloaded, total int64)) error
	updateRun     func()

	// The stage: where it opens, where the open window went, and whether
	// NDI publishes it.
	stageDisplay     int64         // 0 = windowed on the console's screen
	stageDisplayUsed int64         // display the open stage was placed on
	ndiResolution    int           // frame size the feed publishes
	stop             chan struct{} // the stream's stop channel, while it runs
	// drawNdi is how the feed draws a frame, which a test stands in for; the
	// result is named because ndiFrame lives behind a build tag.
	drawNdi  func(w, h int) (frame any, ok bool) // the frame the feed draws
	ndiOn    bool                                // publish the stage when it opens
	displays []mygo.Display                      // refreshed at startup and on display changes

	// Preferences page. statsWanted is the view's request for a stats
	// timer; statsOn says this frame wanted one.
	page        int
	openStage   bool // the frame asked for the stage window
	statsWanted bool // this frame wanted the stats timer

	ndi    *ndiStream
	ndiErr string
}

// newApp builds the console with a small seed set list so a fresh install
// has something on stage.
func newApp() *app {
	a := &app{
		pack:       packs[0].Name,
		newWindow:  mygo.NewWindow,
		mainScreen: mygo.Screen.Displays,
		setBeta:    mygo.Updater.SetPrereleases,
		updateChk:  mygo.Updater.Check,
		updateAsk:  mygo.Dialog.Message,
		updateDl: func(ctx context.Context, up *mygo.Update, progress func(downloaded, total int64)) error {
			return up.Install(ctx, progress)
		},
		updateRun:  func() { mygo.App.Relaunch() },
		askMessage: mygo.Dialog.Message,
	}
	a.songs = cloneSongs(builtinSongs)
	a.editSong, a.editPart, a.editSlide = -1, -1, -1
	a.editorTab = editorSlideTab
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
	a.closeHelp()
	a.closeEditor()
	a.openEditor = false
	a.page = pageConsole
	a.menuOpen = false
	a.signedIn = false
	a.password = ""
	a.authErr = ""
	a.blackout = false
	a.preview, a.stage = 0, 0
	a.editorTab = editorSlideTab
	a.editorStatus, a.editorStatusErr = "", false
	a.newSongOpen, a.pasteOpen, a.confirmRemoveSong = false, false, false
	a.restoreWaiting = nil
	a.folderWaiting = nil
	a.folderNote, a.folderNoteErr = "", false
	a.backupNote, a.backupNoteErr = "", false
}

// clearAuthError hides a stale error as soon as either field changes.
func (a *app) clearAuthError() {
	a.authErr = ""
}

// changed redraws both windows from the new state. It is called from
// main-thread callbacks after every deck mutation, never from the build
// pass.
func (a *app) changed() {
	a.saveShow()
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.live != nil {
		a.live.Invalidate()
	}
	if a.help != nil {
		a.help.Invalidate()
	}
	if a.editor != nil {
		a.editor.Invalidate()
	}
}

// requestStage opens the stage from the next frame: a window has to come
// from a callback, not from the build pass, and the button may be built
// before the console window exists (a headless test never has one).
func (a *app) requestStage() {
	a.openStage = true
	a.changed()
}

// settle applies what the frame asked for, once it has been built. A stage
// that has to be created is left to a later frame (view asks for one with
// Context.After while openStage is set) so the console draws the frame it
// asked for before a new window works takes the main thread.
func (a *app) settle() {
	// A dropped tile reorders the deck after the frame is built, so the
	// build that saw the drop stays a function of the state it read.
	if d := a.pendingDrop; d != nil {
		a.pendingDrop = nil
		a.dropSlide(*d)
	}
	if a.openEditor {
		a.openEditor = false
		if a.win != nil && a.editor == nil {
			a.openEditorWindow()
		}
	}
	if a.openStage {
		a.openStage = false
		if a.win != nil && a.live == nil {
			// An open stage is brought forward now: no window work
			// remains that the event's own frame would wait for.
			a.openOutput()
		}
	}
}

// tickStats keeps the NDI numbers on the Settings page current with one
// timer on the main loop. It runs only while that page is on screen.
func (a *app) tickStats() {
	if !a.statsWanted {
		return
	}
	time.AfterFunc(statsInterval, func() {
		if a.statsWanted {
			a.win.Invalidate()
		}
		a.tickStats()
	})
}

// startNDI publishes the stage as an NDI source. The feed draws the slide
// itself, so it does not need the output window: one monitor is enough, and
// the window may stay closed or behind the console while the room sees the
// slide.
func (a *app) startNDI() error {
	if !ndiSupported() {
		return nil
	}
	if a.ndi != nil {
		return nil
	}
	w, h := a.ndiFrameSize()
	s, err := startNDISender(a, w, h)
	if err != nil {
		a.ndiErr = err.Error()
		return err
	}
	// The sender exists from here, and the stream draws its frames through
	// a.drawNdiFrame, which runs on the main thread.
	a.ndi, a.ndiErr = s, ""
	return nil
}

// ndiFrameSize is the frame published: 720p by default, 1080p when the
// preference asks for it. Both are 16:9, the stage's shape.
func (a *app) ndiFrameSize() (int, int) {
	if a.ndiResolution == ndiResolution1080 {
		return 1920, 1080
	}
	return 1280, 720
}

// restartNDI swaps the source for one at the current size, keeping the
// preference: the Settings page calls it once the size changes.
func (a *app) restartNDI() {
	if a.ndi == nil {
		return
	}
	a.stopNDI()
	a.ndiErr = ""
	if a.ndiOn {
		_ = a.startNDI()
	}
}

// stopNDI removes the source; it is safe when nothing is streaming.
func (a *app) stopNDI() {
	if a.ndi == nil {
		return
	}
	a.ndi.Stop()
	a.ndi = nil
}

// toggleNDI is the header button: it flips the NDI preference through
// setNDI, so the button and the Settings switch cannot disagree.
func (a *app) toggleNDI() {
	a.setNDI(a.ndi == nil)
}

// ndiLabel names the button after what clicking it does.
func (a *app) ndiLabel() string {
	if a.ndi != nil {
		return "Stop NDI"
	}
	return "Start NDI"
}

// page is the console's page switch.
const (
	pageConsole = iota
	pageSettings
)

// The sizes the feed publishes at, in the order Settings offers them.
// 720p is the default: it is what most projectors and streams take, and it
// is what a capture of the stage window published before.
const (
	ndiResolution720 = iota
	ndiResolution1080
)

// statsInterval is how often a visible NDI stats table is refreshed.
const statsInterval = time.Second

func main() {
	// The check for updates outlives a menu click, so one context owns it and
	// quitting cancels what it is still downloading.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := newApp()
	a.ctx = ctx
	mygo.App.WhenReady(func() {
		// Order matters: the display list is needed to normalize the saved
		// preference, and the menu is built from the version. The list itself
		// is fetched once a window exists: the platform's screen call is not
		// safe before that.
		if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
			a.settingsDir = dir
		} else {
			log.Printf("mygo-demo: no app data directory: %v", err)
		}
		a.loadSettings()
		a.loadLibrary()
		a.loadShow()
		if os.Getenv("MYGO_NDI") != "" {
			// Session-only override: publish without rewriting the preference.
			a.ndiOn = true
		}
		a.version = mygo.App.Version()
		a.updatable = mygo.Updater.Enabled() || updatableOverride
		if a.setBeta != nil {
			a.setBeta(a.updateChannel)
		}
		mygo.App.SetMenu(buildMenu(a))
		mygo.Screen.OnDisplaysChanged(a.displaysChanged)
		a.win = a.newWindow(mygo.WindowOptions{
			Title:     appTitle,
			Width:     consoleWidth,
			Height:    consoleHeight,
			MinWidth:  consoleMinWidth,
			MinHeight: consoleMinHeight,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
		a.displaysChanged()
		a.tickStats()
	})
	mygo.App.OnQuit(a.stopNDI)
	mygo.App.OnQuit(cancel)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
