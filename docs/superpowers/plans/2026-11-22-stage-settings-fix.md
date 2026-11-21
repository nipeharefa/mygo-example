# Remove Automatic, keep the stage open, and reorder the run of show (0.2.0-beta.4)

Status: implemented in `7cd53a9` and `e6fe3e5`, released as `v0.2.0-beta.4`.

## The report

After `v0.2.0-beta.3` the operator reported that starting the output looped
by itself — press **Open output**, get one stage, and then another and
another. They asked for the Settings controls to move to their own page, for
the "Automatic" display choice to go away because it crashed, for the run of
show to be reorderable, and for Settings to show something about the NDI
stream.

## Why it looped

1. `closeOutput` cleared `a.live` **before** calling `live.Close()`. The
   window's `OnClosed` callback exists to clear `a.live`, guarded with
   `if a.live != live { return }`; because the field was already nil, the
   guard returned early and MyGo never reached `w.destroy()`. The window was
   not destroyed, and it never saved its state either.
2. The display-replace path closed and immediately reopened the stage.
   Destroying a window persists its fullscreen state under its `StateKey`;
   the recreated window came back fullscreen, and entering fullscreen posts
   a display change from inside `openOutput`, whose handler reopened the
   stage again. With NDI on, the closed window also broke the capture mid
   reopen, which added another cycle.
3. "Automatic" plus fullscreen-on-open was exactly that path, so removing
   the option removed the crash-prone configuration as well.

## What changed

- **No Automatic.** `settings.Display` (0 = windowed) replaces
  `settings.OutputDisplay`; a file that still has `outputDisplay` is read
  through a legacy struct and rewritten under the new key on the next save.
  Settings offers **Windowed on this screen** and one radio per attached
  display.
- **Stage lifecycle.** `shutStage` clears `a.live` and stops NDI; `OnClosed`
  calls it through the identity guard. `placeStage` positions the window on
  the chosen display from its work area and re-asserts that on enter
  fullscreen. `refreshDisplays` only closes and reopens the stage when the
  display the stage was actually placed on (`stageDisplayUsed`) vanished;
  any other change is just a redraw.
- **The console asks for the stage.** The **Open output** button sets
  `openStage`, and `settle()` opens the window after the frame is built, so
  the view never constructs a window. `a.newWindow` and `a.mainScreen` are
  seams over `mygo.NewWindow` and `mygo.Screen.Displays`, so the logic stays
  testable without an application.
- **Settings is a page.** `a.page` switches the console between the run of
  show and `settingsBody`; the settings page has its own way back
  (**Back to console**, Cmd/Ctrl+, again, Cmd/Ctrl+Esc), and the deck
  shortcuts are not registered while it is open.
- **Reorder.** `ui.List.Reorder` calls `reorderSlides(rows, to)`; `to` is
  the gap the rows land in, counted in the deck **without** the moved rows.
  The slide on air keeps its ID and stays on air at its new row; the monitor
  follows the moved rows. `moveSlide` goes through the same function.
- **NDI stats.** The sender counts frames sent, frames skipped when the
  capture missed, the last frame size, and keeps the last error;
  `ndiStream.snapshot()` reports resolution, measured frame rate (from the
  time between the first and last frame), up time and the error. The
  Settings page shows them under **NDI stream** and refreshes through a
  one-second main-thread timer while it is open and publishing. NDI 5 and 6
  have no receiver tally, so this is the sender's view of its own stream.

## Lesson worth keeping

`reorderSlides` was first written with `to` as a row index and "subtracted
the moved rows above it"; the list and `moveSlide` both pass a gap, so a
one-row move collapsed to a no-op. Print-based debugging of a method while
its source kept changing sent me down the wrong path twice: the numbers and
the source disagreed because they were from different builds. Rewriting the
function without shared counters and probing the exact call from a fresh
test file settled it in one run. When a trace contradicts the source, stop
reading traces and re-derive the function.
