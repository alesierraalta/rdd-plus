// Package tui is the hand-rolled, dependency-free interactive surface: a menu over status,
// feature toggles with preview, and the sync dry-run plan, redrawn as full ANSI frames.
package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Deps is what the CLI injects; tests fake it. The tui package never imports cmd.
type Deps struct {
	Status     func() (StatusView, error)
	Features   func() ([]FeatureRow, error)
	SetFeature func(id string, enabled bool) (string, error) // returns updatedAt
	Preview    func(id string) (string, error)
	SyncPlan   func() (string, error) // dry-run report text; must write nothing
}

// StatusView is the read-only status table the status CLI prints, passed whole.
type StatusView struct {
	StateRoot        string
	StateExists      bool
	InstalledVersion string
	AvailableVersion string // "unknown (no update check yet)" until Phase 3
	Features         []FeatureRow
}

// FeatureRow is one registry feature as the CLI lists it.
type FeatureRow struct {
	ID      string
	Title   string
	Enabled bool
}

// errNotTerminal marks an in/out pair that is not a terminal: the loop then runs unmodified so
// tests (and pipes) can drive Run with plain readers.
var errNotTerminal = errors.New("tui: not a terminal")

type view int

const (
	viewMenu view = iota
	viewStatus
	viewFeatures
	viewPlan
)

type app struct {
	deps        Deps
	out         io.Writer
	view        view
	menuSel     int
	featureSel  int
	status      StatusView
	statusErr   error
	rows        []FeatureRow
	detail      string
	planReport  string
	planErr     error
	prevLineCnt int
}

// Run drives the menu until quit. It enters raw mode only when in and out are terminal files
// (per-OS helpers; anything else — bytes.Buffer in tests, pipes — skips it), hides the cursor
// for the loop, and restores the terminal on every exit path via defer.
func Run(in io.Reader, out io.Writer, deps Deps) error {
	restore, err := beginRaw(in, out)
	if err != nil {
		return err
	}
	if restore != nil {
		defer func() { _ = restore() }()
	}
	if _, err := io.WriteString(out, "\x1b[?25l"); err != nil {
		return err
	}
	defer func() { _, _ = io.WriteString(out, "\x1b[?25h") }()

	a := &app{deps: deps, out: out}
	if err := a.redraw(); err != nil {
		return err
	}
	buf := make([]byte, 256)
	for {
		n, readErr := in.Read(buf)
		for _, k := range decode(buf[:n]) {
			if a.handle(k) {
				return nil
			}
			if err := a.redraw(); err != nil {
				return err
			}
		}
		if readErr != nil {
			return readErr
		}
	}
}

// beginRaw enters raw mode when both streams are terminal files; plain readers and non-terminal
// files skip it, and an unsupported platform with a real terminal reports that instead.
func beginRaw(in io.Reader, out io.Writer) (func() error, error) {
	inFile, ok := in.(*os.File)
	if !ok {
		return nil, nil
	}
	outFile, ok := out.(*os.File)
	if !ok {
		return nil, nil
	}
	restore, err := makeRaw(inFile, outFile)
	if errors.Is(err, errNotTerminal) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return restore, nil
}

// handle applies one key to the current view and reports whether the loop should quit. q and
// ctrl+c quit from anywhere; esc at the menu quits because the root has no parent.
func (a *app) handle(k key) bool {
	if k == keyQ || k == keyCtrlC {
		return true
	}
	switch a.view {
	case viewMenu:
		return a.handleMenu(k)
	case viewFeatures:
		return a.handleFeatures(k)
	default:
		if k == keyEsc {
			a.view = viewMenu
		}
		return false
	}
}

func (a *app) handleMenu(k key) bool {
	switch k {
	case keyUp:
		if a.menuSel > 0 {
			a.menuSel--
		}
	case keyDown:
		if a.menuSel < len(menuItems)-1 {
			a.menuSel++
		}
	case keyEsc:
		return true
	case keyEnter:
		switch a.menuSel {
		case 0:
			a.view = viewStatus
			a.status, a.statusErr = a.deps.Status()
		case 1:
			a.enterFeatures()
		case 2:
			a.view = viewPlan
			a.planReport, a.planErr = a.deps.SyncPlan()
		default:
			return true
		}
	}
	return false
}

func (a *app) enterFeatures() {
	a.view = viewFeatures
	a.detail = ""
	a.featureSel = 0
	rows, err := a.deps.Features()
	if err != nil {
		a.rows = nil
		a.detail = "error: " + err.Error()
		return
	}
	a.rows = rows
}

func (a *app) handleFeatures(k key) bool {
	switch k {
	case keyUp:
		if a.featureSel > 0 {
			a.featureSel--
		}
	case keyDown:
		if a.featureSel < len(a.rows)-1 {
			a.featureSel++
		}
	case keyEsc:
		a.view = viewMenu
	case keySpace:
		if a.featureSel >= len(a.rows) {
			return false
		}
		row := &a.rows[a.featureSel]
		enabled := !row.Enabled
		if _, err := a.deps.SetFeature(row.ID, enabled); err != nil {
			a.detail = "error: " + err.Error()
			return false
		}
		row.Enabled = enabled
	case keyEnter:
		if a.featureSel >= len(a.rows) {
			return false
		}
		text, err := a.deps.Preview(a.rows[a.featureSel].ID)
		if err != nil {
			a.detail = "error: " + err.Error()
			return false
		}
		a.detail = text
	}
	return false
}

func (a *app) frame() string {
	switch a.view {
	case viewStatus:
		return renderStatus(a.status, a.statusErr)
	case viewFeatures:
		return renderFeatures(a.rows, a.featureSel, a.detail)
	case viewPlan:
		return renderPlan(a.planReport, a.planErr)
	default:
		return renderMenu(a.menuSel)
	}
}

func (a *app) redraw() error {
	return writeFrame(a.out, a.frame(), &a.prevLineCnt)
}

// writeFrame repaints the region the previous frame occupied: cursor-up to its first line,
// clear each rewritten line, then clear any lines a taller previous frame left behind.
func writeFrame(out io.Writer, frame string, prev *int) error {
	lines := strings.Split(frame, "\n")
	var b strings.Builder
	if *prev > 1 {
		fmt.Fprintf(&b, "\x1b[%dA", *prev-1)
	}
	if *prev > 0 {
		b.WriteString("\r")
	}
	for i, line := range lines {
		b.WriteString("\x1b[K")
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	if *prev > len(lines) {
		for i := len(lines); i < *prev; i++ {
			b.WriteString("\n\x1b[K")
		}
		fmt.Fprintf(&b, "\x1b[%dA", *prev-len(lines))
	}
	*prev = len(lines)
	_, err := io.WriteString(out, b.String())
	return err
}
