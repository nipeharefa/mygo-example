# Architecture

This is the guide to **this app**, not to MyGo. The framework's own
`docs/architecture.md` ships with the pinned module
(`go list -m -f '{{.Dir}}' github.com/egoist/mygo`); read it for the
engine. This file covers what `mygo-demo` is made of and why.

The app is a mock Kubernetes resource explorer drawn with package `ui`:
a sidebar of resource kinds, a table of dummy objects for the chosen
kind, and an inspector for the chosen row. There is no Kubernetes client,
no network access, and no web page. One `app` struct is the whole state.
On macOS it can also publish its own window as an NDI video source; that
is the only non-UI feature (see "NDI output").

## What runs

`main()` in `main.go` starts MyGo and opens one native window:

```go
mygo.App.WhenReady(func() {
    a.win = mygo.NewWindow(mygo.WindowOptions{
        Title:     "Kubernetes",
        Width:     1180,
        Height:    720,
        MinWidth:  960,
        MinHeight: 600,
        StateKey:  "main",
        Content:   ui.View(a.view),
    })
    if os.Getenv("MYGO_NDI") != "" {
        _ = a.startNDI()
    }
})
mygo.App.OnQuit(a.stopNDI)
if err := mygo.App.Run(); err != nil {
    log.Fatal(err)
}
```

The window is kept on `app` because the NDI sender captures it
(`a.win.NativeHandle()`); `OnQuit` stops the sender so the source leaves
the network when the window closes. See "NDI output".

`Content: ui.View(a.view)` makes the window a native UI window: MyGo
calls `a.view(c)` to build a frame and draws it itself, on the GPU where
it can. Nothing loads a URL, so no window requests the `mygo://` frontend
and the `devUrl`/`frontendDist` settings never apply; `mygo.config.ts`
still matters to `mygo build`, which reads it for the app name,
identifier, output directory and the frontend files it embeds.

`StateKey: "main"` makes MyGo remember the window's position, size and
maximized state across runs. `MinWidth`/`MinHeight` of 960×600 keep the
fixed sidebar (244 DIP) and inspector (320 DIP) from squeezing the table
out of existence.

The first frame is the sign-in card, not the dashboard: `view` returns
`a.login(c)` until `a.signedIn` is true (see "Signing in").

## State

All state lives on one `app` (`main.go`), constructed by `newApp()`:

| Field | Role |
| --- | --- |
| `kind string` | The chosen resource kind id. Bound directly to `ui.Sidebar`, so the framework writes it; it also selects the pane. |
| `lastKind string` | The kind the view was built for last pass. Detects a change so `view` can reset selection and scroll. |
| `filter string` | The sidebar filter input, bound to `ui.TextInput`. |
| `namespace string` | The header's namespace picker, bound to `ui.Select`. **Display only**: it changes the header subtitle and nothing else. |
| `row int` | The chosen table row; also what the inspector shows. |
| `table ui.ListState` | The table's persistent state (selection, scroll, keys), rebuilt each pass. |
| `workloadsOpen`, `configOpen`, `networkOpen`, `clusterOpen` | Whether each sidebar section is expanded. Each is a `*bool` handed to `ui.SidebarSection`. |
| `signedIn bool` | The sign-in gate. While false, `view` builds the login card and never the dashboard. |
| `username string` | The login card's username field, bound to `ui.TextInput`. Kept across sign-out. |
| `password string` | The login card's password field, bound to `ui.TextInput` and masked with `.Password()`. Cleared on sign-in and sign-out. |
| `authErr string` | The login card's inline error, shown only while non-empty and cleared when either field changes. |
| `win *mygo.Window` | The one window, kept so the NDI sender can read its native handle. |
| `ndi *ndiStream` | The running NDI sender, or nil. Non-nil drives the header's "Stop NDI" label. |
| `ndiErr string` | Why starting NDI failed, or empty. Rendered as an error strip under the header. |

There is no `ui.Element`, no widget parts and no `*ui.Context` on `app`:
those expire before the next build pass, so only this plain data may
persist. Colors are never stored either; they are read from `c.Theme()`
on each pass so a light/dark switch needs no app code.

`groupOpen` maps a section title to its bool. Its `switch` needs a case
per group title and **defaults to `&a.clusterOpen`**, so a fifth group
added to `groups` without a matching case would share Cluster's expand
state.

## The frame contract

`view` is called on every build pass, and one user event can cause
several passes. It must therefore be a pure function of `app`:

```go
func (a *app) view(c *ui.Context) {
    if !a.signedIn {
        a.login(c)
        return
    }
    if a.kind != a.lastKind {
        a.lastKind = a.kind
        a.table = ui.ListState{}
        a.row = 0
    }
    a.shell(c)
}
```

The reset at the top matters: a row index chosen in one kind is invalid
in the next, and a stale scroll offset could point past the new object
slice. Dropping the whole `ListState` also clears its keys, selection and
scroll in one move.

Two framework rules shape the rest of the code:

- `Children(func())` runs with the window's `*Context`, changing only its
  parent scope for the callback — so the same `c` is passed down the
  tree and layout parenting comes from the call nesting, not from stored
  state.
- Widgets take their input at construction. Here every control binds a
  pointer into `app` — `&a.kind`, `&a.filter`, `&a.namespace`, and
  `a.table.Selected = &a.row` — so no local copies or `Clicked()` checks
  are needed, and an edit is visible on the next pass.

## Signing in

The app opens on a centered sign-in card and shows the dashboard only
after the demo account (`demoUsername` / `demoPassword`, both `admin` in
`main.go`) is accepted. There is no backend, no password store and no
session: restarting the app returns to the card, and the whole gate lives
in four `app` fields, `login`, and three handlers (`signIn`, `signOut`,
`clearAuthError`).

`login` draws a 360 DIP card on `theme.Background`: a surface column with
a `theme.Border` outline, an eyebrow, a "Welcome back" heading, two
labeled `ui.TextInput`s, an inline `theme.Danger` error, a full-width
`ui.PrimaryButton(c, "Sign in")`, and a hint naming the demo account. The
inputs use accessible labels ("Username input", "Password input") that
differ from their visible label text on purpose: `ui.NewTester.Find`
matches the first element showing or labeled a string, so identical
strings would make a test click the static label instead of the control.

The handlers hang off the deferred-action hooks, not the `Clicked()` /
`Submitted()` queries:

- `ui.PrimaryButton(...).OnClick(a.signIn)`
- `ui.TextInput(...).OnSubmit(a.signIn)` on both fields, so Enter submits
  from either one
- `ui.TextInput(...).OnChange(a.clearAuthError)` on both, so typing hides
  a stale error

`OnClick`, `OnSubmit` and `OnChange` run after construction and
bound-value input, before MyGo rebuilds the view, so a state change inside
one of them is what the next frame renders. Setting `a.signedIn` during
the build pass instead risks leaving the card on screen until some other
event forces another frame.

`signIn` trims the username, compares the password exactly, and on a miss
sets one generic error ("Wrong username or password.") so the card never
hints which field is wrong. On success it clears the password, sets
`signedIn`, and resets `table`/`row` so the dashboard starts at its first
row; the chosen `kind` survives. `signOut` (the header's "Sign out"
button) clears `signedIn`, `password`, `authErr` and `filter`, keeping the
username for the next attempt. Neither method persists anything.

Guard rails for edits here: keep the visible heading from being exactly
"Sign in" (the clickable button owns that string), and never store the
password anywhere but the one bound field.

## NDI output

On macOS the window can publish itself as an NDI source, so OBS (with
obs-ndi) or any other NDI receiver can take the live app as a video
input. The header's "Start NDI" / "Stop NDI" button toggles it, and
`MYGO_NDI=1` starts it on launch. `ndi_other.go` is the `!darwin` stub:
the Windows and Linux builds compile, `ndiSupported()` returns false
there, the header leaves the toggle out, and a start request is quietly
ignored rather than failing. That matters for testers: a Windows beta has
no NDI runtime, so the button must not be a dead end.

Three pieces, all in `ndi_darwin.go`:

1. **Capture our own window.** `newWindowCapturer` reads the window number
   with `objc_msgSend(handle, sel_registerName("windowNumber"))` — AppKit
   is main-thread-only, so it runs in `WhenReady`/`OnClick`, never in the
   capture goroutine. Per frame, `CGWindowListCreateImage` with
   `includingWindow | boundsIgnoreFraming | nominalResolution` returns the
   window as the display shows it, title bar included, at 1 point per
   pixel. This is the one capture route macOS grants an app without the
   screen-recording permission, because the window is its own.
2. **Draw it into BGRA.** `CGBitmapContextCreate` over a `w*4` buffer with
   `kCGImageAlphaPremultipliedFirst | kCGImageByteOrder32Little`, then
   `CGContextDrawImage`. That combination is NDI's BGRA; getting the alpha
   or byte-order flag wrong yields inverted or garbage colors, not an
   error. `CGContextSetInterpolationQuality` is set to none so a resize
   scales rather than blurs.
3. **Send it.** A goroutine builds one `gondi.VideoFrameV2` (FourCC
   `BGRA`, 30/1, progressive, `SendTimecodeSynthesize`) and calls
   `SendVideoFrame`, which blocks on the sender's frame clock. That
   blocking call **is** the frame pacer: no ticker, no sleep on success.

`gondi` is a purego wrapper, so this needs no cgo and no NDI SDK headers:
`InitLibrary` dlopens `/usr/local/lib/libndi.dylib` (the NDI Tools
runtime) and the capture uses `purego` bindings to CoreGraphics and
libobjc. Both libraries and every symbol are resolved lazily, and their
panics are caught and turned into `ndiErr`, so a Mac without NDI Tools
gets the error strip instead of a crash.

`startNDI` records failure in `ndiErr` and leaves `ndi` nil; success
clears the error. `stopNDI` is nil-safe, waits for the goroutine to
finish, then destroys the sender, which is what removes the source from
the network — `OnQuit` does it on exit, so closing the app (or Ctrl-C, a
SIGTERM) never leaves a ghost source behind. `toggleNDI` and `ndiLabel`
are only the button's read of `a.ndi`.

The error strip is a row between the header and the divider, styled
`theme.Danger` at 11 DIP: it takes layout space only while `ndiErr` is
non-empty, so a normal launch has no gap.

## Layout

```
shell: Row .Fill().AlignItems(ui.Stretch).Background(theme.Background)
├── sidebar: Column .Width(sidebarWidth).FillHeight().Background(theme.Surface)
│   ├── cluster header row   (status dot, docker-desktop, Kubernetes v1.31.2)
│   ├── filter row           (TextInput bound to a.filter)
│   └── Sidebar .Grow(1)     (SidebarSection → SidebarItem + Badge)
├── Divider
└── pane: Column .Grow(1)
    ├── header: Row .Padding(16, 20)
    ├── NDI error strip: Row .Padding(0, 20, 8)   (only while a.ndiErr is set)
    ├── Divider
    └── Row .Grow(1).AlignItems(ui.Stretch)
        ├── table: Table .Grow(1)
        ├── Divider
        └── inspector: Scroll .Width(320).FillHeight()
```

The sidebar takes the `sidebarWidth` const (244); the inspector is a
literal 320; the table is the only flexible column. `Divider(c)` calls
are used throughout as separators: they append to the current scope and
take no children.

`ui.Row` defaults to `AlignItems(Center)`, so a row's children are
centered vertically unless told otherwise. That default caused two bands
of dead space: the shell row centered the content-height pane in the
window, and the pane's content row centered the table and inspector in
the remaining height. Both rows set `AlignItems(ui.Stretch)`, so the
pane fills the window, the table header sits directly under the pane
divider, and the row list fills down to the bottom at any window size.

## Data model

```go
type object struct {
    Name, Namespace, Status, Age, Image, Node string
    Restarts int
}
type resourceKind struct {
    id, label, group string
    icon             *ui.SVG
    objects          []object
}
type group struct {
    title string
    ids   []string
}
```

- `resources` holds 13 kinds across 4 groups: Workloads (6), Config (2),
  Network (2), Cluster (3), with 50 dummy objects in total. Only Pods fill
  `Image`, `Node` and `Restarts`; `inspector` guards those rows with
  `if o.Image != ""`, so every other kind shows just Namespace and Age.
- `groups` lists ids that refer to those kinds. Section membership and
  order come from `groups`; `resourceKind.group` is assigned in every
  literal but never read (see "Known drift and gaps").
- `resourcesByID` is built once at init for the `resource(id)` lookup.
  `resource` falls back to `resources[0]` (Pods) for an unknown id, so a
  typo in `groups` shows Pods rather than an error.

Two invariants matter:

1. **Every kind needs at least one object.** `inspector` does
   `k.objects[clamp(a.row, 0, len(k.objects)-1)]`, and for an empty kind
   `clamp(0, 0, -1)` returns `-1`, which panics; `a.table.Key` indexes
   `k.objects[i]` too. The current data keeps every kind at 2–6 objects.
2. **Every kind id must appear in a `groups` entry.** Otherwise the
   sidebar never lists it and the pane is unreachable.

## Rendering details worth knowing

**Header.** A 36×36 accent tile (radius 8) holding the kind icon at 20 in
`t.AccentText`, the label at 20 bold, a subtitle
`"<N> objects · <namespace>"` at 11 in `t.TextMuted`, a 160 DIP
`ui.Select` for the mock namespaces, a `Refresh` button and a
`Create` primary button. The first two are decoration; nothing handles
them. Then the NDI toggle ("Start NDI" / "Stop NDI", see "NDI output")
and the `Sign out` button, which are wired.

**Table.** `columns(k)` gives Name (auto), Namespace (110), Status (150),
and, for `pods` only, Restarts (76, `ui.End`), then always Age (64,
`ui.End`). `objectTable` sets `a.table.Selected = &a.row`,
`Key`/`Label` from `object.Name`, then renders cells:

- The chosen row is filled with the accent by the table, so the code
  swaps its muted text and status dot to `t.AccentText` for contrast.
- The status cell is a 7 DIP dot (radius 999) plus the status text.
- The cell switch uses `col == 3 && k.id == "pods"` for restarts and
  `col == len(cols)-1` for Age, so it stays correct whether the pods-only
  column exists.

**Status color.** `statusColor` is a substring ladder and its **order is
load-bearing**: `NotReady` contains `ready`, so the Danger test
(`notready`, `failed`, `crashloop`, `error`) must run before the Success
test (`ready`, `running`, `active`, `bound`, `complete`, `available`,
plus exact `clusterip`/`loadbalancer`/`nodeport`). Warning covers
`terminating`, `pending`, `progressing`, `suspended`, `released`;
anything else is `t.TextMuted`.

**Inspector.** A 320 DIP scroll column: uppercase kind eyebrow (10, 600,
muted), the object name (16, bold), a status row with a dot, then Current
(namespace, age, and for Pods image, node, restarts), Labels as wrapped
badges, and Events.

- `objectLabels` always returns exactly three badges and derives the app
  name from the text before the **first** dash (`api-7d9f8c-x2k4p` →
  `app=api`, `pv-postgres-0` → `app=pv`).
- `objectEvents` switches on the exact status string, not on
  `statusColor`'s substrings: Failed/CrashLoopBackOff get two warnings;
  Pending gets Scheduled plus FailedScheduling; NotReady/Terminating gets
  NodeNotReady; the default (including `Completed`, which only
  *contains* "complete") gets Scheduled plus Pulled.

**Sidebar.** The cluster header is mock text (`docker-desktop`,
`Kubernetes v1.31.2`) with a `t.Success` dot. The filter matches the
lowercased `label` only — not the id and not the group title — hides
empty groups entirely, and prints "No resources" inside the sidebar when
nothing matches. Each item carries a `ui.Badge` with
`len(k.objects)`.

## Testing

Every test runs the real view without a window: `ui.NewTester(a.view,
1180, 720)` builds frames headlessly, `HasText` queries the committed
text, `Click(label)` finds a control by its label and performs the click,
and `Type`/`Key` drive the focused input.

- `TestLoginGate` — a fresh app shows the card and never the dashboard.
- `TestLoginRejectsWrongPassword` — typing a wrong password keeps the
  card, shows the generic error, and editing a field hides it.
- `TestLoginAcceptsDemoAccount` — typing the demo account opens the
  dashboard and clears `a.password`.
- `TestEnterSubmitsLogin` — Tab from the username field to the password
  field and Enter submits, proving the `OnSubmit` wiring.
- `TestSignOut` — the header button returns to the card and clears the
  password and error.
- `TestResourceSwitching` — the default Pods pane, a kind switch to
  Deployments (which proves the pane, not just the highlight, follows the
  sidebar), and a switch to Secrets after setting `a.row = 3`, which
  proves the row reset.
- `TestNDIToggleWithoutWindow` — clicking "Start NDI" with no window
  fails into `a.ndiErr` instead of panicking, the strip renders, and the
  button stays "Start NDI". Skips on a build without a sender.
- `TestNDIHiddenWithoutSender` — the off-macOS counterpart: the header
  offers no toggle and `startNDI` is a quiet no-op. Skips on macOS.
- `TestStopNDIWithoutSender` — `stopNDI` with nothing running is a no-op.

The NDI pair is written so each build runs exactly one of them: the error
path is the one thing a headless tester can prove, and it is the path a
Mac without NDI Tools takes. A real stream is verified by hand (see
"Verification").

The helper `signIn` sets the two fields and clicks "Sign in", so tests
that only need the dashboard do not retype the account.

Because elements are per-pass, tests read `tt.Texts()`/`tt.HasText()`
rather than holding elements.

## Adding a resource kind

1. Add an icon value with the `icon` helper — it wraps your SVG paths in
   the same `viewBox="0 0 24 24"`, `stroke="currentColor"` frame, so the
   icon tints with the theme.
2. Append a `*resourceKind` to `resources` with a unique lower-case `id`,
   a label, at least one object, and the icon. Set `Image`/`Node`/
   `Restarts` only if the kind should show them.
3. Add the id to the right `groups` entry (its `ids` slice). Without this
   the kind is unreachable.
4. Only for a new column (as Pods has Restarts): add it in `columns` and
   extend the cell `switch` in `objectTable`; keep the Age case as
   `col == len(cols)-1`.
5. If you add a whole new group, add its bool to `app`/`newApp` and a
   case in `groupOpen`; the default branch otherwise shares
   `clusterOpen`.

No other wiring is needed: the header, table and inspector all derive
from the chosen `*resourceKind`.

## Verification

```sh
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 GOOS=linux go build ./...   # CGO_ENABLED=0 is required here
go vet ./...
test -z "$(gofmt -l *.go)"
CGO_ENABLED=0 go test ./...
go run .                     # manual check: sidebar, table, inspector
MYGO_NDI=1 go run .          # ...and an NDI source in OBS / NDI Monitor
```

CI (`.github/workflows/ci.yml`) runs the same gate on Linux, plus a
native Windows job that repeats it and then builds `mygo-demo.exe`,
starts it and fails if it has exited after eight seconds — a proxy for
"this binary runs on Windows", which a cross-compile alone cannot claim.
Both jobs then feed the packaging matrix, which packages the app for
macOS, Windows and Linux as artifacts. In a sandboxed agent session, set
`GOCACHE` to a writable directory; the default `~/Library/Caches/go-build`
is denied.

A push of a `v*` tag also runs the `Release` job: it refuses a tag that
disagrees with `version` in `mygo.config.ts`, downloads the matrix's
artifacts and publishes them as a GitHub release. Tags with a hyphen
(`v0.1.0-beta.1`) publish as prereleases, which is what a beta should be;
re-running a tag replaces its assets instead of failing.

A cross-compile to Linux without `CGO_ENABLED=0` fails inside the
standard library ("implicit declaration of function 'setresgid'"), which
is why the flag belongs on every build, not only on the host one.

To check NDI by hand: `MYGO_NDI=1 go run .`, then add an "NDI Source" in
OBS (or open NDI Video Monitor) and pick `mygo-demo (Kubernetes)`; the
listing is prefixed with the machine name, e.g.
`FLOCK-MBP1-PRO (mygo-demo (Kubernetes))`. Frames are the window as it
looks on screen, title bar included. Quitting the app removes the source.

`go tool mygo vet .`, recommended by the maintenance skill, is not
available in this module (it ships no `mygo` tool), so `go vet ./...` is
the gate.

## Known drift and gaps

Recorded as observed state, not as a work list:

1. **Unused web scaffold.** `src/main.ts` is entirely commented out;
   `index.html`, `vite.config.ts` and `src/style.css` style a page no
   window loads; `src/mygo.ts` is `mygo generate` output that still
   declares a `tick` event the Go side no longer sends.
2. **Inert packaging settings.** `mygo.config.ts` still declares
   `devUrl`, `devCommand`, `buildCommand` and `frontendDist`, and CI
   still runs the web build, so `dist/` is embedded without ever being
   displayed. The native window would work with none of it.
3. **Dead field.** `resourceKind.group` is written in every literal and
   never read.
4. **Stale historical plan.**
   `docs/superpowers/plans/2026-11-06-kubernetes-resource-sidebar.md`
   is a completed plan whose details drifted: it says "14 kinds" where
   the code has 13, lists 120/120/80/70 column widths against the shipped
   110/150/76/64, promises a `shownNamespace` helper that landed as
   literal `"—"` values, and calls the directory "not a git repository".
5. **Thin test coverage.** Kind switching, row reset, the sign-in flow
   and the NDI error path are tested; filtering, the inspector,
   empty-kind handling, the namespace picker, the header's Refresh/Create
   buttons and a real NDI stream have no automated tests, and there are
   no golden rendering checks.
6. **The sign-in is a demo gate.** The credentials are two source
   constants, there is no session persistence, rate limiting or password
   hashing, and the password sits in a plain `string`. It hides the
   dashboard from a casual look; it is not security.
7. **NDI is a demo feed, macOS only, and fixed to one window.** The
   sender captures the whole window at 1 point per pixel with the title
   bar, so a maximized 4K window sends a large frame; it never scales to
   a broadcast resolution and offers no audio, alpha or per-window
   choice. The window number is read once at start, so a recreated window
   needs a stop/start, and the source name comes from the NDI runtime's
   machine prefix. `gondi` is pinned at v0.0.2 (2023), which is the only
   NDI binding here that avoids cgo; it is unsupported on Windows, so the
   Windows and Linux builds hide the toggle and send nothing.
8. **Version coupling.** Everything here describes MyGo **v0.10.0**.
   After an upgrade, re-read `ui/doc.go` in the new module and the
   skill's migration note before trusting the API shapes in this file.
   The unused web tooling (`mygo-cli`, `mygo-runtime`) still pins 0.7.x;
   bump it with the Go module when packaging matters.
   bump it with the Go module when packaging matters.
