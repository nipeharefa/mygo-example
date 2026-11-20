package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// signIn opens the dashboard through the login card, failing the test when
// the demo account is rejected.
func signIn(t *testing.T, a *app, tt *ui.Tester) {
	t.Helper()
	a.username, a.password = demoUsername, demoPassword
	if err := tt.Click("Sign in"); err != nil {
		t.Fatal(err)
	}
	if !a.signedIn {
		t.Fatalf("not signed in after the demo account: %q", tt.Texts())
	}
}

// A fresh window is the login card, not the dashboard.
func TestLoginGate(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)

	if !tt.HasText("Sign in") {
		t.Fatalf("the login card did not show: %q", tt.Texts())
	}
	if tt.HasText("Pods") || tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the dashboard showed before signing in: %q", tt.Texts())
	}
	if a.signedIn {
		t.Fatal("a fresh app is signed in")
	}
}

// Typing the wrong password keeps the card and shows no dashboard; editing
// a field clears the error.
func TestLoginRejectsWrongPassword(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)

	if err := tt.Click("Username input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoUsername)
	if err := tt.Click("Password input"); err != nil {
		t.Fatal(err)
	}
	tt.Type("nope")
	if err := tt.Click("Sign in"); err != nil {
		t.Fatal(err)
	}
	if a.signedIn {
		t.Fatal("signed in with the wrong password")
	}
	if !tt.HasText("Wrong username or password") {
		t.Fatalf("no error for the wrong password: %q", tt.Texts())
	}
	if tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the dashboard showed after a wrong password: %q", tt.Texts())
	}

	// Typing clears the error, but the card stays.
	if err := tt.Click("Password input"); err != nil {
		t.Fatal(err)
	}
	tt.Type("x")
	if tt.HasText("Wrong username or password") {
		t.Fatalf("the error outlived the edit: %q", tt.Texts())
	}
	if a.signedIn {
		t.Fatal("signed in while correcting the password")
	}
}

// The demo account opens the dashboard and drops the password.
func TestLoginAcceptsDemoAccount(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)

	if err := tt.Click("Username input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoUsername)
	if err := tt.Click("Password input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoPassword)
	if err := tt.Click("Sign in"); err != nil {
		t.Fatal(err)
	}

	if !a.signedIn {
		t.Fatalf("not signed in with the demo account: %q", tt.Texts())
	}
	if a.password != "" {
		t.Errorf("password kept after signing in: %q", a.password)
	}
	if !tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the dashboard did not show: %q", tt.Texts())
	}
}

// Enter submits from the password field, after Tab from the username field.
func TestEnterSubmitsLogin(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)

	if err := tt.Click("Username input"); err != nil {
		t.Fatal(err)
	}
	tt.Type(demoUsername)
	tt.Key(0, ui.KeyTab)
	tt.Type(demoPassword)
	tt.Key(0, ui.KeyEnter)

	if !a.signedIn {
		t.Fatalf("Enter did not submit the login: %q", tt.Texts())
	}
	if !tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the dashboard did not show after Enter: %q", tt.Texts())
	}
}

// Signing out returns to the card, keeps the username, and clears the rest.
func TestSignOut(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)
	signIn(t, a, tt)

	if err := tt.Click("Sign out"); err != nil {
		t.Fatal(err)
	}
	if a.signedIn {
		t.Fatal("still signed in after Sign out")
	}
	if a.password != "" || a.authErr != "" {
		t.Fatalf("sign out kept password %q or error %q", a.password, a.authErr)
	}
	if !tt.HasText("Sign in") {
		t.Fatalf("the login card did not return: %q", tt.Texts())
	}
	if tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the dashboard outlived sign out: %q", tt.Texts())
	}
}

// Starting NDI without a window, as in a headless test, reports the
// problem instead of streaming, and the button stays on Start.
func TestNDIToggleWithoutWindow(t *testing.T) {
	if !ndiSupported() {
		t.Skip("this build has no NDI sender")
	}
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)
	signIn(t, a, tt)

	if !tt.HasText("Start NDI") {
		t.Fatalf("the header does not offer NDI: %q", tt.Texts())
	}
	if err := tt.Click("Start NDI"); err != nil {
		t.Fatal(err)
	}
	if a.ndi != nil {
		t.Fatal("a sender started without a window")
	}
	if a.ndiErr == "" {
		t.Fatal("a failed start left no error")
	}
	if !tt.HasText("NDI: " + a.ndiErr) {
		t.Fatalf("the header strip did not show the error: %q", tt.Texts())
	}
	if !tt.HasText("Start NDI") {
		t.Fatalf("the button left its start state: %q", tt.Texts())
	}
}

// Stopping with nothing streaming is a no-op, so quitting and signing out
// can call it unconditionally.
func TestStopNDIWithoutSender(t *testing.T) {
	a := newApp()
	a.stopNDI()
	if a.ndi != nil || a.ndiErr != "" {
		t.Fatalf("stop changed state: %v %q", a.ndi, a.ndiErr)
	}
}

// A build without a sender offers no button to press and treats a start
// request as nothing to do, so MYGO_NDI=1 stays quiet away from macOS.
func TestNDIHiddenWithoutSender(t *testing.T) {
	if ndiSupported() {
		t.Skip("this build can publish")
	}
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)
	signIn(t, a, tt)

	if tt.HasText("Start NDI") {
		t.Fatalf("the header offered a toggle this build cannot honour: %q", tt.Texts())
	}
	if err := a.startNDI(); err != nil || a.ndi != nil || a.ndiErr != "" {
		t.Fatalf("startNDI on a build without a sender: err=%v ndi=%v ndiErr=%q", err, a.ndi, a.ndiErr)
	}
}

// The view runs without a window in tests, which click as a user would.
func TestResourceSwitching(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)
	signIn(t, a, tt)

	// The default kind shows its first object in the table and inspector.
	if !tt.HasText("Pods") || !tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the Pods pane did not show: %q", tt.Texts())
	}

	if err := tt.Click("Deployments"); err != nil {
		t.Fatal(err)
	}
	if a.kind != "deployments" {
		t.Fatalf("kind %q after clicking Deployments", a.kind)
	}
	if !tt.HasText("api-gateway") {
		t.Fatalf("the Deployments rows did not show: %q", tt.Texts())
	}

	// A new kind starts at its first object, whatever row was chosen.
	a.row = 3
	if err := tt.Click("Secrets"); err != nil {
		t.Fatal(err)
	}
	if a.kind != "secrets" {
		t.Fatalf("kind %q after clicking Secrets", a.kind)
	}
	if a.row != 0 {
		t.Errorf("row %d after switching kinds, want 0", a.row)
	}
	if !tt.HasText("regcred") || !tt.HasText("2 keys") {
		t.Fatalf("the Secrets rows did not show: %q", tt.Texts())
	}
}
