// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package systray

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// startAtLoginSupported is whether the menu offers to start the systray
// application when the user logs in.
const startAtLoginSupported = true

// runKeyPath is the per-user registry key of programs that Windows starts
// at login.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// runKeyValue is the name of the systray application's value in that key.
// It's a variable for tests.
var runKeyValue = "Tailscale Systray"

// startAtLogin reports whether Windows starts the systray application when
// the user logs in.
func startAtLogin() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runKeyValue)
	return err == nil
}

// setStartAtLogin sets whether Windows starts the systray application, as
// it was started this time, when the user logs in.
func setStartAtLogin(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		err := k.DeleteValue(runKeyValue)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := append([]string{exe}, os.Args[1:]...)
	return k.SetStringValue(runKeyValue, windows.ComposeCommandLine(args))
}
