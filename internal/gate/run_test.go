package gate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The operator asked to be asked: the audit reaches the model as context and the user as a line
// in their own terminal, so the offer of feedback does not depend on the model relaying it.
func TestEmitCarriesAUserFacingLineWhenAsked(t *testing.T) {
	var b bytes.Buffer
	emitWith(&b, "the reason", "rdd-plus: 3 layers assigned and never invoked. Want feedback on this run?")
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v %s", err, b.String())
	}
	hs, _ := got["hookSpecificOutput"].(map[string]any)
	if hs["additionalContext"] != "the reason" {
		t.Fatalf("context = %v", hs["additionalContext"])
	}
	if !strings.Contains(got["systemMessage"].(string), "Want feedback") {
		t.Fatalf("systemMessage = %v", got["systemMessage"])
	}
	var plain bytes.Buffer
	emitWith(&plain, "only context", "")
	var got2 map[string]any
	_ = json.Unmarshal(plain.Bytes(), &got2)
	if _, ok := got2["systemMessage"]; ok {
		t.Fatalf("an empty line must not become an empty message: %s", plain.String())
	}
}
