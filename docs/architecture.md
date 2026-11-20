# Architecture

This is the guide to **this app**, not to MyGo. The framework's own
`docs/architecture.md` ships with the pinned module
(`go list -m -f '{{.Dir}}' github.com/egoist/mygo`); read it for the
engine. This file covers what `mygo-demo` is made of and why.

The app is a mock Kubernetes resource explorer drawn with package `ui`:
a sidebar of resource kinds, a table of dummy objects for the chosen
kind, and an inspector for the chosen row. There is no Kubernetes client,
no network access, and no web page. One `app` struct is the whole state.

## What runs

`main()` in `main.go` starts MyGo and opens one native window:

```go
mygo.App.WhenReady(func() {
    mygo.NewWindow(mygo.WindowOptions{
        Title:     "Kubernetes",
        Width:     1180,
        Height:    720,
        MinWidth:  960,
        MinHeight: 600,
        StateKey:  "main",
        Content:   ui.View(a.view),
    })
})
if err := mygo.App.Run(); err != nil {
    log.Fatal(err)
}
```

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
`Create` primary button. The last two are decoration; nothing handles
them.

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
go vet ./...
test -z "$(gofmt -l *.go)"
CGO_ENABLED=0 go test ./...
go run .            # manual check: sidebar, table, inspector
```

CI (`.github/workflows/ci.yml`) runs the same gate plus darwin/windows
cross-compiles, then packages the app for macOS, Windows and Linux as
artifacts. In a sandboxed agent session, set `GOCACHE` to a writable
directory; the default `~/Library/Caches/go-build` is denied.

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
5. **Thin test coverage.** Kind switching, row reset and the sign-in
   flow are tested; filtering, the inspector, empty-kind handling, the
   namespace picker and the header's Refresh/Create buttons have no
   tests, and there are no golden rendering checks.
6. **The sign-in is a demo gate.** The credentials are two source
   constants, there is no session persistence, rate limiting or password
   hashing, and the password sits in a plain `string`. It hides the
   dashboard from a casual look; it is not security.
7. **Version coupling.** Everything here describes MyGo **v0.7.4**. After
   an upgrade, re-read `ui/doc.go` in the new module and the skill's
   migration note before trusting the API shapes in this file.
