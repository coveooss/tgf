package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// RunUpdate runs the update on the current tgf executable on windows
func RunUpdate() bool {
	app.Debug("Auto update not implemented on windows.")
	return false
}

// ReRun calls tgf with the provided arguments on windows
func ReRun() {
	cmd := exec.Command("cmd", "/C", os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()
}
