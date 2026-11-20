# AGENTS.md

`mygo-demo` is a demo desktop app: a native-user-interface Kubernetes
resource explorer mock, behind a local sign-in card. Package `main` builds
one window with `github.com/egoist/mygo/ui`; there is no Kubernetes
client, no network access, and no web page. The Go side is the whole app.

On macOS, Windows and Linux the window can also publish itself as an NDI
source, so OBS or the NDI tools can take it as a live video input; each
platform needs the NDI runtime installed first.

The sign-in is a demo gate, not security: the account is the
`demoUsername`/`demoPassword` pair in `main.go`, `signedIn` lives on `app`,
and nothing survives the process.

**Read [docs/architecture.md](docs/architecture.md) before changing the
view.** It describes the state model, the frame contract, the widget
choices, and the checklist for adding a resource kind. The MyGo lifetime
and identity rules live in
[.agents/skills/mygo-maintenance/SKILL.md](.agents/skills/mygo-maintenance/SKILL.md).

## Hard rules

- **No cgo.** Build and test with `CGO_ENABLED=0`, as CI does.
- **The view is a function of state.** `app.view` rebuilds the interface
  from the `app` struct on every pass; never store a `ui.Element`, a
  widget's parts, or a `*ui.Context` in `app` or in a goroutine. Elements
  expire before the next build pass.
- **Colors come from `c.Theme()`.** No literal colors, so the light and
  dark appearance both work.
- **Verify against the pinned API, not memory.** Resolve the source with
  `go list -m -f '{{.Dir}}' github.com/egoist/mygo`, then read
  `ui/doc.go` and the widget you are using. The app pins v0.10.0 in
  `go.mod`; `package.json` still points the unused web tooling at 0.7.x.
- **Keep dummy data in `resources` and `groups`.** A kind needs at least
  one object, and its id must appear in a `groups` entry, or the sidebar
  cannot reach it.
- **Rows center their children vertically.** `ui.Row` defaults to
  `AlignItems(Center)`; set `.AlignItems(ui.Stretch)` when a child is
  meant to fill the row's height, as the table/inspector row does.
- **Keep the sign-in a demo gate.** Credentials are the two constants in
  `main.go`; no session, no hashing, no network. Sign-in and sign-out
  mutate `app` from `OnClick`/`OnSubmit`/`OnChange` callbacks (which run
  before the rebuild), never from inside the build pass.
- **Do not edit `src/mygo.ts` by hand.** It is `mygo generate` output, and
  the web frontend is currently unused (see "Known state").
- **NDI output stays pure Go, one file per platform.** `ndi_darwin.go`
  binds the NDI runtime and CoreGraphics through `purego`;
  `ndi_windows.go` loads the DLL with `syscall` and captures through GDI;
  `ndi_linux.go` uses `gondi` plus GDK. No cgo, no SDK headers, no
  third-party NDI package on Windows. `ndi_other.go` is the stub for the
  remaining systems and is the only file with `ndiSupported() == false`.
  Start and stop the sender from main-thread callbacks (`OnClick`,
  `WhenReady`, `OnQuit`), never from the build pass. Every platform reads
  native window objects, so touch them on the main thread only: Linux goes
  through `mygo.RunOnMain` for each frame, because GDK is not thread safe.
  `ndi_windows_test.go` pins the struct offsets the C ABI requires; run
  `GOOS=windows go vet ./...` before trusting a change there.

## Commands

```sh
CGO_ENABLED=0 go build ./...     # compiles the app
GOOS=darwin go build ./...       # CI cross-compiles macOS, Windows and Linux
GOOS=windows go build ./...
GOOS=linux CGO_ENABLED=0 go build ./...   # CGO must be 0: the macOS SDK has no setresgid
go vet ./...                     # the real vet gate for this module
test -z "$(gofmt -l *.go)"       # CI formatting gate
CGO_ENABLED=0 go test ./...      # view test, headless via ui.NewTester
go run .                         # opens the window for a manual check
MYGO_NDI=1 go run .              # ...and publishes it as an NDI source
bun run build                    # packages the app into build/
bun run build -- -platform darwin/arm64,darwin/amd64
git tag -a v0.1.0 -m "..." && git push origin v0.1.0   # publishes a release
```

A `v*` tag runs the full gate and packaging matrix, then attaches the
installers to a GitHub release. The tag's base version must match
`version` in `mygo.config.ts`, and a hyphen (`v0.1.0-beta.1`) publishes a
prerelease.

`go tool mygo vet .` (the skill's command) does not work here: the pinned
module ships no `mygo` tool, so plain `go vet ./...` is the gate.

A sandboxed agent session must point the Go cache at a writable
directory, for example `GOCACHE="$TMPDIR/gocache-mygo-demo"`, because
`~/Library/Caches/go-build` is denied under the workspace sandbox.

## Where things live

| Path | What |
| --- | --- |
| `main.go` | The app: `app` state, `resources`/`groups` dummy data, the view tree, `main()`, the NDI toggle. |
| `main_test.go` | The headless view tests: sign-in gate, validation, Enter submit, sign-out, kind switching, and the NDI toggle without a window. |
| `ndi_darwin.go` | The NDI sender: `CGWindowListCreateImage` capture of our own window through `purego`, then `gondi` frames. |
| `ndi_windows.go` | The NDI sender: `syscall` binding of the NDI DLL plus GDI `PrintWindow` capture into a 32-bit DIB. |
| `ndi_windows_test.go` | Pins the offsets of `ndiVideoFrame`/`ndiSendCreate` to what the C ABI declares; runs on the Windows CI job. |
| `ndi_linux.go` | The NDI sender: `gondi`, and `gdk_pixbuf_get_from_window` capture marshalled onto the main thread with `mygo.RunOnMain`. |
| `ndi_other.go` | The stub for every remaining system: compiles the app, hides the toggle and ignores a start. |
| `README.md` | What a downloader needs: installers, the demo account, and the per-platform instructions for getting the window into OBS. Keep its NDI sections honest: the app publishes NDI on all three platforms, but needs the NDI runtime installed first. |
| `mygo.config.ts` | Packaging: name, identifier, output directory. `devUrl`, `devCommand`, `buildCommand` and `frontendDist` are inert for a native view. |
| `src/`, `index.html`, `vite.config.ts` | Leftover web template; nothing renders it. |
| `.github/workflows/ci.yml` | The test gate, a native Windows run that launches the binary, the macOS/Windows/Linux packaging matrix, and a release job on `v*` tags. |
| `docs/architecture.md` | How the app works; read this next. |
| `docs/superpowers/plans/` | Historical, already-implemented plan; its numbers have drifted from the code. |

## Known state, deliberately untouched

The web scaffold (`src/`, `index.html`, `vite.config.ts`, the `devUrl` and
frontend settings in `mygo.config.ts`) and the `tick` event in
`src/mygo.ts` predate the native UI and are unused. `dist/` is still built
and embedded, but no window loads it. These are recorded in
`docs/architecture.md` under "Known drift and gaps"; do not clean them up
unasked.
