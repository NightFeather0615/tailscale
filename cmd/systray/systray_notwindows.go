// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

//go:build !windows && (cgo || !darwin)

package main

func defaultTheme() string { return "dark" }

func platformInit() {}
