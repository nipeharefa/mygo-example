# Kubernetes Resource Sidebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Turn the MyGo demo into a native-UI Kubernetes resource explorer mock: a sidebar of resource kinds, and a dummy object list plus inspector for the chosen kind.

**Architecture:** One native window. `ui.Sidebar` sections hold one item per Kubernetes kind. The main pane shows a `ui.Table` of dummy objects for the chosen kind, and a fixed-width inspector for the chosen row. All state lives on one `app` struct; there is no Kubernetes client and no network access.

**Tech Stack:** Go 1.27, `github.com/egoist/mygo` v0.7.4, package `ui` (native UI), `ui.NewTester` for tests.

**Spec:** This plan implements the approved short design in the conversation: approach A, sidebar kinds with dummy list, row inspector included, full Cluster section kept.

## Global Constraints

- UI only: no Kubernetes client, no network calls, no frontend files. `src/`, `index.html`, `mygo.config.ts` stay untouched.
- Follow the pinned MyGo API (`go list -m -f '{{.Dir}}' github.com/egoist/mygo`), not memory of it.
- Views build from state each frame; never store `ui.Element` or `*ui.Context` in app state.
- Use `c.Theme()` colors so light and dark both work.
- The directory is not a git repository: skip commit steps, verify with build and tests instead.
- Verify with `CGO_ENABLED=0 go build ./...`, `go vet .`, and `go test ./...`.

## Review Focus

- A sidebar click must change both the pane title and the rows, not just the selection highlight.
- Switching kinds must reset table selection and scroll so a stale row index cannot point past the new object slice.
- Status colors must stay readable on a chosen (accent-filled) row.
- A missing namespace or image must render as `—` or be omitted, never as an empty cell that looks broken.
- The window at its minimum size must keep the sidebar and inspector usable rather than squashing the table to nothing.

---

### Task 1: State, kinds, and dummy data

**Files:**
- Modify: `main.go` (replace the demo)

**Interfaces:**
- Produces: `type app struct { kind, lastKind, filter, namespace string; row int; table ui.ListState; workloadsOpen, configOpen, networkOpen, clusterOpen bool }`
- Produces: `type object struct { Name, Namespace, Status, Age, Image, Node string; Restarts int }`
- Produces: `type resourceKind struct { id, label, group string; icon *ui.SVG; objects []object }`
- Produces: `var resources []*resourceKind`, `func resource(id string) *resourceKind`, `func newApp() *app`

- [x] **Step 1: Define the data types and the 14 kinds**

Ids and labels: `pods/Pods`, `deployments/Deployments`, `statefulsets/StatefulSets`, `daemonsets/DaemonSets`, `jobs/Jobs`, `cronjobs/CronJobs`, `configmaps/ConfigMaps`, `secrets/Secrets`, `services/Services`, `ingresses/Ingresses`, `nodes/Nodes`, `namespaces/Namespaces`, `persistentvolumes/PersistentVolumes`. Groups: Workloads, Config, Network, Cluster. Each kind gets 3–6 realistic dummy objects with distinct names; Pods also get `Image`, `Node`, `Restarts`.

- [x] **Step 2: Add icons**

Package-level `*ui.SVG` values parsed with `ui.MustParseSVG`, one per kind, stroke `currentColor`, viewBox `0 0 24 24`.

- [x] **Step 3: Add lookup and status helpers**

`resource(id)`, `statusColor(t *ui.Theme, status string) ui.Color` (NotReady/Failed first, then *Ready/Running/Active/Bound/Complete/Available → Success, Pending/Progressing/Terminating/Suspended/Released → Warning, else TextMuted), and `shownNamespace(o object) string` returning `—` for cluster-scoped kinds.

- [x] **Step 4: Verify it compiles**

Run: `CGO_ENABLED=0 go build ./...`
Expected: exit 0.

### Task 2: Sidebar

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: Task 1 types.
- Produces: `func (a *app) sidebar(c *ui.Context)`.

- [x] **Step 1: Write the failing test**

In `main_test.go`, `TestResourceSwitching`: build `newApp()`, `ui.NewTester(a.view, 1180, 720)`, assert `Pods` and the first pod name show; `tt.Click("Deployments")`; assert `a.kind == "deployments"` and `api-gateway` shows; `tt.Click("Secrets")`; assert `a.kind == "secrets"` and `2 keys` shows.

- [x] **Step 2: Run it to see it fail**

Run: `go test ./... -run TestResourceSwitching`
Expected: FAIL (no `view` method).

- [x] **Step 3: Build the sidebar column**

`ui.Column(c).Width(244).FillHeight().Background(t.Surface)`: cluster header row (success dot, `docker-desktop`, `Kubernetes v1.31.2`), filter `ui.TextInput`, then `ui.Sidebar(c, &a.kind, ...)` with one `SidebarSection` per group and `ui.SidebarItem(id, icon, label)` plus a `ui.Badge` of the object count. A non-empty filter hides non-matching items and prints `No resources` when nothing matches.

- [x] **Step 4: Run the test**

Run: `go test ./... -run TestResourceSwitching`
Expected: PASS for the sidebar selection and the pane switch once Task 3 lands; until then it fails on missing pane text, which is expected.

### Task 3: Header, table, and inspector

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: Task 1 types, `a.sidebar`.
- Produces: `func (a *app) view(c *ui.Context)`, `func (a *app) pane(c *ui.Context)`, `func (a *app) inspector(c *ui.Context)`, `func (a *app) columns(k *resourceKind) []ui.TableColumn`.

- [x] **Step 1: Build the shell**

At the top of `view`, reset `a.table` and `a.row` when `a.kind != a.lastKind`, then set `a.lastKind`. Root `ui.Row(c).Fill().Background(t.Background)` holding the sidebar, a `ui.Divider(c)`, and the pane.

- [x] **Step 2: Build the header**

Icon tile in the accent color, kind label at 20 bold, `N objects · <namespace>` subtitle, spacer, namespace `ui.Select`, `Refresh` button, `Create` primary button (all dummy).

- [x] **Step 3: Build the table**

`a.table.Selected = &a.row`, `Key`/`Label` from the object name; columns Name, Namespace (120), Status (120), Restarts (80, right) only for pods, Age (70, right). The status cell is a row with a 7 DIP colored dot and the status text; plain text stays default color.

- [x] **Step 4: Build the inspector**

`ui.Scroll(c).Width(320)` with the chosen object (`a.row` clamped): kind eyebrow, name, status row, then CURRENT (image/node/restarts when present), LABELS (badges), EVENTS (two dummy rows). Use `—` for absent values.

- [x] **Step 5: Run the test**

Run: `go test ./... -run TestResourceSwitching`
Expected: PASS.

### Task 4: Window and final verification

**Files:**
- Modify: `main.go` (`main()`)
- Create/Modify: `main_test.go`

- [x] **Step 1: Replace the window setup**

`Title: "Kubernetes"`, `Width: 1180`, `Height: 720`, `MinWidth: 960`, `MinHeight: 600`, `StateKey: "main"`, `Content: ui.View(app.view)`. Remove the URL, the tick event, and the old demo types.

- [x] **Step 2: Verify the whole app**

Run: `CGO_ENABLED=0 go build ./... && go vet . && go test ./...`
Expected: all exit 0.

- [x] **Step 3: Run it by hand**

Run: `go run .`
Expected: a window opens; clicking each sidebar kind swaps the header and table; clicking a row updates the inspector.
