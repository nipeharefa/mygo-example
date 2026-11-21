# Render-to-NDI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpower-subagent-driven-development (recommended) or superpower-executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish the stage over NDI from a rendered image of the slide instead of a screen capture of the stage window, so one monitor is enough and neither macOS nor Windows needs the window to be on screen.

**Architecture:** `ndi_sender.go` stops owning a capturer and asks for one `ndiFrame` through a `ndiFrameSource` interface; the app implements it in `ndi_render.go`. The implementation renders `liveView` into an `*image.RGBA` with `ui.Render` on the main thread and converts it to the BGRA bytes NDI wants. `startNDI` no longer needs `a.live`, and Settings gets an NDI size preference. The three platform files lose their capture code and keep only the library lookup, install hint and failure copy.

**Tech Stack:** Go 1.27, `github.com/egoist/mygo` v0.10.0 (`ui.Render`, `mygo.RunOnMain`), `github.com/ebitengine/purego` for the NDI dynamic library, headless `ui.NewTester` for tests.

**Spec:** [docs/superpowers/plans/2026-11-22-render-to-ndi-spec.md](2026-11-22-render-to-ndi-spec.md) — the design decisions, the measurements and the platform constraints this plan implements.

## Global Constraints

- **No cgo.** Every build and test runs with `CGO_ENABLED=0`.
- **Literal colors live only in `theme.go`.** The frame render reads `stageTheme()` and `stageBackgroundHex`; no hex value appears in `ndi_render.go`.
- **`ui.Render` draws a frame of its own and must run on the main thread**, never inside a window's frame. The sender goroutine reaches the main thread with `mygo.RunOnMain` (`loop.go:88`), which runs the function directly when called on the main thread and **drops it after the event loop shuts down** — so the handoff must never block unboundedly; it selects on `s.stop`.
- **The stage window stays a viewer.** When it is open it shows the same slide the feed sends; it is never required for publishing.
- **NDI publishes `a.stage` only.** The monitor still never reaches the room by itself.
- **Frame format stays NDI's BGRA, top row first**, 4 bytes a pixel, `lineStride = 4*W`.
- **NDI output sizes:** `0` = 720p (1280×720, the default), `1` = 1080p (1920×1080). Both 16:9, so `ndiRenderSize` keeps an aspect ratio it may later be given.
- **Tests never construct a window and never touch the real screen or runtime.** They use `app.frameSource()` with an injected size and `ui.Render`.
- Sandboxed sessions must export `GOCACHE="$TMPDIR/gocache-mygo-demo"` before any `go` command; `~/Library/Caches/go-build` is denied.
- The gate is `test -z "$(gofmt -l *.go)"`, `go vet ./...`, `GOOS=windows CGO_ENABLED=0 go vet ./...`, `GOOS=linux CGO_ENABLED=0 go vet ./...`, and `CGO_ENABLED=0 go test ./...`.

## Review Focus

1. **Publishing with no stage window open, on one monitor** — the switch must start a feed that carries the stage slide while the console stays usable.
2. **Pressing next/previous or blackout while publishing** — the feed must show the stage within a frame or two, never a stale slide, and a blackout must reach the feed as black.
3. **Stopping NDI while a frame handoff is waiting on the main thread** — `Stop` must return, with no goroutine left blocked and no send into a destroyed sender.
4. **Quitting or signing out with NDI still running** — the sender must stop cleanly, without a panic from the destroyed runtime.
5. **A machine with no NDI runtime, or one where the runtime fails to initialize** — the Settings switch reports the error and leaves the preference consistent; it does not hang or claim success. Pinned in Task 3 by `TestNDIStatusReportsAFailure`: set `a.ndiErr = "the NDI runtime failed to initialize"` and `a.ndiOn = true` directly (no runtime needed, as `settings_test.go` already does for the settings surface) and assert `ndiStatus` returns a string starting with `NDI:` and `t.Danger`, and that `setNDI(false)` clears `a.ndiErr`. The real load path is already covered by `ndi_sender_test.go`, which skips without a runtime.

---

### Task 1: The sender takes frames from a source

**Files:**
- Modify: `ndi_sender.go` (frame types, `ndiCapturer` removal, `startNDISender`, the loop)
- Modify: `ndi_darwin.go`, `ndi_windows.go`, `ndi_linux.go` (drop the capture code, keep the library half)
- Modify: `ndi_other.go` (stub signatures)
- Test: `ndi_source_test.go` (create), `ndi_darwin_test.go:1-29` (keep `TestNDIFrameSize`, delete the capture tests)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `type ndiFrame struct { pix []byte; stride, w, h int }` — top row first, four BGRA bytes a pixel. The old `windowFrame` is renamed to this; `ndiFrameSize` keeps returning the frame size.
  - `type ndiFrameSource interface { frame(w, h int) (ndiFrame, bool) }`
  - `func startNDISender(src ndiFrameSource, w, h int) (stream *ndiStream, err error)`
  - `func (s *ndiStream) run()`, `func (s *ndiStream) Stop()`, `func (s *ndiStream) snapshot() ndiStats` — unchanged API.
  - Per-platform, still required: `ndiLibraryCandidates() []string`, `ndiInstallHint() string`, `ndiCaptureFailure() string`, `openNDILibrary(path string) (uintptr, error)`, `closeNDILibrary(handle uintptr)`, `ndiSymbol(handle uintptr, name string) (uintptr, error)`, `registerFunc(lib uintptr, fn any, name string) error`.

- [ ] **Step 1: Write the failing test**

Create `ndi_source_test.go`:

```go
package main

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// A frame of one solid color, top row first, BGRA.
func solidSource(c color.RGBA) ndiFrameSource { return solid{c: c} }

type solid struct{ c color.RGBA }

func (s solid) frame(w, h int) (ndiFrame, bool) {
	pix := make([]byte, 4*w*h)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = s.c.B, s.c.G, s.c.R, s.c.A
	}
	return ndiFrame{pix: pix, stride: 4 * w, w: w, h: h}, true
}

func TestRenderBGRA(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	f := renderFrame(img)
	if f.w != 2 || f.h != 2 || len(f.pix) != 16 || f.stride != 8 {
		t.Fatalf("frame = %dx%d stride %d len %d", f.w, f.h, f.stride, len(f.pix))
	}
	if got := f.pix[0:4]; string(got) != "\x33\x22\x11\xff" {
		t.Fatalf("first pixel = % x, want BGRA", got)
	}
}

func TestNDIRenderSize(t *testing.T) {
	for _, tc := range []struct{ wantW, wantH int }{
		{1280, 720}, {1920, 1080},
	} {
		w, h := ndiRenderSize(tc.wantW, tc.wantH)
		if w != tc.wantW || h != tc.wantH {
			t.Fatalf("ndiRenderSize(%d, %d) = %d, %d", tc.wantW, tc.wantH, w, h)
		}
	}
	if w, h := ndiRenderSize(3840, 2160); w != 1920 || h != 1080 {
		t.Fatalf("a 4K request should cap at 1080p, got %d, %d", w, h)
	}
	if _, h := ndiRenderSize(1280, 800); h%2 != 0 {
		t.Fatalf("height %d is odd", h)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test -run 'TestRenderBGRA|TestNDIRenderSize' ./...`
Expected: FAIL — `undefined: ndiFrameSource`, because the test file's build tag (`//go:build darwin || linux || windows`) matches while the code does not exist yet. Put that tag as the first line of `ndi_source_test.go` when you create it.

- [ ] **Step 3: Implement the frame type and the source interface**

In `ndi_sender.go`, rename `windowFrame` to `ndiFrame` (same fields), delete `ndiCapturer`, `waitFrame` and `windowCapturer`, and add:

```go
// ndiFrameSource draws one frame for the feed at the requested size.
type ndiFrameSource interface {
	frame(w, h int) (ndiFrame, bool)
}
```

`startNDISender(src ndiFrameSource, w, h int) (*ndiStream, error)` keeps every step it has today — load the runtime, initialize it, create the sender with `stageSourceName` — but instead of building a capturer and waiting for a first frame it asks the source for one (`f, ok := src.frame(w, h)`) and returns `errors.New(ndiCaptureFailure())` when `!ok`. The stream keeps the source and the size:

```go
type ndiStream struct {
	...
	src  ndiFrameSource
	w, h int
}
```

The loop replaces `captureFrame(s.capt)` with `s.src.frame(s.w, s.h)`; the rest of the loop, `sent`, `skipped`, `failed`, `snapshot` and `Stop` stay as they are.

Also rewrite `ndi_stats_test.go`'s `fakeCapturer` as a `fakeSource` implementing `ndiFrameSource` (`frame(w, h int) (ndiFrame, bool)` returning the stored frame), since `windowFrame` and `ndiCapturer` no longer exist; `TestNDIStatsSnapshot` and `TestNDIStatsRateNeedsASecond` keep their assertions.

- [ ] **Step 4: Strip the platform capture code**

In `ndi_darwin.go`, delete everything from `func captureFrame` to the end of `windowCapturer.capture` and the `loadCapture`, `sRGBColorSpace` and `loadObjC` helpers and their variables, plus `cgWindowImageBestResolution`, `cgInterpolationHigh`, `cgRect`, `cgSize` and the `windowCapturer` type. Keep the NDI library half, `ndiFrameSize`, and `ndiMaxFrameWidth`/`ndiMaxFrameHeight`. Do the same in `ndi_windows.go` (GDI/`PrintWindow` code) and `ndi_linux.go` (`gdk_pixbuf_get_from_window` and the `mygo.RunOnMain` capture hop), leaving each file with only its library lookup, install hint, failure copy and symbol helpers. `ndi_other.go` keeps `ndiSupported() == false` and the `ndiStats` stub; it gains nothing here because `ndi_sender.go` is not built there.

- [ ] **Step 5: Run the tests and the cross vet**

Run: `CGO_ENABLED=0 go test -run 'TestRenderBGRA|TestNDIRenderSize|TestNDIFrameSize' ./... && GOOS=windows CGO_ENABLED=0 go vet ./... && GOOS=linux CGO_ENABLED=0 go vet ./...`
Expected: PASS, and vet silent on both platforms. `go build ./...` on the host fails until Task 2 because `main.go` still calls `startNDISender(a.live)`; that is the next task.

- [ ] **Step 6: Commit**

```bash
git add ndi_sender.go ndi_darwin.go ndi_windows.go ndi_linux.go ndi_other.go ndi_source_test.go ndi_darwin_test.go
git commit -m "refactor: take NDI frames from a source instead of the window"
```

---

### Task 2: Render the live view into a frame

**Files:**
- Create: `ndi_render.go` (`//go:build darwin || linux || windows`), `ndi_render_stub.go` (`//go:build !darwin && !linux && !windows`)
- Test: `ndi_source_test.go` (extend), `output_test.go` (extend)

**Interfaces:**
- Consumes: `ndiFrame`, `ndiFrameSource` from Task 1.
- Produces:
  - `func renderFrame(img *image.RGBA) ndiFrame`
  - `func ndiRenderSize(w, h int) (int, int)`
  - `func (a *app) frameSource(w, h int) ndiFrameSource` — what Task 3 hands to `startNDISender`.
  - `func (a *app) renderStageFrame(w, h int) (ndiFrame, bool)`

- [ ] **Step 1: Write the failing test**

Append to `ndi_source_test.go` (already tagged `darwin || linux || windows` in Task 1):

```go
// The stage frame is the slide, in the stage theme, at the size asked for,
// and it exists whether or not the stage window is open.
func TestRenderStageFrameFollowsTheStage(t *testing.T) {
	a := newApp()
	a.addSong("amazing-grace")
	a.addVerse("psalm23-1")
	src := a.frameSource(640, 360)

	first, ok := src.frame(640, 360)
	if !ok {
		t.Fatal("the source gave no frame")
	}
	if first.w != 640 || first.h != 360 || first.stride != 2560 {
		t.Fatalf("frame = %dx%d stride %d", first.w, first.h, first.stride)
	}
	if isBlack(first.pix) {
		t.Fatal("the frame is blank")
	}

	a.blackout = true
	black, ok := src.frame(640, 360)
	if !ok || !isBlack(black.pix) {
		t.Fatal("blackout should render a black frame")
	}
}

// Review Focus 2: a stage change reaches the next frame, and a blackout
// frame is black at every size.
func TestRenderStageFrameTracksTheStage(t *testing.T) {
	a := newApp()
	a.addSong("amazing-grace")
	a.addVerse("psalm23-1")
	src := a.frameSource(320, 180)
	before, ok := src.frame(320, 180)
	if !ok {
		t.Fatal("no first frame")
	}
	a.nextStage()
	after, ok := src.frame(320, 180)
	if !ok {
		t.Fatal("no second frame")
	}
	if string(before.pix) == string(after.pix) {
		t.Fatal("the feed did not follow the stage")
	}
	a.blackout = true
	black, ok := src.frame(320, 180)
	if !ok || !isBlack(black.pix) {
		t.Fatal("blackout must render black")
	}
	a.blackout = false
	lit, ok := src.frame(320, 180)
	if !ok || isBlack(lit.pix) {
		t.Fatal("clearing the blackout must restore the slide")
	}
}

func isBlack(pix []byte) bool {
	for i := 0; i+2 < len(pix); i += 4 {
		if pix[i]|pix[i+1]|pix[i+2] != 0 {
			return false
		}
	}
	return true
}
```

Append to `output_test.go`:

```go
// Review Focus 4: signing out leaves no sender behind.
func TestSignOutStopsNDI(t *testing.T) {
	a := newApp()
	a.ndiOn = true
	a.signedIn = true
	a.signOut()
	if a.ndi != nil {
		t.Fatal("NDI still set after sign-out")
	}
}

// NDI no longer waits for the stage window: with the preference on, the
// switch starts publishing and the stats report the configured size.
func TestSetNDIWithoutAStageRenders(t *testing.T) {
	if !ndiSupported() {
		t.Skip("this build has no NDI")
	}
	a := newApp()
	a.ndiOn = true
	a.setNDI(true)
	if a.ndiErr != "" {
		t.Skipf("no NDI runtime here: %s", a.ndiErr)
	}
	defer a.stopNDI()
	if a.ndi == nil {
		t.Fatal("NDI should start without the stage window")
	}
	if s := a.ndi.snapshot(); s.Width == 0 || s.Height == 0 {
		t.Fatalf("stats have no size yet: %+v", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test -run 'TestRenderStageFrameFollowsTheStage|TestSetNDIWithoutAStageRenders' ./...`
Expected: FAIL — `a.frameSource undefined`.

- [ ] **Step 3: Implement the renderer**

`ndi_render.go`:

```go
// renderFrame turns a rendered image into the BGRA, top-row-first frame the
// frame loop sends. The image is Go's RGBA, with premultiplied alpha that
// the stage never uses, so the bytes shuffle one to one.
func renderFrame(img *image.RGBA) ndiFrame

// ndiRenderSize fits a requested size into the HD cap, keeping the aspect
// ratio and even dimensions, as ndiFrameSize does for a captured window.
func ndiRenderSize(w, h int) (int, int)

// renderStageFrame draws liveView with ui.Render, on the main thread.
func (a *app) renderStageFrame(w, h int) (ndiFrame, bool)

// frameSource adapts renderStageFrame to the sender's goroutine: each call
// hops to the main thread, hands the frame back, and gives up when the
// stream stops first.
func (a *app) frameSource(w, h int) ndiFrameSource
```

Use `ui.Render(a.liveView, w, h, 1)` (one device pixel a DIP: the requested size already is the frame size). `frameSource` returns a `stageSource` whose `frame` method builds a buffered channel `out := make(chan ndiFrame, 1)`, calls `mygo.RunOnMain(func(){ if f, ok := a.renderStageFrame(w, h); ok { select { case out <- f: default: } } })`, then `select { case f := <-out: return f, true; case <-stop: return ndiFrame{}, false }`. The stage source carries the stream's stop channel — the `stop` field `ndiStream` already has — so `startNDISender` builds it with `newStageSource(a, w, h, s.stop)` once the stream exists, and the source is never waiting on the main thread when `Stop` closes that channel.

```go
func newStageSource(a *app, w, h int, stop chan struct{}) *stageSource
```

Review Focus 3 — stopping while a frame is in flight — is pinned in Task 1 with a source that blocks:

```go
// A frame waiting on the main thread must not outlive Stop: the loop
// selects on the stream's stop channel, so a source that never answers
// still lets the stream end.
type stuck struct{ stop chan struct{} }

func (s stuck) frame(w, h int) (ndiFrame, bool) {
	<-s.stop
	return ndiFrame{}, false
}
```

`TestStreamStopsWithAStuckSource` creates a stream whose `stop` is closed after the loop starts and expects `run` to return (checked with a two-second timeout). It is a unit test over the loop, not the runtime: it builds an `ndiStream` directly with `send: 0` and a source whose first call blocks, then closes `stop` and waits on `done`.

`frameSource(w, h int)` is the test seam that builds the same source with a fresh `stop` channel, so tests never need a stream. `ndi_render_stub.go` defines the same `frameSource` returning a source whose `frame` reports `false` and `ndiRenderSize` as identity, so the app compiles where NDI is not built.

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test -run 'TestRenderStageFrameFollowsTheStage|TestSetNDIWithoutAStageRenders' ./...`
Expected: PASS; the second test starts NDI when a runtime is installed and skips when it is not.

- [ ] **Step 5: Commit**

```bash
git add ndi_render.go ndi_render_stub.go ndi_source_test.go output_test.go
git commit -m "feat: render the stage slide into the NDI frame"
```

---

### Task 3: Start and stop NDI without the stage window

**Files:**
- Modify: `main.go:111-123` (`signOut`, which stops NDI on the way out), `main.go:178-212` (`startNDI`), `output.go:55-131` (`openOutput`, `shutStage`, `closeOutput`)
- Modify: `settings.go:269-284` (`setNDI`), `settings.go:286-300` (`ndiStatus`)
- Test: `output_test.go`, `settings_test.go`

**Interfaces:**
- Consumes: `a.frameSource(w, h)` and `startNDISender(src, w, h)`.
- Produces: `func (a *app) ndiFrameSize() (int, int)` — the requested render size from the preference; `func (a *app) restartNDI()` — stop and start when the preference is on.

- [ ] **Step 1: Write the failing test**

Append to `output_test.go`:

```go
// The stage window closing must not stop NDI: the feed renders on its own.
func TestShuttingTheStageKeepsNDI(t *testing.T) {
	if !ndiSupported() {
		t.Skip("this build has no NDI")
	}
	a := newApp()
	a.ndiOn = true
	a.setNDI(true)
	if a.ndiErr != "" {
		t.Skipf("no NDI runtime here: %s", a.ndiErr)
	}
	defer a.stopNDI()
	a.shutStage(nil)
	if a.ndi == nil || a.ndiErr != "" {
		t.Fatalf("NDI stopped with the stage: %v", a.ndiErr)
	}
}
```

Append to `settings_test.go`:

```go
func TestNDIStatusReportsAFailure(t *testing.T) {
	a := newApp()
	a.ndiOn = true
	a.ndiErr = "the NDI runtime failed to initialize"
	tm := ui.LightTheme()
	s, c := a.ndiStatus(tm)
	if s != "NDI: the NDI runtime failed to initialize" || c != tm.Danger {
		t.Fatalf("status = %q, colour = %v", s, c)
	}
	a.setNDI(false)
	if a.ndiErr != "" {
		t.Fatalf("error survived the switch going off: %q", a.ndiErr)
	}
}

func TestNDIStatusWithoutStage(t *testing.T) {
	a := newApp()
	a.ndiOn = true
	tt := ui.NewTester(a.settingsBody, 900, 800)
	if !tt.HasText("Starts as soon") {
		t.Fatalf("status should say NDI starts on its own, got %q", tt.Texts())
	}
}

func TestNDIResolutionSetting(t *testing.T) {
	a := newApp()
	if w, h := a.ndiFrameSize(); w != 1280 || h != 720 {
		t.Fatalf("default size = %d×%d, want 720p", w, h)
	}
	a.ndiResolution = ndiResolution1080
	if w, h := a.ndiFrameSize(); w != 1920 || h != 1080 {
		t.Fatalf("1080p size = %d×%d", w, h)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test -run 'TestShuttingTheStageKeepsNDI|TestNDIStatusWithoutStage|TestNDIStatusReportsAFailure|TestNDIResolutionSetting|TestSignOutStopsNDI' ./...`
Expected: FAIL — `a.ndiFrameSize undefined`, and the status text does not exist yet.

- [ ] **Step 3: Change the start, stop and status paths**

In `main.go`:

```go
// startNDI publishes the stage. It needs no window: the feed renders the
// slide itself, so a single monitor can publish with the console in front.
func (a *app) startNDI() error
```

drops the `a.live == nil` branch and calls `startNDISender(a.frameSource(w, h), w, h)` with `w, h := a.ndiFrameSize()`. `shutStage` and `closeOutput` no longer call `stopNDI`; only sign-out, quit and the switch do. In `output.go`, `openOutput` keeps starting NDI through `placeStage` only when the feed is not already running (`if a.ndiOn && a.ndi == nil`).

In `settings.go`, `setNDI(true)` calls `a.startNDI()` unconditionally (no `a.live != nil` guard), and `ndiStatus` replaces `"On. Starts when the stage opens."` with `"On. Starts as soon as publishing begins."` for the case where a start is pending; the case `a.ndi != nil` keeps saying `Publishing as “mygo-demo (Live)”.`

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test -run 'TestShuttingTheStageKeepsNDI|TestNDIStatusWithoutStage|TestNDIResolutionSetting|TestSetNDI' ./...`
Expected: PASS (tests that need a runtime skip without one).

- [ ] **Step 5: Commit**

```bash
git add main.go output.go settings.go output_test.go settings_test.go
git commit -m "feat: publish NDI without the stage window"
```

---

### Task 4: The NDI size preference

**Files:**
- Modify: `settings.go` (the `settings` struct, `readSettings`, page radio row, `stageDisplayChanged`-style save path)
- Modify: `main.go` (the `ndiResolution` field and its load)
- Test: `settings_test.go`

**Interfaces:**
- Consumes: `settings`, `a.ndiFrameSize()` from Task 3.
- Produces: `const (ndiResolution720 = 0; ndiResolution1080 = 1)`; `settings.NDIResolution int \`json:"ndiResolution"\``; `a.ndiResolution int`; `func (a *app) ndiResolutionChanged()`.

- [ ] **Step 1: Write the failing test**

```go
func TestNDIResolutionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := newApp()
	a.settingsDir = dir
	a.ndiResolution = ndiResolution1080
	a.ndiResolutionChanged()
	b := newApp()
	b.settingsDir = dir
	b.loadSettings()
	if b.ndiResolution != ndiResolution1080 {
		t.Fatalf("saved resolution = %d, want 1080p", b.ndiResolution)
	}
}

func TestNDIResolutionPage(t *testing.T) {
	a := newApp()
	a.ndiOn = true
	tt := ui.NewTester(a.settingsBody, 900, 800)
	if tt.HasText("Automatic") {
		t.Fatal("there is no automatic size")
	}
	if err := tt.Click("1080p"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ndiResolution != ndiResolution1080 {
		t.Fatalf("resolution = %d", a.ndiResolution)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test -run 'TestNDIResolution' ./...`
Expected: FAIL — `a.ndiResolutionChanged undefined`.

- [ ] **Step 3: Add the preference**

Add the field to `settings` and to `readSettings`'s zero-value default (0 = 720p, so an old file needs no migration). Add `a.ndiResolution` to `app`, set it in `loadSettings`, and include it in `saveSettings`:

```go
func (a *app) ndiResolutionChanged() {
	a.saveSettings()
	// A size change needs a new sender: the frame header carries the size.
	if a.ndi != nil {
		a.restartNDI()
	} else {
		a.changed()
	}
}
```

`restartNDI` stops and starts the sender when `a.ndiOn`. On the Settings page, under the NDI switch and before the stats, add a row labelled `Stage output size` with `ui.Radio(c, &a.ndiResolution, ndiResolution720, "720p")` and `ui.Radio(c, &a.ndiResolution, ndiResolution1080, "1080p")`, each `.OnChange(a.ndiResolutionChanged)`, plus one muted line: `"The feed draws the slide itself, so it does not depend on the stage window or on a second monitor."`

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test -run 'TestNDIResolution' ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add settings.go main.go settings_test.go
git commit -m "feat: choose the NDI output size in Settings"
```

---

### Task 5: The gate, the docs and the release

**Files:**
- Modify: `AGENTS.md`, `docs/architecture.md`, `README.md`, `docs/superpowers/plans/2026-11-22-render-to-ndi-spec.md` (mark it implemented)

**Interfaces:**
- Consumes: everything above.
- Produces: nothing other code depends on.

- [ ] **Step 1: Run the whole gate**

```sh
export GOCACHE="$TMPDIR/gocache-mygo-demo"
test -z "$(gofmt -l *.go)" && echo gofmt-ok
CGO_ENABLED=0 go build ./... && GOOS=darwin CGO_ENABLED=0 go build ./... && GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=linux CGO_ENABLED=0 go build ./... && echo builds-ok
go vet ./... && GOOS=windows CGO_ENABLED=0 go vet ./... && GOOS=linux CGO_ENABLED=0 go vet ./... && echo vet-ok
CGO_ENABLED=0 go test -count=1 ./...
```
Expected: `gofmt-ok`, `builds-ok`, `vet-ok`, and `ok mygo-demo`.

- [ ] **Step 2: Check the feed for real**

Build with `CGO_ENABLED=0 go build -o /tmp/mygo-demo-ndi .`, run it with `MYGO_NDI=1`, sign in, and turn the NDI switch on **with the stage window closed**. In the NDI Studio Monitor (or OBS with the DistroAV plugin), confirm the source `mygo-demo (Live)` lists at 1280×720, shows the slide, follows Next/Previous, and goes black on B. Then click **Open output** with a second display attached and confirm the feed is unchanged. Record the result in the commit message.

- [ ] **Step 3: Update the docs**

- `AGENTS.md`: the NDI bullets say the feed renders `liveView` with `ui.Render` on the main thread through `mygo.RunOnMain`, never captures a window; no file calls `CGWindowListCreateImage`, `PrintWindow` or `gdk_pixbuf_get_from_window` any more; `startNDI` no longer needs `a.live`; the size preference and the `ndiFrameSource` seam are described. The `ndi_darwin.go` row of the file table loses the capture sentence; `ndi_render.go` is added.
- `docs/architecture.md`: the NDI section describes the render path, the main-thread handoff and the size setting; the stage window is a viewer.
- `README.md`: the NDI sections say no second monitor is needed, that the stage window may stay closed, and that the size is chosen in Settings.

- [ ] **Step 4: Commit, merge and release**

```bash
git add -A
git commit -m "docs: describe the rendered NDI feed"
git checkout main && git merge --ff-only feat/render-ndi
git push origin main
git tag -a v0.2.0-beta.5 -m "mygo-demo 0.2.0-beta.5

Publishes the stage over NDI from a rendered frame instead of a window
capture, so the feed works with one monitor and the stage window closed,
and adds an NDI output size setting."
git push origin v0.2.0-beta.5
```
Expected: CI green on both `main` and the tag, and a prerelease with 8 assets.
