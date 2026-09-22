package tui

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

// decode turns one read of terminal bytes into keys. Terminals deliver an escape sequence in a
// single read, so a lone ESC is the esc key while an ESC-[ that does not finish as an up/down
// arrow inside the same buffer is a partial sequence: unknown, never a panic.
func decode(p []byte) []key {
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
