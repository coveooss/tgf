package main

import (
	"runtime"
	"time"
)

var updateInterval = 2 * time.Hour

// Update runs the update on the current tgf executable for unix systems
func Update(app *TGFApplication) bool {
	currentOs := runtime.GOOS

	if lastRefresh("update") < updateInterval {
		return false
	}

	touchImageRefresh("update")

	if currentOs == "linux" || currentOs == "darwin" && RunUpdateUnix() {
		app.Debug("tgf updated !")
		ForwardCommandUnix()
		return true
	}

	app.Debug("Auto update is only implemented on unix systems")

	return false
}
