# Architecture

`mygo-demo` is a worship presenter: a console window where an operator
builds a run of show, a stage window that shows one slide at a time to the
room, and Settings and Help windows opened on demand. Everything is local
and in memory, and every window is rebuilt from one `app` struct. This
document covers the state, the four windows, the theme split, the NDI path,
and what the tests pin.

## What runs

`main()` builds the `app` and waits for `mygo.App.WhenReady`. Inside that
callback, in order, it resolves the app data directory
(`mygo.App.Path(mygo.PathUserData)`), reads the display list, loads the
saved settings, maps `MYGO_NDI=1` onto the session's NDI preference, reads
the packaged version, installs the application menu, subscribes to display
changes, and then opens the console window with `mygo.NewWindow`:

- Title `mygo-demo`, 1280×800, minimum 1040×660, `StateKey: "main"`.
- `Content: ui.View(a.view)`.

`mygo.App.OnQuit(a.stopNDI)` makes quitting the app unregister any NDI
source, and `mygo.App.Run()` runs the main loop. There is no second
process and no HTTP server; the files are `settings.json` and
`library.json` in the app data directory.

The console's view sets the console theme, draws the login card while
`!a.signedIn`, and otherwise registers the console shortcuts and draws the
shell. The stage is a separate `mygo.Window` created later by
`openOutput`, with its own `ui.View(a.liveView)`; the Song editor is another
window, created by `openEditorWindow`; Settings is a page of the console and
Help is a further window, created by `openHelp`. Every window's view is a
pure builder, and `changed()` invalidates all of them.

## State

`app` in `main.go` is plain data plus widget state; it never holds an
element or a context:

| Field | Meaning |
| --- | --- |
| `signedIn`, `username`, `password`, `authErr` | the demo gate |
| `tab` | 0 = Songs, 1 = Bible |
| `songQuery`, `verseQuery`, `pack` | library filters; `pack` is the selected verse pack name |
| `deck []slide`, `preview int`, `stage int` | the run of show, the tile selected in the monitor, and the slide on air |
| `blackout bool` | the stage shows a black frame, deck unchanged |
| `nextID int` | slide id counter |
| `menuOpen bool` | the Account pane in the top bar is open |
| `pendingDrop *deckDrop` | a tile dropped in this frame, which `settle` applies after the build |
| `win`, `live`, `editor`, `help *mygo.Window` | the console, stage, Song editor and Help windows; Settings is a page of the console |
| `songs []song` | the song library, loaded from `library.json` or seeded from `builtinSongs` |
| `openEditor bool` | the current frame asked to open the Song editor after it is built |
| `editSong`, `editPart`, `editSlide int` | what the Song editor has selected; −1 means nothing is selected |
| `editQuery`, `editorStatus string` | the editor's song filter and its last save/add note |
| `displays []mygo.Display`, `stageDisplay int64` | the display list and the chosen stage display (0 = a window) |
| `stageDisplayUsed int64` | the display the open stage was actually placed on |
| `page int` | `pageConsole` or `pageSettings` |
| `openStage`, `statsWanted bool` | what the current frame asked for: open the stage after it is built, keep the stats timer running |
| `newWindow`, `mainScreen` | seams over `mygo.NewWindow` and `mygo.Screen.Displays`, so logic is testable without an application |
| `ndiOn bool`, `ndi *ndiStream`, `ndiErr string` | NDI preference, sender and last error |
| `ndiResolution int` | frame size: `ndiResolution720` (default) or `ndiResolution1080` |
| `drawWait time.Duration` | a test hook: a delay inside the drawn feed, so Stop can be proven not to wait for a draw |
| `settingsDir string`, `version string` | where `settings.json` lives and what Help reports |
| `ctx context.Context` | the session's context; quit cancels it, which drops a check or a download still running |
| `updatable bool` | whether this build can replace itself (`mygo.Updater.Enabled()` once ready) |
| `updateChk`, `updateAsk`, `updateDl`, `updateRun` | seams over `mygo.Updater.Check`, `mygo.Dialog.Message`, the download, and `mygo.App.Relaunch` |

`newApp()` picks the first pack, clones `builtinSongs` into `songs`, and
seeds the deck with Amazing Grace and Psalm 23:1, so a fresh run has
something to show. `slide` carries an ID (`s1`, `s2`, …), a title, a section
label, the text, a text size and a kind (`slideSong` or `slideVerse`);
`caption` names it in the console and `firstLine` opens it in lists. A
song is `{ID, Title, Author, Key, TextSize, Parts []songPart}`, and a part
is `{Label, Slides []string}`: the label becomes the slide's section, and
each slide string is one stage frame.

Every mutation goes through a method on `app` (`addSong`, `addVerse`,
`selectSlide`, `nextSlide`, `prevSlide`, `takeSlide`, `nextStage`,
`prevStage`, `removeSlide`, `moveSlide`, `toggleBlackout`, `setNDI`, …) and
ends in `changed()`, which invalidates every open window. The monitor
methods move `a.preview` only; `takeSlide` copies the monitor tile's slide
to the stage, and the stage's own next/prev pull the monitor along so the
highlight follows the room. `nextSlide`/`prevSlide` clamp at the ends;
`removeSlide` repairs the stage index when the removed slide was on air or
above it; `moveSlide` swaps and remaps the stage so the on-air content does
not change.

A tile's own controls reach the same methods by slide index instead of by
the monitor: `takeSlideAt`, `moveSlideAt` and `removeSlideAt` each pull
`a.preview` to that slide first, so the tile an operator clicks is the
slide that moves, goes live or goes away, whatever the monitor was on. The
stage-window keys and Cmd/Ctrl+Enter keep the monitor-based entry points.

## The frame contract

Views are pure builders with the same rules as any MyGo app:

- `a.view` and `a.liveView` build the whole tree from `app` on every pass.
  Nothing built survives the pass, so no element, widget part or
  `*ui.Context` is stored anywhere on `app`.
- Callbacks (`OnClick`, `OnSubmit`, `OnChange`, `OnShortcut`) run before
  the rebuild; they mutate `app` and call `changed()`. They never call into
  the build.
- The run of show is not a `ui.List`: it is a plain scroll of tiles with
  song headers between them, so `deckGroups` derives the headers from the
  deck on every pass and nothing about the grouping is stored. A tile's
  label (`Slide 3: Amazing Grace · Verse 3`) is what `ui.NewTester` and
  accessibility see, and the slide's ID is its `Key` on each control, so
  the same song can appear twice without confusing either.
- Shortcuts fall through: the console registers ← / → / B on the root and
  Enter, Cmd/Ctrl+Enter and Cmd/Ctrl+, on the window. A focused button or
  text field takes Enter first, so pressing it there presses that control,
  not the run of show; with the focus anywhere else, Enter takes the
  monitor row. Tiles themselves are clickable but not focusable: the arrows
  already move the monitor, and each tile's toolbar is the keyboard path to
  its actions.
- A `ui.Row` centers its children vertically by default; the console's
  three-column row sets `.AlignItems(ui.Stretch)` so the panels fill the
  height.

## Signing in

The card is the demo gate, not security. `demoUsername`/`demoPassword` are
constants in `main.go`; `signIn` trims the username and compares the
password exactly, showing one generic `Wrong username or password.` error
otherwise. A successful sign-in clears the password, sets `signedIn`, and
resets the selection. `clearAuthError` runs on either field's change.

`signOut` closes the stage, Settings and Help windows, stops NDI, and
clears session state including blackout. The display and NDI preferences
survive it on purpose; the run of show does not.

## The windows

### Console

`shell` is a column: a top bar, an optional NDI error strip, a divider, and
the three-column body.

- **Top bar** — brand dot, `mygo-demo`, `Worship presenter` subtitle, a
  status pill (`PREVIEW`, `BLACK` or `LIVE`), then transport buttons. When
  no output is open it shows **Open output**; once open it shows
  **Present**/**Exit full screen**, **Close output**, and the NDI toggle
  (only where `ndiSupported()`). Then **Song editor**, and last the
  **Account** pane (`accountMenu`): a `ui.Popover` holding **Settings**,
  **Check for Updates** and **Sign out**. They live together so the bar
  stays about the service, and in the window rather than only in the menu
  bar, which Windows and Linux do not have; Escape or a click outside
  closes it.
- **Library** (296 wide, `Surface`) — `ui.Tabs` Songs/Bible. Songs has a
  search box and a scrolling list of `songRow`s with an **Add** button
  whose accessible label is `Add <title>`. Bible has a search box, the
  verse-pack `ui.Select`, and `verseRow`s with `Add <ref>` buttons.
- **Run of show** (grow) — a heading over the slide count, then either an
  empty state or a scrolling grid of slide tiles, full width inside the
  well (`deckInset`). The deck is grouped into runs of the same song: a
  header (`Amazing Grace` · `3 slides` · a hairline) opens each run, and a
  song that comes back later heads its own group again, because the deck is
  the service order. `deckGroups` splits it; `deckGrid` fits as many tiles
  per row as the well allows at `deckTileMin` (three at 1280, two at the
  1040 minimum) and shares the width among them. A tile is the whole slide
  drawn through `slideBody` and `metricsFor` at the tile's size, in the
  stage theme: its text alone, the room's slide made small rather than a
  second sketch of it. Under it: its position, its part
  (`Verse 1`), a `LIVE` badge (or `BLACK` while blacked out) and a
  `ui.Toolbar` of glyph buttons: **▶** go live, **↑** / **↓** move, **✕**
  remove. Glyphs because four text buttons would not fit a tile; each
  carries an accessible name (`Go live slide 3`) and a stable `Key` with
  the slide id (`deck-take-s3`), which automation drives without an index.
  The toolbar stays faceless until hovered or focused, so a grid of tiles
  is not a grid of chips. Clicking a tile selects it for the monitor; the
  tile on air carries the copper frame, and selection is the tile's tint
  and edge.
- **Dragging a tile** — the picture is the handle (`Drag(deckDrag{from})`,
  grab cursor): MyGo starts a drag from the element a press is in, so a
  press on **▶** / **✕** beside it is never a drag, and a press that moves
  nothing is still a click. Every tile takes the dragged picture
  (`ui.DragOver` / `ui.Drop`); the half under the pointer decides the side,
  and a 2px accent bar on that edge shows where the slide will land. A
  drop is recorded in `pendingDrop` during the build and applied by
  `settle` after it, because `ui.Drop` delivers during the build: the frame
  that saw the drop still read the deck it started with. `dropSlide` lands
  the slide and `reorderSlides` keeps the stage on its slide by ID; the
  groups re-derive, so a slide dropped into another song's run heads its
  own group there.
- **Stage panel** (344 wide, `Surface`) — two 16:9 cards drawn in the
  stage theme: **STAGE · LIVE** (what the room sees, with `n / total`) and
  **MONITOR · PREVIEW** (the tile selected in the run of show), then **Go
  Live**, **Prev** / **Next** (which move the monitor), **Black out** /
  **Clear black out**, and a shortcut hint. The panel scrolls, so it still
  fits on a short window.

The console root listens for ← / → (monitor), B (black out), and Cmd/Ctrl+
Enter (go live) via `c.OnShortcut`/`c.OnWindowShortcut`; text fields consume
their keys while focused, so typing in a search box does not move the deck.

### Stage

`openOutput` creates the window only once:

- Title `mygo-demo — Live`, 1280×720, minimum 640×360, and
  `BackgroundColor: stageBackgroundHex` so it never flashes white before
  the first frame.
- `Frameless: true`, so the slide is the whole window: no title bar,
  traffic lights or resize edges, on screen or in the NDI feed. The slide
  column calls `DragWindow()`, so dragging anywhere on the slide moves a
  windowed output.
- `placeStage` asks `stageDisplayFound` for the chosen display. When it
  returns one, the window's position and size are set to that display's
  work area and it opens fullscreen; otherwise it is an ordinary window on
  the current screen, and the operator can drag it to a projector and press
  **Present**. The frame it was placed on is remembered in
  `stageDisplayUsed`, and `OnEnterFullScreen` re-asserts the placement, so
  a fullscreen transition cannot leave it on the wrong display.

`liveView` draws a black fill while `a.blackout`, and otherwise the
stage slide (`a.stageSlide()`, not the monitor row) as its text alone, centered at the song's size on the forced-dark stage theme. Its
shortcuts are → ↓ Space next, ← ↑ prev, B blackout, and Esc, which clears
fullscreen when `a.live.IsFullScreen()` and closes the output otherwise —
the only way out of a window with no title bar.

`openOutput` runs from a frame that no event waits on: `requestStage` sets
`a.openStage`, `settle()` hands it to the next pass through
`c.After(openStageDelay)`, and `openOutput` then creates the window. Creating
and showing a window inside the click that asked for it, and then placing it
with `SetPosition`/`SetFullScreen`, left the main thread stuck on a window
that had not been drawn yet — Open output looked like "not responding". With
a window already open, `openOutput` just calls `Show()` and `Focus()`. `shutStage` nils `a.live` first and invalidates the console;
`OnClosed` calls it guarded with `if a.live != live { return }`, so a late
close event cannot clear a window that was reopened meanwhile. Nilling
before `Close()` matters: a `Close()` whose `OnClosed` guard is already
satisfied never reaches the backend's `destroy()`. `closeOutput` =
`shutStage`, then leave fullscreen, then `Close()`. The stage is a viewer
only: NDI does not start with it and does not stop with it (see "NDI
output"), so a feed can run with no stage window at all.

The stage opens once and stays there. `placeStage` is idempotent: it moves
and goes fullscreen only when the place actually changed, and calls
`SetFullScreen(false)` only on a window that reports fullscreen, so carrying
it to a display never toggles AppKit twice. Nothing reopens it from a display
notification: `refreshDisplays` saves a preference whose display went away,
and when `stageDisplayUsed` itself is gone it closes the stage and asks for a
new one (`requestStage`), so the window is created in a later frame. That is what used to loop — a recreated fullscreen window posted
another display change from inside `openOutput`.

### Settings

Settings is a page of the console (`a.page = pageSettings`), reached from
**Settings** in the **Account** pane, the **Cmd/Ctrl+,** shortcut or the
menu, and left
with **Back to console**, **Cmd/Ctrl+,** again or **Cmd/Ctrl+Esc**. While it
is open the deck shortcuts (arrows, B, Enter) are not registered, so a
keystroke cannot move the monitor behind the page. It shows the stage output
radios (**Windowed on this screen** plus one per display, each with label,
role and size), the NDI switch and status line, the **NDI stream** stats,
and a short data note. Choosing a display or toggling NDI saves
immediately; a display change reopens an open stage on the new display.
The **Stage output size** radios (720p, the default, or 1080p) go through
`ndiResolutionChanged`, which saves and restarts a running sender at the
new size.

The stats are the sender's own: resolution and measured frame rate of the
last frame, frames sent, frames skipped when a draw was not ready in time,
up time and the last error, from `ndiStream.snapshot()`. NDI 5 and 6 expose no
receiver tally, so this describes the stream, not its audience. While the
page is open and NDI is publishing, `console.go` sets `statsWanted` for the
frame and `tickStats` re-arms a one-second `time.AfterFunc` that
invalidates the console — a main-thread timer, never a goroutine touching
the view.

### Song editor

**Song editor** in the console header asks for a separate window:
`requestEditor` sets `a.openEditor`, `settle()` clears the flag in the next
pass and calls `openEditorWindow`, which reuses an open window or creates
one with `a.newWindow` — Title `mygo-demo · Song editor`, 1280×820, minimum
1120×700, `StateKey: "editor"`, `Content: ui.View(a.editorView)`, and an
`OnClosed` guard of `if a.editor != w { return }`. `closeEditor` nils the
pointer before `Close()`, so a stale close cannot drop a new window.

The window is three columns: a 220-wide song list (search, `New song`,
`Restore samples`, one clickable row per song), a 260-wide slide list (one
header per part, one row per slide), and a scrolling inspector with the 16:9
preview, the selected slide's text, the selected part's label, and the song's
title, author, key and text size. The preview is capped at 420 wide and
floored at 360, and `previewWidth` derives that from `c.Size()`. `editorPreview`
draws the slide through the same `slideBody` and `metricsFor` as `liveView`, so
what the editor shows is what the stage shows at that size. The **Part** and
**Song details** headings carry a `Hide` / `Show` hint and fold their controls
away; the choice lasts for the window, not for `library.json`.

Every metadata field, label, text area and the slider updates the `songs`
model and calls `songEdited`/`setTextSize`, which clamp the size, save the
whole `library.json` and set `editorStatus` immediately; there is no Save
button. `addEditorSongToDeck` adds every part through `addSongParts`, and
`addEditorPartToDeck` adds only the selected part through `addPartSlides`;
both leave the stage row alone and put the monitor on the first added slide.
Removing the last part or the last slide is refused with a status note. The
editor does not persist the run of show: a slide copies its text and text
size when added, so later edits do not rewrite what is already on the list.

### Updates

**Check for Updates** is a row in the **Account** pane, and a button on the
help page, because the application menu is macOS-only: MyGo marks the Help
menu role as macOS-only, and a Windows or Linux install shows no menu bar at
all. The button runs `checkForUpdates`: the feed is asked on a
goroutine and the main thread waits for the answer, then `showCheck` puts up
one dialog — the newest version, the failure, or an offer of the new
version with **Install and Restart** and **Not Now**. Choosing to install
runs `installUpdate`, which downloads on a goroutine while the main thread
waits, and restarts the app through `updateRun` when the download verifies.

Waiting on the main thread is deliberate: `Dialog.Message` pumps the main
loop while the operator reads the dialog, and the loop runs what it is
handed one function at a time, so a step posted back with `mygo.RunOnMain`
from inside a callback that is itself waiting would only run after that
callback returned. The dialog would never get its result. The click is also
the only thing the loop is waiting for at that point, so the interface
behind the dialog keeps drawing and the automation socket keeps answering.

A build says it cannot update, and why, when `Updater.Enabled()` is false:
a `go run`/`go build` binary has no version and no feed, and an install a
package manager owns cannot be replaced in place.

### Help

`openHelp` creates the window once. It lists the About block (version, demo
account), the keyboard map, the NDI notes (source name, per-platform
runtime), and the sample-data licence note. The menu's About item opens the
native about panel on macOS and this window elsewhere.

## NDI output

NDI publishes the **stage slide**, never the console and never a window.
The feed is drawn: `ndi_render.go` runs `ui.Render(a.liveView, …)` over the
stage view on the main thread and turns the result into the BGRA frame the
sender sends. Nothing reads window pixels, so the feed is identical on
every platform, needs no second monitor, and keeps running with the stage
window closed, behind the console, or on another Space. `startNDI` never
looks at `a.live`.

The frame path, in three layers:

- `startNDISender(a, w, h)` creates an **asynchronous** sender
  (`clockVideo` 0): the NDI thread takes the frames and sends them, so sending
  never runs on the main loop and never waits for a consumer there. The app
  itself is the `ndiFrameSource` (`(*app) frame`), and nothing is drawn
  inside the create call.
- `ndiStream.draw` runs the source on a helper goroutine and selects on the
  stream's stop channel, so **Stop never waits for a draw**:
  `mygo.RunOnMain` blocks its caller until the loop runs the function, and a
  frame loop waiting on that would deadlock its own Stop.
- `(*app) frame` skips the draw once the stream stopped, draws inline in a
  headless process (`a.win == nil`), and otherwise posts `drawNdiFrame`:
  the draw runs on the main thread, then `win.Invalidate()` asks the console
  for another pass, and the picture goes out with that pass. A draw that
  would have to wait for the very pass that asked for it is skipped rather
  than blocking — the loop is asynchronous, so a missed frame costs nothing.
- `renderStageFrame` draws `a.liveView` at a fixed 1280×720 layout
  (`ndiStageWidth`/`ndiStageHeight`) with a scale that covers the requested
  frame, then `shrinkFrame` averages it down to the frame size. The stage
  stays laid out for 16:9, so no frame size crops the slide;
  `TestRenderStageFrameFitsAnOddLargerFrame` pins that for odd and larger
  requests.

`ndiFrameSize` caps a request at 1920×1080 without upscaling;
`a.ndiFrameSize` asks for 1280×720 or 1920×1080 from the saved preference.
`renderFrame` reorders the rendered RGBA into BGRA, top row first, stride
`4*W`, because the NDI frame is bottom-up only when asked to be — it is not
here, so the top row goes first. (A two-byte Go swap `dst[q],
dst[q+2] = src[q+2], src[q]` compiled to a wrong result on Go 1.27.3
darwin/arm64; `bgrA` names all four bytes and `TestRenderBGRA` pins it.)

The platform split is described in `AGENTS.md`: `ndi_sender.go` owns
runtime discovery, the versioned function table, the sender and the frame
loop; `ndi_darwin.go`, `ndi_windows.go` and `ndi_linux.go` only make the
dynamic-library calls (purego on macOS and Linux, `syscall` on Windows).
The source is registered as `mygo-demo (Live)` (`stageSourceName`).

`MYGO_NDI=1` sets `a.ndiOn` for the session so the sender starts at launch;
the Settings switch and the console header button both go through
`setNDI`, which saves the preference and starts or stops the sender. Only
`setNDI` and sign-out stop it: closing the stage keeps publishing,
`restartNDI` bounces a running sender when the frame size changes, and
quitting the app unregisters it through `mygo.App.OnQuit(a.stopNDI)`.

## Themes

Themes are per frame, so both views set theirs on every pass:

- `consoleTheme(c)` starts from `c.Theme()` and recolors it into warm
  paper-and-amber (light) or dark stone-and-amber (dark), then applies
  `WithAccent` and `WithPreferences(c.Preferences())` so the system accent,
  contrast and text scale still work.
- `stageTheme()` starts from `ui.DarkTheme()` and forces the output dark,
  because a projector in a dim room should not be white. `liveView` and the
  console's stage preview both use it, so the preview matches the room.
- `stageBackgroundHex` is the only reason a literal color appears outside
  a theme: `WindowOptions.BackgroundColor` takes a CSS string.

Literal hex values live only in `theme.go`; every other view reads
`c.Theme()`.

## Data model

`data.go` holds the built-in library and its seed data:

- 5 traditional, public-domain hymns (Amazing Grace, Holy, Holy, Holy, Be
  Thou My Vision, All Creatures of Our God and King, A Mighty Fortress Is
  Our God) as `builtinSongs`. Each is a `song{ID, Title, Author, Key,
  TextSize, Parts []songPart}`, and each part is `{Label, Slides []string}`:
  All Creatures' verse is split into two slides, and the rest carry one.
- 4 verse packs (Assurance, Comfort, Praise, Sending) with 4 King James
  Version verses each (16 total), looked up by `ref` through `verseByRef`.

The song library is `a.songs`, not the package variable: on first launch
`loadLibrary` writes `builtinSongs` to `library.json`, and afterwards that
file is the library. `findSong`/`(a *app) songByID` look songs up there;
`normalizeLibrary` drops empty or duplicate ids and guarantees every song
has at least one part with at least one slide, so the editor always has
something to select. `cloneSongs` deep-copies the nested slices, because a
deck slide must not alias the library text.

The text is deliberately limited: public-domain hymns and the KJV (1611).
Do not add copyrighted lyrics or a modern translation. Verses are unique
by reference, song ids are unique in the library, and every song has at
least one part; `data_test.go` and `deck_test.go` assert these.

## Rendering details worth knowing

- Widget state that must persist between passes is listed in `app`:
  `tab`, `pack`, `songQuery`, `verseQuery`, `preview`, `stage`, `blackout`,
  plus the settings fields. Everything else is rebuilt from data, including
  the run of show's groups.
- `ui.SearchField` only exposes its hint as a placeholder, so both search
  boxes also call `.Label(...)`; that label is what the tests click.
- Buttons that act on a row carry a per-row accessible label (`Add
  Amazing Grace`, `Add Psalm 23:1`) while their visible text stays short.
- The stage window, the NDI feed, the editor preview and the run of show's
  tiles all go through `slideBody` with `metricsFor`, so the whole app draws
  a slide the same way. The console's two stage cards draw their own
  fixed-size text instead, because they are read at arm's length. A tile on
  air carries the copper frame and a `LIVE` badge, so the run of show shows
  what the room sees without a second look.
- Disabled, not hidden, is the pattern for transport that cannot act
  (empty deck, first/last slide), so the layout does not jump.

## Testing

All tests are headless; `ui.NewTester` builds real frames without a window
or a display, and `mygo.NewWindow` is never called from a test (so the
tests must not press **Open output**).

- `view_test.go` — login gate, wrong-password error and its clearing,
  demo-account sign-in, Enter-to-submit, sign-out, library rows, adding a
  song, adding a verse from the Bible tab, search filtering, monitor vs
  stage (click, Go Live, Enter, Cmd+Enter), the tiles wrapping from three
  columns to two at the minimum width, the song headers and their counts
  (including a repeated song), a tile's own go-live/move/remove acting on
  that tile, arrow shortcuts (and that arrows in a text field do not move
  the deck), the
  blackout button, and the two empty states.
- `data_test.go` — every built-in song has parts and slides, All Creatures'
  verse keeps its two slides, a missing or out-of-range text size clamps to
  40/24–72, `slideCount`, and `findSong`.
- `deck_test.go` — data reachability (every song/verse resolves), unique
  slide ids, part expansion (including blank-slide skipping and the text
  size snapshot), single-part adds, unknown ids, monitor clamping, take,
  stage next/prev, removal from and above the stage, reorder without
  changing what is on air (single rows, blocks of rows, and `to` as a
  gap), `Move up`/`Move down` going through the same reorder, the tile's
  own indexed take/move/remove, blackout,
  `stageSlide`/`previewSlide`/`upcomingSlide`, and `changed()` with no
  windows attached.
- `library_test.go` — library round-trip, version and duplicate rejection,
  seeding on first launch, corrupt files left untouched, an empty song list,
  the empty `settingsDir` no-op, normalization, deep clones, and unique new
  song ids.
- `slideview_test.go` — `stageScale` and the metrics at full and half width.
- `editor_test.go` — the editor window lifecycle (request, settle, close,
  sign-out, `changed()` nil-safety), the console button, empty and filtered
  lists, selection and preview, preview width clamping, metadata and slide
  text editing that saves, part and slide add/remove with the last-one
  guard, text size clamping, new/remove/restore songs, and adding a song or
  one part to the run of show without touching the stage.
- `settings_test.go` — the settings round-trip, missing and corrupt files,
  the empty `settingsDir` no-op, the legacy `outputDisplay` key, dropping a
  display that went away, `stageDisplayFound`, the Settings page with and
  without displays, display refresh without a stage and without reopening
  one, and the stats formatting.
- `ndi_stats_test.go` — the sender's counters and measured rate without a
  runtime.
- `update_test.go` — the version comparison, the newest-version and error
  notes, the offer, install-and-restart, Not Now, a failed install, and the
  unversioned and package-manager builds that say they cannot update.
- `help_test.go`, `menu_test.go` — Help sections with and without a
  version; menu item IDs, the Settings accelerator, and click handlers
  present (clicks are not run, because they open windows).
- `output_test.go` — the stage view shows the stage slide rather than the
  monitor row, follows next/prev, hides the text under blackout, honours
  its shortcuts, and NDI refuses without the stage or waits for it; the
  header NDI toggle is not drawn before the stage exists.
- `ndi_sender_test.go`, `ndi_windows_test.go` — the NDI binding: versioned
  table indices against a real installed runtime, and the C ABI struct
  offsets on Windows.

## Adding a song or a verse

For a song, use **Song editor** in the app: `New song`, fill the title,
author and key, add parts and slides, and set the text size. It saves to
`library.json` and the console lists it on the next pass. A built-in seed
goes in `builtinSongs` in `data.go` with a unique `ID`, a title, an author,
a key, at least one part with one slide, and `TextSize: 0` for the default;
`Restore samples` copies a missing built-in into an existing library. A
song the operator edits in the app stays in `library.json` and is not
overwritten.

For a verse, add it to one of the packs' `Verses` with a unique `Ref` and
its KJV text. The Bible tab lists it under that pack and `addVerse(ref)`
adds one slide.

## Verification

Before claiming a change works:

```sh
test -z "$(gofmt -l *.go)"
CGO_ENABLED=0 go build ./...
GOOS=darwin go build ./...
GOOS=windows go build ./...
GOOS=linux CGO_ENABLED=0 go build ./...
go vet ./...
CGO_ENABLED=0 go test ./...
```

Then `go run .` for the manual check: sign in, add/remove/move slides, use
the arrows, black out, open the output (fullscreen with a second display),
change slides, leave fullscreen with Esc, open the **Song editor**, edit a
part's slide and the text size and check the preview and the stage agree,
add one part to the run of show, restart to confirm `library.json` kept the
edit, and with `MYGO_NDI=1` check that the NDI source follows the stage with
the stage window closed too, and that sign-out stops it. On a sandboxed
agent session, prefix Go commands with `GOCACHE="$TMPDIR/gocache-mygo-demo"`.

## Known drift and gaps

- **The stage is one window.** There is no separate preview output and no
  per-display preview; the monitor lives in the console.
- **The library persists; the run of show does not.** `library.json` holds
  every song's parts, slides and text size, and `settings.json` holds the
  display and NDI preferences. The deck, the search boxes and the selected
  pack live only in the process, so closing the app starts over from the
  seed set.
- **A song has no arrangement list.** The run of show is the arrangement:
  adding a part again is how a chorus repeats. There is no per-song order
  or repeat count.
- **No undo/redo and no presentation media.** Slides are text; there are no
  images, videos or lower thirds.
- **The sign-in is not security.** Two constants in `main.go`, no session,
  no hashing.
- **The web scaffold is dead.** `src/`, `index.html`, `vite.config.ts`, the
  `devUrl`/frontend settings in `mygo.config.ts`, and the `tick` event in
  `src/mygo.ts` predate the native UI. `dist/` is still built and embedded,
  but no window loads it. `package.json` pins the web tooling at 0.7.x
  while `go.mod` pins MyGo v0.10.0.
- **NDI needs the runtime, not a window.** macOS needs
  `/usr/local/lib/libndi.dylib`, Windows the x64 DLL from NDI 5 or 6, and
  Linux `/usr/local/lib/libndi.so`; nothing is captured, so one monitor,
  Wayland and a closed stage all work. A sandboxed agent session still
  cannot eyeball the feed in OBS or NDI Video Monitor.
- **The console shortcuts are small.** ← / → for the monitor, B for
  blackout, Enter and Cmd/Ctrl+Enter to go live; everything else
  is a button or a menu item.
- **One stage window and one monitor.** Opening the stage again focuses
  the existing one; there is no multi-output or per-display preview feature.
- **No undo/redo and no session recovery.** A mistaken removal or a crash
  loses the run of show.

The MyGo lifetime and identity rules, and the widget inventory this app
was written against, are in
[`.agents/skills/mygo-maintenance/SKILL.md`](../.agents/skills/mygo-maintenance/SKILL.md)
and the module source pinned at v0.10.0.
