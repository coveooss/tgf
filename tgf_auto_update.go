package main

import (
	"runtime"
)

// Update runs the update on the current tgf executable for unix systems
func Update(app *TGFApplication) bool {
	currentOs := runtime.GOOS

	if currentOs == "linux" || currentOs == "darwin" && RunUpdateUnix() {
		app.Debug("tgf updated !")
		ForwardCommandUnix()
		return true
	}

	app.Debug("Auto update is only implemented on unix systems")

	return false
}
