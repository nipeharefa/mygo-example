package main

import "github.com/egoist/mygo/ui"

// view draws the login card or the console. It is a pure function of app:
// the same state always produces the same interface, and nothing built here
// survives the pass.
func (a *app) view(c *ui.Context) {
	c.SetTheme(consoleTheme(c))
	if !a.signedIn {
		a.login(c)
		return
	}
	a.shell(c)
}

// login is the demo gate: one card, two fields, one account.
func (a *app) login(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Column(c).Width(380).Padding(30, 28).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(9, 9).Radius(999).Background(t.Accent)
				ui.Text(c, appTitle).FontSize(12).FontWeight(600).TextColor(t.TextMuted).SingleLine()
			})
			ui.Text(c, "Sign in to lead the service.").FontSize(22).Bold()
			ui.Text(c, "Your set list and the live output stay on this machine.").FontSize(11).TextColor(t.TextMuted)

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Username").FontSize(11).TextColor(t.TextMuted).SingleLine()
				ui.TextInput(c, &a.username).Label("Username input").Placeholder(demoUsername).
					OnChange(a.clearAuthError).OnSubmit(a.signIn)
			})
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Password").FontSize(11).TextColor(t.TextMuted).SingleLine()
				ui.TextInput(c, &a.password).Label("Password input").Password().Placeholder("Password").
					OnChange(a.clearAuthError).OnSubmit(a.signIn)
			})

			if a.authErr != "" {
				ui.Text(c, a.authErr).FontSize(11).TextColor(t.Danger).SingleLine()
			}
			ui.PrimaryButton(c, "Sign in").OnClick(a.signIn)
			ui.Text(c, "Demo account: "+demoUsername+" / "+demoPassword).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
	})
}

// shell is the console frame. The panels are built in console.go.
func (a *app) shell(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(18).Gap(12).Children(func() {
		ui.Text(c, "Run of show").FontSize(20).Bold()
	})
}
