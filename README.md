# mygo-demo

A simple worship presenter for a small church screen, built with
[MyGo](https://github.com/egoist/mygo). It keeps a run of show of public
domain hymns and King James verses, previews the next slide on a monitor
while one slide goes to a separate stage window, and can publish that stage
as an NDI video source for OBS or a projector feed — with no window on
screen, so one monitor is enough. A separate **Song editor** window edits
the song library itself: parts, slides, text and text size. Everything stays on the machine: no
account, no network, no database.

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
   every part's slides in the list, in order; the search boxes and the pack
   picker narrow the library, not the list.
3. Click a row, use **←** / **→**, or press **Next** / **Prev** to move the
   monitor. Press **Go Live**, **Enter** on the list, or **Cmd/Ctrl+Enter**
   to put that row on stage. The row on air carries a `LIVE` badge.
4. Drag a row to move it, or use **Move up** / **Move down**; the slide on
   air stays on air wherever its row lands. **Remove** drops the selected
   row, and **Black out** hides the text without touching the list.
5. **Open output** opens the stage window. It is a 1280×720 window unless
   **Settings** names a display, in which case it opens fullscreen there.
   **Present** toggles fullscreen, **Close output** closes it. On one
   monitor, skip the window altogether: turn on NDI instead, and OBS or
   NDI Video Monitor shows the stage while the console keeps the keyboard.
   The **Settings** page (Cmd/Ctrl+, or the button in the top bar) also
   holds the NDI switch, the output size and the stream's own stats; the
   **Help** menu lists the shortcuts and the NDI setup.

The console shortcuts are **←** / **→** for the monitor, **B** for blackout,
and **Cmd/Ctrl+Enter** to go live. In the stage window **← → ↑ ↓** and
**Space** move the stage (pulling the monitor along), **B** toggles blackout,
and **Esc** leaves fullscreen; when the stage is windowed, **Esc** closes
it. The stage window has no title bar, so drag the slide itself to move it
to a projector; the console can stay with the operator.

The stage opens once and stays open until you close it: changing anything
that does not touch the display the stage is on leaves it alone. Closing it
does not stop NDI, so a feed can keep running with no stage window at all.

The display and NDI preferences and the song library are saved between
runs; the run of show is not. A saved display that is no longer attached
falls back to a window.

**Check for Updates** (a button on the console, and on the Help page) asks
GitHub for the newest release and offers
to install it; the app then restarts on the new version. Updates work only
for the installers from a release — a development build, or an install a
package manager owns (such as the `.deb`), is updated another way. Every
release on GitHub carries per-platform update archives that are signed with
the project's key, so an app accepts only updates it can verify.

## Editing songs

**Song editor** in the console header opens a separate window for the song
library. Pick a song in the left list, a slide in the middle list, and edit it
in the inspector on the right; **Part** and **Song details** fold away with
their **Show** / **Hide** control. A song has an author, a key, a text size, and
one or more **parts** (a verse, a chorus, a bridge); each part holds one or
more **slides**, and a slide is the text that fills one stage frame. **Add
song** puts every part in the run of show in order; a part's **Add part to
run of show** adds it again where the arrangement needs it, so a chorus can
follow every verse without another copy of the song.

Every edit saves to `library.json` in the app data directory, so a song
written once survives a restart. A first launch seeds that file from the
five built-in hymns. **Restore samples** brings back any built-in song whose
id the library lost and never overwrites a song you edited. **New song**
starts an empty draft, and **Remove song** drops the selected one.

**Text size** is one number per song, in stage units at the 1280×720 design:
40 is the built-in size and the slider spans 24 to 72. The editor's 16:9
preview, the stage window and the NDI feed all scale from that number, so a
smaller output shrinks the text with it instead of clipping it. A slide
copies its size when it is added, so changing a song later does not rewrite
slides already in the run of show.

## The live output as an NDI source

The stage can publish itself as an NDI source on macOS, Windows and Linux.
The feed is **drawn, not captured**: the app renders the slide and hands the
picture to the NDI sender, so it needs no window on screen, no second
monitor and nothing in the foreground. Start NDI, keep the console where it
is, and the source carries the slide — and only the slide, with no window
chrome.

Each platform needs the NDI Runtime installed (version 5 or 6). A missing
or unusable runtime is reported in a strip under the console header, and
the same goes for a sender that cannot start, instead of failing silently.
The switch on the **Settings** page sets the same preference as **Start
NDI** in the console header, and `MYGO_NDI=1` starts it at launch. Closing
the stage window does **not** stop the stream; sign out, or turn the switch
off, to stop it.

**Stage output size** in Settings picks 720p (the default) or 1080p. The
feed is the slide at 1280×720 laid out and drawn at the size requested, so
text stays sharp at 1080p.

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
2. Turn on **Start NDI** in the console header (or **Publish the stage over
   NDI** in **Settings**, or start the app with `MYGO_NDI=1`). The stage
   window does not have to be open.
3. In OBS add an **NDI Source** (with
   [DistroAV](https://github.com/DistroAV/DistroAV/releases)), or open NDI
   Video Monitor, and pick `mygo-demo (Live)`.

### Windows

1. Install the **NDI Runtime** from
   [NDI Tools](https://ndi.video/tools/). It has to be the x64 runtime;
   the installer puts `Processing.NDI.Lib.x64.dll` under
   `C:\Program Files\NDI\NDI 6 Tools\Runtime` (or
   `C:\Program Files\NDI\NDI 6 Runtime`) and sets `NDI_RUNTIME_DIR_V6`.
   The app searches both folders.
2. Press **Start NDI** in the console header or turn on the switch in
   **Settings**. The stage window does not have to be open.

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
2. Press **Start NDI** in the console header or turn on the switch in
   **Settings**. The stage window does not have to be open.
3. To receive it in OBS, install DistroAV from Flatpak or its `.deb`
   release asset; NDI Video Monitor from NDI Tools also works.

Window Capture in OBS remains a simpler fallback when the output and OBS
are on the same machine.

## Build from source

```sh
CGO_ENABLED=0 go build ./...   # the app, no cgo anywhere
CGO_ENABLED=0 go test ./...    # headless view tests
go run .                       # opens the console
MYGO_NDI=1 go run .            # ...and publishes the stage over NDI
go build -tags e2e -o demo .   # adds the automation socket for driving it (below)
bun run build                  # packaged installers in build/
```

The `e2e` tag adds the MyGo automation plugin, so a script can drive the
built app from outside:

```sh
go build -tags e2e -o demo . && MYGO_AUTOMATION=/tmp/demo.sock ./demo
# another shell: send newline-delimited JSON to the socket
# {"id":1,"method":"windows"}   then   {"id":2,"method":"capture","params":{"window":1}}
```

It listens only when `MYGO_AUTOMATION` names a socket, and no release build
includes it.

A `v*` tag builds every platform, signs the update archives against
`updates.publicKey` and publishes the release; the tag's base version must
match `version` in `mygo.config.ts`, and the repository needs the
`MYGO_UPDATER_PRIVATE_KEY` secret (`gh secret set MYGO_UPDATER_PRIVATE_KEY <
mygo-update.key`) or the release job refuses to publish. See
[AGENTS.md](AGENTS.md) for the conventions and
[docs/architecture.md](docs/architecture.md) for how the UI works.
