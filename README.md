# mygo-demo

A native desktop mock of a Kubernetes resource explorer, built with
[MyGo](https://github.com/egoist/mygo). One window, dummy data, no
cluster and no network access. On macOS, Windows and Linux the window can
also publish itself as an NDI video source for OBS.

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

The app publishes itself as an NDI source on macOS, Windows and Linux.
Each platform needs the NDI Runtime installed, and the app reports the
missing runtime in a strip under the header instead of failing silently.

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

### Windows: the app is the NDI source

1. Install the **NDI Runtime** from
   [NDI Tools](https://ndi.video/tools/). It has to be the x64 runtime;
   the installer puts `Processing.NDI.Lib.x64.dll` under
   `C:\Program Files\NDI\NDI <version> Tools\Runtime` and sets
   `NDI_RUNTIME_DIR_V6`. Without it the header reports that the runtime is
   missing.
2. Launch the app, sign in, then press **Start NDI** in the header. (Or
   start it with `MYGO_NDI=1` in the environment.)
3. In OBS add an **NDI Source**, or open NDI Video Monitor, and pick
   `mygo-demo (Kubernetes)`. Other machines see it as
   `<machine name> (mygo-demo (Kubernetes))`.
4. The feed is the window's client area at one pixel per point, no title
   bar: crop or scale it with an OBS transform.

To *receive* NDI in OBS you still need the plugin:
[DistroAV](https://github.com/DistroAV/DistroAV/releases) (formerly
obs-ndi), via `winget install DistroAV.DistroAV` or the
`distroav-<version>-windows-x64-Installer.exe` asset, with OBS Studio
**31.1.1 or newer**. That is for OBS only — the app needs nothing but the
runtime.

If you only need the window inside OBS on the same machine, OBS **Window
Capture** (window `Kubernetes`) is still simpler than the whole NDI
route; NDI is worth it when the picture has to leave the machine or reach
another app.

### Linux: the app is the NDI source

1. Install the NDI Runtime, which ships with
   [NDI Tools](https://ndi.video/tools/) and lands in `/usr/local/lib/libndi.so`.
2. Launch the app, sign in, then press **Start NDI** in the header.
3. The feed is the window's client area, captured through GDK, so it
   works on **X11**. On Wayland the app cannot read its own pixels and
   the header reports that instead.
4. To receive it in OBS, install DistroAV from Flatpak or its `.deb`
   release asset; NDI Video Monitor from NDI Tools also works.

Plain OBS **Window Capture** remains the fallback on either session type.

## Build from source

```sh
CGO_ENABLED=0 go build ./...   # the app, no cgo anywhere
CGO_ENABLED=0 go test ./...    # headless view tests
go run .                       # opens the window
MYGO_NDI=1 go run .            # ...and publishes it as NDI
bun run build                  # packaged installers in build/
```

A `v*` tag builds every platform and publishes a release; the tag's base
version must match `version` in `mygo.config.ts`. See
[AGENTS.md](AGENTS.md) for the conventions and
[docs/architecture.md](docs/architecture.md) for how the UI works.
