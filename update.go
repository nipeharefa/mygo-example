package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// updateTimeout bounds one check against the release feed, so a network that
// never answers cannot leave the check pending for the rest of the session.
const updateTimeout = 20 * time.Second

// updatableOverride lets a test build say it can update itself: a plain
// binary outside an app bundle never can, so Updater.Enabled answers false
// there and the dialog path would stay out of reach.
var updatableOverride bool

// checkResult is what one look at the release feed found. It travels from the
// goroutine that checked to the main thread that tells the operator, and the
// view never reads it.
type checkResult struct {
	version string // the newer version's number, "" when there is none
	notes   string // that version's release notes, which may be empty
	err     error
}

// lookForUpdate asks the release feed whether a newer version is published.
// It is called off the main thread and takes nothing from the app but the
// running version, so it never touches the window.
func lookForUpdate(ctx context.Context, check func(context.Context) (*mygo.Update, error), current string) checkResult {
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	up, err := check(ctx)
	if err != nil {
		return checkResult{err: err}
	}
	if up == nil || !newerVersion(up.Version, current) {
		// The updater also answers with the version it runs when that version
		// is the feed's, which is not an update.
		return checkResult{}
	}
	return checkResult{version: up.Version, notes: strings.TrimSpace(up.Notes)}
}

// checkForUpdates is the Help menu item. The whole step runs on the main
// thread; only the request itself is handed to a goroutine. A dialog waits
// for the operator by pumping the main loop, and the loop runs what it is
// handed one function at a time: a post made from inside a callback waits
// behind that callback, which is what made the menu item hang while the
// check ran underneath it.
func (a *app) checkForUpdates() {
	current := a.versionString()
	if current == "" {
		a.sayUpdates("Check for Updates", "This development build has no version, so there is nothing for the feed to compare.")
		return
	}
	if !a.updatable {
		a.sayUpdates("Check for Updates", "Only the installers from a release can update themselves. A development build, or an install by a package manager, is updated another way.")
		return
	}
	if a.updateChk == nil || a.updateAsk == nil {
		return // a test that never stands the feed in for this
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	check := a.updateChk
	// The result arrives on this channel, one send at a time: the ask that
	// follows cannot want two.
	answer := make(chan checkResult, 1)
	go func() { answer <- lookForUpdate(ctx, check, current) }()
	a.showCheck(ctx, current, <-answer)
}

// showCheck reports what the check found and, when there is a new version,
// asks whether to install it. Main thread only.
func (a *app) showCheck(ctx context.Context, current string, res checkResult) {
	switch {
	case ctx.Err() != nil:
		// Quitting: there is no interface left to ask.
		return
	case res.err != nil:
		a.updateAsk(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageError,
			Title:   "Check for Updates",
			Message: "The update feed could not be read.",
			Detail:  res.err.Error(),
			Buttons: []string{"OK"},
		})
		return
	case res.version == "":
		a.sayUpdates("Check for Updates", appTitle+" "+current+" is the newest version.")
		return
	}
	opts := mygo.MessageOptions{
		Parent:        a.win,
		Type:          mygo.MessageInfo,
		Title:         "Check for Updates",
		Message:       "Version " + res.version + " is available.",
		Buttons:       []string{"Install and Restart", "Not Now"},
		DefaultButton: 0,
		CancelButton:  1,
	}
	if res.notes != "" {
		opts.Detail = res.notes
	}
	res2, err := a.updateAsk(opts)
	if err != nil || res2.Button != 0 {
		return
	}
	a.installUpdate(ctx, res)
}

// installUpdate downloads the version the check offered and starts the app
// again on it. Like the check, it waits for the transfer here rather than
// posting back: the loop would only run the post after this callback
// returned, and nothing would return it.
func (a *app) installUpdate(ctx context.Context, res checkResult) {
	up := &mygo.Update{Version: res.version, Notes: res.notes}
	if a.updateDl == nil {
		return
	}
	answer := make(chan error, 1)
	go func() { answer <- a.updateDl(ctx, up, nil) }()
	err := <-answer
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		a.updateAsk(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageError,
			Title:   "Update not installed",
			Message: "This version keeps running.",
			Detail:  err.Error(),
			Buttons: []string{"OK"},
		})
		return
	}
	if a.updateRun != nil {
		a.updateRun()
	}
}

// sayUpdates shows one note with a single button, which is all a check that
// found nothing has to ask.
func (a *app) sayUpdates(title, message string) {
	if a.updateAsk == nil {
		return
	}
	_, _ = a.updateAsk(mygo.MessageOptions{
		Parent:  a.win,
		Type:    mygo.MessageInfo,
		Title:   title,
		Message: message,
		Buttons: []string{"OK"},
	})
}

// versionString is the version the menu reports: the packaged one once the
// app is ready, or "" for a build made by go run and go build, which carries
// none. A test sets version and stays headless.
func (a *app) versionString() string {
	return a.version
}

// newerVersion reports whether a is a later version than b, comparing the
// numbers as semantic versioning orders them and ignoring a build suffix.
// An unreadable pair is not an update, which keeps the check quiet rather
// than offering the version that is already running.
func newerVersion(a, b string) bool {
	pa, ok := parseVersion(a)
	if !ok {
		return false
	}
	pb, ok := parseVersion(b)
	if !ok {
		return false
	}
	return compareVersion(pa, pb) > 0
}

// versionParts is the numeric part of a version, as 1.2.3 gives {1, 2, 3}.
type versionParts [3]int

// parseVersion reads "1.2.3", "v1.2.3" and "1.2" into numbers. A prerelease
// suffix, as in 1.3.0-beta.1, is dropped for the comparison: the check only
// offers versions the feed publishes as stable.
func parseVersion(s string) (versionParts, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if s == "" {
		return versionParts{}, false
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	var p versionParts
	parts := strings.Split(s, ".")
	if len(parts) > len(p) {
		return versionParts{}, false
	}
	for i, part := range parts {
		n := 0
		if part == "" {
			return versionParts{}, false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return versionParts{}, false
			}
			n = n*10 + int(c-'0')
			if n > 1<<20 {
				return versionParts{}, false
			}
		}
		p[i] = n
	}
	return p, true
}

func compareVersion(a, b versionParts) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] > b[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}
