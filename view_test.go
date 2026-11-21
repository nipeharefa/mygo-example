package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// signIn opens the console through the login card, failing the test when the
// demo account is rejected.
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

// A fresh window is the login card, not the console.
func TestLoginGate(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

	if !tt.HasText("Sign in") {
		t.Fatalf("the login card did not show: %q", tt.Texts())
	}
	if tt.HasText("Run of show") {
		t.Fatalf("the console showed before signing in: %q", tt.Texts())
	}
	if a.signedIn {
		t.Fatal("a fresh app is signed in")
	}
}

// Typing the wrong password keeps the card and shows no console; editing a
// field clears the error.
func TestLoginRejectsWrongPassword(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

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

// The demo account opens the console and drops the password.
func TestLoginAcceptsDemoAccount(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

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
	if !tt.HasText("Run of show") {
		t.Fatalf("the console did not show: %q", tt.Texts())
	}
}

// Enter submits from the password field, after Tab from the username field.
func TestEnterSubmitsLogin(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)

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
}

// Signing out returns to the card, drops the password and clears the error.
func TestSignOut(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, consoleWidth, consoleHeight)
	signIn(t, a, tt)

	a.signOut()
	tt.Frame()

	if a.signedIn {
		t.Fatal("still signed in after signing out")
	}
	if a.password != "" || a.authErr != "" {
		t.Fatalf("password = %q authErr = %q after signing out", a.password, a.authErr)
	}
	if !tt.HasText("Sign in") {
		t.Fatalf("the login card did not come back: %q", tt.Texts())
	}
}
