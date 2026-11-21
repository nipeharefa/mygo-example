# AGENTS.md

`mygo-demo` is a demo desktop app: a small worship presenter for a church
screen. It keeps a run of show of public-domain hymns and King James
verses, shows one slide on a separate output window, and can publish that
window as an NDI source on macOS, Windows and Linux. The Go side is the
whole app: package `main` builds two native windows with
`github.com/egoist/mygo/ui`, and there is no network access, no database,
and no web page in use.

The sign-in is a demo gate, not security: the account is the
`demoUsername`/`demoPassword` pair in `main.go`, `signedIn` lives on
`app`, and nothing survives the process. There is no persistence at all:
closing the app loses the run of show.

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
- **Mutate state from callbacks, then redraw both windows.** Every deck
  method ends with `a.changed()`, which invalidates `a.win` and `a.live`.
  Call it from `OnClick`/`OnSubmit`/`OnChange`/shortcut callbacks, never
  from inside the build pass.
- **The live window is the output, and NDI publishes only it.** The
  window is `Frameless`, so the slide fills it and the capture carries no
  window chrome; the slide columns are `DragWindow()` handles so a
  windowed output can still be moved. `a.live`
  is created by `openOutput` and nilled in its `OnClosed` (guarded so a
  stale close cannot drop a new window) and in `closeOutput`. `startNDI`
  fails with `NDI output needs the live window` when `a.live == nil`; it
  never falls back to the console window. Start, present and stop NDI from
  main-thread callbacks only.
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
- **Do not edit `src/mygo.ts` by hand.** It is `mygo generate` output, and
  the web frontend is currently unused (see "Known state").
- **NDI output stays pure Go, and the binding lives in one file.**
  `ndi_sender.go` (`//go:build darwin || linux || windows`) owns the
  library-facing half — runtime discovery, binding, sender lifecycle, the
  frame loop — for every platform. Each platform file supplies only the
  dynamic-library calls and the window capture: `ndi_darwin.go` uses
  `purego` plus CoreGraphics, `ndi_windows.go` uses `syscall` plus GDI,
  `ndi_linux.go` uses `purego` plus GDK. No cgo, no SDK headers, no
  third-party NDI package. `ndi_other.go` is the stub for the remaining
  systems and is the only file with `ndiSupported() == false`. Every
  platform reads native window objects, so touch them on the main thread
  only: Linux goes through `mygo.RunOnMain` for each frame, because GDK is
  not thread safe. `ndi_windows_test.go` pins the struct offsets the C ABI
  requires; `ndi_sender_test.go` pins the versioned table's indices against
  a real runtime. Run `GOOS=windows go vet ./...` and
  `GOOS=linux go vet ./...` before trusting a change there. Resolve the
  runtime through that table first — NDI 6 runtimes do not export the
  individual functions, and `NDIlib_send_create_v2` no longer exists in the
  SDK (the export is `NDIlib_send_create`). On macOS the capture only works
  while the window is on screen; that limit is the window server's, so
  report it instead of working around it, and do not add an AppKit pixel
  readback — objc struct-return through `purego` crashed. macOS captures
  at the display's pixel density and `ndiFrameSize` scales the frame down
  to at most 1920×1080 (`ndi_darwin_test.go` pins that rule); Windows and
  Linux send the window's own client pixels.

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
MYGO_NDI=1 go run .              # ...and starts NDI when the output opens
bun run build                    # packages the app into build/
bun run build -- -platform darwin/arm64,darwin/amd64
git tag -a v0.2.0-beta.1 -m "..." && git push origin v0.2.0-beta.1   # publishes a release
```

A `v*` tag runs the full gate and packaging matrix, then attaches the
installers to a GitHub release. The tag's base version must match
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
| `main.go` | The app: `app` state, the demo account, sign-in/out, `changed`, the NDI toggle, and `main()` (console window, `MYGO_NDI`, `OnQuit`). |
| `data.go` | The library: 5 public-domain hymns (10 sections) and 4 KJV verse packs (16 verses), plus `songByID`/`verseByRef`. No data here is fetched or stored. |
| `deck.go` | The run-of-show model: `slide`, `slideKind`, add/select/next/prev/remove/move/blackout, `currentSlide`/`upcomingSlide`, `clampIndex`. |
| `console.go` | The console view: top bar, library (Songs/Bible tabs, search, pack picker), run of show list, stage panel, and the login card. |
| `output.go` | The live window: `liveView`, `openOutput`/`closeOutput`/`togglePresent`, and `secondDisplay` for fullscreen on a projector. |
| `theme.go` | The only file with literal colors: warm console theme (light/dark) and the forced-dark stage theme. |
| `view_test.go` | Headless console tests: login gate, library, add/search, arrow shortcuts, blackout, empty deck, sign out. |
| `deck_test.go` | Headless model tests: data reachability, ids, clamping, reorder, blackout, `changed()` without windows. |
| `output_test.go` | Headless output tests: current slide, follow next, blackout hides text, output shortcuts, NDI refuses without the live window. |
| `ndi_sender.go` | The NDI half every platform shares: candidate walk, versioned-table binding with a named-export fallback, `startNDISender`, the frame loop, `Stop`. |
| `ndi_sender_test.go` | Proves the table indices (0/8/9/50) match the functions a real installed runtime also exports by name; skips without NDI. |
| `ndi_darwin.go` | `purego` dlopen/dlsym plus `CGWindowListCreateImage` capture of the frameless live window at the display's pixel density, capped to 1920×1080 by `ndiFrameSize`, on screen only. |
| `ndi_windows.go` | The NDI sender: `syscall` binding of the NDI DLL plus GDI `PrintWindow` capture into a 32-bit DIB. |
| `ndi_windows_test.go` | Pins the offsets of `ndiVideoFrame`/`ndiSendCreate` to what the C ABI declares; runs on the Windows CI job. |
| `ndi_linux.go` | `purego` dlopen/dlsym plus `gdk_pixbuf_get_from_window` capture, marshalled onto the main thread with `mygo.RunOnMain`. |
| `ndi_other.go` | The stub for every remaining system: compiles the app, hides the toggle and ignores a start. |
| `README.md` | What a downloader needs: installers, the demo account, running a service, shortcuts, and the per-platform NDI instructions. |
| `mygo.config.ts` | Packaging: name, identifier, version, output directory. `devUrl`, `devCommand`, `buildCommand` and `frontendDist` are inert for a native view. |
| `src/`, `index.html`, `vite.config.ts` | Leftover web template; nothing renders it. |
| `.github/workflows/ci.yml` | The test gate, a native Windows run that launches the binary, the macOS/Windows/Linux packaging matrix, and a release job on `v*` tags. |
| `docs/architecture.md` | How the app works; read this next. |
| `docs/superpowers/plans/` | Historical plans, including the 0.2.0 worship-presenter plan. |

## Known state, deliberately untouched

The web scaffold (`src/`, `index.html`, `vite.config.ts`, the `devUrl` and
frontend settings in `mygo.config.ts`) and the `tick` event in
`src/mygo.ts` predate the native UI and are unused. `dist/` is still built
and embedded, but no window loads it. There is no persistence, no
undo/redo, and no media beyond text. These are recorded in
`docs/architecture.md` under "Known drift and gaps"; do not clean them up
unasked.
