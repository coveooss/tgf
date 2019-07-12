package main

import (
	"fmt"
	"time"
)

func ExampleRunWithUpdateCheck_forcing_no_update_cli() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: true,
			AutoUpdate:    false,
			DebugMode:     true,
		},
		NoRun: true,
	}

	ErrPrintf = fmt.Printf
	version = "1.20.0"
	RunWithUpdateCheck(cfg)
	// Output:
	// Auto update is force disabled. Bypassing update version check.
}

func ExampleRunWithUpdateCheck_forcing_update_cli_up_to_date() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: true,
			AutoUpdate:    true,
			DebugMode:     true,
		},
		NoRun: true,
	}

	version = "1.0.0"
	ErrPrintf = fmt.Printf
	RunWithUpdateCheck(cfg)
	// Output:
	// Auto update is forced. Checking version...
	// Comparing local and latest versions...
}

func ExampleRunWithUpdateCheck_no_update_config() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: false,
			DebugMode:     true,
		},
		AutoUpdate: false,
		NoRun:      true,
	}

	version = "1.20.0"
	ErrPrintf = fmt.Printf
	RunWithUpdateCheck(cfg)
	// Output:
	// Auto update is disabled in the config. Bypassing update version check.
}

func ExampleRunWithUpdateCheck_forcing_update_config() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: false,
			DebugMode:     true,
		},
		AutoUpdate:      true,
		AutoUpdateDelay: 2 * time.Hour,
		NoRun:           true,
	}

	version = "1.20.0"
	ErrPrintf = fmt.Printf
	RunWithUpdateCheck(cfg)
	// Output:
	// Less than 2h0m0s since last check. Bypassing update version check.
}
