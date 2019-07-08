package main

import (
	"fmt"
	"time"
)

func ExampleTGFConfig_RunWithUpdateCheck_forcing_update_cli() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: true,
			AutoUpdate:    false,
			DebugMode:     true,
		},
	}

	ErrPrintf = PrintMock
	cfg.RunWithUpdateCheck(lastRefresh1Hour)
	// Output:
	// Auto update is force disabled. Bypassing update version check.
}

func ExampleTGFConfig_RunWithUpdateCheck_no_update_config() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: false,
			DebugMode:     true,
		},
		AutoUpdate: false,
	}

	ErrPrintf = PrintMock
	cfg.RunWithUpdateCheck(lastRefresh1Hour)
	// Output:
	// Auto update is disabled in the config. Bypassing update version check.
}

func ExampleTGFConfig_RunWithUpdateCheck_forcing_update_config() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: false,
			DebugMode:     true,
		},
		AutoUpdate:      true,
		AutoUpdateDelay: 2 * time.Hour,
	}

	ErrPrintf = fmt.Printf
	cfg.RunWithUpdateCheck(lastRefresh1Hour)
	// Output:
	// Less than 2h0m0s since last check. Bypassing update version check.
}

func lastRefresh1Hour(image string) time.Duration {
	return 1 * time.Hour
}
