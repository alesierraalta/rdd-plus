package tui

import (
	"slices"
	"testing"
)

func TestDecode(t *testing.T) {
	cases := []struct {
		name      string
		in        []byte
		want      []key
		flushWant []key
		next      []byte // second read appended after in
		nextWant  []key
	}{
		{name: "up arrow", in: []byte("\x1b[A"), want: []key{keyUp}},
		{name: "down arrow", in: []byte("\x1b[B"), want: []key{keyDown}},
		{name: "enter cr", in: []byte("\r"), want: []key{keyEnter}},
		{name: "enter lf", in: []byte("\n"), want: []key{keyEnter}},
		{name: "lone esc", in: []byte("\x1b"), want: nil, flushWant: []key{keyEsc}},
		{name: "space", in: []byte(" "), want: []key{keySpace}},
		{name: "q", in: []byte("q"), want: []key{keyQ}},
		{name: "ctrl c", in: []byte{3}, want: []key{keyCtrlC}},
		{name: "empty input", in: nil, want: nil},
		{name: "partial esc bracket", in: []byte("\x1b["), want: nil, flushWant: []key{keyUnknown}},
		{name: "esc bracket wrong final", in: []byte("\x1b[?"), want: []key{keyUnknown}},
		{name: "esc then other byte", in: []byte("\x1bx"), want: []key{keyEsc, keyUnknown}},
		{name: "digits ignored", in: []byte("42"), want: []key{keyUnknown, keyUnknown}},
		{name: "garbage", in: []byte{0xff, 0x01, 'z'}, want: []key{keyUnknown, keyUnknown, keyUnknown}},
		{name: "mixed", in: []byte("\x1b[Bqa "), want: []key{keyDown, keyQ, keyUnknown, keySpace}},
		{name: "adjacent sequences", in: []byte("\x1b[A\x1b[B"), want: []key{keyUp, keyDown}},
		{name: "split arrow up", in: []byte("\x1b"), want: nil, next: []byte("[A"), nextWant: []key{keyUp}},
		{name: "split arrow down", in: []byte("\x1b"), want: nil, next: []byte("[B"), nextWant: []key{keyDown}},
		{name: "split after bracket", in: []byte("\x1b["), want: nil, next: []byte("A"), nextWant: []key{keyUp}},
		{name: "split esc then byte", in: []byte("\x1b"), want: nil, next: []byte("x"), nextWant: []key{keyEsc, keyUnknown}},
		{name: "application up arrow", in: []byte("\x1bOA"), want: []key{keyUp}},
		{name: "page up", in: []byte("\x1b[5~"), want: []key{keyPgUp}},
		{name: "page down", in: []byte("\x1b[6~"), want: []key{keyPgDn}},
		{name: "split page down", in: []byte("\x1b[6"), want: nil, next: []byte("~"), nextWant: []key{keyPgDn}},
		{name: "pending cleared", in: []byte("\x1b[A"), want: []key{keyUp}, next: []byte("q"), nextWant: []key{keyQ}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var d decoder
			got := d.decode(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("decode(%q) = %v, want %v", tt.in, got, tt.want)
			}
			if tt.next != nil {
				if got := d.decode(tt.next); !slices.Equal(got, tt.nextWant) {
					t.Errorf("decode(%q) = %v, want %v", tt.next, got, tt.nextWant)
				}
			}
			if tt.flushWant != nil {
				if got := d.flush(); !slices.Equal(got, tt.flushWant) {
					t.Errorf("flush() = %v, want %v", got, tt.flushWant)
				}
			}
		})
	}
}
