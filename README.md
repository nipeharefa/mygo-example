# mygo-demo

A native desktop mock of a Kubernetes resource explorer, built with
[MyGo](https://github.com/egoist/mygo). One window, dummy data, no
cluster and no network access. On macOS the window can also publish
itself as an NDI video source for OBS.

Sign in with **admin** / **admin**.

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

## Getting the window into OBS

The app publishes NDI on macOS only. On Windows and Linux, OBS captures
the window instead — same result on screen, one more step to set up.

### macOS: the app is the NDI source

1. Install the NDI Runtime, which ships with [NDI Tools](https://ndi.video/tools/).
   Without it the app has nothing to load and the button reports an
   error. The app looks for `/usr/local/lib/libndi.dylib`, where the
   installer puts it.
2. Launch the app, sign in, then press **Start NDI** in the header. (Or
   start it with `MYGO_NDI=1` in the environment.)
3. In OBS add an **NDI Source**, or open NDI Video Monitor, and pick
   `mygo-demo (Kubernetes)`. Other machines see it as
   `<machine name> (mygo-demo (Kubernetes))`.
4. The feed is the whole window at one pixel per point, title bar
   included: crop or scale it with an OBS transform.

**Stop NDI** in the header unregisters the source, and quitting the app
does the same.

### Windows: OBS captures the window

The Windows build has no **Start NDI** button — it cannot publish NDI
itself. To get the picture into OBS, and from there onto the network:

1. Install [OBS Studio](https://obsproject.com) **31.1.1 or newer**: the
   current NDI plugin refuses older releases.
2. Install the **NDI Runtime 6.3 or newer** from
   [NDI Tools](https://ndi.video/tools/). It is a separate download, and
   the plugin cannot load without it.
3. Install the NDI plugin for OBS,
   [DistroAV](https://github.com/DistroAV/DistroAV/releases) (formerly
   obs-ndi), either with `winget install DistroAV.DistroAV` or by running
   the `distroav-<version>-windows-x64-Installer.exe` release asset. It
   publishes Windows x64 only, and needs a restart of OBS.
4. In OBS, under **Sources**, press **+** → **Window Capture** → window
   `Kubernetes`. Leave the capture method on **Windows 10 (1903 and up)**.
5. To publish that source to other machines, right-click it → **Filters**
   → **+** → **NDI Output** (DistroAV also calls it *NDI Dedicated
   Output*) and give it a name. For the whole program feed instead, use
   **Tools** → **NDI Output Settings**.

If you only need the window inside OBS on the same machine, steps 2 to 5
are unnecessary — plain **Window Capture** is enough, and NDI is only
worth the install when the picture has to leave the machine or reach
another app.

NDI Tools also installs NDI Scan Converter, which publishes a screen or a
region as an NDI source by itself. Keep the app window on that screen and
open Scan Converter — that gets the picture onto the network without the
OBS plugin. The catch is that OBS still cannot *receive* NDI without
DistroAV, so this only helps when the consumer is another machine or an
NDI app such as NDI Video Monitor.

### Linux

Neither the app nor OBS publishes NDI out of the box here, and the app
has no NDI support on Linux at all. Use OBS **Window Capture** (X11; on
Wayland, capture through the desktop portal), and install DistroAV from
Flatpak or its `.deb` release asset if the feed has to leave the machine.

## Build from source

```sh
CGO_ENABLED=0 go build ./...   # the app, no cgo anywhere
CGO_ENABLED=0 go test ./...    # headless view tests
go run .                       # opens the window
MYGO_NDI=1 go run .            # ...and publishes it as NDI (macOS)
bun run build                  # packaged installers in build/
```

A `v*` tag builds every platform and publishes a release; the tag's base
version must match `version` in `mygo.config.ts`. See
[AGENTS.md](AGENTS.md) for the conventions and
[docs/architecture.md](docs/architecture.md) for how the UI works.
