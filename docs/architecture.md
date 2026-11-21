# Architecture

`mygo-demo` is a worship presenter: a console window where an operator
builds a run of show, and a second output window that shows one slide at a
time to the room. Everything is local, in memory, and rebuilt from one
`app` struct. This document covers the state, both windows, the theme
split, the NDI path, and what the tests pin.

## What runs

`main()` builds the `app`, reads `MYGO_NDI` into `autoNDI`, and waits for
`mygo.App.WhenReady`. Then it opens the console window with
`mygo.NewWindow`:

- Title `mygo-demo`, 1280×800, minimum 1040×660, `StateKey: "main"`.
- `Content: ui.View(a.view)`.

`mygo.App.OnQuit(a.stopNDI)` makes quitting the app unregister any NDI
source, and `mygo.App.Run()` runs the main loop. There is no second
process, no HTTP server, and no file on disk.

The console's view sets the console theme, draws the login card while
`!a.signedIn`, and otherwise registers the console shortcuts (← / →) and
draws the shell. The output is a separate `mygo.Window` created later by
`openOutput`, with its own `ui.View(a.liveView)`.

## State

`app` in `main.go` is plain data plus widget state; it never holds an
element or a context:

| Field | Meaning |
| --- | --- |
| `signedIn`, `username`, `password`, `authErr` | the demo gate |
| `tab` | 0 = Songs, 1 = Bible |
| `songQuery`, `verseQuery`, `pack` | library filters; `pack` is the selected verse pack name |
| `deck []slide`, `index` | the run of show and the selected slide |
| `blackout bool` | output shows a black frame, deck unchanged |
| `nextID int`, `deckList ui.ListState` | slide id counter and the list's persistent state |
| `win`, `live *mygo.Window` | the console and output windows |
| `autoNDI bool`, `ndi *ndiStream`, `ndiErr string` | NDI intent, sender and last error |

`newApp()` picks the first pack and seeds the deck with Amazing Grace and
Psalm 23:1, so a fresh run has something to show. `slide` carries an ID
(`s1`, `s2`, …), a title, a section label, the text, and a kind
(`slideSong` or `slideVerse`); `caption`, `eyebrow` and `firstLine` derive
the strings the views need.

Every mutation goes through a method on `app` (`addSong`, `addVerse`,
`selectSlide`, `nextSlide`, `prevSlide`, `removeSlide`, `moveSlide`,
`toggleBlackout`) and ends in `changed()`, which invalidates both windows.
`nextSlide`/`prevSlide` clamp at the ends; `removeSlide` calls
`clampIndex`; `moveSlide` swaps and follows the moved slide.

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
  selection. `deckList.Selected = &a.index` is pointed at the app's index
  every pass, and the row label is what `ui.NewTester` and accessibility
  see.
- A `ui.Row` centers its children vertically by default; the console's
  three-column row sets `.AlignItems(ui.Stretch)` so the panels fill the
  height.

## Signing in

The card is the demo gate, not security. `demoUsername`/`demoPassword` are
constants in `main.go`; `signIn` trims the username and compares the
password exactly, showing one generic `Wrong username or password.` error
otherwise. A successful sign-in clears the password, sets `signedIn`, and
resets the selection. `clearAuthError` runs on either field's change.

`signOut` closes the output window, stops NDI, and clears session state
including blackout. Nothing is persisted in either direction.

## The two windows

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
- **Stage panel** (344 wide, `Surface`) — "On stage" and `n / total`, a
  16:9 preview card drawn in the output's own theme, "Next up" (or "End of
  set"), **Prev** / **Next**, **Black out** / **Clear black out**, and a
  shortcut hint.

The console itself listens for ← and → via `c.OnShortcut`; text fields
consume arrows while focused, so typing in the search box does not move
the deck.

### Output

`openOutput` creates the window only once:

- Title `mygo-demo — Live`, 1280×720, minimum 640×360, and
  `BackgroundColor: stageBackgroundHex` so it never flashes white before
  the first frame.
- `Frameless: true`, so the slide is the whole window: no title bar,
  traffic lights or resize edges, on screen or in the NDI feed. The slide
  column calls `DragWindow()`, so dragging anywhere on the slide moves a
  windowed output.
- If `secondDisplay()` finds a display that is not the primary one, the
  window's position and size are set to that display's bounds and it opens
  fullscreen. Otherwise it is an ordinary windowed window on the current
  display; the operator can drag it to a projector and press **Present**.

`liveView` draws a black fill while `a.blackout`, and otherwise a centered
slide (eyebrow, 40 pt text, caption) on the forced-dark stage theme. Its
shortcuts are → ↓ Space next, ← ↑ prev, B blackout, and Esc, which clears
fullscreen when `a.live.IsFullScreen()` and closes the output otherwise —
the only way out of a window with no title bar.

`openOutput` when a window already exists just calls `Show()` and
`Focus()`. `closeOutput` nils `a.live` first, stops NDI, invalidates the
console, and closes the window. The window's `OnClosed` callback does the
same cleanup for a close from the window's own chrome, guarded with
`if a.live != live { return }` so a late close event cannot clear a window
that was reopened in the meantime. If `autoNDI` is set, `openOutput` starts
NDI right after the window exists.

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

`MYGO_NDI=1` sets `autoNDI` so opening the output starts streaming
immediately; otherwise the operator presses **Start NDI** in the console
header. Closing the output stops the sender through the same paths as
**Stop NDI** and sign-out.

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
  `tab`, `pack`, `deckList`, `songQuery`, `verseQuery`, `index`,
  `blackout`. Everything else is rebuilt from data.
- `ui.SearchField` only exposes its hint as a placeholder, so both search
  boxes also call `.Label(...)`; that label is what the tests click.
- Buttons that act on a row carry a per-row accessible label (`Add
  Amazing Grace`, `Add Psalm 23:1`) while their visible text stays short.
- The deck preview and the output share `stageTheme`; only the size and
  padding differ.
- Disabled, not hidden, is the pattern for transport that cannot act
  (empty deck, first/last slide), so the layout does not jump.

## Testing

All tests are headless; `ui.NewTester` builds real frames without a window
or a display, and `mygo.NewWindow` is never called from a test (so the
tests must not press **Open output**).

- `view_test.go` — login gate, wrong-password error and its clearing,
  demo-account sign-in, Enter-to-submit, sign-out, library rows, adding a
  song, adding a verse from the Bible tab, search filtering, arrow
  shortcuts (and that arrows in a text field do not move the deck), the
  blackout button, and the two empty states.
- `deck_test.go` — data reachability (every song/verse resolves), unique
  slide ids, section expansion, unknown ids, index clamping on next/prev
  and removal, reorder, blackout, `currentSlide`/`upcomingSlide`, and
  `changed()` with no windows attached.
- `output_test.go` — the live view shows the current slide, follows next,
  hides the text under blackout, honours its shortcuts, and `startNDI`
  refuses without the live window; the NDI toggle is not drawn before the
  output exists.
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

- **No persistence.** The deck, the search boxes and the selected pack live
  only in the process; closing the app starts over from the seed set.
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
- **The console has only ← / → shortcuts.** Everything else is a button.
- **One output window.** Opening it again focuses the existing one; there
  is no multi-output or confidence-monitor feature.

The MyGo lifetime and identity rules, and the widget inventory this app
was written against, are in
[`.agents/skills/mygo-maintenance/SKILL.md`](../.agents/skills/mygo-maintenance/SKILL.md)
and the module source pinned at v0.10.0.
