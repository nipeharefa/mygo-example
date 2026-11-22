package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// fakeFeed stands in for the release feed: it answers the check with one
// version, asks nothing of a window, and records what the app asked of the
// dialogs and the installer.
type fakeFeed struct {
	up    *mygo.Update
	err   error
	calls int
	done  chan struct{} // closed when a check has answered
}

func newFeed(up *mygo.Update, err error) *fakeFeed {
	return &fakeFeed{up: up, err: err, done: make(chan struct{})}
}

func (f *fakeFeed) check(context.Context) (*mygo.Update, error) {
	f.calls++
	select {
	case <-f.done:
	default:
		close(f.done)
	}
	return f.up, f.err
}

// asked is one dialog the app put up, kept so a test can say what it said.
type asked struct {
	opts mygo.MessageOptions
	res  mygo.MessageResult
}

// updaterFor wires an app to a feed that answers up and to dialogs that
// answer every question with button 0, which is "Install and Restart" and
// "OK" alike. The menu item waits for the check on its own thread, as the
// real one does, so the test can read the dialogs once it returns.
func updaterFor(t *testing.T, a *app, feed *fakeFeed) *updaterProbe {
	t.Helper()
	p := &updaterProbe{installedOnce: make(chan struct{}, 1)}
	a.updateChk = feed.check
	a.updatable = true
	a.updateAsk = func(opts mygo.MessageOptions) (mygo.MessageResult, error) {
		res := mygo.MessageResult{Button: p.button}
		p.mu.Lock()
		p.asked = append(p.asked, asked{opts: opts, res: res})
		p.mu.Unlock()
		return res, nil
	}
	a.updateDl = func(ctx context.Context, up *mygo.Update, progress func(downloaded, total int64)) error {
		p.mu.Lock()
		p.installed = up.Version
		p.mu.Unlock()
		return p.installErr
	}
	a.updateRun = func() {
		p.mu.Lock()
		p.restarted = true
		p.mu.Unlock()
		p.installedOnce <- struct{}{}
	}
	return p
}

// updaterProbe records what the app asked of the dialogs and the installer.
// The check answers on a goroutine, so its fields are read under mu.
type updaterProbe struct {
	mu            sync.Mutex
	button        int
	installErr    error
	asked         []asked
	installed     string
	restarted     bool
	installedOnce chan struct{}
}

// dialogs copies what the app asked so far.
func (p *updaterProbe) dialogs() []asked {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]asked(nil), p.asked...)
}

// state copies the install outcome.
func (p *updaterProbe) state() (installed string, restarted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.installed, p.restarted
}

// waitInstalled waits for the install to come back with its verdict.
func (p *updaterProbe) waitInstalled(t *testing.T) {
	t.Helper()
	select {
	case <-p.installedOnce:
	case <-time.After(10 * time.Second):
		t.Fatal("the install never came back to the window")
	}
}

// runCheck runs the menu item and waits for the check to answer. The menu
// item itself returns only once the dialogs and the install are done, so the
// tests read their results after this call, not after feed.done. What the
// feed answers on is what lets this wait: a step that never asks it would
// hang here instead.
func runCheck(t *testing.T, a *app, feed *fakeFeed) {
	t.Helper()
	a.checkForUpdates()
	select {
	case <-feed.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the check never answered")
	}
}

func TestVersionOrder(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"0.2.1", "0.2.0", true},
		{"0.3.0", "0.2.9", true},
		{"1.0.0", "0.9.9", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.0", "0.2.1", false},
		{"v0.2.1", "0.2.0", true},
		{"0.2", "0.1.9", true},
		{"0.2.1-beta.2", "0.2.0", true}, // the suffix does not hide the number
		{"", "0.2.0", false},
		{"0.2.0", "", false},
		{"next", "0.2.0", false},
		{"0.2.0.1", "0.2.0", false},
	} {
		if got := newerVersion(tc.a, tc.b); got != tc.want {
			t.Fatalf("newerVersion(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckForUpdatesSaysWhenThereIsNothing(t *testing.T) {
	a := &app{version: "0.2.0"}
	feed := newFeed(&mygo.Update{Version: "0.2.0"}, nil)
	p := updaterFor(t, a, feed)
	runCheck(t, a, feed)
	got := p.dialogs()
	if len(got) != 1 {
		t.Fatalf("asked %d dialogs, want 1", len(got))
	}
	if got[0].opts.Message != "mygo-demo 0.2.0 is the newest version." {
		t.Fatalf("message = %q", got[0].opts.Message)
	}
	if installed, restarted := p.state(); installed != "" || restarted {
		t.Fatalf("installed %q and restarted %v with nothing new", installed, restarted)
	}
}

func TestCheckForUpdatesInstallsAndRestarts(t *testing.T) {
	a := &app{version: "0.2.0"}
	feed := newFeed(&mygo.Update{Version: "0.3.0", Notes: "A steadier feed."}, nil)
	p := updaterFor(t, a, feed)
	runCheck(t, a, feed)
	if feed.calls != 1 {
		t.Fatalf("checked %d times, want 1", feed.calls)
	}
	asked := p.dialogs()
	if len(asked) != 1 {
		t.Fatalf("asked %d dialogs, want 1", len(asked))
	}
	opts := asked[0].opts
	if opts.Message != "Version 0.3.0 is available." || opts.Detail != "A steadier feed." {
		t.Fatalf("dialog = %q / %q", opts.Message, opts.Detail)
	}
	if len(opts.Buttons) != 2 || opts.Buttons[0] != "Install and Restart" || opts.CancelButton != 1 {
		t.Fatalf("buttons = %v, cancel = %d", opts.Buttons, opts.CancelButton)
	}
	p.waitInstalled(t)
	if installed, _ := p.state(); installed != "0.3.0" {
		t.Fatalf("installed %q, want 0.3.0", installed)
	}
	if _, restarted := p.state(); !restarted {
		t.Fatal("did not restart after installing")
	}
}

func TestCheckForUpdatesDeclinesQuietly(t *testing.T) {
	a := &app{version: "0.2.0"}
	feed := newFeed(&mygo.Update{Version: "0.3.0"}, nil)
	p := updaterFor(t, a, feed)
	p.button = 1 // Not Now
	runCheck(t, a, feed)
	if installed, restarted := p.state(); installed != "" || restarted {
		t.Fatalf("installed %q and restarted %v after Not Now", installed, restarted)
	}
}

func TestCheckForUpdatesReportsAnError(t *testing.T) {
	a := &app{version: "0.2.0"}
	feed := newFeed(nil, errors.New("no route to the feed"))
	p := updaterFor(t, a, feed)
	runCheck(t, a, feed)
	asked := p.dialogs()
	if len(asked) != 1 {
		t.Fatalf("asked %d dialogs, want 1", len(asked))
	}
	if got := asked[0].opts.Detail; got != "no route to the feed" {
		t.Fatalf("detail = %q", got)
	}
	if asked[0].opts.Type != mygo.MessageError {
		t.Fatalf("type = %q, want an error", asked[0].opts.Type)
	}
}

func TestCheckForUpdatesReportsAFailedInstall(t *testing.T) {
	a := &app{version: "0.2.0"}
	feed := newFeed(&mygo.Update{Version: "0.3.0"}, nil)
	p := updaterFor(t, a, feed)
	p.installErr = errors.New("the download stopped")
	runCheck(t, a, feed)
	// The menu item returns after the install, so both dialogs are in: the
	// offer, then the verdict on the download.
	asked := p.dialogs()
	if len(asked) != 2 {
		t.Fatalf("asked %d dialogs, want the offer and the failure", len(asked))
	}
	if asked[1].opts.Title != "Update not installed" || asked[1].opts.Detail != "the download stopped" {
		t.Fatalf("failure dialog = %q / %q", asked[1].opts.Title, asked[1].opts.Detail)
	}
	if _, restarted := p.state(); restarted {
		t.Fatal("restarted after a failed install")
	}
}

func TestCheckForUpdatesWithoutABuildNumber(t *testing.T) {
	a := &app{}
	// Both of these say no before the feed is asked, so there is no check
	// to wait for: the dialog has happened when the call returns.
	feed := newFeed(&mygo.Update{Version: "0.3.0"}, nil)
	p := updaterFor(t, a, feed)
	a.checkForUpdates()
	if asked := p.dialogs(); len(asked) != 1 || asked[0].opts.Message != "This development build has no version, so there is nothing for the feed to compare." {
		t.Fatalf("dialogs = %+v", asked)
	}
	if installed, _ := p.state(); installed != "" {
		t.Fatal("installed an update for an unversioned build")
	}
}

func TestCheckForUpdatesSaysWhenTheBuildCannotUpdate(t *testing.T) {
	a := &app{version: "0.2.0"}
	feed := newFeed(&mygo.Update{Version: "0.3.0"}, nil)
	p := updaterFor(t, a, feed)
	a.updatable = false
	a.checkForUpdates()
	asked := p.dialogs()
	if len(asked) != 1 || asked[0].opts.Title != "Check for Updates" {
		t.Fatalf("dialogs = %+v", asked)
	}
	if want := "Only the installers from a release can update themselves. A development build, or an install by a package manager, is updated another way."; asked[0].opts.Message != want {
		t.Fatalf("message = %q", asked[0].opts.Message)
	}
	if asked[0].opts.Type != mygo.MessageInfo {
		t.Fatalf("type = %q, want info", asked[0].opts.Type)
	}
}
