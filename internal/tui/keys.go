package tui

import "bytes"

// key is one decoded input event. Everything the interaction model binds has a value; every
// other byte, including digits the model ignores, decodes to keyUnknown.
type key int

const (
	keyUnknown key = iota
	keyUp
	keyDown
	keyPgUp
	keyPgDn
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
			k, n := decodeEscape(p[i:])
			keys = append(keys, k)
			i += n
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

// decodeEscape decodes the sequence at the start of p (p[0] is ESC) and reports how many bytes it
// consumed. CSI arrows (ESC [ A) and application-mode arrows (ESC O A) are both accepted, because
// terminals send either depending on their cursor-key mode.
func decodeEscape(p []byte) (key, int) {
	if len(p) == 1 || (p[1] != '[' && p[1] != 'O') {
		return keyEsc, 1
	}
	if len(p) == 2 {
		return keyUnknown, 2
	}
	switch p[2] {
	case 'A':
		return keyUp, 3
	case 'B':
		return keyDown, 3
	case '5', '6':
		if p[1] == '[' && len(p) > 3 && p[3] == '~' {
			if p[2] == '5' {
				return keyPgUp, 4
			}
			return keyPgDn, 4
		}
	}
	return keyUnknown, 3
}

// decoder holds a trailing partial escape sequence across reads: VMIN=1 can end a read after ESC
// or ESC-[ alone, and firing those early would quit or mis-navigate the menu. The caller flushes
// what is held once no continuation arrives in time, so a real Esc press still fires.
type decoder struct{ pending []byte }

func (d *decoder) decode(p []byte) []key {
	buf := append(append(make([]byte, 0, len(d.pending)+len(p)), d.pending...), p...)
	d.pending = nil
	if i := bytes.LastIndexByte(buf, 0x1b); i >= 0 && incompleteEscape(buf[i:]) {
		d.pending = append([]byte(nil), buf[i:]...)
		buf = buf[:i]
	}
	return decodeComplete(buf)
}

// incompleteEscape reports whether seq (starting at ESC) could still grow into a bound sequence.
func incompleteEscape(seq []byte) bool {
	switch len(seq) {
	case 1:
		return true
	case 2:
		return seq[1] == '[' || seq[1] == 'O'
	case 3:
		return seq[1] == '[' && (seq[2] == '5' || seq[2] == '6')
	}
	return false
}

// holding reports whether a partial escape sequence is waiting for its continuation.
func (d *decoder) holding() bool { return len(d.pending) > 0 }

// flush emits held bytes once no continuation is coming, so a trailing lone ESC still becomes keyEsc.
func (d *decoder) flush() []key {
	p := d.pending
	d.pending = nil
	return decodeComplete(p)
}
