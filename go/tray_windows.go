//go:build windows

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// Native Windows system tray: no third-party DLL, .NET, PowerShell or console window.
var (
	trayUser32           = syscall.NewLazyDLL("user32.dll")
	trayShell32          = syscall.NewLazyDLL("shell32.dll")
	trayKernel32         = syscall.NewLazyDLL("kernel32.dll")
	trayRegisterClass    = trayUser32.NewProc("RegisterClassW")
	trayCreateWindow     = trayUser32.NewProc("CreateWindowExW")
	trayDefProc          = trayUser32.NewProc("DefWindowProcW")
	trayDestroyWindow    = trayUser32.NewProc("DestroyWindow")
	trayGetMessage       = trayUser32.NewProc("GetMessageW")
	trayTranslateMessage = trayUser32.NewProc("TranslateMessage")
	trayDispatchMessage  = trayUser32.NewProc("DispatchMessageW")
	trayPostQuit         = trayUser32.NewProc("PostQuitMessage")
	trayRegisterMessage  = trayUser32.NewProc("RegisterWindowMessageW")
	trayCreateMenu       = trayUser32.NewProc("CreatePopupMenu")
	trayAppendMenu       = trayUser32.NewProc("AppendMenuW")
	trayDestroyMenu      = trayUser32.NewProc("DestroyMenu")
	trayGetCursorPos     = trayUser32.NewProc("GetCursorPos")
	trayTrackMenu        = trayUser32.NewProc("TrackPopupMenu")
	traySetForeground    = trayUser32.NewProc("SetForegroundWindow")
	trayLoadImage        = trayUser32.NewProc("LoadImageW")
	trayLoadIcon         = trayUser32.NewProc("LoadIconW")
	trayDestroyIcon      = trayUser32.NewProc("DestroyIcon")
	trayNotify           = trayShell32.NewProc("Shell_NotifyIconW")
	trayGetModule        = trayKernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmLDouble       = 0x0203
	wmRButtonUp     = 0x0205
	wmContextMenu   = 0x007B
	wmUser          = 0x0400
	trayMessage     = wmUser + 83
	nimAdd          = 0x00
	nimDelete       = 0x02
	nifMessage      = 0x01
	nifIcon         = 0x02
	nifTip          = 0x04
	imageIcon       = 1
	loadFromFile    = 0x0010
	loadDefaultSize = 0x0040
	mfString        = 0x0
	mfGray          = 0x1
	mfSeparator     = 0x0800
	tpmRightButton  = 0x0002
	tpmReturnCmd    = 0x0100
	menuHome        = 1001
	menuWorkspace   = 1002
	menuExit        = 1003
	menuLogs        = 1004
	menuStartup     = 1005
)

type trayPoint struct {
	X int32
	Y int32
}
type trayMsg struct {
	Hwnd     uintptr
	Message  uint32
	_        uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       trayPoint
	LPrivate uint32
}
type trayWndClass struct {
	Style       uint32
	WndProc     uintptr
	ClassExtra  int32
	WindowExtra int32
	Instance    uintptr
	Icon        uintptr
	Cursor      uintptr
	Background  uintptr
	MenuName    *uint16
	ClassName   *uint16
}
type trayGUID struct {
	A uint32
	B uint16
	C uint16
	D [8]byte
}
type trayNotifyData struct {
	Size            uint32
	Window          uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            trayGUID
	BalloonIcon     uintptr
}

func trayUTF16(s string) *uint16      { p, _ := syscall.UTF16PtrFromString(s); return p }
func trayCopy(dst []uint16, s string) { v, _ := syscall.UTF16FromString(s); copy(dst, v) }

func runWindowsTray(baseURL, iconFile string, onExit func(), mode string) error {
	unlock, e := acquireTrayInstance()
	if e != nil {
		return e
	}
	defer unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	inst, _, _ := trayGetModule.Call(0)
	className := trayUTF16("OrgAxisSystemTrayWindow_2026")
	taskbarMsg, _, _ := trayRegisterMessage.Call(uintptr(unsafe.Pointer(trayUTF16("TaskbarCreated"))))
	var hwnd uintptr
	var hIcon uintptr
	var notice trayNotifyData
	var registered bool
	addIcon := func() bool {
		if hwnd == 0 {
			return false
		}
		notice.Size = uint32(unsafe.Sizeof(notice))
		notice.Window = hwnd
		notice.ID = 1
		notice.CallbackMessage = trayMessage
		notice.Flags = nifMessage | nifIcon | nifTip
		notice.Icon = hIcon
		trayCopy(notice.Tip[:], "OrgAxis - Workforce Workspace (local)")
		ok, _, _ := trayNotify.Call(nimAdd, uintptr(unsafe.Pointer(&notice)))
		registered = ok != 0
		return registered
	}
	showMenu := func() {
		status := "Offline (check data/orgaxis.log)"
		client := http.Client{Timeout: 600 * time.Millisecond}
		if resp, err := client.Get(baseURL + "health"); err == nil {
			if resp.StatusCode == 200 {
				status = "Running on " + baseURL
			}
			resp.Body.Close()
		}
		menu, _, _ := trayCreateMenu.Call()
		if menu == 0 {
			return
		}
		defer trayDestroyMenu.Call(menu)
		appendItem := func(flags uintptr, id uintptr, label string) {
			var p uintptr
			if label != "" {
				p = uintptr(unsafe.Pointer(trayUTF16(label)))
			}
			trayAppendMenu.Call(menu, flags, id, p)
		}
		appendItem(mfString, menuHome, "Open OrgAxis")
		appendItem(mfString, menuWorkspace, "Open HR Workspace")
		appendItem(mfString, menuLogs, "View Logs / Diagnostics")
		on, regErr := startupEnabled()
		startLabel := "Enable Start with Windows"
		if on {
			startLabel = "Disable Start with Windows"
		}
		if regErr != nil {
			appendItem(mfString|mfGray, menuStartup, "Windows Startup unavailable")
		} else {
			appendItem(mfString, menuStartup, startLabel)
		}
		appendItem(mfSeparator, 0, "")
		appendItem(mfGray|mfString, 0, status)
		appendItem(mfSeparator, 0, "")
		appendItem(mfString, menuExit, "Exit OrgAxis")
		var p trayPoint
		trayGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
		traySetForeground.Call(hwnd)
		cmd, _, _ := trayTrackMenu.Call(menu, tpmRightButton|tpmReturnCmd,
			uintptr(p.X), uintptr(p.Y), 0, hwnd, 0)
		switch cmd {
		case menuHome:
			launchBrowser(baseURL)
		case menuWorkspace:
			launchBrowser(baseURL + "app")
		case menuLogs:
			launchBrowser(baseURL + "logs")
		case menuStartup:
			active, e := startupEnabled()
			if e == nil {
				if e = setStartup(!active, mode); e != nil {
					showStartupError("Windows Startup: " + e.Error())
				}
			}
		case menuExit:
			trayDestroyWindow.Call(hwnd)
		}
	}
	callback := syscall.NewCallback(func(w uintptr, msg uint32, wp, lp uintptr) uintptr {
		if msg == uint32(taskbarMsg) && taskbarMsg != 0 {
			addIcon()
			return 0
		}
		switch msg {
		case trayMessage:
			switch lp & 0xffff {
			case wmLDouble:
				launchBrowser(baseURL + "app")
			case wmRButtonUp, wmContextMenu:
				showMenu()
			}
			return 0
		case wmClose:
			trayDestroyWindow.Call(w)
			return 0
		case wmDestroy:
			if registered {
				trayNotify.Call(nimDelete, uintptr(unsafe.Pointer(&notice)))
				registered = false
			}
			trayPostQuit.Call(0)
			return 0
		}
		res, _, _ := trayDefProc.Call(w, uintptr(msg), wp, lp)
		return res
	})
	wc := trayWndClass{WndProc: callback, Instance: inst, ClassName: className}
	atom, _, e := trayRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassW failed: %w", e)
	}
	title := trayUTF16("OrgAxis Tray")
	hwnd, _, e = trayCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, inst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed: %w", e)
	}
	// Load our own ICO, exported once beside the local SQLite database.
	if b, err := builtIn.ReadFile("assets/orgaxis-tray.ico"); err == nil {
		if err = os.WriteFile(iconFile, b, 0600); err == nil {
			path := trayUTF16(filepath.Clean(iconFile))
			hIcon, _, _ = trayLoadImage.Call(0, uintptr(unsafe.Pointer(path)), imageIcon, 0, 0, loadFromFile|loadDefaultSize)
		}
	}
	if hIcon == 0 {
		hIcon, _, _ = trayLoadIcon.Call(0, 32512)
	}
	if !addIcon() {
		trayDestroyWindow.Call(hwnd)
		return fmt.Errorf("Shell_NotifyIconW failed to install notification icon")
	}
	var message trayMsg
	for {
		rc, _, err := trayGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(rc) == -1 {
			return fmt.Errorf("GetMessageW failed: %w", err)
		}
		if rc == 0 {
			break
		}
		trayTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		trayDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	if hIcon != 0 {
		trayDestroyIcon.Call(hIcon)
	}
	if onExit != nil {
		onExit()
	}
	runtime.KeepAlive(callback)
	return nil
}

func showStartupError(detail string) {
	title := trayUTF16("OrgAxis — Startup error")
	message := trayUTF16(detail + "\n\nCheck data/orgaxis.log or whether port 8765 is already used.")
	trayUser32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
