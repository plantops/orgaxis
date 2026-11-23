//go:build !windows

package main

import "fmt"

func runWindowsTray(baseURL, iconFile string, onExit func(), mode string) error {
	return fmt.Errorf("tray is Windows-only")
}

func showStartupError(detail string) {}
