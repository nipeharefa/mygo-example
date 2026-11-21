# Render-to-NDI: spec

Status: approved direction (user, 2026-11-22), implemented by
[2026-11-22-render-to-ndi.md](2026-11-22-render-to-ndi.md).

## The report

After `v0.2.0-beta.4` the operator reported (verbatim): "jadi agak ribet jika
di test di satu monitor ya, karna waktu open output dan window tidak aktif
maka ndi juga tidak aktif". Testing on a single monitor is awkward: the stage
window covers the console, and NDI needs that window.

The underlying limits are the capture, not the windows:

- the sender captures the app's own window, so a frame needs a window that
  exists and is on screen;
- macOS captures with `CGWindowListCreateImage`, which returns nothing for a
  window the window server does not composite (minimized, hidden, all but
  the last frame after a screen lock). `AGENTS.md` already records that
  limit as the window server's;
- on Windows the capture is `PrintWindow` into a DIB, which a compositor
  may answer with a blank or stale image for an occluded window;
- on Linux the capture reads the window's GdkWindow, which needs X11 and a
  mapped window.

So a single-monitor operator can only test by covering their console with
the output, and a non-active or occluded window can stop the feed.

## The decision

**The feed is rendered, not captured.** The stage slide is drawn from the
app's state into an in-memory image, and those pixels go to NDI. The stage
window becomes a viewer of the same state; it is not needed for publishing.

### Why this works on macOS and Windows alike

`ui.Render(view func(c *ui.Context), width, height int, scale float32) *image.RGBA`
(MyGo v0.10.0, `ui/api_gen.go:2152`) draws a frame of any view into an
`*image.RGBA` with no window. It is the renderer the headless tester uses
(CPU rasterizer, `internal/raster`), so the pixels are the same on every
platform, independent of window server, compositor or display server.

MyGo's own example does exactly this at runtime: `examples/wind/texture.go:266`
renders an atlas offscreen with `ui.Render` and runs the work **on the main
thread** through `tex.later`, which the app sets to `win.Update`
(`examples/wind/main.go:42`). The comment in `examples/wind/glyphs.go:46-47`
states the rule: "It draws a frame of its own (ui.Render), which must not
happen within a window's."

### What it costs

Measured on the development machine with the real `liveView`, headless, at
1920×1080 (960×540 DIP at scale 2):

- about 6.3–7.8 ms a frame warm, 6.0 ms for the first frame after a cold start;
- with a changed slide (`a.stage` flipped each frame) 8.6 ms a frame;
- at the sender's 25–30 fps that is under a third of the main thread.

The conversion to NDI's BGRA is a byte shuffle, `ui.Render` returns RGBA.

### What it does not change

- The stage theme, the layout, the text size and the frame format (BGRA,
  top row first, `lineStride = 4*W`).
- The source name, `mygo-demo (Live)`, and the stats on the Settings page.
- The rule that the monitor never reaches the room by itself: the feed
  renders `a.stage`.

## What has to change

1. **Threading.** `ui.Render` must run on the main thread. The sender's frame
   loop runs on its own goroutine; it hops with `mygo.RunOnMain` (`loop.go:88`),
   which waits for the function. After the event loop shuts down `onMain`
   drops the function, so the wait must also watch the stream's stop channel.
2. **Sender shape.** `ndi_sender.go` stops building a per-platform capturer.
   It takes an `ndiFrameSource` and the frame size:
   `startNDISender(src ndiFrameSource, w, h int)`. Platform files keep only
   the library lookup (`ndiLibraryCandidates`, `openNDILibrary`,
   `ndiSymbol`, `registerFunc`), the install hint, the failure copy and the
   HD cap (`ndiFrameSize`, `ndiMaxFrameWidth/Height`).
3. **App.** `startNDI` no longer needs `a.live`, and closing the stage
   window does not stop NDI. Settings gains an NDI size: 720p (default) or
   1080p; a size change restarts the sender, because the frame header
   carries the size.
4. **Renderer.** `ndi_render.go` (all NDI platforms) renders `liveView`,
   converts to BGRA and adapts between the sender's goroutine and the main
   thread. `ndi_render_stub.go` compiles the app where NDI is not built.

## Platform constraints kept

- **No cgo**, builds and tests with `CGO_ENABLED=0`.
- **No AppKit pixel readback and no new platform capture code**: the plan
  deletes capture code rather than adding any.
- **Tests never open a window and never touch the screen or the runtime.**
  `app.frameSource(w, h)` is the seam.
- **The stage stays the only thing published**, and the window stays
  frameless when it is open.

## Open questions

- **One window or none on one monitor.** Rendering means the feed no longer
  needs the stage, but the operator may still want a preview. The console's
  MONITOR card already shows the slide, so Settings could hide the **Open
  output** button when only one display is attached; not in this plan.
- **N frames of latency.** The handoff renders synchronously, so the frame
  sent is the state at the moment of the hop. If a future change moves the
  render off the sender goroutine, the feed must not fall behind the stage;
  the stats' measured frame rate is the test.
