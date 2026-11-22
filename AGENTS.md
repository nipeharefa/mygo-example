# AGENTS.md

`mygo-demo` is a demo desktop app: a small worship presenter for a church
screen. It keeps a run of show of public-domain hymns and King James
verses, previews the next slide on a monitor and sends one slide to a
separate stage window, and can publish that stage as an NDI source on
macOS, Windows and Linux. The NDI feed draws the slide itself, so publishing
needs no window on screen and one monitor is enough. The Go side is the whole app: package `main`
builds the console plus on-demand stage, Settings and Help windows with
`github.com/egoist/mygo/ui`, and there is no network access, no database,
and no web page in use.

The sign-in is a demo gate, not security: the account is the
`demoUsername`/`demoPassword` pair in `main.go`, `signedIn` lives on
`app`, and nothing lasts beyond the process except the display and NDI
preferences, which are a small JSON file in the app data directory.

**Read [docs/architecture.md](docs/architecture.md) before changing the
view.** It describes the state model, the two windows, the frame contract,
the widget choices, and the testing approach. The MyGo lifetime and
identity rules live in
[.agents/skills/mygo-maintenance/SKILL.md](.agents/skills/mygo-maintenance/SKILL.md).

## Hard rules

- **No cgo.** Build and test with `CGO_ENABLED=0`, as CI does.
- **The view is a function of state.** `app.view` and `app.liveView`
  rebuild the interface from the `app` struct on every pass; never store a
  `ui.Element`, a widget's parts, or a `*ui.Context` in `app` or in a
  goroutine. Elements expire before the next build pass.
- **Colors come from the theme.** Literal colors live only in `theme.go`:
  `consoleTheme` for the console, `stageTheme` for the output, and
  `stageBackgroundHex` for the output window's background. Everything else
  reads `c.Theme()`, so light and dark appearance and the system accent and
  text-scale preferences keep working.
- **Mutate state from callbacks, then redraw every window.** Every state
  method ends with `a.changed()`, which invalidates `a.win`, `a.live`,
  `a.settings` and `a.help` (each nil-safe). Call it from
  `OnClick`/`OnSubmit`/`OnChange`/shortcut/menu callbacks, never from
  inside the build pass.
- **The stage is the output, and NDI publishes only it.** The feed is
  drawn, not captured: `ndi_render.go` runs `ui.Render` over `a.liveView`
  on the main thread and hands the sender a BGRA frame, so the feed shows
  the slide whatever the stage window is doing — closed, behind the
  console, or on another display. The window is `Frameless`, so the slide
  fills it; the slide columns are `DragWindow()` handles so a windowed
  output can still be moved. `a.live` is created by `openOutput` and nilled
  in its `OnClosed` (guarded so a stale close cannot drop a new window) and
  in `closeOutput`. Publishing does not need it: `startNDI` never reads
  `a.live`, and closing the stage keeps the source running. Only sign-out
  and the NDI switch stop it. Start, present and stop NDI from
  main-thread callbacks only.
- **The feed never crops and never drags Stop down.** The stage is laid out
  at 1280×720 (`ndiStageWidth`/`ndiStageHeight`) and drawn at the scale the
  frame needs, so a larger or oddly shaped frame is the whole slide made
  bigger, then averaged down; `shrinkFrame` must never take its crop path
  for a size `ndiRenderSize` allows. The sender is asynchronous
  (`clockVideo` 0), so sending runs on the NDI thread, never on the main
  loop. The app is the frame source (`(*app) frame`). That loop cannot draw
  itself: `drawNdiFrame` posts the render with `mygo.RunOnMain` and asks the
  console to paint, and the picture arrives in the pass that follows, so a
  draw that would have to wait for the very pass that asked for it is
  simply skipped. `ndiStream.draw` runs the source on a helper goroutine and
  selects on the stream's stop channel, so Stop never waits for a draw.
- **The monitor never reaches the room by itself.**
  `selectSlide`/`nextSlide`/`prevSlide` move `a.preview`; only
  `takeSlide` (Go Live, Enter on the list, Cmd/Ctrl+Enter) and the stage
  window's own next/prev move `a.stage`. `nextStage`/`prevStage` pull the
  monitor along so the highlight follows the room. Do not wire a console
  control straight to `a.stage`.
- **Keep the song and scripture text public domain.** `data.go` holds the
  5 traditional hymns and the 4 KJV verse packs; the KJV (1611) and those
  hymn texts are public domain. Do not paste in copyrighted lyrics or a
  modern Bible translation. Each song needs at least one section, and every
  verse reference must be unique, because the deck and the library look
  verses up by reference (`deck_test.go` checks both).
- **Verify against the pinned API, not memory.** Resolve the source with
  `go list -m -f '{{.Dir}}' github.com/egoist/mygo`, then read `ui/doc.go`
  and the widget you are using. The app pins v0.10.0 in `go.mod`;
  `package.json` still points the unused web tooling at 0.7.x.
- **Rows center their children vertically.** `ui.Row` defaults to
  `AlignItems(Center)`; set `.AlignItems(ui.Stretch)` when a child is meant
  to fill the row's height, as the console's three-column row does.
- **Keep the sign-in a demo gate.** Credentials are the two constants in
  `main.go`; no session, no hashing, no network. Sign-in and sign-out
  mutate `app` from callbacks, never from inside the build pass. Sign-out
  closes the output and stops NDI.
- **Windows are created outside the event that asks for them, and tests
  never open one.** `mygo.NewWindow` is called only through `a.newWindow`,
  from `openOutput` and `openHelp` after `WhenReady`, never from the build
  pass and never from a test, because a headless `ui.Tester` has no
  application. The console asks for the stage with `requestStage`;
  `settle()` turns that into `a.openStage`, and `view` asks for a later
  pass (`c.After(openStageDelay)`), so `openOutput` runs in a frame that no
  event is waiting on. That is what keeps Open output from hanging: creating
  and showing a window inside the click, and then placing it with
  `SetPosition`/`SetFullScreen`, waited on a window that was not on screen
  yet. Never call a `mygo.Window` method from inside the build pass either;
  ask for another pass instead. Tests inject `a.newWindow`/`a.mainScreen`
  or leave them alone; either way they never call one. `buildMenu` is
  headless-safe and its structure is tested; its click callbacks are not.
- **The stage opens once.** `openOutput` places it from a display
  (`stageDisplayFound`) or keeps it windowed, and `placeStage` re-asserts
  that on enter-fullscreen. Only an explicit action (Open output, picking a
  display) or the display the stage is using going away (`refreshDisplays`)
  closes and reopens it. Never reopen it from a display notification, and
  nil `a.live` before `Close()` — a `Close()` whose `OnClosed` guard has
  already been satisfied never destroys the window.
- **Settings persistence is the only persistence.**
  `settings.go` reads and writes `settings.json` (display ID, NDI on/off,
  NDI frame size)
  in `mygo.App.Path(mygo.PathUserData)`. `a.settingsDir` is empty until
  `WhenReady` and in tests, and `saveSettings` then does nothing. The run
  of show is session-only; do not persist it unasked.
- **Do not edit `src/mygo.ts` by hand.** It is `mygo generate` output, and
  the web frontend is currently unused (see "Known state").
- **NDI output stays pure Go, and the binding lives in one file.**
  `ndi_sender.go` (`//go:build darwin || linux || windows`) owns the
  library-facing half — runtime discovery, binding, sender lifecycle, the
  frame loop — for every platform. Frames come from an `ndiFrameSource`, not
  from a window: the app is the only source (`(*app) frame`), and
  `ndi_render.go` draws its frames. Each platform file supplies only the
  dynamic-library calls: `ndi_darwin.go` uses `purego`, `ndi_windows.go`
  uses `syscall`, `ndi_linux.go` uses `purego`. No cgo, no SDK headers, no
  third-party NDI package. `ndi_other.go` and `ndi_render_stub.go` are the
  stubs for the remaining systems and are the only files with
  `ndiSupported() == false`; keep `GOOS=freebsd CGO_ENABLED=0 go build ./...`
  compiling, and keep the headless files free of `ndiFrame` by name: the
  test seam `a.drawNdi` returns `any` for that reason.
  `ndi_windows_test.go` pins the struct offsets the C ABI requires;
  `ndi_sender_test.go` pins the versioned table's indices against a real
  runtime. Run `GOOS=windows go vet ./...` and
  `GOOS=linux go vet ./...` before trusting a change there. Resolve the
  runtime through that table first — NDI 6 runtimes do not export the
  individual functions, and `NDIlib_send_create_v2` no longer exists in the
  SDK (the export is `NDIlib_send_create`). `ndiFrameSize` fits a frame into
  1920×1080 keeping its aspect (`ndi_darwin_test.go` pins that rule), and
  the app asks for 720p by default or 1080p from Settings (`a.ndiFrameSize`).
  Do not bring back a window capture: it needed the window on screen, which
  is exactly what a single-monitor machine cannot give it, and the AppKit
  pixel readback crashed through `purego`.

## Commands

```sh
CGO_ENABLED=0 go build ./...     # compiles the app
GOOS=darwin go build ./...       # CI cross-compiles macOS, Windows and Linux
GOOS=windows go build ./...
GOOS=linux CGO_ENABLED=0 go build ./...   # CGO must be 0: the macOS SDK has no setresgid
go vet ./...                     # the real vet gate for this module
test -z "$(gofmt -l *.go)"       # CI formatting gate
CGO_ENABLED=0 go test ./...      # view, deck and output tests, headless via ui.NewTester
go run .                         # opens the console for a manual check
MYGO_NDI=1 go run .              # ...and publishes the stage over NDI
go build -tags e2e -o demo .     # adds the automation socket (automation_e2e.go)
bun run build                    # packages the app into build/
bun run build -- -platform darwin/arm64,darwin/amd64
git tag -a v0.2.0-beta.1 -m "..." && git push origin v0.2.0-beta.1   # publishes a release
```

A `v*` tag runs the full gate and packaging matrix, then attaches the
installers to a GitHub release. The build signs update archives and
`update-<platform>.json` manifests with `MYGO_UPDATER_PRIVATE_KEY` (the
repository secret), which is the private half of `updates.publicKey` in
`mygo.config.ts`; without the secret the release job fails rather than ship
installers nothing can update. Keep the key private: installed apps accept
only updates signed with it. The tag's base version must match
`version` in `mygo.config.ts`, and a hyphen (`v0.2.0-beta.1`) publishes a
prerelease.

`go tool mygo vet .` (the skill's command) does not work here: the pinned
module ships no `mygo` tool, so plain `go vet ./...` is the gate.

A sandboxed agent session must point the Go cache at a writable
directory, for example `GOCACHE="$TMPDIR/gocache-mygo-demo"`, because
`~/Library/Caches/go-build` is denied under the workspace sandbox.

## Where things live

| Path | What |
| --- | --- |
| `main.go` | The app: `app` state and its `newWindow`/`mainScreen` seams, the demo account, sign-in/out, `changed`, `requestStage`/`settle`, `tickStats`, `setNDI`/`toggleNDI`/`ndiLabel`, `main()` startup order (app data dir, saved preferences, menu, display subscription, console window, first display refresh, stats timer), and the `stageSourceName` constant. |
| `data.go` | The library: 5 public-domain hymns (10 sections) and 4 KJV verse packs (16 verses), plus `songByID`/`verseByRef`. No data here is fetched or stored. |
| `deck.go` | The run-of-show model: `slide`, `slideKind`, add/select/next/prev (monitor), `takeSlide`, `nextStage`/`prevStage`, remove/move/blackout, `reorderSlides` (the drag and Move up/down path, `to` is a gap), `stageSlide`/`previewSlide`/`upcomingSlide`, `clampSlides`. |
| `console.go` | The console view: top bar, library (Songs/Bible tabs, search, pack picker), run of show list with the LIVE badge, the two-card stage/monitor panel with Go Live, and the login card. |
| `output.go` | The stage window: `liveView`, stage shortcuts, `openOutput`/`closeOutput`/`togglePresent`, `placeStage`/`shutStage`. |
| `theme.go` | The only file with literal colors: warm console theme (light/dark) and the forced-dark stage theme. |
| `menu.go` | `buildMenu` (app, edit, Settings, Help menus) and `showAbout`. |
| `settings.go` | The Settings page of the console: display radios, NDI switch and status line, the NDI stream stats, `settings.json` read/write, `loadSettings`/`saveSettings`, `refreshDisplays`. |
| `help.go` | The Help window: about, keyboard table, NDI notes, sample-data notes. |
| `view_test.go` | Headless console tests: login gate, library, add/search, monitor vs stage (click, Go Live, Enter, Cmd+Enter), arrows, blackout, empty deck, sign out. |
| `menu_test.go` | Headless menu structure: item IDs, accelerator, click handlers present (clicks not run). |
| `settings_test.go` | Settings round-trip (display, NDI, frame size), missing/corrupt defaults, legacy `outputDisplay` migration, `stageDisplayFound`, the Settings page rendering, display refresh without reopening the stage. |
| `deck_test.go` | Headless model tests: data reachability, ids, clamping, reorder, blackout, `changed()` without windows. |
| `output_test.go` | Headless stage tests: the stage slide (not the monitor), next/prev move both, blackout hides text, stage shortcuts, NDI publishing with no window, closing the stage keeping it, sign-out stopping it, the status line, and the frame size. |
| `ndi_sender.go` | The NDI half every platform shares: candidate walk, versioned-table binding with a named-export fallback, `startNDISender`, the frame loop, `Stop`, and the `ndiStats` counters `snapshot` reports. |
| `ndi_stats_test.go` | The stats counters and rate without a runtime; skips where NDI is not built. |
| `ndi_sender_test.go` | Proves the table indices (0/8/9/50) match the functions a real installed runtime also exports by name; skips without NDI. |
| `ndi_darwin.go` | The `purego` dynamic-library calls: dlopen candidates, dlsym, the runtime's own functions. No capture, no window handle. |
| `ndi_windows.go` | The `syscall` dynamic-library calls into the NDI DLL. No capture, no window handle. |
| `ndi_windows_test.go` | Pins the offsets of `ndiVideoFrame`/`ndiSendCreate` to what the C ABI declares; runs on the Windows CI job. |
| `ndi_linux.go` | The `purego` dynamic-library calls into the NDI library. No capture, no window handle. |
| `ndi_other.go` | The stub for every remaining system: `ndiSupported() == false`, a no-op stream and a sender that always fails. |
| `ndi_render.go` | The feed's draw: `renderFrame` (RGBA to BGRA), `ndiFrameSize`/`ndiRenderSize` (the HD cap), `renderStageFrame` (the `ui.Render` draw) and `shrinkFrame`. The main-thread handoff lives with the sender, in `drawNdiFrame`. |
| `ndi_render_stub.go` | The same size functions for the builds without a sender, so `main.go` compiles everywhere. |
| `automation_e2e.go` | The `e2e`-tagged import of the MyGo automation plugin, so a built app can be driven over `MYGO_AUTOMATION`; no release build has it. |
| `ndi_source_test.go` | The rendered feed: BGRA order, the size cap, the whole slide at odd and large sizes, a blacked-out frame, following the stage, and Stop not waiting on a draw. |
| `README.md` | What a downloader needs: installers, the demo account, running a service, shortcuts, and the per-platform NDI instructions (the runtime, not a window, is what NDI needs). |
| `mygo.config.ts` | Packaging: name, identifier, version, output directory. `devUrl`, `devCommand`, `buildCommand` and `frontendDist` are inert for a native view. |
| `src/`, `index.html`, `vite.config.ts` | Leftover web template; nothing renders it. |
| `.github/workflows/ci.yml` | The test gate, a native Windows run that launches the binary, the macOS/Windows/Linux packaging matrix, and a release job on `v*` tags. |
| `docs/architecture.md` | How the app works; read this next. |
| `docs/superpowers/plans/` | Historical plans: the 0.2.0 worship presenter, the monitor/Settings beta, the stage/Settings fix in beta.4, and the rendered NDI feed in beta.5. |

## Known state, deliberately untouched

The web scaffold (`src/`, `index.html`, `vite.config.ts`, the `devUrl` and
frontend settings in `mygo.config.ts`) and the `tick` event in
`src/mygo.ts` predate the native UI and are unused. `dist/` is still built
and embedded, but no window loads it. The run of show is not persisted,
there is no undo/redo, and no media beyond text; only the display and NDI
preferences survive a restart. These are recorded in
`docs/architecture.md` under "Known drift and gaps"; do not clean them up
unasked.
