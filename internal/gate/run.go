package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const gitTimeout = 10 * time.Second

func realGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	return string(out), err
}

// RealDeps wires the process boundaries to the real git, filesystem, and clock.
func RealDeps(now time.Time) Deps {
	wd, _ := os.Getwd()
	return Deps{
		Git:  realGit,
		Stat: os.Stat,
		OpenTranscript: func(path string) (io.ReadCloser, error) {
			return os.Open(path)
		},
		Now:     now,
		WorkDir: wd,
	}
}

// DefaultLogPath resolves the telemetry log: env override, else <configDir>/telemetry.
func DefaultLogPath(configDir string) string {
	if p := os.Getenv("TESTING_GATE_LOG"); p != "" {
		return p
	}
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".claude")
	}
	return filepath.Join(configDir, "telemetry", "testing-gate.jsonl")
}

// appendEntry never fails the turn: a lost log line is cheaper than a broken session.
func appendEntry(path string, e *Entry) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
}

func emit(w io.Writer, reason string) { emitWith(w, reason, "") }

// emitWith sends the reason to the model as context and, when there is one, a line the operator
// reads in their own terminal: the offer of feedback must not depend on the model relaying it.
func emitWith(w io.Writer, reason, userLine string) {
	payload := map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "Stop",
			"additionalContext": reason,
		},
	}
	if userLine != "" {
		payload["systemMessage"] = userLine
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
	_, _ = w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
}

// Run is the Stop hook: stdin payload → decision → log line → optional Stop feedback.
// It always returns 0; an unreadable payload or an internal failure is silence, never a
// broken turn.
func Run(stdin io.Reader, stdout io.Writer, logPath string, now time.Time) (code int) {
	defer func() {
		if r := recover(); r != nil {
			code = 0
		}
	}()
	raw, _ := io.ReadAll(stdin)
	payload := strings.TrimSpace(string(raw))
	if payload == "" || !strings.HasPrefix(payload, "{") {
		return 0
	}
	var in Input
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return 0
	}
	res := Decide(in, RealDeps(now))
	if res.Entry != nil {
		appendEntry(logPath, res.Entry)
	}
	switch {
	case res.Fire:
		emit(stdout, res.Reason)
	case res.Audit:
		emitWith(stdout, res.Reason, auditLine(res))
	}
	return 0
}

// auditLine is what the operator sees without the model saying anything.
func auditLine(res Result) string {
	n := strings.Count(res.Reason, "assigned and never invoked:")
	if n == 0 {
		return "rdd-plus: the testing plan still owes work. Want feedback on this run?"
	}
	return fmt.Sprintf("rdd-plus: %d layer(s) assigned and never invoked. Want feedback on this run?", n)
}
