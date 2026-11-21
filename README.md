# mygo-demo

A simple worship presenter for a small church screen, built with
[MyGo](https://github.com/egoist/mygo). It keeps a run of show of public
domain hymns and King James verses, previews the next slide on a monitor
while one slide goes to a separate stage window, and can publish that stage
as an NDI video source for OBS or a projector feed. Everything stays on the
machine: no account, no network, no database.

Sign in with **admin** / **admin**. The sign-in is a demo gate, not
security.

## Download

Grab the installers from the [latest release](https://github.com/nipeharefa/mygo-example/releases).

| Platform | File |
| --- | --- |
| macOS (Apple silicon) | `mygo-demo-<version>-arm64.dmg` |
| macOS (Intel) | `mygo-demo-<version>-amd64.dmg` |
| Windows | `mygo-demo-setup-<version>-amd64.exe`, `...-arm64.exe` |
| Linux | `mygo-demo-<version>-linux-<arch>.tar.gz`, `mygo-demo_<version>_<arch>.deb` |

The builds are not signed. On macOS, right-click the app and choose
**Open** the first time. On Windows, SmartScreen shows **More info** →
**Run anyway**.

## Running a service

The window is the **console**: the song and Bible library on the left,
the run of show in the middle, and the stage panel on the right. The panel
shows the **stage** (what the room sees) above the **monitor** (the row you
are looking at). Moving the monitor never changes the room until you press
**Go Live**.

1. Sign in. A fresh session starts with Amazing Grace and Psalm 23:1 on
   the list.
2. Add songs from **Songs** and verses from **Bible**. Adding a song puts
   every section in the list; the search boxes and the pack picker narrow
   the library, not the list.
3. Click a row, use **←** / **→**, or press **Next** / **Prev** to move the
   monitor. Press **Go Live**, **Enter** on the list, or **Cmd/Ctrl+Enter**
   to put that row on stage. The row on air carries a `LIVE` badge.
4. Drag a row to move it, or use **Move up** / **Move down**; the slide on
   air stays on air wherever its row lands. **Remove** drops the selected
   row, and **Black out** hides the text without touching the list.
5. **Open output** opens the stage window. It is a 1280×720 window unless
   **Settings** names a display, in which case it opens fullscreen there.
   **Present** toggles fullscreen, **Close output** closes it. The
   **Settings** page (Cmd/Ctrl+, or the button in the top bar) also holds
   the NDI switch and the stream's own stats; the **Help** menu lists the
   shortcuts and the NDI setup.

The console shortcuts are **←** / **→** for the monitor, **B** for blackout,
and **Cmd/Ctrl+Enter** to go live. In the stage window **← → ↑ ↓** and
**Space** move the stage (pulling the monitor along), **B** toggles blackout,
and **Esc** leaves fullscreen; when the stage is windowed, **Esc** closes
it. The stage window has no title bar, so drag the slide itself to move it
to a projector; the console can stay with the operator.

The stage opens once and stays open until you close it: changing anything
that does not touch the display the stage is on leaves it alone.

The display and NDI preferences are saved between runs; the run of show is
not. A saved display that is no longer attached falls back to a window.

## The live output as an NDI source

The stage window can publish itself as an NDI source on macOS, Windows
and Linux. Each platform needs the NDI Runtime installed (version 5 or 6),
and the console reports a missing or unusable runtime in a strip under the
header instead of failing silently. NDI publishes only while the stage
window is open; closing it stops the stream. The switch on the
**Settings** page sets the same preference as **Start NDI** in the header.

The **NDI stream** section of Settings reports the sender's own numbers —
resolution, measured frame rate, frames sent, dropped frames, up time and
the last error — so "is it actually publishing, and in HD?" has an answer
without opening OBS. NDI 5 and 6 give a sender no receiver tally, so the
numbers describe the stream, not who is watching it.

The source is called `mygo-demo (Live)`. Other machines see it as
`<machine name> (mygo-demo (Live))`.

### macOS

1. Install the NDI Runtime, which ships with [NDI Tools](https://ndi.video/tools/).
   The app looks for `/usr/local/lib/libndi.dylib`, where the installer
   puts it.
2. Open the stage window, then turn on **Publish the stage over NDI** in
   **Settings** (or press **Start NDI** in the console header, or start the
   app with `MYGO_NDI=1` in the environment).
3. In OBS add an **NDI Source**, or open NDI Video Monitor, and pick
   `mygo-demo (Live)`.
4. The feed is the slide alone: the output window has no title bar or
   window chrome, so no desktop edge frames it. On a Retina display the
   capture runs at the display's pixel density and is scaled down to at
   most 1920×1080, so text stays sharp; a fullscreen output on a 1080p
   projector is native 1080p.

The output window has to be **on screen** while it streams: macOS gives
an app the image of its own window only while the window server shows it,
so a minimized window, one on another Space, or one behind a full-screen
app yields no frames and **Start NDI** says so. A fullscreen output on a
projector is on screen, which is the normal case.

### Windows

1. Install the **NDI Runtime** from
   [NDI Tools](https://ndi.video/tools/). It has to be the x64 runtime;
   the installer puts `Processing.NDI.Lib.x64.dll` under
   `C:\Program Files\NDI\NDI 6 Tools\Runtime` (or
   `C:\Program Files\NDI\NDI 6 Runtime`) and sets `NDI_RUNTIME_DIR_V6`.
   The app searches both folders.
2. Open the stage window, then press **Start NDI** in the console header
   or turn on the switch in **Settings**.
3. The feed is the slide alone, captured from the window's client area at
   the display's pixel density; a frameless output sends no window chrome.

To *receive* NDI in OBS you still need the plugin:
[DistroAV](https://github.com/DistroAV/DistroAV/releases) (formerly
obs-ndi), via `winget install DistroAV.DistroAV` or the
`distroav-<version>-windows-x64-Installer.exe` asset, with OBS Studio
**31.1.1 or newer**. That is for OBS only — the app needs nothing but the
runtime.

### Linux

1. Install the NDI Runtime, which ships with
   [NDI Tools](https://ndi.video/tools/) and lands in
   `/usr/local/lib/libndi.so`.
2. Open the stage window, then press **Start NDI** in the console header
   or turn on the switch in **Settings**.
3. The feed is the slide alone, captured through GDK at the window's pixel
   size, so it works on **X11**. On Wayland the app cannot read its own
   pixels and the header reports that instead.
4. To receive it in OBS, install DistroAV from Flatpak or its `.deb`
   release asset; NDI Video Monitor from NDI Tools also works.

Window Capture in OBS remains a simpler fallback when the output and OBS
are on the same machine.

## Build from source

```sh
CGO_ENABLED=0 go build ./...   # the app, no cgo anywhere
CGO_ENABLED=0 go test ./...    # headless view tests
go run .                       # opens the console
MYGO_NDI=1 go run .            # ...and starts NDI once the output opens
bun run build                  # packaged installers in build/
```

A `v*` tag builds every platform and publishes a release; the tag's base
version must match `version` in `mygo.config.ts`. See
[AGENTS.md](AGENTS.md) for the conventions and
[docs/architecture.md](docs/architecture.md) for how the UI works.
