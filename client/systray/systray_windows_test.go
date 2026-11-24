// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package systray

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestStartAtLogin(t *testing.T) {
	// Don't touch a real installation's value.
	runKeyValue = "Tailscale Systray Test"
	t.Cleanup(func() { setStartAtLogin(false) })

	if err := setStartAtLogin(false); err != nil {
		t.Fatalf("disabling when not enabled: %v", err)
	}
	if startAtLogin() {
		t.Fatal("enabled before it was set")
	}
	if err := setStartAtLogin(true); err != nil {
		t.Fatal(err)
	}
	if !startAtLogin() {
		t.Fatal("not enabled after it was set")
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	cmdLine, _, err := k.GetStringValue(runKeyValue)
	if err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine(cmdLine)
	if err != nil {
		t.Fatalf("command line %q: %v", cmdLine, err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != len(os.Args) || args[0] != exe {
		t.Errorf("command line %q; want %q with arguments %q", cmdLine, exe, os.Args[1:])
	}

	if err := setStartAtLogin(false); err != nil {
		t.Fatal(err)
	}
	if startAtLogin() {
		t.Fatal("still enabled after it was unset")
	}
}

func TestCopyUTF16(t *testing.T) {
	tests := []struct {
		name string
		in   string
		size int
		want string
	}{
		{"fits", "hello", 8, "hello"},
		{"exact", "hello", 6, "hello"},
		{"truncated", "hello", 4, "hel"},
		{"empty", "", 4, ""},
		{"non_ascii", "已複製位址", 4, "已複製"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]uint16, tt.size)
			for i := range dst {
				dst[i] = 'x'
			}
			copyUTF16(dst, tt.in)
			if got := windows.UTF16ToString(dst); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
	// A string with a NUL can't be converted, and is left out.
	dst := make([]uint16, 4)
	copyUTF16(dst, "a\x00b")
	if got := windows.UTF16ToString(dst); strings.Contains(got, "b") {
		t.Errorf("got %q for a string with a NUL", got)
	}
}
