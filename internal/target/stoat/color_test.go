// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package stoat

import "testing"

func TestIntToCSS(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0xFF5733, "#FF5733"},
		{0x000000, "#000000"},
		{0xFFFFFF, "#FFFFFF"},
		{0x1ABC9C, "#1ABC9C"},
		{0, "#000000"},
	}
	for _, c := range cases {
		got := intToCSS(c.in)
		if got != c.want {
			t.Errorf("intToCSS(%#x) = %q, want %q", c.in, got, c.want)
		}
	}
}
