// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package systray

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

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
