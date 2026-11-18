//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func acquireInstance(data string) (func(), error) {
	p := filepath.Join(data, ".orgaxis-instance.lock")
	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("OrgAxis already running for this data directory: %w", e)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func runWindowsService(fn func(ctx context.Context, ready func()) error) error {
	return fmt.Errorf("Windows Service unsupported on this OS")
}
func startupEnabled() (bool, error)         { return false, nil }
func setStartup(on bool, mode string) error { return fmt.Errorf("Windows Startup is Windows-only") }
