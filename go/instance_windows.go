//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var singletonKernel = syscall.NewLazyDLL("kernel32.dll")
var singletonCreate = singletonKernel.NewProc("CreateMutexW")
var singletonClose = singletonKernel.NewProc("CloseHandle")

const winAlreadyExists syscall.Errno = 183

// Named kernel object works across separate Windows sessions, unlike a process-local mutex.
func acquireInstance(data string) (func(), error) {
	abs, e := filepath.Abs(data)
	if e != nil {
		return nil, e
	}
	fingerprint := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	name := "Global\\OrgAxis_DB_" + hex.EncodeToString(fingerprint[:12])
	p, e := syscall.UTF16PtrFromString(name)
	if e != nil {
		return nil, e
	}
	h, _, lastErr := singletonCreate.Call(0, 0, uintptr(unsafe.Pointer(p)))
	if h == 0 {
		return nil, fmt.Errorf("CreateMutexW failed: %v", lastErr)
	}
	if lastErr == winAlreadyExists {
		singletonClose.Call(h)
		return nil, fmt.Errorf("OrgAxis is already running with database folder %s", abs)
	}
	return func() { singletonClose.Call(h) }, nil
}

// One tray UI per user session. Unlike the database mutex this is session-local.
func acquireTrayInstance() (func(), error) {
	name := trayUTF16("Local\\OrgAxis_UI_2026")
	h, _, e := singletonCreate.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, fmt.Errorf("CreateMutexW tray: %v", e)
	}
	if e == winAlreadyExists {
		singletonClose.Call(h)
		return nil, fmt.Errorf("OrgAxis tray is already running in this session")
	}
	return func() { singletonClose.Call(h) }, nil
}

var advapi = syscall.NewLazyDLL("advapi32.dll")
var regCreate = advapi.NewProc("RegCreateKeyExW")
var regSet = advapi.NewProc("RegSetValueExW")
var regQuery = advapi.NewProc("RegQueryValueExW")
var regDelete = advapi.NewProc("RegDeleteValueW")
var regClose = advapi.NewProc("RegCloseKey")

const hkeyCurrentUser uintptr = 0x80000001
const keyReadSet = 0x20019 | 0x0002
const regSz = 1

func openStartupKey() (uintptr, error) {
	var key uintptr
	path := trayUTF16(`Software\Microsoft\Windows\CurrentVersion\Run`)
	rc, _, _ := regCreate.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(path)), 0, 0, 0, keyReadSet, 0, uintptr(unsafe.Pointer(&key)), 0)
	if rc != 0 {
		return 0, fmt.Errorf("RegCreateKeyExW: code %d", rc)
	}
	return key, nil
}
func startupEnabled() (bool, error) {
	key, e := openStartupKey()
	if e != nil {
		return false, e
	}
	defer regClose.Call(key)
	name := trayUTF16("OrgAxis")
	var typ uint32
	var size uint32
	rc, _, _ := regQuery.Call(key, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	if rc == 2 {
		return false, nil
	}
	if rc != 0 {
		return false, fmt.Errorf("RegQueryValueExW: code %d", rc)
	}
	return size > 2, nil
}
func setStartup(on bool, mode string) error {
	key, e := openStartupKey()
	if e != nil {
		return e
	}
	defer regClose.Call(key)
	name := trayUTF16("OrgAxis")
	if !on {
		rc, _, _ := regDelete.Call(key, uintptr(unsafe.Pointer(name)))
		if rc == 0 || rc == 2 {
			return nil
		}
		return fmt.Errorf("RegDeleteValueW: code %d", rc)
	}
	if mode != "client" {
		mode = "desktop"
	}
	command := startupModeCommand(mode)
	if command == "" {
		return fmt.Errorf("cannot resolve EXE path")
	}
	data, e := syscall.UTF16FromString(command)
	if e != nil {
		return e
	}
	rc, _, _ := regSet.Call(key, uintptr(unsafe.Pointer(name)), 0, regSz, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if rc != 0 {
		return fmt.Errorf("RegSetValueExW: code %d", rc)
	}
	return nil
}

const svcWinOwnProcess = 0x10
const svcWinStartPending = 2
const svcWinStopPending = 3
const svcWinRunning = 4
const svcWinStopped = 1
const svcAcceptStop = 1
const svcAcceptShutdown = 4
const svcControlStop = 1
const svcControlShutdown = 5

type svcTableEntry struct {
	Name *uint16
	Proc uintptr
}
type svcStatus struct {
	Type         uint32
	State        uint32
	Accepted     uint32
	Win32Exit    uint32
	SpecificExit uint32
	Checkpoint   uint32
	WaitHint     uint32
}

var svcDispatcher = advapi.NewProc("StartServiceCtrlDispatcherW")
var svcRegister = advapi.NewProc("RegisterServiceCtrlHandlerExW")
var svcSetStatus = advapi.NewProc("SetServiceStatus")

// SCM service entry: handles STOP/SHUTDOWN and reports lifecycle to Service Control Manager.
func runWindowsService(start func(context.Context, func()) error) error {
	var runErr error
	serviceMain := syscall.NewCallback(func(_argc, _argv uintptr) uintptr {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler := syscall.NewCallback(func(control, eventType, eventData, contextArg uintptr) uintptr {
			if control == svcControlStop || control == svcControlShutdown {
				cancel()
			}
			return 0
		})
		name := trayUTF16("OrgAxis")
		sh, _, err := svcRegister.Call(uintptr(unsafe.Pointer(name)), handler, 0)
		if sh == 0 {
			runErr = fmt.Errorf("RegisterServiceCtrlHandlerExW: %v", err)
			return 0
		}
		set := func(state, accepted, checkpoint, hint uint32) {
			status := svcStatus{Type: svcWinOwnProcess, State: state, Accepted: accepted, Checkpoint: checkpoint, WaitHint: hint}
			svcSetStatus.Call(sh, uintptr(unsafe.Pointer(&status)))
		}
		set(svcWinStartPending, 0, 1, 30000)
		ready := func() { set(svcWinRunning, svcAcceptStop|svcAcceptShutdown, 0, 0) }
		runErr = start(ctx, ready)
		set(svcWinStopPending, 0, 1, 8000)
		set(svcWinStopped, 0, 0, 0)
		return 0
	})
	entries := [2]svcTableEntry{{Name: trayUTF16("OrgAxis"), Proc: serviceMain}, {}}
	rc, _, err := svcDispatcher.Call(uintptr(unsafe.Pointer(&entries[0])))
	if rc == 0 {
		return fmt.Errorf("StartServiceCtrlDispatcherW: %v (service mode must be started by Windows SCM)", err)
	}
	return runErr
}
