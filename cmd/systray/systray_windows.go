// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// defaultTheme returns the icon theme that suits the taskbar: Windows has
// separate light and dark modes for apps and for the taskbar.
func defaultTheme() string {
	const personalize = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	k, err := registry.OpenKey(registry.CURRENT_USER, personalize, registry.QUERY_VALUE)
	if err != nil {
		return "dark:nobg"
	}
	defer k.Close()
	if light, _, err := k.GetIntegerValue("SystemUsesLightTheme"); err == nil && light != 0 {
		return "light:nobg"
	}
	return "dark:nobg"
}

// platformInit exits if the systray application is already running in this
// session. Otherwise, if it has no console to log to, as when built with
// -H=windowsgui, it sends the log to a file.
func platformInit() {
	// The mutex handle is left open so the process holds it until it exits.
	name, _ := windows.UTF16PtrFromString(`Local\TailscaleSystray`)
	if _, err := windows.CreateMutex(nil, false, name); err == windows.ERROR_ALREADY_EXISTS {
		os.Exit(0)
	}

	if h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil && h != 0 && h != windows.InvalidHandle {
		return
	}
	if f, err := openLogFile(); err == nil {
		log.SetOutput(f)
	}
}

// openLogFile opens %LocalAppData%\Tailscale\systray.log to append to,
// starting it over once it's large.
func openLogFile() (*os.File, error) {
	const maxSize = 1 << 20
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "Tailscale")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "systray.log")
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxSize {
		flags |= os.O_TRUNC
	}
	return os.OpenFile(path, flags, 0o600)
}
