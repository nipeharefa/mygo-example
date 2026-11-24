# Song Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpower-subagent-driven-development (recommended) or superpower-executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a third window, the Song Editor, where every song in a persisted library is authored as labeled parts holding one or more slides; the same per-song text size, scaled to 1280×720, drives the editor preview, the stage window and the NDI feed, and the console still builds the run of show.

**Architecture:** The library gains `songPart{Label, Slides []string}` and `song.TextSize`; the deck flattens parts into slides that snapshot the size (a slide already on air never changes when the song is edited). `library.json` in the app data directory becomes the sole song library, seeded from the public-domain built-ins on first load. The editor is a normal `mygo.Window` opened through the existing `requestEditor → settle → a.newWindow` path; it renders the same `slideBody` helper as `liveView`, so the preview cannot drift from the output. All sizes are stage units at a 1280-wide design and are multiplied by `width/1280`.

**Tech Stack:** Go 1.27.3, `github.com/egoist/mygo` v0.10.0 native UI, `CGO_ENABLED=0`, headless `ui.NewTester` tests, JSON persistence with `encoding/json`, no new dependencies.

**Spec:** The Design section of this plan. Product decisions were confirmed with the user on 2026-11-23: separate editor window; whole library saved to `library.json`; one text size per song in stage units; the run of show is the arrangement.

## Starting Point

- Repo: `/Users/nipeharefa/github/mygo-demo`, branch `main`, HEAD `0104d5e`.
- The worktree is dirty with unrelated update-channel work: `main.go` +10/−1, `settings.go` +12/−1, `update.go` +34, plus untracked `mygo-demo`/`mygo-demo.exe`. **Do not mix it into editor commits.** If that work builds and tests green, commit it first as its own `feat: update channel preference` commit on `main`; otherwise stash it and stop, telling the user. Never commit the binaries.
- Read `docs/architecture.md` and `.agents/skills/mygo-maintenance/SKILL.md` before touching Go files.
- Create branch `feat/song-editor` and save this plan to `docs/superpowers/plans/2026-11-23-song-editor.md` as the first commit.

## Global Constraints

- `CGO_ENABLED=0` everywhere. No cgo, no SDK headers, no new modules.
- Views are pure functions of `app` state: `a.view`, `a.liveView`, `a.editorView` rebuild every pass; never store a `ui.Element`, widget part or `*ui.Context` on `app` or in a goroutine.
- Never call a `mygo.Window` method from inside a build pass. Windows are created only through `a.newWindow`, only from `settle()` (which runs after the build pass), and `openEditorWindow` is guarded on `a.win != nil`. Tests never create a window.
- Every mutation callback ends with `a.changed()`, and `changed()` also invalidates `a.editor`.
- Literal colors stay in `theme.go`. The editor uses `consoleTheme`; the preview and the stage use `stageTheme`; selected rows use `t.Selection`.
- Monitor/stage separation is untouched: the editor never writes `a.stage` or `a.preview` except through the existing deck methods (`addSong`, `addPart`) that already set the preview.
- Persistence is `settings.json` (existing) **and** `library.json` (new). The run of show stays session-only. `a.settingsDir == ""` in tests, so both `libraryPath()` and `saveLibrary()` must no-op there.
- Song text stays public domain (the 5 traditional hymns and KJV). Do not add copyrighted lyrics.
- Sizes: `defaultTextSize = 40`, `minTextSize = 24`, `maxTextSize = 72`, design width `1280`, design height `720`. A size is stage pixels at 1280 and is multiplied by `canvasWidth/1280` everywhere.
- Tests are headless (`ui.NewTester`) and must not require a display, NDI runtime, or network.
- Gate for every task: `gofmt -l *.go` empty, `go vet ./...`, `CGO_ENABLED=0 go test ./...`, `CGO_ENABLED=0 go build ./...`. Cross-build `GOOS=darwin`, `GOOS=windows`, `GOOS=linux CGO_ENABLED=0` before the final commit. Inside the sandbox set `GOCACHE="$TMPDIR/gocache-mygo-demo"`.

## Design

### 1. Data model: what a song with many parts looks like

Today `song.Sections []songSection{Label, Text}` is one label per slide, so a verse too long for one screen needs fake labels ("Verse 1a", "Verse 1b") and parts have no identity beyond a string on a slide. Four options:

| Option | Shape | Verdict |
| --- | --- | --- |
| A. Flat sections (today) | `Sections []{Label,Text}` | Fails multi-slide parts; labels drift; no place for song-level settings. |
| B. Parts own slides | `Parts []songPart{Label, Slides []string}`, `TextSize float64` | **Chosen.** A labeled part spans 1..N slides that all show that label. |
| C. Normalized parts + arrangement | `Parts` plus `Order []partRef` | Deferred. The run of show already expresses order; a second order model would drift from the deck. |
| D. Flat slides with a Part string | `Slides []{Part,Text}` | Part-level reorder/delete becomes multi-slide bookkeeping and labels can diverge. |

Concrete types in `data.go`:

```go
const (
    defaultTextSize = 40.0
    minTextSize     = 24.0
    maxTextSize     = 72.0
)

// songPart is one labeled part of a song. It spans one or more slides so a
// long verse keeps a single name on every screen it covers.
type songPart struct {
    Label  string   `json:"label"`
    Slides []string `json:"slides"`
}

type song struct {
    ID       string     `json:"id"`
    Title    string     `json:"title"`
    Author   string     `json:"author"`
    Key      string     `json:"key"`
    TextSize float64    `json:"textSize"` // stage px at 1280x720
    Parts    []songPart `json:"parts"`
}

func (s song) textSize() float64 // 0 -> default, then clamp to [min,max]
func (s song) slideCount() int
func findSong(list []song, id string) (song, bool)
```

Answering the verse-1/verse-2 question directly:

- **Verse 1 and Verse 2** are two `songPart`s, labels `"Verse 1"` and `"Verse 2"`, each holding its own slides.
- **A long verse split over two screens** is one part with two `Slides`; both slides carry `Section == "Verse 1"`, and the deck does not care that they share a label.
- **Chorus** is just another part labeled `"Chorus"`. Repetition is not stored in the song: `addSong` adds every part once in order, and the editor's "Add part to show" adds one part again. A V1–C–V2–C service is four deck actions, two of them "add the chorus part".
- **Scripture** stays the existing flat `verse` slide; a future multi-slide verse can be authored in the editor as song-like parts without touching the deck.

Built-ins are converted one-to-one: each existing section becomes a part with one slide, and `all-creatures` (7 lines) is split into one part `"Verse 1"` with **two** slides so a real multi-slide part exists in the seed data. All built-ins keep `TextSize: 0` (meaning default 40); `newApp` and `loadLibrary` normalize the in-memory value to 40 before the UI binds a slider to it.

### 2. One renderer, scaled by output width

`liveView`, the editor preview and the NDI feed all draw through one helper, so the preview is WYSIWYG by construction:

```go
const (
    stageDesignWidth  = 1280
    stageDesignHeight = 720
)

// stageScale is how much bigger or smaller the canvas is than the 1280x720
// design the sizes were picked at.
func stageScale(width float32) float32 // width <= 0 -> 1, else width/stageDesignWidth

type slideMetrics struct {
    Eyebrow, Letter, Text, Caption, PadX, PadY float32
}

// metricsFor scales the design at 1280 to this canvas width.
func metricsFor(width float32, textSize float64) slideMetrics

// slideBody draws eyebrow, text and caption at m; the caller owns the
// background, padding, centering and DragWindow.
func slideBody(c *ui.Context, s slide, m slideMetrics)
```

At width 1280: eyebrow 14, letter spacing 2, body `textSize` (default 40), caption 16, padding x 56 / y 72. At 640 everything halves; at 1920 it grows 1.5×; a stage window can be any aspect because the layout centers and pads the same design. The console's 16:9 thumbnail cards keep their stylized fixed fonts — they are thumbnails, not output. A slide's `TextSize` is snapshotted by the deck when added, so editing a song later cannot silently change a slide already in the service.

### 3. Editor window

Layout, 1200×800 (min 980×660), `StateKey: "editor"`, title `mygo-demo — Song editor`:

- **Left column (260):** "Songs" heading, `New song`, `Restore samples`, search field (`Label("Search songs")`), scrollable rows. A row is clickable and labeled `"Edit "+Title`; the selected row uses `t.Selection`.
- **Right column (scroll):** toolbar (`Close`, `Add song to run of show`, `Remove song`), metadata row (`Song title input`, `Song author input`, `Song key input`), text-size row (`Text size` slider, 24–72, step 1, live number), the 16:9 preview (max 720 wide, min 320, height = w·9/16), then the parts editor, then the selected slide's `Slide text` text area, then the status line.
- **Parts editor:** for each part a header row with `Part label N` text input, `Add slide`, `Remove part`; under it one row per slide (`Select slide N` showing its first line) plus `Remove slide`. After the list: `Add part`. Removing the last part or the last slide is disabled — a song always keeps ≥1 part with ≥1 slide; a slide's text may be empty while drafting.
- **Selection** is `editSong/editPart/editSlide` indices into `a.songs`; every structural change calls `clampEditor()`. A filtered list row still captures the real index, so search never remaps the selection.
- **Preview width:** `w = min(editorPreviewMax, float32(c.Size().W) - editorListWidth - 3*editorPadding)`, clamped to ≥320; the preview calls `metricsFor(w, size)` and `slideBody`, so at 720 the text is exactly 0.5625 of the stage's.

### 4. Library file

`library.json` in `a.settingsDir`:

```json
{ "version": 1, "songs": [ { "id": "amazing-grace", "title": "Amazing Grace", "author": "...", "key": "G", "textSize": 40, "parts": [ { "label": "Verse 1", "slides": ["..."] } ] } ] }
```

- `loadLibrary()`: no dir → `a.songs = cloneSongs(builtinSongs)`; file missing → seed, best-effort write; read/validate error → log `mygo-demo: load library: %v`, keep built-ins and leave the bad file for the operator; success → use it (an empty `songs` array is a valid, empty library).
- `saveLibrary()`: no-op without a dir; `MkdirAll` + `MarshalIndent` + trailing newline, log errors.
- Validation on read: every ID non-empty and unique; otherwise error.
- Every edit saves immediately (structural changes and text-field `OnChange`). The file is a few KB; durability beats a debounce.
- `Restore samples` appends any built-in whose ID is absent, normalizes sizes, saves, and selects the first restored song. It never overwrites an edited song with the same ID.

### 5. App wiring and lifecycle

New `app` fields: `songs []song` (active library), `editor *mygo.Window`, `openEditor bool`, `editSong/editPart/editSlide int`, `editorStatus string`, `editQuery string`.

- `newApp`: `a.songs = cloneSongs(builtinSongs)`, seed the demo deck through `addSong`/`addVerse`, set editor indices to −1.
- `main()` `WhenReady`: after `settingsDir` and `loadSettings()`, call `a.loadLibrary()` before creating the console.
- `requestEditor()`: `a.openEditor = true; a.changed()`.
- `settle()`: handle both requests after the build pass; for the editor, focus the existing window or create it when `a.win != nil && a.editor == nil`.
- `openEditorWindow()` follows `openHelp` exactly: early return + `Focus()` when open, `mygo.NewWindow(...)`, assign before `OnClosed` whose guard is `if a.editor != w { return }`.
- `closeEditor()` nils then closes; `signOut()` calls it (the library itself is not session data, but the gate hides the editor).
- Console entry point: a `Song editor` button in the top bar (after NDI, before Settings) calling `a.requestEditor`. `songLibrary` iterates `a.songs`.
- `deck.songByID` becomes `(a *app) songByID`, and `addSong`/`addPart` read `a.songs`. The package var `songs` is renamed `builtinSongs`.

## Review Focus

1. **Text overflow at a fixed size.** No auto-fit exists in MyGo (no shrink-to-fit API; `Transform` scales paint but not layout). The expected behavior is visible overflow in the preview, identical to the stage. Pinned by exact `metricsFor` values at 1280/640 and by rendering the 9-line `mighty-fortress` slide at 40 without truncation.
2. **Blank slides reaching air.** A newly added slide starts empty and `addSong`/`addPart` flatten parts; blank or whitespace-only text must never create a deck slide. Pinned by `TestAddSongSkipsBlankSlides` and `TestAddPartSkipsBlankSlides`.
3. **Deck drift after a library edit.** A slide already in the run of show must keep its own text and size after the song is resized or rewritten. Pinned by `TestEditingSongLeavesDeckUntouched`.
4. **Window lifecycle.** Opening the editor must not hang or double-open; closing it, signing out, or closing the console must not leave a stale pointer or touch NDI. Pinned by headless `settle` tests (no window in tests), `closeEditor` nil-safety, and a sign-out test asserting the editor flag/pointer are cleared while NDI behavior is unchanged.
5. **Extreme output sizes.** `stageScale(0)` must not divide by zero, and 640/1920 widths must scale exactly. Pinned by `TestStageScale` and `TestLiveViewScalesTextAtHalfWidth` (the same text still renders at 640×360).
6. **Typing is not a shortcut.** The editor has no single-letter shortcuts; typing `b` in the text area must not black out. Pinned by `TestTypingInEditorDoesNotBlackout` (editor view only; blackout stays console/stage).

## Tasks

### Task 0: Workspace setup

**Files:** Create `docs/superpowers/plans/2026-11-23-song-editor.md` (this plan).

- [ ] Commit or stash the unrelated update-channel work as described in Starting Point; confirm `git status --short` shows only untracked binaries.
- [ ] `git switch -c feat/song-editor`; copy this plan to the path above; commit `docs: plan the song editor`.
- [ ] Run the baseline gate now and record it: `CGO_ENABLED=0 go test ./...`.

### Task 1: Song parts and per-song size in the library data

**Files:** Modify `data.go`, `deck_test.go`, `console.go` (only where it reads section fields); Test `data_test.go` (create).

**Interfaces:** Produces `songPart`, `song.Parts`, `song.TextSize`, `song.textSize()`, `song.slideCount()`, `findSong`, `builtinSongs` (renamed from `songs`, done here to avoid a second rename).

- [ ] Write failing `data_test.go`:

```go
func TestBuiltinSongsHaveParts(t *testing.T)  // every song: non-empty ID/Title, >=1 part, every part non-empty Label and >=1 slide
func TestAllCreaturesVerseOneHasTwoSlides(t *testing.T) // one part labeled "Verse 1" with 2 slides
func TestSongTextSizeDefaultsAndClamps(t *testing.T)    // 0 -> 40, 10 -> 24, 99 -> 72, 55 -> 55
func TestFindSong(t *testing.T) // finds amazing-grace, rejects unknown
```

- [ ] Run `CGO_ENABLED=0 go test ./...` and confirm the new tests fail to compile (type mismatch).
- [ ] Implement the types above in `data.go`; convert each section to `songPart{Label, Slides: []string{text}}`; split `all-creatures` Verse 1 after its 4th line; rename the package var to `builtinSongs`; replace `songByID` with `findSong` (keep a thin `songByID` only if a non-test caller remains, otherwise update console/tests in this task).
- [ ] Update `deck_test.go`'s `TestDataIsReachable` to walk `Parts`/`Slides` (still ≥4 songs, unique IDs/titles, ≥8 slides total, non-empty labels/text).
- [ ] Run the gate; commit `feat: song parts own their slides`.

### Task 2: Expand parts into deck slides with a size snapshot

**Files:** Modify `deck.go`, `deck_test.go`.

**Interfaces:** Produces `slide.TextSize float64`, `newSlide(title, section, text string, kind slideKind, size float64) slide`, `addSong(id string)` (method, flattened), `addPart(id string, part int) bool`.

- [ ] Add failing tests:

```go
func TestAddSongAppendsSections(t)        // amazing-grace -> 3 slides, preview 0, section "Verse 1", TextSize 40
func TestAddSongAllCreaturesHasTwoSlides(t) // 2 slides, both Section "Verse 1"
func TestAddSongSkipsBlankSlides(t)       // song with one blank slide adds nothing, preview unchanged
func TestAddPartAppendsOnePart(t)         // addPart("amazing-grace", 1) adds Verse 2 only; stage unchanged
func TestAddPartIgnoresUnknown(t)         // unknown id/part returns false, deck unchanged
func TestAddSongTwiceKeepsIDsUnique(t)    // 6 slides, IDs unique
```

- [ ] Run tests; confirm `TextSize`/`addPart` do not compile.
- [ ] Implement: `addSong` loops parts→slides, appends non-blank text with `Section = part.Label`, `TextSize = song.textSize()`, `preview` = first added; `addPart` does the same for one part and returns false when nothing was added. `newSlide` stamps `s%d` as today.
- [ ] Gate; commit `feat: expand song parts into slides with a size snapshot`.

### Task 3: `library.json` persistence

**Files:** Create `library.go`, `library_test.go`; modify `main.go` (`app.songs` field, `newApp` seeding, startup `loadLibrary`).

**Interfaces:** Produces `libraryFile`, `libraryVersion`, `readLibrary(path) (libraryFile, error)`, `writeLibrary(path, libraryFile) error`, `(a *app) libraryPath()`, `(a *app) loadLibrary()`, `(a *app) saveLibrary()`, `(a *app) normalizeLibrary()`, `cloneSongs([]song) []song`, `newSongID(title string, existing []song) string`.

- [ ] Failing `library_test.go` with `t.TempDir()`:

```go
func TestLibraryRoundTrip(t)          // write then read equal, trailing newline in file
func TestReadLibraryRejectsDuplicateIDs(t)
func TestLoadLibrarySeedsBuiltinsWhenMissing(t) // file created, a.songs == builtins (deep copy), TextSize 40
func TestLoadLibraryKeepsBuiltinsOnCorruptFile(t) // a.songs still builtins, file bytes untouched
func TestLoadLibraryAcceptsEmptySongs(t)
func TestSaveLibraryWithoutDirIsNoop(t) // GOCACHE-safe, no path, no panic
func TestCloneSongsIsDeep(t)           // editing the clone leaves builtinSongs untouched
func TestNewSongID(t)                  // "New Song" -> "new-song", second -> "new-song-2", punctuation -> dash
```

- [ ] Run; confirm failure.
- [ ] Implement `library.go` following the `settings.go` patterns (`os.MkdirAll`, `MarshalIndent`, trailing newline, log-only errors). Add `songs []song` to `app`; `newApp` sets `a.songs = cloneSongs(builtinSongs)` and seeds the demo deck after that; `main()` calls `a.loadLibrary()` right after `a.loadSettings()`.
- [ ] Gate; commit `feat: persist the song library to library.json`.

### Task 4: Editor window lifecycle and the active library

**Files:** Modify `main.go` (`requestEditor`, `openEditorWindow`, `closeEditor`, `settle`, `changed`, `signOut`, `newApp` indices), `console.go` (top-bar `Song editor` button, iterate `a.songs`), `deck.go` (`(a *app) songByID`, `addSong`/`addPart` read `a.songs`), `deck_test.go`; Test `editor_test.go` (create).

**Interfaces:** Consumes `library.go`, `settle`; Produces `(a *app) requestEditor()`, `openEditorWindow()`, `closeEditor()`, `(a *app) songByID(id)`.

- [ ] Failing tests:

```go
func TestRequestEditorSetsOpenFlag(t)        // after requestEditor, a.openEditor true, a.editor nil
func TestSettleWithoutWindowDoesNotOpen(t)   // request then a.settle(); a.editor nil, flag cleared
func TestCloseEditorNilSafe(t)
func TestChangedKeepsEditorNilSafe(t)        // changed() with editor nil does not panic
func TestSignOutClosesEditorState(t)         // sign-out clears openEditor and editor pointer
func TestConsoleSongEditorButton(t)          // click "Song editor" -> a.openEditor true or editor nil
```

- [ ] Run; confirm failure.
- [ ] Implement lifecycle exactly as designed; add the console button; switch console and deck to `a.songs`; rename remaining `songs` references to `builtinSongs`.
- [ ] Gate; commit `feat: open the song editor window`.

### Task 5: Shared slide renderer and width scaling

**Files:** Create `slideview.go`, `slideview_test.go`; modify `output.go`, `output_test.go`.

**Interfaces:** Produces `stageDesignWidth`, `stageDesignHeight`, `stageScale`, `slideMetrics`, `metricsFor`, `slideBody`.

- [ ] Failing tests:

```go
func TestStageScale(t)     // 1280->1, 640->0.5, 1920->1.5, 0->1
func TestMetricsFor(t)     // (1280,40) -> {14,2,40,16,56,72}; (640,40) -> halves; 0->40; 90->72
func TestLiveViewUsesSlideSize(t)      // slide TextSize 60 at 1280 renders its first line
func TestLiveViewScalesTextAtHalfWidth(t) // liveView in a 640x360 tester still HasText(first line)
```

- [ ] Run; confirm failure.
- [ ] Implement `metricsFor` as 14/2/`textSize`/16/56/72 times `stageScale(width)`; implement `slideBody` (eyebrow `SingleLine`, body `LineHeight(1.3).TextAlign(ui.Center)`, muted caption) and use it in `liveView`, with padding from the metrics and `s.TextSize`.
- [ ] Gate; confirm the existing 1280×720 `output_test.go` tests still pass unchanged.
- [ ] Commit `feat: scale slide text from the output width`.

### Task 6: Editor shell, song list, selection and preview

**Files:** Create `editor.go`, `editor_test.go`.

**Interfaces:** Consumes `slideBody`, `metricsFor`, `stageTheme`, `consoleTheme`; Produces `(a *app) editorView(c)`, `editorList(c)`, `editorMain(c)`, `editorPreview(c, w)`, `selectSong(i int)`, `clampEditor()`, and the editor constants.

- [ ] Failing tests:

```go
func TestEditorEmptyLibrary(t)        // editorView has "No songs yet" and "New song"
func TestEditorSelectsSong(t)         // click "Edit Amazing Grace" -> editSong index, preview shows first line
func TestEditorPreviewShowsSlideParts(t) // preview has "VERSE 1", first line, "Amazing Grace · Verse 1"
func TestEditorPreviewWidthClamped(t) // at 1200 wide editorView renders; preview helper returns 720; at 900 returns >=320
func TestEditorSearchKeepsSelectionStable(t) // filter then click "Edit Holy, Holy, Holy" selects the real index
```

- [ ] Run; confirm failure.
- [ ] Implement the shell: console theme, `Row` with the 260 list and the scrolling main column, toolbar with `Close`, clickable song rows labeled `"Edit "+Title` with `t.Selection` when selected, and the 16:9 preview box using `metricsFor` + `slideBody`. Keep all window work out of the view.
- [ ] Gate; commit `feat: add the song editor shell and preview`.

### Task 7: Editing metadata, parts, slides and text size

**Files:** Modify `editor.go`, `library.go` (normalization), `editor_test.go`, `library_test.go`.

**Interfaces:** Produces `(a *app) setTextSize(v float64)`, `addPart()`, `removePart()`, `addSlide()`, `removeSlide()`, `removeSong()`, `newSong()`, `restoreSamples()`, `songEdited()`, `saveLibrary()` calls on change.

- [ ] Failing tests (set `a.settingsDir = t.TempDir()` and seed the library in each test):

```go
func TestEditingTitleSavesLibrary(t)      // click "Song title input", Type(" Grace"), file contains the new title
func TestEditingSlideTextSavesLibrary(t)  // click "Slide text", Type("new line"), model and file updated
func TestAddPartAddsOneSlide(t)           // "Add part" -> Parts grows, selection on it, file saved
func TestRemovePartKeepsLastPart(t)       // remove disabled at one part; status says a part is needed
func TestRemoveSlideKeepsLastSlide(t)
func TestSetTextSizeClampsAndSaves(t)     // 99 -> 72, 10 -> 24, file stores 40 by default
func TestNewSongIDIsUnique(t)             // newSong twice -> ids differ, both in file
func TestRestoreSamplesOnlyAddsMissing(t) // deleting builtins then restore brings them back, edits preserved
func TestRemoveSong(t)                    // song removed from model, file and selection clamped
```

- [ ] Run; confirm failure.
- [ ] Implement the metadata fields, slider (`ui.Slider(&a.songs[i].TextSize, 24, 72).Step(1)` with `.OnChange(a.setTextSize)`), labels, slide `TextArea` bound to `&a.songs[i].Parts[j].Slides[k]` with `.OnChange(a.songEdited)`, add/remove buttons with disabled guards, status line, `newSong`, `restoreSamples`, and immediate saves. Normalize sizes on load so the slider never receives 0.
- [ ] Gate; commit `feat: edit song parts, slides and text size in the editor`.

### Task 8: Send the edited song to the run of show

**Files:** Modify `editor.go`, `editor_test.go`.

**Interfaces:** Consumes `addSong`, `addPart`; Produces `(a *app) addEditorSongToDeck()`, `addEditorPartToDeck()`.

- [ ] Failing tests:

```go
func TestAddSongToRunOfShow(t)        // deck +3, preview on first added, status "Added 3 slides"
func TestAddSongWithNoTextDoesNothing(t) // status explains, deck unchanged
func TestAddPartToRunOfShow(t)        // only selected part's slides added, stage unchanged
func TestEditingSongLeavesDeckUntouched(t) // add, then change TextSize/text; deck slide keeps old size/text
```

- [ ] Run; confirm failure.
- [ ] Implement the two buttons with status messages; blank-only songs add nothing.
- [ ] Gate; commit `feat: add song parts to the run of show from the editor`.

### Task 9: Documentation

**Files:** Modify `README.md`, `docs/architecture.md`, `AGENTS.md`.

- [ ] `README.md`: the Song editor button, what a part/slide is, `library.json`, the size slider and how it maps to the output.
- [ ] `docs/architecture.md`: state fields, the third window lifecycle, data model diagram (song → parts → slides → deck), persistence, and known gaps (no undo, no arrangement, deck session-only).
- [ ] `AGENTS.md`: where-things-live rows for `editor.go`, `library.go`, `slideview.go`, `editor_test.go`, `library_test.go`; update the persistence hard rule and the window-creation rule to mention the editor.
- [ ] Gate; commit `docs: describe the song editor and library`.

### Task 10: Final gate

- [ ] `test -z "$(gofmt -l *.go)"`, `go vet ./...`, `CGO_ENABLED=0 go test ./...`, `CGO_ENABLED=0 go build ./...`.
- [ ] `GOOS=darwin go build ./...`, `GOOS=windows go build ./...`, `GOOS=linux CGO_ENABLED=0 go build ./...`.
- [ ] Manual: `go run .`, sign in `admin`/`admin`, open Song editor, split a verse across two slides, add a Chorus, set 60, add song to run of show, Open output; confirm the stage text matches the preview proportionally and NDI (with `MYGO_NDI=1`) matches the stage.
- [ ] Confirm `git status` shows no binaries and no unrelated files.

## Acceptance Criteria

- The console's `Song editor` button opens a third window; it closes, reopens and survives sign-out without stale pointers, and never opens during a build pass or a test.
- Every song is a list of labeled parts, each with ≥1 slide; Verse 1/Verse 2/Chorus are separate parts; a long verse is one part with multiple slides.
- The editor edits title, author, key, per-song size (24–72, default 40), part labels, slide text, adds/removes parts and slides, creates songs, restores samples, and saves every change to `library.json`; a restart shows the edited library.
- "Add song to run of show" adds every non-blank slide in part order with the part label and the snapshot size; "Add part to show" adds one part; the stage is never moved by the editor.
- Editor preview, stage window and NDI feed draw the same slide through `slideBody`; at equal aspect the composition is identical and all sizes scale by `width/1280`.
- Blank slides never reach the deck; editing a song never changes slides already on air.
- All gates green, no literal colors outside `theme.go`, docs updated.

## Assumptions and Out of Scope

- "Multiple windows" means console + stage + song editor. No second output/preview monitor, no detachable panels.
- The run of show stays session-only; no undo/redo; no images, video, themes, chord charts or arrangements.
- The editor is a single window editing one song at a time; the library is the file, and deleting a song rewrites it (Restore samples brings back built-ins by ID).
- Scripture editing is out of scope; verses keep their current flat data and deck behavior.
- Text is not auto-fitted: the preview shows overflow exactly as the output will, which is the deliberate simplicity tradeoff.
