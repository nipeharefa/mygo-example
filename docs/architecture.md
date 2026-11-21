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
process and no HTTP server; the only file is `settings.json` in the app
data directory.

The console's view sets the console theme, draws the login card while
`!a.signedIn`, and otherwise registers the console shortcuts and draws the
shell. The stage is a separate `mygo.Window` created later by
`openOutput`, with its own `ui.View(a.liveView)`; Settings and Help are two
more windows, created by `openSettings` and `openHelp`. Every window's
view is a pure builder, and `changed()` invalidates all four of them.

## State

`app` in `main.go` is plain data plus widget state; it never holds an
element or a context:

| Field | Meaning |
| --- | --- |
| `signedIn`, `username`, `password`, `authErr` | the demo gate |
| `tab` | 0 = Songs, 1 = Bible |
| `songQuery`, `verseQuery`, `pack` | library filters; `pack` is the selected verse pack name |
| `deck []slide`, `preview int`, `stage int` | the run of show, the row selected in the monitor, and the row on air |
| `blackout bool` | the stage shows a black frame, deck unchanged |
| `nextID int`, `deckList ui.ListState` | slide id counter and the list's persistent state |
| `win`, `live`, `settings`, `help *mygo.Window` | the console, stage, Settings and Help windows |
| `displays []mygo.Display`, `outputDisplay int64` | the display list and the chosen stage display (0 = automatic) |
| `ndiOn bool`, `ndi *ndiStream`, `ndiErr string` | NDI preference, sender and last error |
| `settingsDir string`, `version string` | where `settings.json` lives and what Help reports |

`newApp()` picks the first pack and seeds the deck with Amazing Grace and
Psalm 23:1, so a fresh run has something to show. `slide` carries an ID
(`s1`, `s2`, …), a title, a section label, the text, and a kind
(`slideSong` or `slideVerse`); `caption`, `eyebrow` and `firstLine` derive
the strings the views need.

Every mutation goes through a method on `app` (`addSong`, `addVerse`,
`selectSlide`, `nextSlide`, `prevSlide`, `takeSlide`, `nextStage`,
`prevStage`, `removeSlide`, `moveSlide`, `toggleBlackout`, `setNDI`, …) and
ends in `changed()`, which invalidates every open window. The monitor
methods move `a.preview` only; `takeSlide` copies the monitor row to the
stage, and the stage's own next/prev pull the monitor along so the
highlight follows the room. `nextSlide`/`prevSlide` clamp at the ends;
`removeSlide` repairs the stage index when the removed row was on air or
above it; `moveSlide` swaps and remaps the stage so the on-air content does
not change.

## The frame contract

Views are pure builders with the same rules as any MyGo app:

- `a.view` and `a.liveView` build the whole tree from `app` on every pass.
  Nothing built survives the pass, so no element, widget part or
  `*ui.Context` is stored anywhere on `app`.
- Callbacks (`OnClick`, `OnSubmit`, `OnChange`, `OnShortcut`) run before
  the rebuild; they mutate `app` and call `changed()`. They never call into
  the build.
- `ui.List` identity comes from `deckList.Key`, a slide ID, not the row
  index, so the same song can appear twice and removal does not confuse
  selection. `deckList.Selected = &a.preview` is pointed at the monitor row
  every pass, and the row label is what `ui.NewTester` and accessibility
  see. The list's `OnSubmit` takes the slide, so Enter and a double click
  both go live.
- Shortcuts fall through: the focused list consumes Enter and the arrows,
  so the console's root ← / → handlers only fire when the list is not
  focused. `c.OnWindowShortcut` handles Cmd/Ctrl+Enter and B everywhere,
  and a focused text field consumes its keys first.
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
  (only where `ndiSupported()`). **Sign out** is always last.
- **Library** (296 wide, `Surface`) — `ui.Tabs` Songs/Bible. Songs has a
  search box and a scrolling list of `songRow`s with an **Add** button
  whose accessible label is `Add <title>`. Bible has a search box, the
  verse-pack `ui.Select`, and `verseRow`s with `Add <ref>` buttons.
- **Run of show** (grow) — heading, slide count, and **Move up** /
  **Move down** / **Remove** (disabled where they cannot act), then either
  an empty state or the `ui.List` of `deckRow`s. A row shows its position,
  caption and first line; clicking it selects the slide.
- **Stage panel** (344 wide, `Surface`) — two 16:9 cards drawn in the
  stage theme: **STAGE · LIVE** (what the room sees, with `n / total`) and
  **MONITOR · PREVIEW** (the row selected in the run of show), then **Go
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
- If `chooseDisplay` finds the chosen display (an explicit Settings choice,
  else the first non-primary display), the window's position and size are
  set to that display's bounds and it opens fullscreen. Otherwise it is an
  ordinary windowed window on the current display; the operator can drag it
  to a projector and press **Present**. Changing the display in Settings
  saves the choice and reopens an open stage on the new display.

`liveView` draws a black fill while `a.blackout`, and otherwise the
stage slide (`a.stageSlide()`, not the monitor row) as a centered slide (eyebrow, 40 pt text, caption) on the forced-dark stage theme. Its
shortcuts are → ↓ Space next, ← ↑ prev, B blackout, and Esc, which clears
fullscreen when `a.live.IsFullScreen()` and closes the output otherwise —
the only way out of a window with no title bar.

`openOutput` when a window already exists just calls `Show()` and
`Focus()`. `closeOutput` nils `a.live` first, stops NDI, invalidates the
console, and closes the window. The window's `OnClosed` callback does the
same cleanup for a close from the window's own chrome, guarded with
`if a.live != live { return }` so a late close event cannot clear a window
that was reopened in the meantime. If `a.ndiOn` is set, `openOutput`
starts NDI right after the window exists.

### Settings

`openSettings` creates the window once and focuses it afterwards. It has no
login gate, but it exposes nothing private: the stage display radios
(**Automatic** plus one per display, each showing label and size), the NDI
switch, the NDI status line, and a short data note. Choosing a display or
toggling NDI saves immediately; both callbacks return through `changed()`,
and an open stage is reopened so the new display takes effect.

### Help

`openHelp` creates the window once. It lists the About block (version, demo
account), the keyboard map, the NDI notes (source name, per-platform
runtime), and the sample-data licence note. The menu's About item opens the
native about panel on macOS and this window elsewhere.

## NDI output

NDI publishes the **live window**, never the console. `startNDI` returns an
error and sets the header strip when `a.live == nil`, so the operator sees
`NDI output needs the live window` instead of a silent failure; when the
window exists it calls `startNDISender(a.live)`.

The platform split is described in `AGENTS.md`: `ndi_sender.go` owns
runtime discovery, the versioned function table, the sender and the frame
loop; `ndi_darwin.go`, `ndi_windows.go` and `ndi_linux.go` only bind the
library and capture the live window (CoreGraphics, GDI, GDK). The source
is registered as `mygo-demo (Live)`.

The feed is the slide alone, at the capture's own pixel size capped to HD.
macOS captures the frameless window at the display's pixel density
(`cgWindowImageBestResolution`) and `ndiFrameSize` scales it down to at
most 1920×1080 without upscaling, so a Retina window streams crisp 1080p;
Windows and Linux send the window's client pixels at its DPI.

`MYGO_NDI=1` sets `a.ndiOn` for the session so opening the stage starts
streaming immediately; the Settings switch and the console header button
both go through `setNDI`, which saves the preference, starts the sender
when a stage exists, and stops it otherwise. Closing the stage stops the
sender through the same paths as **Stop NDI** and sign-out; turning NDI on
with no stage open records the preference and waits for one to open.

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

`data.go` is the whole library:

- 5 traditional, public-domain hymns with 10 sections in total (Amazing
  Grace, Holy, Holy, Holy, Be Thou My Vision, All Creatures of Our God and
  King, A Mighty Fortress Is Our God). Each has `ID`, `Title`, `Author`,
  `Key` and `[]songSection{Label, Text}`; the text carries one line per
  newline so a slide renders as written.
- 4 verse packs (Assurance, Comfort, Praise, Sending) with 4 King James Version verses each (16 total), looked up by `ref` (`verseByRef`) or `song` id (`songByID`).

The text is deliberately limited: public-domain hymns and the KJV (1611).
Do not add copyrighted lyrics or a modern translation. Verses are unique
by reference, and every song has at least one section; `deck_test.go`
asserts both.

## Rendering details worth knowing

- Widget state that must persist between passes is listed in `app`:
  `tab`, `pack`, `deckList`, `songQuery`, `verseQuery`, `preview`, `stage`,
  `blackout`, plus the settings fields. Everything else is rebuilt from
  data.
- `ui.SearchField` only exposes its hint as a placeholder, so both search
  boxes also call `.Label(...)`; that label is what the tests click.
- Buttons that act on a row carry a per-row accessible label (`Add
  Amazing Grace`, `Add Psalm 23:1`) while their visible text stays short.
- The console's two stage cards and the stage window share `stageTheme`;
  only the size and padding differ. A row on air carries a `LIVE` badge, so
  the run of show shows what the room sees without a second look.
- Disabled, not hidden, is the pattern for transport that cannot act
  (empty deck, first/last slide), so the layout does not jump.

## Testing

All tests are headless; `ui.NewTester` builds real frames without a window
or a display, and `mygo.NewWindow` is never called from a test (so the
tests must not press **Open output**).

- `view_test.go` — login gate, wrong-password error and its clearing,
  demo-account sign-in, Enter-to-submit, sign-out, library rows, adding a
  song, adding a verse from the Bible tab, search filtering, monitor vs
  stage (click, Go Live, Enter, Cmd+Enter), arrow shortcuts (and that
  arrows in a text field do not move the deck), the blackout button, and
  the two empty states.
- `deck_test.go` — data reachability (every song/verse resolves), unique
  slide ids, section expansion, unknown ids, monitor clamping, take, stage
  next/prev, removal from and above the stage, reorder without changing
  what is on air, blackout, `stageSlide`/`previewSlide`/`upcomingSlide`,
  and `changed()` with no windows attached.
- `settings_test.go` — the settings round-trip, missing and corrupt files,
  the empty `settingsDir` no-op, dropping a display that went away,
  `chooseDisplay`, the Settings view with and without displays, and display
  refresh without a stage.
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

For a song, add an entry to `songs` in `data.go` with a unique `ID`, a
title, an author, a key, and at least one section. Nothing else is needed:
the library lists it, the search filters it, and `addSong` expands its
sections.

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
change slides, leave fullscreen with Esc, and with `MYGO_NDI=1` check that
the NDI source follows the output and stops when it closes. On a sandboxed
agent session, prefix Go commands with `GOCACHE="$TMPDIR/gocache-mygo-demo"`.

## Known drift and gaps

- **Only the display and NDI preferences persist.** They live in
  `settings.json` under the app data directory; the deck, the search boxes
  and the selected pack live only in the process, so closing the app starts
  over from the seed set.
- **No undo/redo and no presentation media.** Slides are text; there are no
  images, videos or lower thirds.
- **The sign-in is not security.** Two constants in `main.go`, no session,
  no hashing.
- **The web scaffold is dead.** `src/`, `index.html`, `vite.config.ts`, the
  `devUrl`/frontend settings in `mygo.config.ts`, and the `tick` event in
  `src/mygo.ts` predate the native UI. `dist/` is still built and embedded,
  but no window loads it. `package.json` pins the web tooling at 0.7.x
  while `go.mod` pins MyGo v0.10.0.
- **NDI is per platform.** macOS needs the output window on screen and the
  NDI runtime at `/usr/local/lib/libndi.dylib`; Windows uses the x64 DLL
  from NDI 6; Linux captures through GDK, so X11 works and Wayland reports
  that it cannot read its own pixels.
- **The console shortcuts are small.** ← / → for the monitor, B for
  blackout, Cmd/Ctrl+Enter to go live, Enter on the list; everything else
  is a button or a menu item.
- **One stage window and one monitor.** Opening the stage again focuses
  the existing one; there is no multi-output or per-display preview feature.
- **No undo/redo and no session recovery.** A mistaken removal or a crash
  loses the run of show.

The MyGo lifetime and identity rules, and the widget inventory this app
was written against, are in
[`.agents/skills/mygo-maintenance/SKILL.md`](../.agents/skills/mygo-maintenance/SKILL.md)
and the module source pinned at v0.10.0.
