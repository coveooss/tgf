package main

import (
	"time"
)

var updateInterval = 2 * time.Hour

// Update runs the update on the current tgf executable
func Update(app *TGFApplication) bool {
	if lastRefresh("update") < updateInterval {
		return false
	}

	touchImageRefresh("update")

	if RunUpdate() {
		app.Debug("tgf updated !")
		ForwardCommand()
		return true
	}

	return false
}
