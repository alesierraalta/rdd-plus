package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeKey(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, KeyFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadKey(t *testing.T) {
	valid := `{"id":"c1","language":"node","suite":"node --test","surface":"lib","defects":[{"id":"d1","file":"src/a.js","line":3,"class":"boundary","keywords":["limit"],"description":"x","trigger":{"input":"i","expected":"e","actual":"a"},"why_missed":"w"}]}`
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"valid key parses", valid, ""},
		{"malformed json", "{not json", "parse key"},
		{"language outside node/go", strings.Replace(valid, `"node"`, `"rust"`, 1), "not node or go"},
		{"defect without keywords", strings.Replace(valid, `["limit"]`, `[]`, 1), "no keywords"},
		{"defect without a positive line", strings.Replace(valid, `"line":3`, `"line":0`, 1), "positive line"},
		{"no defects", `{"id":"c1","language":"go","suite":"go test ./...","defects":[]}`, "no defects"},
		{"repeated defect id", strings.Replace(valid, `"defects":[`, `"defects":[{"id":"d1","file":"f.go","line":1,"keywords":["k"]},`, 1), "repeated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, err := LoadKey(writeKey(t, tc.body))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if k.ID != "c1" || len(k.Defects) != 1 || k.Defects[0].Trigger.Expected != "e" {
					t.Fatalf("parsed key wrong: %+v", k)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadKeyMissingFile(t *testing.T) {
	if _, err := LoadKey(t.TempDir()); err == nil || !strings.Contains(err.Error(), "read key") {
		t.Fatalf("error = %v", err)
	}
}

// A case's bounded request is the unit of work the run is asked to test. A key without one is the
// generic case the bench has always run; a key that supplies a blank one is a mistake in the key, so
// it is refused instead of being read back as "no request".
func TestLoadKeyValidatesTheCaseRequest(t *testing.T) {
	valid := `{"id":"c1","language":"node","suite":"node --test","surface":"lib","defects":[{"id":"d1","file":"src/a.js","line":3,"class":"boundary","keywords":["limit"],"description":"x","trigger":{"input":"i","expected":"e","actual":"a"},"why_missed":"w"}]}`
	withRequest := func(value string) string {
		return strings.Replace(valid, `"surface":"lib"`, `"surface":"lib","request":`+value, 1)
	}
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr string
	}{
		{"no request is the generic case", valid, "", ""},
		{"a request names the bounded unit of work", withRequest(`"src/slug.js"`), "src/slug.js", ""},
		{"a request is trimmed", withRequest(`"  src/slug.js  "`), "src/slug.js", ""},
		{"a blank request is refused", withRequest(`"   "`), "", "request"},
		{"a null request is the generic case", withRequest("null"), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, err := LoadKey(writeKey(t, tc.body))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := k.RequestText(); got != tc.want {
				t.Fatalf("RequestText = %q, want %q", got, tc.want)
			}
		})
	}
}
