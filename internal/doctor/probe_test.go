package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Doctor reported a wired gate while the wiring was broken: it matched the shape of the command
// string and never asked whether running it worked. A hook that exits non-zero on a payload is
// not wired, whatever it looks like.
func TestProbeDecidesWiring(t *testing.T) {
	settings := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"/h/bin/testing-gate gate"}]}]}}`
	cases := []struct {
		name        string
		probeErr    error
		wantHealthy bool
		wantProblem string
		wantReport  string
	}{
		{name: "the hook answers", wantHealthy: true, wantReport: "answers a payload"},
		{
			name:        "the hook exits non-zero",
			probeErr:    errors.New("exit status 2"),
			wantProblem: "wired command does not answer a hook payload",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
				t.Fatal(err)
			}
			var probed string
			r := RunWith(dir, func(string) (string, error) { return "/usr/bin/x", nil }, func(cmd string) error {
				probed = cmd
				return tc.probeErr
			})
			if probed != "/h/bin/testing-gate gate" {
				t.Fatalf("probed %q", probed)
			}
			if got := containsProblem(r.Problems, "wired command does not answer"); got != (tc.wantProblem != "") {
				t.Fatalf("problems = %v", r.Problems)
			}
			if tc.wantHealthy && !strings.Contains(r.String(), tc.wantReport) {
				t.Fatalf("report missing %q:\n%s", tc.wantReport, r.String())
			}
			if r.HookProbed != (tc.probeErr == nil) {
				t.Fatalf("HookProbed = %v", r.HookProbed)
			}
		})
	}
}

// With no probe available the report says the wiring was not verified rather than claiming it works.
func TestWithoutAProbeTheWiringIsUnverified(t *testing.T) {
	dir := t.TempDir()
	settings := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"/h/bin/testing-gate gate"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Run(dir, func(string) (string, error) { return "/usr/bin/x", nil })
	if r.HookProbed {
		t.Fatal("nothing was probed")
	}
	if strings.Contains(r.String(), "answers a payload") {
		t.Fatalf("unverified wiring must not read as verified:\n%s", r.String())
	}
}

func containsProblem(problems []string, sub string) bool {
	for _, p := range problems {
		if strings.Contains(p, sub) {
			return true
		}
	}
	return false
}
