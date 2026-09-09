package bench

import (
	"strings"
	"testing"
)

func TestParseStream(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s-123"}`,
		`not json at all`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`,
		``,
		`{"type":"result","subtype":"success","result":"done","total_cost_usd":1.25,"num_turns":7,"duration_ms":4200,"session_id":"s-123"}`,
	}, "\n")
	got := ParseStream(strings.NewReader(stream))
	if got.Result != "done" || got.CostUSD != 1.25 || got.Turns != 7 || got.DurationMS != 4200 || got.SessionID != "s-123" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseStreamPartial(t *testing.T) {
	got := ParseStream(strings.NewReader(`{"type":"system","session_id":"s-9"}` + "\n" + `{"type":"assistant"}`))
	if got.SessionID != "s-9" || got.Result != "" || got.CostUSD != 0 || got.Turns != 0 {
		t.Fatalf("partial stream should yield only the session id: %+v", got)
	}
}

func TestParseStreamKeepsTheLastResult(t *testing.T) {
	stream := `{"type":"result","result":"first","total_cost_usd":0.1,"num_turns":1}` + "\n" +
		`{"type":"result","result":"second","total_cost_usd":0.3,"num_turns":3}`
	if got := ParseStream(strings.NewReader(stream)); got.Result != "second" || got.CostUSD != 0.3 {
		t.Fatalf("got %+v", got)
	}
}
