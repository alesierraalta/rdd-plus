package bench

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// AgentResult is what one agent run produced, as read from Claude Code's stream-json output.
type AgentResult struct {
	Result     string  `json:"result"`
	CostUSD    float64 `json:"cost_usd"`
	Turns      int     `json:"turns"`
	DurationMS int64   `json:"duration_ms"`
	SessionID  string  `json:"session_id"`
	ExitCode   int     `json:"exit_code"`
	TimedOut   bool    `json:"timed_out"`
}

// ParseStream keeps the last "result" event of a stream-json transcript; every other line is
// ignored, including lines that are not JSON, so a partial stream still yields what it has.
func ParseStream(r io.Reader) AgentResult {
	var out AgentResult
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var ev struct {
			Type       string  `json:"type"`
			Result     string  `json:"result"`
			CostUSD    float64 `json:"total_cost_usd"`
			Turns      int     `json:"num_turns"`
			DurationMS int64   `json:"duration_ms"`
			SessionID  string  `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.SessionID != "" && out.SessionID == "" {
			out.SessionID = ev.SessionID
		}
		if ev.Type == "result" {
			out.Result, out.CostUSD, out.Turns, out.DurationMS = ev.Result, ev.CostUSD, ev.Turns, ev.DurationMS
			if ev.SessionID != "" {
				out.SessionID = ev.SessionID
			}
		}
	}
	return out
}
