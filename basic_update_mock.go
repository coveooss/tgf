package main

import (
	"fmt"
	"time"
)

type AutoUpdateRunner struct {
	tgf *TGFApplication
}

func (runner *AutoUpdateRunner) Debug(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (runner *AutoUpdateRunner) GetUpdateVersion() (string, error) {
	return "", nil
}

func (runner *AutoUpdateRunner) GetLastRefresh(file string) time.Duration {
	return 0 * time.Hour
}

func (runner *AutoUpdateRunner) SetLastRefresh(file string) {}

func (runner *AutoUpdateRunner) ShouldUpdate() bool {
	return false
}

func (runner *AutoUpdateRunner) Run() int {
	return 1
}

func (runner *AutoUpdateRunner) Restart() int {
	return 0
}
