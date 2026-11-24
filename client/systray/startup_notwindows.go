// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

//go:build !windows && (cgo || !darwin)

package systray

import "errors"

// startAtLoginSupported is whether the menu offers to start the systray
// application when the user logs in. Linux desktops use
// [InstallStartupScript] instead.
const startAtLoginSupported = false

func startAtLogin() bool { return false }

func setStartAtLogin(bool) error { return errors.ErrUnsupported }
