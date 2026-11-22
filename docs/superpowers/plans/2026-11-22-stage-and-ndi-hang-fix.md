# The stage window, NDI and the "not responding" hang

## Report

With **Settings → Stage output** on **Windowed on this screen**, pressing
**Open output** froze the app: the console stopped answering and macOS
showed it as not responding. Publishing NDI first made it worse — the
request that opened the stage never returned at all.

## Root causes

1. **A window was created inside the click that asked for it.** `settle()`
   ran `openOutput` → `mygo.NewWindow`, which shows the window from inside
   the immediate context of the caller, and then `SetPosition` /
   `SetFullScreen(false)` waited on a window the backend had not drawn yet.
   The main thread parked there with the event still on its stack.
2. **The NDI frame loop posted a draw into the pass that posted it.**
   `startNDISender` pulled the first frame (a preflight) and the loop drew
   through `mygo.RunOnMain`. When `placeStage` started NDI from inside a
   view pass, that render waited for the very pass running it. `RunOnMain`
   blocks its caller on the main thread, so the app nested into itself and
   hung; a `kill -QUIT` dump showed `stageSource.frame` →
   `startNDISender` → `placeStage` → `settle` → `app.view` →
   `engine.runFrame`.
3. **A synchronous sender took frames on the calling thread**, which is the
   main loop, so every send waited for the consumer there.

## What changed

- `requestStage` → `settle` → `a.openStage`; `view` asks for another pass
  (`c.After(openStageDelay)`) and `openOutput` creates the window in that
  pass, in no event's context. `placeStage` is idempotent and only calls
  `SetFullScreen(false)` when the window reports fullscreen.
- The sender is asynchronous (`clockVideo` 0): sending runs on the NDI
  thread. The app is the `ndiFrameSource`; `(*app) frame` draws inline
  headless and otherwise posts `drawNdiFrame`, which renders on the main
  thread, calls `win.Invalidate()`, and lets the picture go out with the
  next pass — a draw that would have to wait for that pass is skipped
  instead of blocking. No preflight frame; `ndiStream.draw` runs the source
  on a helper goroutine and selects on the stop channel, so Stop still never
  waits for a draw.
- `automation_e2e.go` adds the MyGo automation plugin under the `e2e` tag,
  which is how the fix was verified: built app, `MYGO_AUTOMATION` set,
  driven over the socket.

## Verification

Built with `-tags e2e`, `MYGO_AUTOMATION=/tmp/app.sock MYGO_NDI=1`, driven
by a script that signs in, publishes, opens and closes the stage twice,
switches 720p/1080p, blacks out, and stops and restarts NDI. Every request
answered in under 5 ms; frames went out at 115–120 fps at 720p, 54 fps at
1080p, and closing the stage kept publishing.
