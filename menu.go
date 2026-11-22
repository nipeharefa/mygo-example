package main

import (
	"runtime"

	"github.com/egoist/mygo"
)

// buildMenu is the application menu. macOS gets the standard app, edit, help
// and window menus around it; Windows and Linux get a File menu with Quit,
// because a custom menu replaces the platform default.
func buildMenu(a *app) *mygo.Menu {
	settings := &mygo.MenuItem{
		ID:          "settings",
		Label:       "Settings…",
		Accelerator: "CmdOrCtrl+,",
		Click:       func(*mygo.MenuItem, *mygo.Window) { a.showSettings() },
	}
	help := &mygo.MenuItem{
		ID:    "help",
		Label: "mygo-demo Help",
		Click: func(*mygo.MenuItem, *mygo.Window) { a.openHelp() },
	}
	updates := &mygo.MenuItem{
		ID:    "check-updates",
		Label: "Check for Updates…",
		Click: func(*mygo.MenuItem, *mygo.Window) { a.checkForUpdates() },
	}
	about := &mygo.MenuItem{
		ID:    "about",
		Label: "About mygo-demo",
		Click: func(*mygo.MenuItem, *mygo.Window) { a.showAbout() },
	}

	var items []*mygo.MenuItem
	if runtime.GOOS == "darwin" {
		items = append(items, &mygo.MenuItem{Role: mygo.RoleAppMenu})
	} else {
		items = append(items, &mygo.MenuItem{
			Label:   "File",
			Submenu: []*mygo.MenuItem{{Role: mygo.RoleQuit}},
		})
	}

	items = append(items,
		&mygo.MenuItem{
			Label: "Edit",
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleUndo},
				{Role: mygo.RoleRedo},
				mygo.Separator(),
				{Role: mygo.RoleCut},
				{Role: mygo.RoleCopy},
				{Role: mygo.RolePaste},
				{Role: mygo.RoleSelectAll},
			},
		},
		&mygo.MenuItem{
			Label:   "Settings",
			Submenu: []*mygo.MenuItem{settings},
		},
		&mygo.MenuItem{
			Role:    mygo.RoleHelp,
			Submenu: []*mygo.MenuItem{help, mygo.Separator(), updates, about},
		},
	)
	if runtime.GOOS == "darwin" {
		items = append(items, &mygo.MenuItem{Role: mygo.RoleWindowMenu})
	}
	return mygo.NewMenu(items)
}

// showAbout uses the native About panel on macOS, where one exists, and the
// Help window elsewhere.
func (a *app) showAbout() {
	if runtime.GOOS == "darwin" {
		mygo.App.ShowAboutPanel(mygo.AboutPanelOptions{
			ApplicationName: appTitle,
			Version:         a.version,
			Credits:         "Worship presenter demo. Public-domain hymns and King James verses.",
		})
		return
	}
	a.openHelp()
}
