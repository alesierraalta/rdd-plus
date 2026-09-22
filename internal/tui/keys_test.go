package tui

import (
	"slices"
	"testing"
)

func TestDecode(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []key
	}{
		{name: "up arrow", in: []byte("\x1b[A"), want: []key{keyUp}},
		{name: "down arrow", in: []byte("\x1b[B"), want: []key{keyDown}},
		{name: "enter cr", in: []byte("\r"), want: []key{keyEnter}},
		{name: "enter lf", in: []byte("\n"), want: []key{keyEnter}},
		{name: "lone esc", in: []byte("\x1b"), want: []key{keyEsc}},
		{name: "space", in: []byte(" "), want: []key{keySpace}},
		{name: "q", in: []byte("q"), want: []key{keyQ}},
		{name: "ctrl c", in: []byte{3}, want: []key{keyCtrlC}},
		{name: "empty input", in: nil, want: nil},
		{name: "partial esc bracket", in: []byte("\x1b["), want: []key{keyUnknown}},
		{name: "esc bracket wrong final", in: []byte("\x1b[?"), want: []key{keyUnknown}},
		{name: "esc then other byte", in: []byte("\x1bx"), want: []key{keyEsc, keyUnknown}},
		{name: "digits ignored", in: []byte("42"), want: []key{keyUnknown, keyUnknown}},
		{name: "garbage", in: []byte{0xff, 0x01, 'z'}, want: []key{keyUnknown, keyUnknown, keyUnknown}},
		{name: "mixed", in: []byte("\x1b[Bqa "), want: []key{keyDown, keyQ, keyUnknown, keySpace}},
		{name: "adjacent sequences", in: []byte("\x1b[A\x1b[B"), want: []key{keyUp, keyDown}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := decode(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("decode(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
