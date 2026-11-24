// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package systray

import (
	"errors"
	"fmt"
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procFindWindowEx    = user32.NewProc("FindWindowExW")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
)

// The window class and icon ID that fyne.io/systray gives its notification
// area icon. It doesn't export them or any way to show a notification, so
// these must match its systray_windows.go.
const (
	trayWindowClass = "SystrayClass"
	trayIconID      = 100
)

// notifyIconData is the Win32 NOTIFYICONDATAW structure.
type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout, Version           uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

// sendNotification sends a desktop notification with the given title and content.
func (menu *Menu) sendNotification(title, content string) {
	if err := showBalloon(title, content); err != nil {
		log.Printf("notification: %v", err)
	}
}

// showBalloon shows a notification from the systray icon. Windows 10 and
// later display it as a toast.
func showBalloon(title, content string) error {
	const (
		nimModify   = 0x1
		nifInfo     = 0x10
		niifNoSound = 0x10
	)
	wnd, err := trayWindow()
	if err != nil {
		return err
	}
	// NIM_MODIFY only changes the members that Flags names, so this leaves
	// the icon and tooltip that the systray package set alone.
	nid := notifyIconData{
		Wnd:       wnd,
		ID:        trayIconID,
		Flags:     nifInfo,
		InfoFlags: niifNoSound,
	}
	nid.Size = uint32(unsafe.Sizeof(nid))
	copyUTF16(nid.InfoTitle[:], title)
	copyUTF16(nid.Info[:], content)
	if ok, _, err := procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid))); ok == 0 {
		return fmt.Errorf("Shell_NotifyIcon: %w", err)
	}
	return nil
}

// trayWindow returns this process's hidden window that owns the systray icon.
func trayWindow() (windows.Handle, error) {
	class, err := windows.UTF16PtrFromString(trayWindowClass)
	if err != nil {
		return 0, err
	}
	pid := windows.GetCurrentProcessId()
	var wnd uintptr
	for {
		// Other programs that use the systray package have a window of the
		// same class, so look through all of them for ours.
		wnd, _, _ = procFindWindowEx.Call(0, wnd, uintptr(unsafe.Pointer(class)), 0)
		if wnd == 0 {
			return 0, errors.New("systray window not found")
		}
		var wndPID uint32
		windows.GetWindowThreadProcessId(windows.HWND(wnd), &wndPID)
		if wndPID == pid {
			return windows.Handle(wnd), nil
		}
	}
}

// copyUTF16 copies s to dst as NUL-terminated UTF-16, truncating it to fit.
func copyUTF16(dst []uint16, s string) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return // s contains a NUL
	}
	n := copy(dst[:len(dst)-1], u)
	dst[n] = 0
}
