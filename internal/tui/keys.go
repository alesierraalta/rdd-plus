package tui

import "bytes"

// key is one decoded input event. Everything the interaction model binds has a value; every
// other byte, including digits the model ignores, decodes to keyUnknown.
type key int

const (
	keyUnknown key = iota
	keyUp
	keyDown
	keyEnter
	keyEsc
	keySpace
	keyQ
	keyCtrlC
)

func decodeComplete(p []byte) []key {
	var keys []key
	for i := 0; i < len(p); {
		b := p[i]
		switch {
		case b == 0x1b:
			switch {
			case i+1 >= len(p):
				keys = append(keys, keyEsc)
				i++
			case p[i+1] != '[':
				keys = append(keys, keyEsc)
				i++
			case i+2 >= len(p):
				keys = append(keys, keyUnknown)
				i += 2
			case p[i+2] == 'A':
				keys = append(keys, keyUp)
				i += 3
			case p[i+2] == 'B':
				keys = append(keys, keyDown)
				i += 3
			default:
				keys = append(keys, keyUnknown)
				i += 3
			}
		case b == '\r' || b == '\n':
			keys = append(keys, keyEnter)
			i++
		case b == ' ':
			keys = append(keys, keySpace)
			i++
		case b == 'q':
			keys = append(keys, keyQ)
			i++
		case b == 3:
			keys = append(keys, keyCtrlC)
			i++
		default:
			keys = append(keys, keyUnknown)
			i++
		}
	}
	return keys
}

// decoder holds a trailing partial escape sequence across reads: VMIN=1 can end a read after ESC or ESC-[ alone, and firing those early would quit or mis-navigate the menu.
type decoder struct{ pending []byte }

func (d *decoder) decode(p []byte) []key {
	buf := append(append(make([]byte, 0, len(d.pending)+len(p)), d.pending...), p...)
	d.pending = nil
	i := bytes.LastIndexByte(buf, 0x1b)
	if i >= 0 && (len(buf)-i == 1 || (len(buf)-i == 2 && buf[i+1] == '[')) {
		d.pending = append([]byte(nil), buf[i:]...)
		buf = buf[:i]
	}
	return decodeComplete(buf)
}

// flush emits held bytes once input is exhausted, so a trailing lone ESC still becomes keyEsc.
func (d *decoder) flush() []key {
	p := d.pending
	d.pending = nil
	return decodeComplete(p)
}
