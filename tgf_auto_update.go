package main

import (
	"runtime"
)

// Update runs the update on the current tgf executable for unix systems
func Update() bool {
	currentOs := runtime.GOOS

	if currentOs == "linux" || currentOs == "darwin" && RunUpdateUnix() {
		Println("tgf updated !")
		ForwardCommandUnix()
		return true
	}

	Println("Auto update is only implemented on unix systems")

	return false
}
