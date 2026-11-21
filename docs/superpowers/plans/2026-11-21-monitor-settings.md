# mygo-demo 0.2.0 — Settings & Help, Monitor vs Stage

## Goal

Turn the 0.2.0 worship console into a two-monitor workflow: a **monitor (preview)** the operator drives freely, and a **stage (live)** that only changes when taken. Add native **Settings** and **Help** menus; Settings holds the stage display choice and the NDI publish preference, both persisted across launches. Ship as `v0.2.0-beta.3` (app version stays 0.2.0).

Success means: choosing a slide or pressing ←/→ never changes what the room sees until Go Live; the stage window and NDI show only the stage slide; Settings changes where the stage opens and whether it publishes over NDI, and those choices survive a restart; Help explains shortcuts, NDI setup and the version. All gates green, docs updated, release published.

## Decisions already confirmed with the user
- Preview model: **Monitor → Go Live**. Selection/arrows move the monitor; Go Live takes it to the stage.
- Settings live in a **Settings window** opened from a top-level Settings menu (`CmdOrCtrl+,`).
- Display and NDI settings are **persisted**; the deck/run-of-show is not.
- Changing display while the stage is open applies **immediately** (close + reopen the stage window).

## Starting point
- main at `14da25b`, `v0.2.0-beta.2` shipped; branch `feat/monitor-settings` off main.
- Existing files: `main.go` (app state, auth, NDI), `console.go`, `deck.go`, `output.go`, `theme.go`, `data.go`, `ndi_*.go`; tests `view_test.go`, `deck_test.go`, `output_test.go`, `ndi_*_test.go`.
- Current `app.index` doubles as the deck selection and the live slide; `currentSlide()` returns it; the console stage card, output window and NDI all read it.
- Pinned MyGo v0.10.0 already exposes everything needed (verified in the module source): `mygo.App.SetMenu`, `mygo.NewMenu`, `MenuItem` roles/accelerators/`Click`, per-window `SetMenu`, `App.Version`, `App.Path(PathUserData)`, `App.ShowAboutPanel` (macOS), `Screen.Displays`/`OnDisplaysChanged`, and `ui.Radio`, `ui.Switch`, `ui.Toggle`, `ui.RadioGroup`, `Element.OnChange`/`.OnSubmit`. `ui.List` reports Enter/double-click as `Submitted` and consumes arrows/Enter when focused, so the root shortcut is not reached.

## Design

### 1. State model (`main.go`)
Replace `index int` with:

```go
deck    []slide
preview int      // row selected in the monitor
stage   int      // row on air
blackout bool
```

Add window/state fields:

```go
settings *mygo.Window
help     *mygo.Window
settingsDir string      // app data dir, empty until ready (keeps tests headless-safe)
outputDisplay int64     // 0 = automatic (projector, else windowed)
ndiOn bool              // preference: publish the stage when it opens
displays []mygo.Display // refreshed at startup and on display changes
version string          // mygo.App.Version(), "" under go run
```

Keep `win`, `live`, `ndi *ndiStream`, `ndiErr`, auth fields, library fields, `deckList`. Remove `autoNDI` (replaced by `ndiOn`).

`newApp()` seeds the deck and sets `preview = stage = 0`. `signIn` resets both to 0; `signOut` closes the stage, stops NDI, resets `preview`, `stage`, `blackout`.

`changed()` invalidates `win`, `live`, `settings`, `help` (nil-safe).

Main-thread startup in `WhenReady` (order matters):
1. `a.settingsDir = mygo.App.Path(mygo.PathUserData)`
2. `a.displays = mygo.Screen.Displays()`
3. `a.loadSettings()` (sets `outputDisplay`, `ndiOn`; ignores a saved display that is absent; `MYGO_NDI` env forces `ndiOn` on for the session without saving)
4. `a.version = mygo.App.Version()`
5. `mygo.App.SetMenu(buildMenu(a))`
6. `mygo.Screen.OnDisplaysChanged(a.displaysChanged)`
7. create the console window as today.

### 2. Deck model (`deck.go`, `deck_test.go`)
- `selectSlide`/`nextSlide`/`prevSlide` move **preview only** (clamped, no wrap).
- `takeSlide()`: `stage = preview` (no-op on empty deck).
- `nextStage()`/`prevStage()`: advance **stage**, then set `preview = stage` so the console highlight follows the live window.
- `stageSlide()` (renamed from `currentSlide`) is what `output.go` renders; `previewSlide()` feeds the monitor card. Both return `(slide, bool)` and are false on an empty deck.
- `addSong`/`addVerse`: append and move `preview` to the first/last new slide; `stage` untouched.
- `removeSlide` (removes the preview row, then repairs indices): after removal of row `r` with old stage `s`, new length `n`: `preview = clamp(r, 0, n-1)`; if `n == 0` both are 0; if `s == r` stage becomes the slide that took the removed row (`clamp(r, 0, n-1)`); if `s > r` stage is `s-1`; otherwise unchanged.
- `moveSlide(delta)` (swaps preview with target `t`): keep on-air content stable — if `stage == preview` stage becomes `t`, and if `stage == t` stage becomes the old preview index; then `preview = t`.
- Delete `clampIndex` in favour of `clampSlides()`.
- Tests rewrite: monitor-only add/select, take, removal of monitor vs stage row, reorder keeping stage content, stage shortcuts syncing monitor, empty-deck safety.

### 3. Console (`console.go`, `view_test.go`)
Layout stays 296 library | flex run of show | 344 stage panel (in a `ui.Scroll` inside the panel so short windows can still reach the controls).

Run of show:
- `deckList.Selected = &a.preview`; row Key stays the slide ID; row Label stays `"Slide %d: %s"`.
- Deck row shows a `ui.Badge(c,"LIVE")` when `i == a.stage`; selection background on `i == a.preview`.
- `ui.List(...).OnSubmit(a.takeSlide)` so Enter takes the selected row without a global handler fight.
- Move up/down/remove act on `a.preview`.

Stage panel (top to bottom):
- `STAGE · LIVE` header with dot and `deckCounter(a.stage)` (e.g. `2 / 6`).
- Live card (16:9, `stageTheme`, stage slide, slightly smaller type than the output window; "Nothing on stage" when empty).
- `MONITOR · PREVIEW` header with dot and `deckCounter(a.preview)`.
- Preview card (same renderer, preview slide; "Nothing selected" when empty).
- `ui.PrimaryButton(c,"Go Live").OnClick(a.takeSlide)`.
- Prev/Next buttons (move the monitor) + Black out / Clear black out.
- Hint: `← → monitor · Enter go live · B black out`.

Shortcuts in `view`: keep root `←`/`→` → `prevSlide`/`nextSlide` (the focused list consumes them first, so one press moves one row) and `B` → `toggleBlackout`; add `c.OnWindowShortcut(ui.Cmd, ui.KeyEnter, a.takeSlide)` and the same for `ui.Ctrl` for Windows/Linux. Remove the root `Left/Right` handlers from the output window (see below).

Top bar keeps: title dot, app title/subtitle, status pill (BLACK / LIVE / PREVIEW), Open output / Present / Close output, NDI button (`ndiLabel` now derived from `a.ndi`), Sign out. The NDI button stays hidden until the stage window exists, as today.

### 4. Stage window (`output.go`, `output_test.go`)
- `liveView` renders `a.stageSlide()`; blackout frame unchanged; frameless + drag handle unchanged.
- Live-window shortcuts now call `nextStage`/`prevStage` (move stage and monitor together), `B` blackout, `Esc` exit fullscreen / close windowed.
- `openOutput` builds placement from the chosen display (see §6) and starts NDI when `a.ndiOn`.
- `closeOutput`/`OnClosed` stop the sender but keep `a.ndiOn`; reopen resumes it.
- `togglePresent` unchanged.

### 5. Settings window + persistence (`settings.go`, `settings_test.go`)
Settings window: title `mygo-demo — Settings`, 560×520 (min 460×420), `StateKey: "settings"`, `Frameless: false`, `Content: ui.View(a.settingsView)`. Reopening focuses the existing window; `OnClosed` nils the pointer with the `a.settings == w` guard.

`settingsView` (console theme, scrollable):
- Header "Settings" + Close button.
- **STAGE OUTPUT**: muted explainer ("Automatic follows a projector, otherwise the stage opens as a window on this screen"); `ui.RadioGroup` with `ui.Radio(c,&a.outputDisplay,0,"Automatic")` and one radio per display labelled `name · W×H · primary/built-in`; each `.OnChange(a.outputDisplayChanged)`. If `len(a.displays)==0`, show "No displays reported." and the Automatic row only.
- **NDI**: text "Publish the stage over NDI" + `ui.Switch(c,&a.ndiOn).Label("Publish the stage over NDI").OnChange(a.ndiToggled).Disabled(!ndiSupported())`; status line: unavailable in this build / `NDI: <a.ndiErr>` in danger / "Publishing as “mygo-demo (Live)”" / "Starts when the stage opens".
- **DATA**: muted note that the run of show is not saved and the sample songs and KJV verses are public domain.

Persistence in `settings.go`:

```go
type settings struct {
    OutputDisplay int64 `json:"outputDisplay"`
    NDI           bool  `json:"ndi"`
}
func readSettings(path string) settings          // defaults on missing/corrupt
func writeSettings(path string, s settings) error
func (a *app) settingsPath() string              // filepath.Join(a.settingsDir,"settings.json"), "" when no dir
func (a *app) loadSettings()
func (a *app) saveSettings()                     // no-op without a dir; log failures only
```

`loadSettings` normalizes: a non-zero `outputDisplay` not present in `a.displays` resets to 0 (auto). Tests cover round-trip, missing file, corrupt JSON, and the app-level save/load with `a.settingsDir = t.TempDir()`.

### 6. Display choice and NDI preference (`output.go`, `settings.go`, `main.go`)
Pure helper for placement:

```go
func chooseDisplay(displays []mygo.Display, id int64) (mygo.Display, bool)
```
Explicit ID wins when present; `0` picks the first non-primary display (projector); primary-only returns false (windowed placement). Replace `secondDisplay()` and `openOutput`'s ad-hoc branch with it: a found display sets `X/Y/Width/Height` from `Bounds` and `FullScreen: true`; otherwise 1280×720 windowed.

`a.displaysChanged()` (main thread): refresh `a.displays`; if the chosen ID vanished, set `outputDisplay = 0` and save; if `a.live != nil`, `a.closeOutput(); a.openOutput()` to move it; `a.changed()`.

`a.outputDisplayChanged()`: save, then if the stage is open close + reopen, else `changed()`.

`a.setNDI(on bool)`:
- no-op when `!ndiSupported()`;
- set `a.ndiOn = on`, save;
- on: `startNDI()` only when `a.live != nil` (no error when closed — the preference waits for the stage);
- off: `stopNDI()`, clear `ndiErr`; then `changed()`.
`a.ndiToggled()` is the Switch handler: same body without reassigning `ndiOn` (the switch already wrote it).
`a.toggleNDI()` = `setNDI(a.ndi == nil)` keeps the header button working; `ndiLabel` still says Start/Stop NDI.

### 7. Menus (`menu.go`, `menu_test.go`)
`buildMenu(a) *mygo.Menu`:
- macOS: `{Role: mygo.RoleAppMenu}`; Windows/Linux: a File menu with `RoleQuit`.
- Edit menu: `RoleUndo`, `RoleRedo`, separator, `RoleCut`, `RoleCopy`, `RolePaste`, `RoleSelectAll` (needed for the text fields once a custom menu replaces the default).
- Settings menu: one item, ID `settings`, label `Settings…`, accelerator `CmdOrCtrl+,`, click → `a.openSettings()`.
- Help menu: `{Role: mygo.RoleHelp, Submenu: [...]}` with ID `help` "Help" → `a.openHelp()`, separator, ID `about` "About mygo-demo" → `a.showAbout()`.
- macOS only: `{Role: mygo.RoleWindowMenu}`.

`a.showAbout()`: on `runtime.GOOS == "darwin"` call `mygo.App.ShowAboutPanel(mygo.AboutPanelOptions{ApplicationName: appTitle, Version: a.version, Credits: "Worship presenter demo. Public-domain hymns and King James verses."})`; otherwise open Help. Menu click callbacks run on the main thread, so they may create windows.

Tests (headless): `buildMenu` produces a menu with `ItemByID("settings")` non-nil and accelerator `CmdOrCtrl+,`; `help` and `about` items exist; clicking is not tested (it would create a window).

### 8. Help window (`help.go`)
Title `mygo-demo — Help`, 640×620 (min 520×420), `StateKey: "help"`, console theme, reused/focused when open. Sections:
- **About**: app title, version (`a.version` or "development build"), one-paragraph description, demo account.
- **Keyboard**: rows with key chips — Console: ←/→ monitor, Enter go live, ⌘/Ctrl+Enter go live anywhere, B black out, ⌘/Ctrl+, settings. Live window: →/↓/Space next, ←/↑ previous, B black out, Esc leave full screen then close.
- **NDI output**: runtime required; source name `mygo-demo (Live)`; OBS/DistroAV steps; the feed is the slide alone (frameless stage window) up to 1920×1080; macOS needs the stage window on screen; Settings toggle/persistence note.
- Demo-data note: songs/KJV are public domain; the set list is not saved, settings are.

`a.openHelp()` mirrors `openSettings`; `a.closeHelp()` optional (Close button only if useful — not required).

### 9. Docs (`README.md`, `AGENTS.md`, `docs/architecture.md`)
- README: monitor vs stage usage, Go Live, Settings (display, NDI, persisted location), Help, updated NDI instructions (toggle lives in Settings; header button remains), release tag.
- AGENTS: update hard rules to the new model — preview/stage separation, settings persistence only, windows created from main-thread callbacks, tests must never call `NewWindow`; update the "Where things live" table (`settings.go`, `help.go`, `menu.go`) and the state/version notes; drop the "no persistence" claim for settings.
- architecture.md: state model (`preview`/`stage`), window inventory (console, stage, settings, help), menu structure, settings JSON schema + location, display placement rules, shortcut table, edge cases, test list, known gaps.

## Edge cases and failure modes
- Empty deck: cards show empty text, Go Live/Enter/next/prev no-op, counters `0 / 0`, stage window shows "Nothing on stage".
- Removing the preview row shifts indices as specified; removing the on-air row leaves the next row (or the previous if the last was removed) on air; both indices stay valid.
- Reordering never changes what is on air; the LIVE badge follows the stage index.
- Blackout survives monitor moves and takes; Go Live while blacked out changes the slide behind black until cleared.
- Stage window closed externally: `OnClosed` guard nils the pointer and stops NDI; `ndiOn` stays true.
- NDI enabled with the stage closed: no error, hint says it starts with the stage; header button hidden as today.
- NDI start failure: `ndiErr` shows in the console strip and the Settings status line.
- Saved display unplugged at startup or runtime: fall back to Automatic, persist the fallback, reopen the stage if open. Display gained: appears in Settings on next open; Automatic begins using it without reopening until the next open.
- Corrupt/unreadable settings file or unwritable dir: defaults used, save failures logged, app keeps running.
- Multiple Settings/Help opens: focus the existing window, never duplicate.
- `MYGO_NDI=1`: session-only NDI preference override, not written to disk.
- Window-creation rule preserved: no `mygo.NewWindow` from the build pass or tests.

## Tests
- `deck_test.go`: monitor-only add/select; take; remove monitor vs stage row; reorder keeps stage content; stage shortcuts sync monitor; empty deck.
- `settings_test.go`: settings round-trip; missing/corrupt defaults; app-level save/load via `t.TempDir()`; `chooseDisplay` (explicit present, auto projector, primary-only, explicit missing).
- `view_test.go`: deck click moves only the monitor (counters `1 / N` vs `2 / N`); Go Live and Enter take the slide (old stage counter disappears); arrows move exactly one row; LIVE badge on the stage row; empty-deck console; settings view lists displays and NDI switch with fabricated `a.displays`; help view renders the shortcut/NDI sections; `buildMenu` item IDs/accelerator.
- `output_test.go`: live view renders the stage slide, not the monitor slide; next/prev in the live window move stage + preview; blackout hides text; `setNDI(true)` without a stage sets the expected error (or is a no-op when unsupported).
- Keep existing auth/NDI/data tests green.

## Gates and acceptance
Run with the sandbox cache (`GOCACHE="$TMPDIR/gocache-mygo-demo"`):
- `gofmt -l *.go` empty; `CGO_ENABLED=0 go build ./...`; `GOOS=darwin go build ./...`; `GOOS=windows go build ./...`; `GOOS=linux CGO_ENABLED=0 go build ./...`.
- `go vet ./...`, `GOOS=windows go vet ./...`, `GOOS=linux CGO_ENABLED=0 go vet ./...`.
- `CGO_ENABLED=0 go test -count=1 ./...`.
- Headless screenshots with `ui.Tester` + `Image()` for console (light/dark) at 1280×800 and 1040×660, plus Settings and Help; visual check for panel overflow.
- Manual smoke: launch the binary from a temp build for a few seconds, open Settings/Help from the menu, verify the display list and Go Live behavior.
- Merge `feat/monitor-settings` fast-forward into main, tag `v0.2.0-beta.3`, push main + tag; wait for CI and confirm the prerelease has the usual 8 assets. `mygo.config.ts` stays at `0.2.0`.

Acceptance checklist:
1. Menus Settings/Help (and Edit) exist on macOS, Windows and Linux builds; `Cmd/Ctrl+,` opens Settings.
2. Settings lists real displays, moves an open stage immediately, starts/stops NDI, and survives a restart; missing/corrupt settings fall back safely.
3. Slide selection/arrows never change the stage; Go Live/Enter does; LIVE badge and counters track stage vs monitor; stage window and NDI show the stage only.
4. Live-window navigation advances both stage and monitor.
5. Help explains shortcuts, NDI and version; About opens the platform-appropriate surface.
6. All gates green; README/AGENTS/architecture match the code; prerelease published.

## Assumptions
- The monitor is the preview card inside the console; a separate third “preview window” is out of scope.
- One stage window at a time; NDI publishes that window only.
- Display ID 0 means Automatic (MyGo display IDs are non-zero in practice); the value is persisted as-is.
- Settings persistence stores only display + NDI; the run of show, filter text and search stay session-only.
- MyGo stays pinned at v0.10.0 (no dependency bump); all APIs used are from that version.
- App version stays `0.2.0`; this work ships as `v0.2.0-beta.3`.
