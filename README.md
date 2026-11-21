# mygo-demo

A simple worship presenter for a small church screen, built with
[MyGo](https://github.com/egoist/mygo). It keeps a run of show of public
domain hymns and King James verses, puts one slide on a separate output
window, and can publish that window as an NDI video source for OBS or a
projector feed. Everything stays on the machine: no account, no network,
no database.

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
the run of show in the middle, and the stage panel on the right.

1. Sign in. A fresh session starts with Amazing Grace and Psalm 23:1 on
   the list.
2. Add songs from **Songs** and verses from **Bible**. Adding a song puts
   every section in the list; the search boxes and the pack picker narrow
   the library, not the list.
3. Select a row to put that slide on stage. **Next**, **Prev**, the arrow
   keys and clicking a row all move the live slide.
4. **Move up**, **Move down** and **Remove** edit the list; **Black out**
   hides the text without touching the list.
5. **Open output** opens the second window with the current slide. If
   there is a second display, the output opens there fullscreen; otherwise
   it is a normal 1280×720 window. **Present** toggles fullscreen,
   **Close output** closes it.

The console shortcuts are **←** / **→**. In the output window **← → ↑ ↓**
and **Space** move the slide, **B** toggles blackout, and **Esc** leaves
fullscreen; when the output is windowed, **Esc** closes it. The output
window has no title bar, so drag the slide itself to move it to a
projector; the console can stay with the operator.

## The live output as an NDI source

The output window can publish itself as an NDI source on macOS, Windows
and Linux. Each platform needs the NDI Runtime installed (version 5 or 6),
and the console reports a missing or unusable runtime in a strip under the
header instead of failing silently. NDI starts only while the output
window is open; closing it stops the stream.

The source is called `mygo-demo (Live)`. Other machines see it as
`<machine name> (mygo-demo (Live))`.

### macOS

1. Install the NDI Runtime, which ships with [NDI Tools](https://ndi.video/tools/).
   The app looks for `/usr/local/lib/libndi.dylib`, where the installer
   puts it.
2. Open the output window, then press **Start NDI** in the console header
   (or start the app with `MYGO_NDI=1` in the environment).
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
2. Open the output window, then press **Start NDI** in the console header.
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
2. Open the output window, then press **Start NDI** in the console header.
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
